package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/combi/pkg/domain/catalog"
	"github.com/combi/pkg/log"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ catalog.Repository = (*CatalogRepository)(nil)

type CatalogRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewCatalogRepository(db *sqlx.DB, logger log.Logger) *CatalogRepository {
	return &CatalogRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "catalog_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *CatalogRepository) WithTx(ctx context.Context, fn func(txRepo catalog.Repository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	txRepo := &CatalogRepository{
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
// Items
// ---------------------------------------------------------

func (r *CatalogRepository) CreateItem(ctx context.Context, item *catalog.Item) error {
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	item.UpdatedAt = item.CreatedAt

	dbItem, err := toDBItem(item)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("items").
		Columns("id", "type", "name", "slug", "description", "status", "properties", "tags", "created_at", "updated_at").
		Values(dbItem.ID, dbItem.Type, dbItem.Name, dbItem.Slug, dbItem.Description, dbItem.Status, dbItem.Properties, dbItem.Tags, dbItem.CreatedAt, dbItem.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isCatalogUniqueViolation(err) {
			return catalog.ErrItemAlreadyExists
		}
		return err
	}
	return nil
}

func (r *CatalogRepository) GetItem(ctx context.Context, id uuid.UUID) (*catalog.Item, error) {
	query, args, _ := r.sq.Select("*").From("items").Where(squirrel.Eq{"id": id}).ToSql()
	return r.fetchItem(ctx, query, args...)
}

func (r *CatalogRepository) GetItemBySlug(ctx context.Context, slug string) (*catalog.Item, error) {
	query, args, _ := r.sq.Select("*").From("items").Where(squirrel.Eq{"slug": slug}).ToSql()
	return r.fetchItem(ctx, query, args...)
}

func (r *CatalogRepository) ListItems(ctx context.Context, filter catalog.ItemFilter) ([]*catalog.Item, int, error) {
	builder := r.sq.Select("i.*").From("items i")
	countBuilder := r.sq.Select("COUNT(DISTINCT i.id)").From("items i")

	if filter.CategoryID != nil {
		// Join for category filtering
		builder = builder.Join("item_categories ic ON ic.item_id = i.id").Where(squirrel.Eq{"ic.category_id": *filter.CategoryID})
		countBuilder = countBuilder.Join("item_categories ic ON ic.item_id = i.id").Where(squirrel.Eq{"ic.category_id": *filter.CategoryID})
	}

	if filter.Type != nil {
		builder = builder.Where(squirrel.Eq{"i.type": *filter.Type})
		countBuilder = countBuilder.Where(squirrel.Eq{"i.type": *filter.Type})
	}
	if filter.Status != nil {
		builder = builder.Where(squirrel.Eq{"i.status": *filter.Status})
		countBuilder = countBuilder.Where(squirrel.Eq{"i.status": *filter.Status})
	}
	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		builder = builder.Where(squirrel.Or{
			squirrel.Like{"i.name": like},
			squirrel.Like{"i.description": like},
		})
		countBuilder = countBuilder.Where(squirrel.Or{
			squirrel.Like{"i.name": like},
			squirrel.Like{"i.description": like},
		})
	}

	// Count
	var total int
	cQuery, cArgs, _ := countBuilder.ToSql()
	if err := r.runner.QueryRowxContext(ctx, cQuery, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// List
	query, args, err := builder.OrderBy("i.created_at DESC").
		Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}

	var dbItems []dbItem
	if err := sqlx.SelectContext(ctx, r.runner, &dbItems, query, args...); err != nil {
		return nil, 0, err
	}

	items := make([]*catalog.Item, len(dbItems))
	for i, row := range dbItems {
		item, err := row.toDomain()
		if err != nil {
			return nil, 0, err
		}
		items[i] = item
	}

	return items, total, nil
}

