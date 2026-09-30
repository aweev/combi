// internal/persistence/postgres/content_repository.go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/content"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ content.Repository = (*ContentRepository)(nil)

type ContentRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewContentRepository(db *sqlx.DB, logger log.Logger) *ContentRepository {
	return &ContentRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "content_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *ContentRepository) WithTx(ctx context.Context, fn func(txRepo content.Repository) error) error {
	if _, ok := r.runner.(*sqlx.Tx); ok {
		return errors.New("cannot begin transaction from within another transaction")
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()
	txRepo := &ContentRepository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     r.sq,
	}
	return fn(txRepo)
}

// --- SpaceRepository ---

func (r *ContentRepository) CreateSpace(ctx context.Context, space *content.Space) error {
	if space.ID == uuid.Nil {
		space.ID = uuid.New()
	}
	now := time.Now().UTC()
	space.CreatedAt = now
	space.UpdatedAt = now

	dbModel := dbSpaceFromDomain(space)
	if org := organization.OrgFromContext(ctx); org != nil {
		dbModel.OrganizationID = &org.OrgID
	}

	query, args, err := r.sq.Insert("spaces").
		Columns("id", "organization_id", "key", "name", "is_default", "locales", "default_locale", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.OrganizationID, dbModel.Key, dbModel.Name, dbModel.IsDefault, dbModel.Locales, dbModel.DefaultLocale, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return content.ErrSpaceAlreadyExists
		}
		return fmt.Errorf("failed to create space: %w", err)
	}
	return nil
}

func (r *ContentRepository) GetSpace(ctx context.Context, id uuid.UUID) (*content.Space, error) {
	builder := r.sq.Select("*").From("spaces").Where(squirrel.Eq{"id": id})
	builder = WithTenant(ctx, builder)
	query, args, _ := builder.ToSql()
	var dbs dbSpace
	if err := sqlx.GetContext(ctx, r.runner, &dbs, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, content.ErrSpaceNotFound
		}
		return nil, err
	}
	return dbs.toDomain(), nil
}

func (r *ContentRepository) ListSpaces(ctx context.Context) ([]*content.Space, error) {
	builder := r.sq.Select("*").From("spaces").OrderBy("name ASC")
	builder = WithTenant(ctx, builder)
	query, args, _ := builder.ToSql()
	var dbs []dbSpace
	if err := sqlx.SelectContext(ctx, r.runner, &dbs, query, args...); err != nil {
		return nil, err
	}
	spaces := make([]*content.Space, len(dbs))
	for i, s := range dbs {
		spaces[i] = s.toDomain()
	}
	return spaces, nil
}

func (r *ContentRepository) UpdateSpace(ctx context.Context, space *content.Space) error {
	space.UpdatedAt = time.Now().UTC()
	dbModel := dbSpaceFromDomain(space)
	builder := r.sq.Update("spaces").
		Set("name", dbModel.Name).
		Set("locales", dbModel.Locales).
		Set("default_locale", dbModel.DefaultLocale).
		Set("updated_at", dbModel.UpdatedAt).
		Where(squirrel.Eq{"id": dbModel.ID})
	
	builder = WithTenantUpdate(ctx, builder)
	query, args, err := builder.ToSql()
	if err != nil {
		return err
	}
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return content.ErrSpaceNotFound
	}
	return nil
}

func (r *ContentRepository) DeleteSpace(ctx context.Context, id uuid.UUID) error {
	builder := r.sq.Delete("spaces").Where(squirrel.Eq{"id": id})
	builder = WithTenantDelete(ctx, builder)
	query, args, _ := builder.ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return content.ErrDefaultSpaceCannotBeDeleted // Or generic constraint error
		}
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return content.ErrSpaceNotFound
	}
	return nil
}

// --- ModelRepository ---

