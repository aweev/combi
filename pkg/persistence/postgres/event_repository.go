// internal/persistence/postgres/repository.go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/combi/pkg/domain/event"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/log"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Ensure implementation satisfies interface
var _ event.Repository = (*Repository)(nil)

type Repository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewRepository(db *sqlx.DB, logger log.Logger) *Repository {
	return &Repository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "event_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// WithTx executes the given function within a database transaction.
func (r *Repository) WithTx(ctx context.Context, fn func(txRepo event.Repository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	// Create a new repository using the transaction as the runner
	txRepo := &Repository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     r.sq,
	}

	if err := fn(txRepo); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// ---------------------------------------------------------
// Event Repository
// ---------------------------------------------------------

func (r *Repository) CreateEvent(ctx context.Context, e *event.Event) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	now := time.Now().UTC()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	e.UpdatedAt = now

	dbE, err := toDBEvent(e)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("events").
		Columns("id", "organization_id", "venue_id", "created_by", "name", "slug", "description",
			"status", "format", "start_time", "end_time", "timezone", "stream_url", "properties", "created_at", "updated_at").
		Values(dbE.ID, dbE.OrganizationID, dbE.VenueID, dbE.CreatedBy, dbE.Name, dbE.Slug, dbE.Description,
			dbE.Status, dbE.Format, dbE.StartTime, dbE.EndTime, dbE.Timezone, dbE.StreamURL, dbE.Properties, dbE.CreatedAt, dbE.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isEventUniqueViolation(err) {
			return event.ErrEventAlreadyExists
		}
		return err
	}
	return nil
}

func (r *Repository) GetEvent(ctx context.Context, id uuid.UUID) (*event.Event, error) {
	query, args, _ := r.sq.Select("*").From("events").Where(squirrel.Eq{"id": id}).ToSql()
	return r.fetchEvent(ctx, query, args...)
}

func (r *Repository) GetEventBySlug(ctx context.Context, slug string) (*event.Event, error) {
	query, args, _ := r.sq.Select("*").From("events").Where(squirrel.Eq{"slug": slug}).ToSql()
	return r.fetchEvent(ctx, query, args...)
}

func (r *Repository) ListEvents(ctx context.Context, filter event.EventFilter) ([]*event.Event, int, error) {
	builder := r.sq.Select("*").From("events")
	countBuilder := r.sq.Select("COUNT(*)").From("events")

	if filter.Status != nil {
		builder = builder.Where(squirrel.Eq{"status": *filter.Status})
		countBuilder = countBuilder.Where(squirrel.Eq{"status": *filter.Status})
	}
	if filter.Format != nil {
		builder = builder.Where(squirrel.Eq{"format": *filter.Format})
		countBuilder = countBuilder.Where(squirrel.Eq{"format": *filter.Format})
	}
	if filter.IsUpcoming {
		now := time.Now().UTC()
		builder = builder.Where(squirrel.Gt{"start_time": now})
		countBuilder = countBuilder.Where(squirrel.Gt{"start_time": now})
	}
	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		builder = builder.Where(squirrel.Or{
			squirrel.Like{"name": like},
			squirrel.Like{"description": like},
		})
		countBuilder = countBuilder.Where(squirrel.Or{
			squirrel.Like{"name": like},
			squirrel.Like{"description": like},
		})
	}

	var total int
	cQuery, cArgs, _ := countBuilder.ToSql()
	if err := r.runner.QueryRowxContext(ctx, cQuery, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query, args, err := builder.OrderBy("start_time ASC").
		Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}

	var dbRows []dbEvent
	if err := sqlx.SelectContext(ctx, r.runner, &dbRows, query, args...); err != nil {
		return nil, 0, err
	}

	events := make([]*event.Event, len(dbRows))
	for i, row := range dbRows {
		e, err := row.toDomain()
		if err != nil {
			return nil, 0, err
		}
		events[i] = e
	}

	return events, total, nil
}

