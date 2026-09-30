package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/campaign"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ campaign.Repository = (*CampaignRepository)(nil)

type CampaignRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewCampaignRepository(db *sqlx.DB, logger log.Logger) *CampaignRepository {
	return &CampaignRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "campaign_repo")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// WithTx enables transactional execution
func (r *CampaignRepository) WithTx(ctx context.Context, fn func(txRepo campaign.Repository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	txRepo := &CampaignRepository{
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

// --- Campaign CRUD ---

func (r *CampaignRepository) CreateCampaign(ctx context.Context, c *campaign.Campaign) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	c.UpdatedAt = c.CreatedAt

	query, args, err := r.sq.Insert("campaigns").
		Columns("id", "organization_id", "slug", "name", "description", "goal_amount", "currency", "status", "allow_p2p", "is_featured", "image_url", "start_date", "end_date", "created_at", "updated_at").
		Values(c.ID, c.OrganizationID, c.Slug, c.Name, c.Description, c.GoalAmount, c.Currency, c.Status, c.AllowP2P, c.IsFeatured, c.ImageURL, c.StartDate, c.EndDate, c.CreatedAt, c.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return campaign.ErrSlugTaken
		}
		return err
	}
	return nil
}

func (r *CampaignRepository) GetCampaignByID(ctx context.Context, id uuid.UUID) (*campaign.Campaign, error) {
	return r.getCampaign(ctx, squirrel.Eq{"id": id})
}

func (r *CampaignRepository) GetCampaignBySlug(ctx context.Context, slug string) (*campaign.Campaign, error) {
	return r.getCampaign(ctx, squirrel.Eq{"slug": slug})
}

func (r *CampaignRepository) getCampaign(ctx context.Context, pred interface{}) (*campaign.Campaign, error) {
	builder := WithTenant(ctx, r.sq.Select("*").From("campaigns").Where(pred))
	query, args, _ := builder.ToSql()
	var dest dbCampaign
	if err := sqlx.GetContext(ctx, r.runner, &dest, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, campaign.ErrCampaignNotFound
		}
		return nil, err
	}
	return dest.toDomain(), nil
}

func (r *CampaignRepository) UpdateCampaign(ctx context.Context, c *campaign.Campaign) error {
	c.UpdatedAt = time.Now().UTC()
	builder := r.sq.Update("campaigns").
		Set("name", c.Name).
		Set("description", c.Description).
		Set("goal_amount", c.GoalAmount).
		Set("status", c.Status).
		Set("is_featured", c.IsFeatured).
		Set("image_url", c.ImageURL).
		Set("updated_at", c.UpdatedAt).
		Where(squirrel.Eq{"id": c.ID})

	builder = WithTenantUpdate(ctx, builder)
	query, args, _ := builder.ToSql()

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return campaign.ErrCampaignNotFound
	}
	return nil
}

func (r *CampaignRepository) ListCampaigns(ctx context.Context, filter campaign.CampaignFilter) ([]*campaign.Campaign, int, error) {
	builder := WithTenant(ctx, r.sq.Select("*, COUNT(*) OVER() as total_count").From("campaigns"))

	if filter.OrganizationID != uuid.Nil {
		builder = builder.Where(squirrel.Eq{"organization_id": filter.OrganizationID})
	}

	if filter.Status != nil {
		builder = builder.Where(squirrel.Eq{"status": *filter.Status})
	}

	query, args, err := builder.
		OrderBy("created_at DESC").
		Limit(uint64(filter.Limit)).
		Offset(uint64(filter.Offset)).
		ToSql()
	if err != nil {
		return nil, 0, err
	}

	var rows []dbCampaignWithTotal
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, 0, err
	}

	list := make([]*campaign.Campaign, len(rows))
	total := 0
	if len(rows) > 0 {
		total = rows[0].Total
	}

	for i, row := range rows {
		list[i] = row.toDomain()
	}
	return list, total, nil
}

// --- Fundraiser CRUD ---