func (r *ContentRepository) CreateModel(ctx context.Context, model *content.Model) error {
	if model.ID == uuid.Nil {
		model.ID = uuid.New()
	}
	now := time.Now().UTC()
	model.CreatedAt = now
	model.UpdatedAt = now

	dbModel, err := dbModelFromDomain(model)
	if err != nil {
		return err
	}
	if org := organization.OrgFromContext(ctx); org != nil {
		dbModel.OrganizationID = &org.OrgID
	}

	query, args, err := r.sq.Insert("models").
		Columns("id", "organization_id", "space_id", "api_key", "name", "fields", "is_system", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.OrganizationID, dbModel.SpaceID, dbModel.APIKey, dbModel.Name, dbModel.Fields, dbModel.IsSystem, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return content.ErrModelAlreadyExists
		}
		return err
	}
	return nil
}

func (r *ContentRepository) GetModel(ctx context.Context, id uuid.UUID) (*content.Model, error) {
	builder := r.sq.Select("*").From("models").Where(squirrel.Eq{"id": id})
	builder = WithTenant(ctx, builder)
	query, args, _ := builder.ToSql()
	var dbm dbModel
	if err := sqlx.GetContext(ctx, r.runner, &dbm, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, content.ErrModelNotFound
		}
		return nil, err
	}
	return dbm.toDomain()
}

func (r *ContentRepository) ListModels(ctx context.Context, spaceID uuid.UUID) ([]*content.Model, error) {
	builder := r.sq.Select("*").From("models").Where(squirrel.Eq{"space_id": spaceID}).OrderBy("name ASC")
	builder = WithTenant(ctx, builder)
	query, args, _ := builder.ToSql()
	var dbms []dbModel
	if err := sqlx.SelectContext(ctx, r.runner, &dbms, query, args...); err != nil {
		return nil, err
	}
	models := make([]*content.Model, len(dbms))
	for i, m := range dbms {
		dom, err := m.toDomain()
		if err != nil {
			r.logger.Error("failed to unmarshal model", err)
			continue
		}
		models[i] = dom
	}
	return models, nil
}

func (r *ContentRepository) UpdateModel(ctx context.Context, model *content.Model) error {
	model.UpdatedAt = time.Now().UTC()
	dbModel, err := dbModelFromDomain(model)
	if err != nil {
		return err
	}
	builder := r.sq.Update("models").
		Set("name", dbModel.Name).
		Set("api_key", dbModel.APIKey).
		Set("fields", dbModel.Fields).
		Set("updated_at", dbModel.UpdatedAt).
		Where(squirrel.Eq{"id": dbModel.ID})

	builder = WithTenantUpdate(ctx, builder)
	query, args, err := builder.ToSql()
	if err != nil {
		return err
	}
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return content.ErrModelNotFound
	}
	return nil
}

func (r *ContentRepository) DeleteModel(ctx context.Context, id uuid.UUID) error {
	builder := r.sq.Delete("models").Where(squirrel.Eq{"id": id})
	builder = WithTenantDelete(ctx, builder)
	query, args, _ := builder.ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return content.ErrModelInUse
		}
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return content.ErrModelNotFound
	}
	return nil
}