func (r *Repository) UpdateEvent(ctx context.Context, e *event.Event) error {
	e.UpdatedAt = time.Now().UTC()
	dbE, err := toDBEvent(e)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Update("events").
		Set("name", dbE.Name).
		Set("description", dbE.Description).
		Set("start_time", dbE.StartTime).
		Set("end_time", dbE.EndTime).
		Set("status", dbE.Status).
		Set("venue_id", dbE.VenueID).
		Set("stream_url", dbE.StreamURL).
		Set("properties", dbE.Properties).
		Set("updated_at", dbE.UpdatedAt).
		Where(squirrel.Eq{"id": dbE.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return event.ErrEventNotFound
	}
	return nil
}

func (r *Repository) DeleteEvent(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("events").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return event.ErrEventNotFound
	}
	return nil
}

// ---------------------------------------------------------
// Ticket Repository
// ---------------------------------------------------------

func (r *Repository) CreateTicket(ctx context.Context, t *event.Ticket) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}

	dbT := toDBTicket(t)

	query, args, err := r.sq.Insert("tickets").
		Columns("id", "event_id", "name", "price_amount", "price_currency", "capacity", "registrations_count",
			"sale_starts_at", "sale_ends_at", "is_active", "created_at").
		Values(dbT.ID, dbT.EventID, dbT.Name, dbT.PriceAmount, dbT.PriceCurrency, dbT.Capacity, dbT.RegistrationsCount,
			dbT.SaleStartsAt, dbT.SaleEndsAt, dbT.IsActive, dbT.CreatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *Repository) GetTicket(ctx context.Context, id uuid.UUID) (*event.Ticket, error) {
	query, args, _ := r.sq.Select("*").From("tickets").Where(squirrel.Eq{"id": id}).ToSql()
	var dbT dbTicket
	if err := sqlx.GetContext(ctx, r.runner, &dbT, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, event.ErrTicketNotFound
		}
		return nil, err
	}
	return dbT.toDomain(), nil
}

func (r *Repository) ListTicketsForEvent(ctx context.Context, eventID uuid.UUID) ([]*event.Ticket, error) {
	query, args, _ := r.sq.Select("*").From("tickets").
		Where(squirrel.Eq{"event_id": eventID}).
		OrderBy("price_amount ASC").ToSql()

	var dbTickets []dbTicket
	if err := sqlx.SelectContext(ctx, r.runner, &dbTickets, query, args...); err != nil {
		return nil, err
	}

	tickets := make([]*event.Ticket, len(dbTickets))
	for i, t := range dbTickets {
		tickets[i] = t.toDomain()
	}
	return tickets, nil
}

func (r *Repository) UpdateTicket(ctx context.Context, t *event.Ticket) error {
	dbT := toDBTicket(t)
	query, args, err := r.sq.Update("tickets").
		Set("name", dbT.Name).
		Set("price_amount", dbT.PriceAmount).
		Set("capacity", dbT.Capacity).
		Set("sale_starts_at", dbT.SaleStartsAt).
		Set("sale_ends_at", dbT.SaleEndsAt).
		Set("is_active", dbT.IsActive).
		Where(squirrel.Eq{"id": dbT.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return event.ErrTicketNotFound
	}
	return nil
}

func (r *Repository) DeleteTicket(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("tickets").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return event.ErrTicketNotFound
	}
	return nil
}