func (r *CatalogRepository) UpdateItem(ctx context.Context, item *catalog.Item) error {
	item.UpdatedAt = time.Now().UTC()
	dbItem, err := toDBItem(item)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Update("items").
		Set("name", dbItem.Name).
		Set("slug", dbItem.Slug).
		Set("description", dbItem.Description).
		Set("status", dbItem.Status).
		Set("properties", dbItem.Properties).
		Set("tags", dbItem.Tags).
		Set("updated_at", dbItem.UpdatedAt).
		Where(squirrel.Eq{"id": dbItem.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isCatalogUniqueViolation(err) {
			return catalog.ErrItemAlreadyExists
		}
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return catalog.ErrItemNotFound
	}
	return nil
}

func (r *CatalogRepository) DeleteItem(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("items").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return catalog.ErrItemNotFound
	}
	return nil
}

// ---------------------------------------------------------
// Categorization
// ---------------------------------------------------------

func (r *CatalogRepository) AddItemToCategory(ctx context.Context, itemID, categoryID uuid.UUID) error {
	query, args, _ := r.sq.Insert("item_categories").
		Columns("item_id", "category_id").
		Values(itemID, categoryID).
		Suffix("ON CONFLICT DO NOTHING"). // Idempotent
		ToSql()

	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *CatalogRepository) RemoveAllCategoriesFromItem(ctx context.Context, itemID uuid.UUID) error {
	query, args, _ := r.sq.Delete("item_categories").Where(squirrel.Eq{"item_id": itemID}).ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *CatalogRepository) GetCategoryIDsForItem(ctx context.Context, itemID uuid.UUID) ([]uuid.UUID, error) {
	query, args, _ := r.sq.Select("category_id").From("item_categories").Where(squirrel.Eq{"item_id": itemID}).ToSql()
	var ids []uuid.UUID
	err := sqlx.SelectContext(ctx, r.runner, &ids, query, args...)
	return ids, err
}

// ---------------------------------------------------------
// Categories
// ---------------------------------------------------------

func (r *CatalogRepository) CreateCategory(ctx context.Context, c *catalog.Category) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	c.UpdatedAt = c.CreatedAt

	dbC := toDBCategory(c)
	query, args, err := r.sq.Insert("categories").
		Columns("id", "parent_id", "name", "slug", "description", "created_at", "updated_at").
		Values(dbC.ID, dbC.ParentID, dbC.Name, dbC.Slug, dbC.Description, dbC.CreatedAt, dbC.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isCatalogUniqueViolation(err) {
			return catalog.ErrCategoryAlreadyExists
		}
		return err
	}
	return nil
}

func (r *CatalogRepository) GetCategory(ctx context.Context, id uuid.UUID) (*catalog.Category, error) {
	query, args, _ := r.sq.Select("*").From("categories").Where(squirrel.Eq{"id": id}).ToSql()
	var dbC dbCategory
	if err := sqlx.GetContext(ctx, r.runner, &dbC, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, catalog.ErrCategoryNotFound
		}
		return nil, err
	}
	return dbC.toDomain(), nil
}

func (r *CatalogRepository) ListCategories(ctx context.Context) ([]*catalog.Category, error) {
	query, args, _ := r.sq.Select("*").From("categories").OrderBy("name ASC").ToSql()
	var rows []dbCategory
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}
	cats := make([]*catalog.Category, len(rows))
	for i, r := range rows {
		cats[i] = r.toDomain()
	}
	return cats, nil
}

func (r *CatalogRepository) UpdateCategory(ctx context.Context, c *catalog.Category) error {
	c.UpdatedAt = time.Now().UTC()
	dbC := toDBCategory(c)
	query, args, err := r.sq.Update("categories").
		Set("name", dbC.Name).
		Set("slug", dbC.Slug).
		Set("description", dbC.Description).
		Set("parent_id", dbC.ParentID).
		Set("updated_at", dbC.UpdatedAt).
		Where(squirrel.Eq{"id": dbC.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isCatalogUniqueViolation(err) {
			return catalog.ErrCategoryAlreadyExists
		}
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return catalog.ErrCategoryNotFound
	}
	return nil
}

func (r *CatalogRepository) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	// Check items constraints usually done in Service, but DB cascade handles join table.
	// If you want to prevent deleting a category that has items:
	count, _ := r.CountItemsInCategory(ctx, id)
	if count > 0 {
		return catalog.ErrCategoryInUse
	}

	query, args, _ := r.sq.Delete("categories").Where(squirrel.Eq{"id": id}).ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *CatalogRepository) CountItemsInCategory(ctx context.Context, categoryID uuid.UUID) (int, error) {
	query, args, _ := r.sq.Select("COUNT(*)").From("item_categories").Where(squirrel.Eq{"category_id": categoryID}).ToSql()
	var count int
	err := r.runner.QueryRowxContext(ctx, query, args...).Scan(&count)
	return count, err
}