func (r *CampaignRepository) CreateFundraiser(ctx context.Context, f *campaign.Fundraiser) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now().UTC()
	}
	f.UpdatedAt = f.CreatedAt

	query, args, _ := r.sq.Insert("fundraisers").
		Columns("id", "organization_id", "campaign_id", "owner_id", "slug", "name", "description", "goal_amount", "image_url", "created_at", "updated_at").
		Values(f.ID, f.OrganizationID, f.CampaignID, f.OwnerID, f.Slug, f.Name, f.Description, f.GoalAmount, f.ImageURL, f.CreatedAt, f.UpdatedAt).
		ToSql()

	_, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return campaign.ErrSlugTaken
		}
		return err
	}
	return nil
}

func (r *CampaignRepository) GetFundraiserByID(ctx context.Context, id uuid.UUID) (*campaign.Fundraiser, error) {
	return r.getFundraiser(ctx, squirrel.Eq{"id": id})
}

func (r *CampaignRepository) GetFundraiserBySlug(ctx context.Context, slug string) (*campaign.Fundraiser, error) {
	return r.getFundraiser(ctx, squirrel.Eq{"slug": slug})
}

func (r *CampaignRepository) getFundraiser(ctx context.Context, pred interface{}) (*campaign.Fundraiser, error) {
	builder := WithTenant(ctx, r.sq.Select("*").From("fundraisers").Where(pred))
	query, args, _ := builder.ToSql()
	var dest dbFundraiser
	if err := sqlx.GetContext(ctx, r.runner, &dest, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, campaign.ErrFundraiserNotFound
		}
		return nil, err
	}
	return dest.toDomain(), nil
}

func (r *CampaignRepository) UpdateFundraiser(ctx context.Context, f *campaign.Fundraiser) error {
	f.UpdatedAt = time.Now().UTC()
	builder := r.sq.Update("fundraisers").
		Set("name", f.Name).
		Set("description", f.Description).
		Set("goal_amount", f.GoalAmount).
		Set("image_url", f.ImageURL).
		Set("updated_at", f.UpdatedAt).
		Where(squirrel.Eq{"id": f.ID})

	builder = WithTenantUpdate(ctx, builder)
	query, args, _ := builder.ToSql()

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return campaign.ErrFundraiserNotFound
	}
	return nil
}

func (r *CampaignRepository) ListFundraisers(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]*campaign.Fundraiser, int, error) {
	builder := WithTenant(ctx, r.sq.Select("*, COUNT(*) OVER() as total_count").
		From("fundraisers").
		Where(squirrel.Eq{"campaign_id": campaignID}))

	query, args, _ := builder.
		OrderBy("total_raised DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).
		ToSql()

	var rows []dbFundraiserWithTotal
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, 0, err
	}

	list := make([]*campaign.Fundraiser, len(rows))
	total := 0
	if len(rows) > 0 {
		total = rows[0].Total
	}
	for i, row := range rows {
		list[i] = row.toDomain()
	}
	return list, total, nil
}

// --- Stats & Attribution ---

func (r *CampaignRepository) CreateAttribution(ctx context.Context, attr *campaign.DonationAttribution) error {
	if attr.CreatedAt.IsZero() {
		attr.CreatedAt = time.Now().UTC()
	}

	query, args, _ := r.sq.Insert("donation_attributions").
		Columns("transaction_id", "organization_id", "campaign_id", "fundraiser_id", "amount", "created_at").
		Values(attr.TransactionID, attr.OrganizationID, attr.CampaignID, attr.FundraiserID, attr.Amount, attr.CreatedAt).
		ToSql()

	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *CampaignRepository) IncrementStats(ctx context.Context, campaignID uuid.UUID, fundraiserID *uuid.UUID, amount int64) error {
	// 1. Update Campaign
	campQuery, campArgs, _ := r.sq.Update("campaigns").
		Set("total_raised", squirrel.Expr("total_raised + ?", amount)).
		Set("donor_count", squirrel.Expr("donor_count + 1")).
		Where(squirrel.Eq{"id": campaignID}).
		ToSql()
	if _, err := r.runner.ExecContext(ctx, campQuery, campArgs...); err != nil {
		return fmt.Errorf("failed to increment campaign stats: %w", err)
	}

	// 2. Update Fundraiser (if applicable)
	if fundraiserID != nil {
		fundQuery, fundArgs, _ := r.sq.Update("fundraisers").
			Set("total_raised", squirrel.Expr("total_raised + ?", amount)).
			Set("donor_count", squirrel.Expr("donor_count + 1")).
			Where(squirrel.Eq{"id": *fundraiserID}).
			ToSql()
		if _, err := r.runner.ExecContext(ctx, fundQuery, fundArgs...); err != nil {
			return fmt.Errorf("failed to increment fundraiser stats: %w", err)
		}
	}

	return nil
}