func (r *Repository) IncrementRegistrationsCount(ctx context.Context, ticketID uuid.UUID, amount int) error {
	query, args, _ := r.sq.Update("tickets").
		Set("registrations_count", squirrel.Expr("registrations_count + ?", amount)).
		Where(squirrel.Eq{"id": ticketID}).
		ToSql()

	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

// ---------------------------------------------------------
// Registration Repository
// ---------------------------------------------------------

func (r *Repository) CreateRegistration(ctx context.Context, reg *event.Registration) error {
	if reg.ID == uuid.Nil {
		reg.ID = uuid.New()
	}
	if reg.CreatedAt.IsZero() {
		reg.CreatedAt = time.Now().UTC()
	}
	reg.UpdatedAt = reg.CreatedAt

	dbR := toDBRegistration(reg)

	query, args, err := r.sq.Insert("registrations").
		Columns("id", "user_id", "event_id", "ticket_id", "transaction_id", "status", "checked_in_at", "created_at", "updated_at").
		Values(dbR.ID, dbR.UserID, dbR.EventID, dbR.TicketID, dbR.TransactionID, dbR.Status, dbR.CheckedInAt, dbR.CreatedAt, dbR.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isEventUniqueViolation(err) {
			return event.ErrAlreadyRegistered
		}
		return err
	}
	return nil
}

func (r *Repository) GetRegistration(ctx context.Context, id uuid.UUID) (*event.Registration, error) {
	query, args, _ := r.sq.Select("*").From("registrations").Where(squirrel.Eq{"id": id}).ToSql()
	return r.fetchRegistration(ctx, query, args...)
}

func (r *Repository) GetUserRegistrationForEvent(ctx context.Context, userID, eventID uuid.UUID) (*event.Registration, error) {
	query, args, _ := r.sq.Select("*").From("registrations").
		Where(squirrel.Eq{"user_id": userID, "event_id": eventID}).
		ToSql()
	return r.fetchRegistration(ctx, query, args...)
}

func (r *Repository) ListRegistrationsForEvent(ctx context.Context, eventID uuid.UUID, filter event.RegistrationFilter) ([]*event.Registration, int, error) {
	builder := r.sq.Select("*").From("registrations").Where(squirrel.Eq{"event_id": eventID})
	countBuilder := r.sq.Select("COUNT(*)").From("registrations").Where(squirrel.Eq{"event_id": eventID})

	if filter.Status != nil {
		builder = builder.Where(squirrel.Eq{"status": *filter.Status})
		countBuilder = countBuilder.Where(squirrel.Eq{"status": *filter.Status})
	}

	var total int
	cQuery, cArgs, _ := countBuilder.ToSql()
	if err := r.runner.QueryRowxContext(ctx, cQuery, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query, args, err := builder.OrderBy("created_at DESC").
		Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}

	var dbRows []dbRegistration
	if err := sqlx.SelectContext(ctx, r.runner, &dbRows, query, args...); err != nil {
		return nil, 0, err
	}

	regs := make([]*event.Registration, len(dbRows))
	for i, row := range dbRows {
		regs[i] = row.toDomain()
	}
	return regs, total, nil
}

func (r *Repository) ListRegistrationsForUser(ctx context.Context, actorID uuid.UUID, filter event.RegistrationFilter) ([]*event.Registration, int, error) {
	builder := r.sq.Select("*").From("registrations").Where(squirrel.Eq{"actor_id": actorID})
	countBuilder := r.sq.Select("COUNT(*)").From("registrations").Where(squirrel.Eq{"actor_id": actorID})

	if filter.Status != nil {
		builder = builder.Where(squirrel.Eq{"status": *filter.Status})
		countBuilder = countBuilder.Where(squirrel.Eq{"status": *filter.Status})
	}

	var total int
	cQuery, cArgs, _ := countBuilder.ToSql()
	if err := r.runner.QueryRowxContext(ctx, cQuery, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query, args, err := builder.OrderBy("created_at DESC").
		Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}

	var dbRows []dbRegistration
	if err := sqlx.SelectContext(ctx, r.runner, &dbRows, query, args...); err != nil {
		return nil, 0, err
	}

	regs := make([]*event.Registration, len(dbRows))
	for i, row := range dbRows {
		regs[i] = row.toDomain()
	}
	return regs, total, nil
}

func (r *Repository) UpdateRegistration(ctx context.Context, reg *event.Registration) error {
	reg.UpdatedAt = time.Now().UTC()
	dbR := toDBRegistration(reg)

	query, args, err := r.sq.Update("registrations").
		Set("status", dbR.Status).
		Set("checked_in_at", dbR.CheckedInAt).
		Set("updated_at", dbR.UpdatedAt).
		Where(squirrel.Eq{"id": dbR.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return event.ErrRegistrationNotFound
	}
	return nil
}

// ---------------------------------------------------------
// Venue Repository
// ---------------------------------------------------------

func (r *Repository) CreateVenue(ctx context.Context, v *event.Venue) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}
	v.UpdatedAt = v.CreatedAt

	query, args, err := r.sq.Insert("venues").
		Columns("id", "name", "address", "city", "country", "created_at", "updated_at").
		Values(v.ID, v.Name, v.Address, v.City, v.Country, v.CreatedAt, v.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *Repository) GetVenue(ctx context.Context, id uuid.UUID) (*event.Venue, error) {
	var dbV dbVenue
	query, args, _ := r.sq.Select("*").From("venues").Where(squirrel.Eq{"id": id}).ToSql()
	err := sqlx.GetContext(ctx, r.runner, &dbV, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, event.ErrVenueNotFound
	}
	return dbV.toDomain(), err
}

func (r *Repository) ListVenues(ctx context.Context) ([]*event.Venue, error) {
	var dbRows []dbVenue
	query, args, _ := r.sq.Select("*").From("venues").OrderBy("name ASC").ToSql()
	err := sqlx.SelectContext(ctx, r.runner, &dbRows, query, args...)
	if err != nil {
		return nil, err
	}

	venues := make([]*event.Venue, len(dbRows))
	for i, v := range dbRows {
		venues[i] = v.toDomain()
	}
	return venues, nil
}

func (r *Repository) UpdateVenue(ctx context.Context, v *event.Venue) error {
	v.UpdatedAt = time.Now().UTC()
	query, args, err := r.sq.Update("venues").
		Set("name", v.Name).
		Set("address", v.Address).
		Set("city", v.City).
		Set("country", v.Country).
		Set("updated_at", v.UpdatedAt).
		Where(squirrel.Eq{"id": v.ID}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *Repository) DeleteVenue(ctx context.Context, id uuid.UUID) error {
	// Check usage first
	var count int
	checkQuery, checkArgs, _ := r.sq.Select("COUNT(*)").From("events").Where(squirrel.Eq{"venue_id": id}).ToSql()
	if err := r.runner.QueryRowxContext(ctx, checkQuery, checkArgs...).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return event.ErrVenueInUse
	}

	query, args, _ := r.sq.Delete("venues").Where(squirrel.Eq{"id": id}).ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

// ---------------------------------------------------------
// Speaker Repository
// ---------------------------------------------------------

func (r *Repository) CreateSpeaker(ctx context.Context, s *event.Speaker) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	s.UpdatedAt = s.CreatedAt

	query, args, err := r.sq.Insert("speakers").
		Columns("id", "name", "title", "bio", "avatar_url", "created_at", "updated_at").
		Values(s.ID, s.Name, s.Title, s.Bio, s.AvatarURL, s.CreatedAt, s.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *Repository) GetSpeaker(ctx context.Context, id uuid.UUID) (*event.Speaker, error) {
	var dbS dbSpeaker
	query, args, _ := r.sq.Select("*").From("speakers").Where(squirrel.Eq{"id": id}).ToSql()
	err := sqlx.GetContext(ctx, r.runner, &dbS, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, event.ErrSpeakerNotFound
	}
	return dbS.toDomain(), err
}

func (r *Repository) ListSpeakers(ctx context.Context) ([]*event.Speaker, error) {
	var dbRows []dbSpeaker
	query, args, _ := r.sq.Select("*").From("speakers").OrderBy("name ASC").ToSql()
	err := sqlx.SelectContext(ctx, r.runner, &dbRows, query, args...)
	if err != nil {
		return nil, err
	}

	speakers := make([]*event.Speaker, len(dbRows))
	for i, s := range dbRows {
		speakers[i] = s.toDomain()
	}
	return speakers, err
}

func (r *Repository) UpdateSpeaker(ctx context.Context, s *event.Speaker) error {
	s.UpdatedAt = time.Now().UTC()
	query, args, err := r.sq.Update("speakers").
		Set("name", s.Name).
		Set("title", s.Title).
		Set("bio", s.Bio).
		Set("avatar_url", s.AvatarURL).
		Set("updated_at", s.UpdatedAt).
		Where(squirrel.Eq{"id": s.ID}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *Repository) DeleteSpeaker(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("speakers").Where(squirrel.Eq{"id": id}).ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

// ---------------------------------------------------------
// Schedule Item Repository
// ---------------------------------------------------------

func (r *Repository) CreateScheduleItem(ctx context.Context, item *event.ScheduleItem) error {
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}

	dbItem, err := toDBScheduleItem(item)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("schedule_items").
		Columns("id", "event_id", "title", "description", "start_time", "end_time", "track", "speaker_ids", "created_at").
		Values(dbItem.ID, dbItem.EventID, dbItem.Title, dbItem.Description, dbItem.StartTime, dbItem.EndTime,
			dbItem.Track, dbItem.SpeakerIDsJSON, dbItem.CreatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *Repository) GetScheduleItem(ctx context.Context, id uuid.UUID) (*event.ScheduleItem, error) {
	query, args, _ := r.sq.Select("*").From("schedule_items").Where(squirrel.Eq{"id": id}).ToSql()
	return r.fetchScheduleItem(ctx, query, args...)
}

func (r *Repository) ListScheduleItemsForEvent(ctx context.Context, eventID uuid.UUID) ([]*event.ScheduleItem, error) {
	query, args, _ := r.sq.Select("*").From("schedule_items").
		Where(squirrel.Eq{"event_id": eventID}).
		OrderBy("start_time ASC").ToSql()

	var dbItems []dbScheduleItem
	if err := sqlx.SelectContext(ctx, r.runner, &dbItems, query, args...); err != nil {
		return nil, err
	}

	items := make([]*event.ScheduleItem, len(dbItems))
	for i, row := range dbItems {
		item, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		items[i] = item
	}
	return items, nil
}

func (r *Repository) UpdateScheduleItem(ctx context.Context, item *event.ScheduleItem) error {
	dbItem, err := toDBScheduleItem(item)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Update("schedule_items").
		Set("title", dbItem.Title).
		Set("description", dbItem.Description).
		Set("start_time", dbItem.StartTime).
		Set("end_time", dbItem.EndTime).
		Set("track", dbItem.Track).
		Set("speaker_ids", dbItem.SpeakerIDsJSON).
		Where(squirrel.Eq{"id": dbItem.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return event.ErrSessionNotFound
	}
	return nil
}

func (r *Repository) DeleteScheduleItem(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("schedule_items").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return event.ErrSessionNotFound
	}
	return nil
}

// ---------------------------------------------------------
// Helpers & Mapping
// ---------------------------------------------------------

func (r *Repository) fetchEvent(ctx context.Context, query string, args ...interface{}) (*event.Event, error) {
	var dbE dbEvent
	if err := sqlx.GetContext(ctx, r.runner, &dbE, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, event.ErrEventNotFound
		}
		return nil, err
	}
	return dbE.toDomain()
}

func (r *Repository) fetchRegistration(ctx context.Context, query string, args ...interface{}) (*event.Registration, error) {
	var dbR dbRegistration
	if err := sqlx.GetContext(ctx, r.runner, &dbR, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, event.ErrRegistrationNotFound
		}
		return nil, err
	}
	return dbR.toDomain(), nil
}

func (r *Repository) fetchScheduleItem(ctx context.Context, query string, args ...interface{}) (*event.ScheduleItem, error) {
	var dbI dbScheduleItem
	if err := sqlx.GetContext(ctx, r.runner, &dbI, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, event.ErrSessionNotFound
		}
		return nil, err
	}
	return dbI.toDomain()
}

func isEventUniqueViolation(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code == "23505"
	}
	return false
}