// ---------------------------------------------------------
// Variants
// ---------------------------------------------------------

func (r *CatalogRepository) CreateVariant(ctx context.Context, v *catalog.Variant) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}
	v.UpdatedAt = v.CreatedAt

	dbV, err := toDBVariant(v)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("variants").
		Columns("id", "item_id", "sku", "name", "price_amount", "price_currency", "stock", "attributes", "is_active", "created_at", "updated_at").
		Values(dbV.ID, dbV.ItemID, dbV.SKU, dbV.Name, dbV.PriceAmount, dbV.PriceCurrency, dbV.Stock, dbV.Attributes, dbV.IsActive, dbV.CreatedAt, dbV.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *CatalogRepository) ListVariants(ctx context.Context, itemID uuid.UUID) ([]*catalog.Variant, error) {
	query, args, _ := r.sq.Select("*").From("variants").
		Where(squirrel.Eq{"item_id": itemID}).
		OrderBy("price_amount ASC").ToSql()

	var rows []dbVariant
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}
	variants := make([]*catalog.Variant, len(rows))
	for i, r := range rows {
		v, err := r.toDomain()
		if err != nil {
			return nil, err
		}
		variants[i] = v
	}
	return variants, nil
}

// ---------------------------------------------------------
// Applications
// ---------------------------------------------------------

func (r *CatalogRepository) CreateApplication(ctx context.Context, app *catalog.Application) error {
	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}
	if app.CreatedAt.IsZero() {
		app.CreatedAt = time.Now().UTC()
	}
	app.UpdatedAt = app.CreatedAt

	dbApp, err := toDBApplication(app)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("applications").
		Columns("id", "user_id", "item_id", "variant_id", "status", "answers", "created_at", "updated_at").
		Values(dbApp.ID, dbApp.UserID, dbApp.ItemID, dbApp.VariantID, dbApp.Status, dbApp.Answers, dbApp.CreatedAt, dbApp.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isCatalogUniqueViolation(err) {
			return catalog.ErrAlreadyApplied
		}
		return err
	}
	return nil
}

func (r *CatalogRepository) GetApplication(ctx context.Context, id uuid.UUID) (*catalog.Application, error) {
	query, args, _ := r.sq.Select("*").From("applications").Where(squirrel.Eq{"id": id}).ToSql()
	var dbApp dbApplication
	if err := sqlx.GetContext(ctx, r.runner, &dbApp, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, catalog.ErrApplicationNotFound
		}
		return nil, err
	}
	return dbApp.toDomain()
}

func (r *CatalogRepository) GetApplicationByUserAndItem(ctx context.Context, userID, itemID uuid.UUID) (*catalog.Application, error) {
	query, args, _ := r.sq.Select("*").From("applications").
		Where(squirrel.Eq{"user_id": userID, "item_id": itemID}).ToSql()

	var dbApp dbApplication
	if err := sqlx.GetContext(ctx, r.runner, &dbApp, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, catalog.ErrApplicationNotFound
		}
		return nil, err
	}
	return dbApp.toDomain()
}

func (r *CatalogRepository) ListApplications(ctx context.Context, itemID uuid.UUID, filter catalog.ApplicationFilter) ([]*catalog.Application, int, error) {
	builder := r.sq.Select("*").From("applications").Where(squirrel.Eq{"item_id": itemID})
	countBuilder := r.sq.Select("COUNT(*)").From("applications").Where(squirrel.Eq{"item_id": itemID})

	if filter.Status != nil {
		builder = builder.Where(squirrel.Eq{"status": *filter.Status})
		countBuilder = countBuilder.Where(squirrel.Eq{"status": *filter.Status})
	}

	var total int
	cQuery, cArgs, _ := countBuilder.ToSql()
	if err := r.runner.QueryRowxContext(ctx, cQuery, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query, args, _ := builder.OrderBy("created_at DESC").
		Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset)).ToSql()

	var rows []dbApplication
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, 0, err
	}

	apps := make([]*catalog.Application, len(rows))
	for i, row := range rows {
		a, err := row.toDomain()
		if err != nil {
			return nil, 0, err
		}
		apps[i] = a
	}
	return apps, total, nil
}