func (r *ContentRepository) IsModelInUse(ctx context.Context, id uuid.UUID) (bool, error) {
	query, args, _ := r.sq.Select("COUNT(*)").From("entries").Where(squirrel.Eq{"model_id": id}).ToSql()
	var count int
	if err := r.runner.QueryRowxContext(ctx, query, args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// --- EntryRepository ---

func (r *ContentRepository) CreateEntry(ctx context.Context, entry *content.Entry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	now := time.Now().UTC()
	entry.CreatedAt = now
	entry.UpdatedAt = now
	entry.Version = 1

	dbModel, err := dbEntryFromDomain(entry)
	if err != nil {
		return err
	}
	if org := organization.OrgFromContext(ctx); org != nil {
		dbModel.OrganizationID = &org.OrgID
	}

	query, args, err := r.sq.Insert("entries").
		Columns("id", "organization_id", "model_id", "space_id", "slug", "status", "fields", "version", "published_version", "created_by", "updated_by", "published_at", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.OrganizationID, dbModel.ModelID, dbModel.SpaceID, dbModel.Slug, dbModel.Status, dbModel.Fields, dbModel.Version, dbModel.PublishedVersion, dbModel.CreatedBy, dbModel.UpdatedBy, dbModel.PublishedAt, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to create entry: %w", err)
	}
	return nil
}

func (r *ContentRepository) GetEntry(ctx context.Context, id uuid.UUID) (*content.Entry, error) {
	builder := r.sq.Select("*").From("entries").Where(squirrel.Eq{"id": id})
	builder = WithTenant(ctx, builder)
	query, args, _ := builder.ToSql()
	var dbe dbEntry
	if err := sqlx.GetContext(ctx, r.runner, &dbe, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, content.ErrEntryNotFound
		}
		return nil, err
	}
	return dbe.toDomain()
}

func (r *ContentRepository) ListEntries(ctx context.Context, modelID uuid.UUID, filter content.EntryFilter) ([]*content.Entry, int, error) {
	builder := r.sq.Select("*").From("entries").Where(squirrel.Eq{"model_id": modelID})
	countBuilder := r.sq.Select("COUNT(*)").From("entries").Where(squirrel.Eq{"model_id": modelID})

	builder = WithTenant(ctx, builder)
	countBuilder = WithTenant(ctx, countBuilder)

	if filter.Status != nil {
		builder = builder.Where(squirrel.Eq{"status": *filter.Status})
		countBuilder = countBuilder.Where(squirrel.Eq{"status": *filter.Status})
	}

	var total int
	cQuery, cArgs, _ := countBuilder.ToSql()
	if err := r.runner.QueryRowxContext(ctx, cQuery, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query, args, err := builder.OrderBy("updated_at DESC").Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}

	var dbes []dbEntry
	if err := sqlx.SelectContext(ctx, r.runner, &dbes, query, args...); err != nil {
		return nil, 0, err
	}

	entries := make([]*content.Entry, len(dbes))
	for i, e := range dbes {
		dom, err := e.toDomain()
		if err != nil {
			r.logger.Error("failed to unmarshal entry", err)
			continue
		}
		entries[i] = dom
	}
	return entries, total, nil
}

func (r *ContentRepository) UpdateEntry(ctx context.Context, entry *content.Entry) error {
	entry.UpdatedAt = time.Now().UTC()
	// Increment version for optimistic locking check in Service layer, or handle here.
	// Here we assume service handles incrementing logic, repo just saves state.

	dbModel, err := dbEntryFromDomain(entry)
	if err != nil {
		return err
	}

	builder := r.sq.Update("entries").
		Set("slug", dbModel.Slug).
		Set("status", dbModel.Status).
		Set("fields", dbModel.Fields).
		Set("published_version", dbModel.PublishedVersion).
		Set("updated_by", dbModel.UpdatedBy).
		Set("published_at", dbModel.PublishedAt).
		Set("updated_at", dbModel.UpdatedAt).
		Set("version", dbModel.Version).
		Where(squirrel.Eq{
			"id":      dbModel.ID,
			"version": entry.Version - 1,
		})

	builder = WithTenantUpdate(ctx, builder)
	query, args, err := builder.ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return content.ErrConflict
	}
	return nil
}

func (r *ContentRepository) DeleteEntry(ctx context.Context, id uuid.UUID) error {
	builder := r.sq.Delete("entries").Where(squirrel.Eq{"id": id})
	builder = WithTenantDelete(ctx, builder)
	query, args, _ := builder.ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return content.ErrEntryNotFound
	}
	return nil
}

// --- VersionRepository ---

func (r *ContentRepository) CreateVersion(ctx context.Context, version *content.Version) error {
	if version.ID == uuid.Nil {
		version.ID = uuid.New()
	}
	version.CreatedAt = time.Now().UTC()

	dbModel, err := dbVersionFromDomain(version)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("versions").
		Columns("id", "entry_id", "version", "fields", "created_by", "created_at").
		Values(dbModel.ID, dbModel.EntryID, dbModel.Version, dbModel.Fields, dbModel.CreatedBy, dbModel.CreatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *ContentRepository) GetVersion(ctx context.Context, id uuid.UUID) (*content.Version, error) {
	query, args, _ := r.sq.Select("*").From("versions").Where(squirrel.Eq{"id": id}).ToSql()
	var dbv dbVersion
	if err := sqlx.GetContext(ctx, r.runner, &dbv, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, content.ErrVersionNotFound
		}
		return nil, err
	}
	return dbv.toDomain()
}