// -- Database Models --

type dbEvent struct {
	ID             uuid.UUID      `db:"id"`
	OrganizationID uuid.NullUUID  `db:"organization_id"`
	VenueID        uuid.NullUUID  `db:"venue_id"`
	CreatedBy      uuid.UUID      `db:"created_by"`
	Name           string         `db:"name"`
	Slug           string         `db:"slug"`
	Description    string         `db:"description"`
	Status         string         `db:"status"`
	Format         string         `db:"format"`
	StartTime      time.Time      `db:"start_time"`
	EndTime        time.Time      `db:"end_time"`
	Timezone       string         `db:"timezone"`
	StreamURL      sql.NullString `db:"stream_url"`
	Properties     []byte         `db:"properties"` // JSONB
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
}

func (d *dbEvent) toDomain() (*event.Event, error) {
	e := &event.Event{
		ID:          d.ID,
		CreatedBy:   d.CreatedBy,
		Name:        d.Name,
		Slug:        d.Slug,
		Description: d.Description,
		Status:      event.EventStatus(d.Status),
		Format:      event.EventFormat(d.Format),
		StartTime:   d.StartTime,
		EndTime:     d.EndTime,
		Timezone:    d.Timezone,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
	if d.OrganizationID.Valid {
		e.OrganizationID = &d.OrganizationID.UUID
	}
	if d.VenueID.Valid {
		e.VenueID = &d.VenueID.UUID
	}
	if d.StreamURL.Valid {
		e.StreamURL = &d.StreamURL.String
	}
	if len(d.Properties) > 0 {
		if err := json.Unmarshal(d.Properties, &e.Properties); err != nil {
			return nil, err
		}
	}
	return e, nil
}

func toDBEvent(e *event.Event) (*dbEvent, error) {
	props, err := json.Marshal(e.Properties)
	if err != nil {
		return nil, err
	}

	d := &dbEvent{
		ID:          e.ID,
		CreatedBy:   e.CreatedBy,
		Name:        e.Name,
		Slug:        e.Slug,
		Description: e.Description,
		Status:      string(e.Status),
		Format:      string(e.Format),
		StartTime:   e.StartTime,
		EndTime:     e.EndTime,
		Timezone:    e.Timezone,
		Properties:  props,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
	if e.OrganizationID != nil {
		d.OrganizationID = uuid.NullUUID{UUID: *e.OrganizationID, Valid: true}
	}
	if e.VenueID != nil {
		d.VenueID = uuid.NullUUID{UUID: *e.VenueID, Valid: true}
	}
	if e.StreamURL != nil {
		d.StreamURL = sql.NullString{String: *e.StreamURL, Valid: true}
	}
	return d, nil
}

type dbTicket struct {
	ID                 uuid.UUID     `db:"id"`
	EventID            uuid.UUID     `db:"event_id"`
	Name               string        `db:"name"`
	PriceAmount        int64         `db:"price_amount"`
	PriceCurrency      string        `db:"price_currency"`
	Capacity           sql.NullInt32 `db:"capacity"`
	RegistrationsCount int           `db:"registrations_count"`
	SaleStartsAt       sql.NullTime  `db:"sale_starts_at"`
	SaleEndsAt         sql.NullTime  `db:"sale_ends_at"`
	IsActive           bool          `db:"is_active"`
	CreatedAt          time.Time     `db:"created_at"`
}

func (d *dbTicket) toDomain() *event.Ticket {
	t := &event.Ticket{
		ID:                 d.ID,
		EventID:            d.EventID,
		Name:               d.Name,
		Price:              financial.Price{Amount: d.PriceAmount, Currency: d.PriceCurrency},
		RegistrationsCount: d.RegistrationsCount,
		IsActive:           d.IsActive,
		CreatedAt:          d.CreatedAt,
	}
	if d.Capacity.Valid {
		val := int(d.Capacity.Int32)
		t.Capacity = &val
	}
	if d.SaleStartsAt.Valid {
		t.SaleStartsAt = &d.SaleStartsAt.Time
	}
	if d.SaleEndsAt.Valid {
		t.SaleEndsAt = &d.SaleEndsAt.Time
	}
	return t
}

func toDBTicket(t *event.Ticket) *dbTicket {
	d := &dbTicket{
		ID:                 t.ID,
		EventID:            t.EventID,
		Name:               t.Name,
		PriceAmount:        t.Price.Amount,
		PriceCurrency:      t.Price.Currency,
		RegistrationsCount: t.RegistrationsCount,
		IsActive:           t.IsActive,
		CreatedAt:          t.CreatedAt,
	}
	if t.Capacity != nil {
		d.Capacity = sql.NullInt32{Int32: int32(*t.Capacity), Valid: true}
	}
	if t.SaleStartsAt != nil {
		d.SaleStartsAt = sql.NullTime{Time: *t.SaleStartsAt, Valid: true}
	}
	if t.SaleEndsAt != nil {
		d.SaleEndsAt = sql.NullTime{Time: *t.SaleEndsAt, Valid: true}
	}
	return d
}

type dbRegistration struct {
	ID            uuid.UUID     `db:"id"`
	UserID        uuid.UUID     `db:"user_id"`
	EventID       uuid.UUID     `db:"event_id"`
	TicketID      uuid.UUID     `db:"ticket_id"`
	TransactionID uuid.NullUUID `db:"transaction_id"`
	Status        string        `db:"status"`
	CheckedInAt   sql.NullTime  `db:"checked_in_at"`
	CreatedAt     time.Time     `db:"created_at"`
	UpdatedAt     time.Time     `db:"updated_at"`
}

func (d *dbRegistration) toDomain() *event.Registration {
	r := &event.Registration{
		ID:        d.ID,
		UserID:    d.UserID,
		EventID:   d.EventID,
		TicketID:  d.TicketID,
		Status:    event.RegistrationStatus(d.Status),
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
	if d.TransactionID.Valid {
		r.TransactionID = &d.TransactionID.UUID
	}
	if d.CheckedInAt.Valid {
		r.CheckedInAt = &d.CheckedInAt.Time
	}
	return r
}

func toDBRegistration(r *event.Registration) *dbRegistration {
	d := &dbRegistration{
		ID:        r.ID,
		UserID:    r.UserID,
		EventID:   r.EventID,
		TicketID:  r.TicketID,
		Status:    string(r.Status),
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
	if r.TransactionID != nil {
		d.TransactionID = uuid.NullUUID{UUID: *r.TransactionID, Valid: true}
	}
	if r.CheckedInAt != nil {
		d.CheckedInAt = sql.NullTime{Time: *r.CheckedInAt, Valid: true}
	}
	return d
}

type dbScheduleItem struct {
	ID             uuid.UUID      `db:"id"`
	EventID        uuid.UUID      `db:"event_id"`
	Title          string         `db:"title"`
	Description    sql.NullString `db:"description"`
	StartTime      time.Time      `db:"start_time"`
	EndTime        time.Time      `db:"end_time"`
	Track          sql.NullString `db:"track"`
	SpeakerIDsJSON []byte         `db:"speaker_ids"`
	CreatedAt      time.Time      `db:"created_at"`
}

func (d *dbScheduleItem) toDomain() (*event.ScheduleItem, error) {
	s := &event.ScheduleItem{
		ID:        d.ID,
		EventID:   d.EventID,
		Title:     d.Title,
		StartTime: d.StartTime,
		EndTime:   d.EndTime,
		CreatedAt: d.CreatedAt,
	}
	if d.Description.Valid {
		s.Description = &d.Description.String
	}
	if d.Track.Valid {
		s.Track = &d.Track.String
	}

	if len(d.SpeakerIDsJSON) > 0 {
		if err := json.Unmarshal(d.SpeakerIDsJSON, &s.SpeakerIDs); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func toDBScheduleItem(s *event.ScheduleItem) (*dbScheduleItem, error) {
	speakerJSON, err := json.Marshal(s.SpeakerIDs)
	if err != nil {
		return nil, err
	}

	d := &dbScheduleItem{
		ID:             s.ID,
		EventID:        s.EventID,
		Title:          s.Title,
		StartTime:      s.StartTime,
		EndTime:        s.EndTime,
		SpeakerIDsJSON: speakerJSON,
		CreatedAt:      s.CreatedAt,
	}
	if s.Description != nil {
		d.Description = sql.NullString{String: *s.Description, Valid: true}
	}
	if s.Track != nil {
		d.Track = sql.NullString{String: *s.Track, Valid: true}
	}
	return d, nil
}

// -- New DTOs for Venue and Speaker to fix scanning errors --

type dbVenue struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	Address   string    `db:"address"`
	City      string    `db:"city"`
	Country   string    `db:"country"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (d *dbVenue) toDomain() *event.Venue {
	return &event.Venue{
		ID:        d.ID,
		Name:      d.Name,
		Address:   d.Address,
		City:      d.City,
		Country:   d.Country,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

type dbSpeaker struct {
	ID        uuid.UUID      `db:"id"`
	Name      string         `db:"name"`
	Title     string         `db:"title"`
	Bio       sql.NullString `db:"bio"`
	AvatarURL sql.NullString `db:"avatar_url"`
	CreatedAt time.Time      `db:"created_at"`
	UpdatedAt time.Time      `db:"updated_at"`
}

func (d *dbSpeaker) toDomain() *event.Speaker {
	s := &event.Speaker{
		ID:        d.ID,
		Name:      d.Name,
		Title:     d.Title,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
	if d.Bio.Valid {
		s.Bio = &d.Bio.String
	}
	if d.AvatarURL.Valid {
		s.AvatarURL = &d.AvatarURL.String
	}
	return s
}