func (r *CatalogRepository) ListUserApplications(ctx context.Context, userID uuid.UUID) ([]*catalog.Application, error) {
	query, args, _ := r.sq.Select("*").From("applications").
		Where(squirrel.Eq{"user_id": userID}).
		OrderBy("created_at DESC").ToSql()

	var rows []dbApplication
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}

	apps := make([]*catalog.Application, len(rows))
	for i, row := range rows {
		a, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		apps[i] = a
	}
	return apps, nil
}

func (r *CatalogRepository) UpdateApplication(ctx context.Context, app *catalog.Application) error {
	app.UpdatedAt = time.Now().UTC()
	dbApp, err := toDBApplication(app)
	if err != nil {
		return err
	}

	query, args, _ := r.sq.Update("applications").
		Set("status", dbApp.Status).
		Set("answers", dbApp.Answers).
		Set("updated_at", dbApp.UpdatedAt).
		Where(squirrel.Eq{"id": dbApp.ID}).
		ToSql()

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return catalog.ErrApplicationNotFound
	}
	return nil
}

// ---------------------------------------------------------
// Helpers
// ---------------------------------------------------------

func (r *CatalogRepository) fetchItem(ctx context.Context, query string, args ...interface{}) (*catalog.Item, error) {
	var dbI dbItem
	if err := sqlx.GetContext(ctx, r.runner, &dbI, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, catalog.ErrItemNotFound
		}
		return nil, err
	}
	return dbI.toDomain()
}

func isCatalogUniqueViolation(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code == "23505"
	}
	return false
}

// ---------------------------------------------------------
// DB Models
// ---------------------------------------------------------

type dbItem struct {
	ID          uuid.UUID `db:"id"`
	Type        string    `db:"type"`
	Name        string    `db:"name"`
	Slug        string    `db:"slug"`
	Description string    `db:"description"`
	Status      string    `db:"status"`
	Properties  []byte    `db:"properties"`
	Tags        []byte    `db:"tags"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (d *dbItem) toDomain() (*catalog.Item, error) {
	item := &catalog.Item{
		ID:          d.ID,
		Type:        d.Type,
		Name:        d.Name,
		Slug:        d.Slug,
		Description: d.Description,
		Status:      catalog.ItemStatus(d.Status),
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
	if len(d.Properties) > 0 {
		item.Properties = json.RawMessage(d.Properties)
	}
	if len(d.Tags) > 0 {
		item.Tags = json.RawMessage(d.Tags)
	}
	return item, nil
}

func toDBItem(i *catalog.Item) (*dbItem, error) {
	// Ensure non-nil JSONB
	if i.Properties == nil {
		i.Properties = json.RawMessage("{}")
	}
	if i.Tags == nil {
		i.Tags = json.RawMessage("[]")
	}

	return &dbItem{
		ID:          i.ID,
		Type:        i.Type,
		Name:        i.Name,
		Slug:        i.Slug,
		Description: i.Description,
		Status:      string(i.Status),
		Properties:  []byte(i.Properties),
		Tags:        []byte(i.Tags),
		CreatedAt:   i.CreatedAt,
		UpdatedAt:   i.UpdatedAt,
	}, nil
}

type dbCategory struct {
	ID          uuid.UUID      `db:"id"`
	ParentID    uuid.NullUUID  `db:"parent_id"`
	Name        string         `db:"name"`
	Slug        string         `db:"slug"`
	Description sql.NullString `db:"description"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}