// --- Helpers ---

func isUniqueViolation(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
		return true
	}
	return false
}

// --- DB Models ---

type dbCampaign struct {
	ID             uuid.UUID    `db:"id"`
	OrganizationID *uuid.UUID   `db:"organization_id"`
	Slug           string       `db:"slug"`
	Name           string       `db:"name"`
	Description    string       `db:"description"`
	GoalAmount     int64        `db:"goal_amount"`
	Currency       string       `db:"currency"`
	Status         string       `db:"status"`
	AllowP2P       bool         `db:"allow_p2p"`
	IsFeatured     bool         `db:"is_featured"`
	ImageURL       string       `db:"image_url"`
	TotalRaised    int64        `db:"total_raised"`
	DonorCount     int          `db:"donor_count"`
	StartDate      sql.NullTime `db:"start_date"`
	EndDate        sql.NullTime `db:"end_date"`
	CreatedAt      time.Time    `db:"created_at"`
	UpdatedAt      time.Time    `db:"updated_at"`
}

type dbCampaignWithTotal struct {
	dbCampaign
	Total int `db:"total_count"`
}

func (d *dbCampaign) toDomain() *campaign.Campaign {
	var start, end *time.Time
	if d.StartDate.Valid {
		start = &d.StartDate.Time
	}
	if d.EndDate.Valid {
		end = &d.EndDate.Time
	}
	orgID := uuid.Nil
	if d.OrganizationID != nil {
		orgID = *d.OrganizationID
	}
	return &campaign.Campaign{
		ID: d.ID, OrganizationID: orgID, Slug: d.Slug, Name: d.Name, Description: d.Description, GoalAmount: d.GoalAmount,
		Currency: d.Currency, Status: campaign.CampaignStatus(d.Status), AllowP2P: d.AllowP2P,
		IsFeatured: d.IsFeatured, ImageURL: d.ImageURL, TotalRaised: d.TotalRaised, DonorCount: d.DonorCount,
		StartDate: start, EndDate: end, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

type dbFundraiser struct {
	ID             uuid.UUID  `db:"id"`
	OrganizationID *uuid.UUID `db:"organization_id"`
	CampaignID     uuid.UUID  `db:"campaign_id"`
	OwnerID        uuid.UUID  `db:"owner_id"`
	Slug           string     `db:"slug"`
	Name           string     `db:"name"`
	Description    string     `db:"description"`
	GoalAmount     int64      `db:"goal_amount"`
	ImageURL       string     `db:"image_url"`
	TotalRaised    int64      `db:"total_raised"`
	DonorCount     int        `db:"donor_count"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

type dbFundraiserWithTotal struct {
	dbFundraiser
	Total int `db:"total_count"`
}

func (d *dbFundraiser) toDomain() *campaign.Fundraiser {
	orgID := uuid.Nil
	if d.OrganizationID != nil {
		orgID = *d.OrganizationID
	}
	return &campaign.Fundraiser{
		ID: d.ID, OrganizationID: orgID, CampaignID: d.CampaignID, OwnerID: d.OwnerID, Slug: d.Slug, Name: d.Name,
		Description: d.Description, GoalAmount: d.GoalAmount, ImageURL: d.ImageURL, TotalRaised: d.TotalRaised,
		DonorCount: d.DonorCount, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}
