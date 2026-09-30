package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/tagging"
	"github.com/combi/pkg/log"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ tagging.Repository = (*TaggingRepository)(nil)

type TaggingRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewTaggingRepository(db *sqlx.DB, logger log.Logger) *TaggingRepository {
	return &TaggingRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "tagging_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *TaggingRepository) WithTx(ctx context.Context, fn func(txRepo tagging.Repository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	txRepo := &TaggingRepository{
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

// -----------------------------------------------------------------------------
// Tags
// -----------------------------------------------------------------------------

func (r *TaggingRepository) CreateTag(ctx context.Context, t *tagging.Tag) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	t.UpdatedAt = t.CreatedAt

	query, args, err := r.sq.Insert("tags").
		Columns("id", "name", "slug", "type", "description", "color", "count", "is_approved", "created_at", "updated_at").
		Values(t.ID, t.Name, t.Slug, t.Type, t.Description, t.Color, t.Count, t.IsApproved, t.CreatedAt, t.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isTaggingUniqueViolation(err) {
			return tagging.ErrTagAlreadyExists
		}
		return err
	}
	return nil
}

func (r *TaggingRepository) GetTagByID(ctx context.Context, id uuid.UUID) (*tagging.Tag, error) {
	query, args, _ := r.sq.Select("*").From("tags").Where(squirrel.Eq{"id": id}).ToSql()
	var t tagging.Tag
	if err := sqlx.GetContext(ctx, r.runner, &t, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, tagging.ErrTagNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *TaggingRepository) GetTagBySlug(ctx context.Context, slug string, tType tagging.TagType) (*tagging.Tag, error) {
	query, args, _ := r.sq.Select("*").From("tags").
		Where(squirrel.Eq{"slug": slug, "type": tType}).ToSql()

	var t tagging.Tag
	if err := sqlx.GetContext(ctx, r.runner, &t, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, tagging.ErrTagNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *TaggingRepository) SearchTags(ctx context.Context, queryStr string, tType *tagging.TagType, limit int) ([]tagging.Tag, error) {
	builder := r.sq.Select("*").From("tags")

	if tType != nil {
		builder = builder.Where(squirrel.Eq{"type": *tType})
	}

	if queryStr != "" {
		// Case insensitive partial match
		builder = builder.Where(squirrel.Like{"name": "%" + queryStr + "%"})
	}

	sqlQuery, args, err := builder.OrderBy("count DESC", "name ASC").Limit(uint64(limit)).ToSql()
	if err != nil {
		return nil, err
	}

	var tags []tagging.Tag
	if err := sqlx.SelectContext(ctx, r.runner, &tags, sqlQuery, args...); err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *TaggingRepository) UpdateTag(ctx context.Context, t *tagging.Tag) error {
	t.UpdatedAt = time.Now().UTC()
	query, args, err := r.sq.Update("tags").
		Set("name", t.Name).
		Set("slug", t.Slug).
		Set("description", t.Description).
		Set("color", t.Color).
		Set("is_approved", t.IsApproved).
		Set("updated_at", t.UpdatedAt).
		Where(squirrel.Eq{"id": t.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isTaggingUniqueViolation(err) {
			return tagging.ErrTagAlreadyExists
		}
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return tagging.ErrTagNotFound
	}
	return nil
}

func (r *TaggingRepository) DeleteTag(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("tags").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return tagging.ErrTagNotFound
	}
	return nil
}

// -----------------------------------------------------------------------------
// Taggings
// -----------------------------------------------------------------------------

func (r *TaggingRepository) AddTagging(ctx context.Context, t *tagging.Tagging) (bool, error) {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}

	query, args, err := r.sq.Insert("taggings").
		Columns("id", "tag_id", "entity_type", "entity_id", "created_at").
		Values(t.ID, t.TagID, t.EntityType, t.EntityID, t.CreatedAt).
		Suffix("ON CONFLICT (tag_id, entity_type, entity_id) DO NOTHING").
		ToSql()
	if err != nil {
		return false, err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return false, err
	}

	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

func (r *TaggingRepository) RemoveTagging(ctx context.Context, tagID uuid.UUID, entityType string, entityID uuid.UUID) error {
	query, args, _ := r.sq.Delete("taggings").
		Where(squirrel.Eq{
			"tag_id":      tagID,
			"entity_type": entityType,
			"entity_id":   entityID,
		}).ToSql()

	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *TaggingRepository) GetTagsForEntity(ctx context.Context, entityType string, entityID uuid.UUID) ([]tagging.Tag, error) {
	// Join to get full tag details
	query, args, err := r.sq.Select("t.*").
		From("tags t").
		Join("taggings tg ON tg.tag_id = t.id").
		Where(squirrel.Eq{
			"tg.entity_type": entityType,
			"tg.entity_id":   entityID,
		}).
		OrderBy("t.name ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	var tags []tagging.Tag
	err = sqlx.SelectContext(ctx, r.runner, &tags, query, args...)
	return tags, err
}

func (r *TaggingRepository) GetEntityIDsByTag(ctx context.Context, tagID uuid.UUID, entityType string, limit, offset int) ([]uuid.UUID, int, error) {
	builder := r.sq.Select("entity_id").From("taggings").
		Where(squirrel.Eq{"tag_id": tagID, "entity_type": entityType})

	// Count
	countBuilder := r.sq.Select("COUNT(*)").From("taggings").
		Where(squirrel.Eq{"tag_id": tagID, "entity_type": entityType})

	var total int
	cQuery, cArgs, _ := countBuilder.ToSql()
	if err := r.runner.QueryRowxContext(ctx, cQuery, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query, args, err := builder.Limit(uint64(limit)).Offset(uint64(offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}

	var ids []uuid.UUID
	if err := sqlx.SelectContext(ctx, r.runner, &ids, query, args...); err != nil {
		return nil, 0, err
	}

	return ids, total, nil
}

// -----------------------------------------------------------------------------
// Batch Ops
// -----------------------------------------------------------------------------

func (r *TaggingRepository) UpdateTagCount(ctx context.Context, tagID uuid.UUID, delta int) error {
	query, args, _ := r.sq.Update("tags").
		Set("count", squirrel.Expr("count + ?", delta)).
		Where(squirrel.Eq{"id": tagID}).
		ToSql()

	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *TaggingRepository) ReassignTaggings(ctx context.Context, oldTagID, newTagID uuid.UUID) error {
	// 1. Update tag_id where conflict does NOT exist
	// SQL: UPDATE taggings SET tag_id = new_id WHERE tag_id = old_id
	//      AND (entity_type, entity_id) NOT IN (SELECT entity_type, entity_id FROM taggings WHERE tag_id = new_id)

	// A simpler approach in Postgres often involves raw SQL because standard builders struggle with complex correlated subqueries or `ON CONFLICT` during UPDATE.
	// However, usually "Merge" implies we move everything. If the target already exists, we effectively just delete the old one to avoid Unique Constraint violation.

	// Strategy:
	// A. Delete records from 'old' that would conflict with 'new' (because 'new' already has them)
	// B. Update remaining 'old' to 'new'

	// A. Delete duplicates
	// DELETE FROM taggings t1
	// WHERE tag_id = $1
	// AND EXISTS (SELECT 1 FROM taggings t2 WHERE t2.tag_id = $2 AND t2.entity_type = t1.entity_type AND t2.entity_id = t1.entity_id)

	deleteQuery := `
		DELETE FROM taggings t1
		WHERE tag_id = $1
		AND EXISTS (
			SELECT 1 FROM taggings t2 
			WHERE t2.tag_id = $2 
			AND t2.entity_type = t1.entity_type 
			AND t2.entity_id = t1.entity_id
		)`

	if _, err := r.runner.ExecContext(ctx, deleteQuery, oldTagID, newTagID); err != nil {
		return fmt.Errorf("failed to clean duplicates during merge: %w", err)
	}

	// B. Move remaining
	updateQuery, uArgs, _ := r.sq.Update("taggings").
		Set("tag_id", newTagID).
		Where(squirrel.Eq{"tag_id": oldTagID}).
		ToSql()

	if _, err := r.runner.ExecContext(ctx, updateQuery, uArgs...); err != nil {
		return fmt.Errorf("failed to reassign taggings: %w", err)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func isTaggingUniqueViolation(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code == "23505"
	}
	return false
}