func (d *dbCategory) toDomain() *catalog.Category {
	c := &catalog.Category{
		ID:        d.ID,
		Name:      d.Name,
		Slug:      d.Slug,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
	if d.ParentID.Valid {
		c.ParentID = &d.ParentID.UUID
	}
	if d.Description.Valid {
		c.Description = &d.Description.String
	}
	return c
}

func toDBCategory(c *catalog.Category) *dbCategory {
	d := &dbCategory{
		ID:        c.ID,
		Name:      c.Name,
		Slug:      c.Slug,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	if c.ParentID != nil {
		d.ParentID = uuid.NullUUID{UUID: *c.ParentID, Valid: true}
	}
	if c.Description != nil {
		d.Description = sql.NullString{String: *c.Description, Valid: true}
	}
	return d
}

type dbVariant struct {
	ID            uuid.UUID      `db:"id"`
	ItemID        uuid.UUID      `db:"item_id"`
	SKU           sql.NullString `db:"sku"`
	Name          string         `db:"name"`
	PriceAmount   int64          `db:"price_amount"`
	PriceCurrency string         `db:"price_currency"`
	Stock         sql.NullInt32  `db:"stock"`
	Attributes    []byte         `db:"attributes"`
	IsActive      bool           `db:"is_active"`
	CreatedAt     time.Time      `db:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at"`
}

func (d *dbVariant) toDomain() (*catalog.Variant, error) {
	v := &catalog.Variant{
		ID:        d.ID,
		ItemID:    d.ItemID,
		Name:      d.Name,
		Price:     d.PriceAmount,
		Currency:  d.PriceCurrency,
		IsActive:  d.IsActive,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
	if d.SKU.Valid {
		v.SKU = d.SKU.String
	}
	if d.Stock.Valid {
		val := int(d.Stock.Int32)
		v.Stock = &val
	}
	if len(d.Attributes) > 0 {
		v.Attributes = json.RawMessage(d.Attributes)
	}
	return v, nil
}

func toDBVariant(v *catalog.Variant) (*dbVariant, error) {
	if v.Attributes == nil {
		v.Attributes = json.RawMessage("{}")
	}

	d := &dbVariant{
		ID:            v.ID,
		ItemID:        v.ItemID,
		Name:          v.Name,
		PriceAmount:   v.Price,
		PriceCurrency: v.Currency,
		IsActive:      v.IsActive,
		Attributes:    []byte(v.Attributes),
		CreatedAt:     v.CreatedAt,
		UpdatedAt:     v.UpdatedAt,
	}
	if v.SKU != "" {
		d.SKU = sql.NullString{String: v.SKU, Valid: true}
	}
	if v.Stock != nil {
		d.Stock = sql.NullInt32{Int32: int32(*v.Stock), Valid: true}
	}
	return d, nil
}

type dbApplication struct {
	ID        uuid.UUID     `db:"id"`
	UserID    uuid.UUID     `db:"user_id"`
	ItemID    uuid.UUID     `db:"item_id"`
	VariantID uuid.NullUUID `db:"variant_id"`
	Status    string        `db:"status"`
	Answers   []byte        `db:"answers"`
	CreatedAt time.Time     `db:"created_at"`
	UpdatedAt time.Time     `db:"updated_at"`
}

func (d *dbApplication) toDomain() (*catalog.Application, error) {
	a := &catalog.Application{
		ID:        d.ID,
		UserID:    d.UserID,
		ItemID:    d.ItemID,
		Status:    catalog.ApplicationStatus(d.Status),
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
	if d.VariantID.Valid {
		a.VariantID = &d.VariantID.UUID
	}
	if len(d.Answers) > 0 {
		a.Answers = json.RawMessage(d.Answers)
	}
	return a, nil
}

func toDBApplication(a *catalog.Application) (*dbApplication, error) {
	if a.Answers == nil {
		a.Answers = json.RawMessage("{}")
	}

	d := &dbApplication{
		ID:        a.ID,
		UserID:    a.UserID,
		ItemID:    a.ItemID,
		Status:    string(a.Status),
		Answers:   []byte(a.Answers),
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
	if a.VariantID != nil {
		d.VariantID = uuid.NullUUID{UUID: *a.VariantID, Valid: true}
	}
	return d, nil
}