func (r *ContentRepository) GetLatestVersionForEntry(ctx context.Context, entryID uuid.UUID) (*content.Version, error) {
	query, args, _ := r.sq.Select("*").From("versions").
		Where(squirrel.Eq{"entry_id": entryID}).
		OrderBy("version DESC").
		Limit(1).
		ToSql()
	var dbv dbVersion
	if err := sqlx.GetContext(ctx, r.runner, &dbv, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, content.ErrVersionNotFound
		}
		return nil, err
	}
	return dbv.toDomain()
}

func (r *ContentRepository) ListVersionsForEntry(ctx context.Context, entryID uuid.UUID) ([]*content.Version, error) {
	query, args, _ := r.sq.Select("*").From("versions").
		Where(squirrel.Eq{"entry_id": entryID}).
		OrderBy("version DESC").
		ToSql()
	var dbvs []dbVersion
	if err := sqlx.SelectContext(ctx, r.runner, &dbvs, query, args...); err != nil {
		return nil, err
	}
	versions := make([]*content.Version, len(dbvs))
	for i, v := range dbvs {
		dom, err := v.toDomain()
		if err != nil {
			r.logger.Error("failed to unmarshal version", err)
			continue
		}
		versions[i] = dom
	}
	return versions, nil
}

func (r *ContentRepository) GetModelByAPIKey(ctx context.Context, apiKey string) (*content.Model, error) {
	builder := r.sq.Select("*").From("models").Where(squirrel.Eq{"api_key": apiKey})
	builder = WithTenant(ctx, builder)
	query, args, _ := builder.ToSql()
	var dbm dbModel
	if err := sqlx.GetContext(ctx, r.runner, &dbm, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, content.ErrModelNotFound
		}
		return nil, err
	}
	return dbm.toDomain()
}

// --- DB Models ---

type dbSpace struct {
	ID             uuid.UUID      `db:"id"`
	OrganizationID *uuid.UUID     `db:"organization_id"`
	Key            string         `db:"key"`
	Name           string         `db:"name"`
	IsDefault      bool           `db:"is_default"`
	Locales        pq.StringArray `db:"locales"`
	DefaultLocale  string         `db:"default_locale"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
}

func (d *dbSpace) toDomain() *content.Space {
	return &content.Space{
		ID:            d.ID,
		Key:           d.Key,
		Name:          d.Name,
		IsDefault:     d.IsDefault,
		Locales:       d.Locales,
		DefaultLocale: d.DefaultLocale,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

func dbSpaceFromDomain(s *content.Space) *dbSpace {
	return &dbSpace{
		ID:            s.ID,
		Key:           s.Key,
		Name:          s.Name,
		IsDefault:     s.IsDefault,
		Locales:       pq.StringArray(s.Locales),
		DefaultLocale: s.DefaultLocale,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}
}

type dbModel struct {
	ID             uuid.UUID      `db:"id"`
	SpaceID        uuid.UUID      `db:"space_id"`
	OrganizationID *uuid.UUID     `db:"organization_id"`
	APIKey         string         `db:"api_key"`
	Name           string         `db:"name"`
	Fields         string         `db:"fields"` // JSONB
	IsSystem       bool           `db:"is_system"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
}

func (d *dbModel) toDomain() (*content.Model, error) {
	var fields []content.Field
	if err := json.Unmarshal([]byte(d.Fields), &fields); err != nil {
		return nil, err
	}
	return &content.Model{
		ID:        d.ID,
		SpaceID:   d.SpaceID,
		APIKey:    d.APIKey,
		Name:      d.Name,
		Fields:    fields,
		IsSystem:  d.IsSystem,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}, nil
}

func dbModelFromDomain(m *content.Model) (*dbModel, error) {
	bytes, err := json.Marshal(m.Fields)
	if err != nil {
		return nil, err
	}
	if m.Fields == nil {
		bytes = []byte("[]")
	}
	return &dbModel{
		ID:        m.ID,
		SpaceID:   m.SpaceID,
		APIKey:    m.APIKey,
		Name:      m.Name,
		Fields:    string(bytes),
		IsSystem:  m.IsSystem,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

type dbEntry struct {
	ID               uuid.UUID      `db:"id"`
	ModelID          uuid.UUID      `db:"model_id"`
	SpaceID          uuid.UUID      `db:"space_id"`
	OrganizationID   *uuid.UUID     `db:"organization_id"`
	Slug             sql.NullString `db:"slug"`
	Status           string         `db:"status"`
	Fields           string         `db:"fields"` // JSONB
	Version          int            `db:"version"`
	PublishedVersion sql.NullInt32  `db:"published_version"`
	CreatedBy        uuid.UUID      `db:"created_by"`
	UpdatedBy        uuid.UUID      `db:"updated_by"`
	PublishedAt      sql.NullTime   `db:"published_at"`
	CreatedAt        time.Time      `db:"created_at"`
	UpdatedAt        time.Time      `db:"updated_at"`
}

func (d *dbEntry) toDomain() (*content.Entry, error) {
	var fields map[string]map[string]any
	if err := json.Unmarshal([]byte(d.Fields), &fields); err != nil {
		return nil, err
	}
	e := &content.Entry{
		ID:        d.ID,
		ModelID:   d.ModelID,
		SpaceID:   d.SpaceID,
		Status:    content.EntryStatus(d.Status),
		Fields:    fields,
		Version:   d.Version,
		CreatedBy: d.CreatedBy,
		UpdatedBy: d.UpdatedBy,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
	if d.Slug.Valid {
		e.Slug = d.Slug.String
	}
	if d.PublishedVersion.Valid {
		pv := int(d.PublishedVersion.Int32)
		e.PublishedVersion = &pv
	}
	if d.PublishedAt.Valid {
		e.PublishedAt = &d.PublishedAt.Time
	}
	return e, nil
}

func dbEntryFromDomain(e *content.Entry) (*dbEntry, error) {
	bytes, err := json.Marshal(e.Fields)
	if err != nil {
		return nil, err
	}
	if e.Fields == nil {
		bytes = []byte("{}")
	}
	var pv sql.NullInt32
	if e.PublishedVersion != nil {
		pv.Int32 = int32(*e.PublishedVersion)
		pv.Valid = true
	}
	var pa sql.NullTime
	if e.PublishedAt != nil {
		pa.Time = *e.PublishedAt
		pa.Valid = true
	}
	return &dbEntry{
		ID:               e.ID,
		ModelID:          e.ModelID,
		SpaceID:          e.SpaceID,
		Slug:             sql.NullString{String: e.Slug, Valid: e.Slug != ""},
		Status:           string(e.Status),
		Fields:           string(bytes),
		Version:          e.Version,
		PublishedVersion: pv,
		CreatedBy:        e.CreatedBy,
		UpdatedBy:        e.UpdatedBy,
		PublishedAt:      pa,
		CreatedAt:        e.CreatedAt,
		UpdatedAt:        e.UpdatedAt,
	}, nil
}

type dbVersion struct {
	ID        uuid.UUID `db:"id"`
	EntryID   uuid.UUID `db:"entry_id"`
	Version   int       `db:"version"`
	Fields    string    `db:"fields"` // JSONB
	CreatedBy uuid.UUID `db:"created_by"`
	CreatedAt time.Time `db:"created_at"`
}

func (d *dbVersion) toDomain() (*content.Version, error) {
	var fields map[string]map[string]any
	if err := json.Unmarshal([]byte(d.Fields), &fields); err != nil {
		return nil, err
	}
	return &content.Version{
		ID:        d.ID,
		EntryID:   d.EntryID,
		Version:   d.Version,
		Fields:    fields,
		CreatedBy: d.CreatedBy,
		CreatedAt: d.CreatedAt,
	}, nil
}

func dbVersionFromDomain(v *content.Version) (*dbVersion, error) {
	bytes, err := json.Marshal(v.Fields)
	if err != nil {
		return nil, err
	}
	if v.Fields == nil {
		bytes = []byte("{}")
	}
	return &dbVersion{
		ID:        v.ID,
		EntryID:   v.EntryID,
		Version:   v.Version,
		Fields:    string(bytes),
		CreatedBy: v.CreatedBy,
		CreatedAt: v.CreatedAt,
	}, nil
}
