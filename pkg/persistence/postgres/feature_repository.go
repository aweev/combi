// internal/persistence/postgres/feature_repository.go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/feature"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var _ feature.Repository = (*FeatureRepository)(nil)

type FeatureRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewFeatureRepository(db *sqlx.DB, logger log.Logger) *FeatureRepository {
	return &FeatureRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "feature_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *FeatureRepository) Get(ctx context.Context, key string) (*feature.Flag, error) {
	query, args, _ := r.sq.Select("*").From("feature_flags").Where(squirrel.Eq{"key": key}).ToSql()

	var dbf dbFeatureFlag
	if err := sqlx.GetContext(ctx, r.runner, &dbf, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, feature.ErrFlagNotFound
		}
		return nil, fmt.Errorf("failed to get feature flag: %w", err)
	}

	return dbf.toDomain()
}

func (r *FeatureRepository) ListAll(ctx context.Context) ([]feature.Flag, error) {
	query, args, _ := r.sq.Select("*").From("feature_flags").OrderBy("key ASC").ToSql()

	var dbFlags []dbFeatureFlag
	if err := sqlx.SelectContext(ctx, r.runner, &dbFlags, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list feature flags: %w", err)
	}

	result := make([]feature.Flag, len(dbFlags))
	for i, f := range dbFlags {
		domain, err := f.toDomain()
		if err != nil {
			r.logger.Error("Failed to unmarshal flag", err, log.String("key", f.Key))
			continue
		}
		result[i] = *domain
	}

	return result, nil
}

func (r *FeatureRepository) Upsert(ctx context.Context, flag feature.Flag) error {
	flag.UpdatedAt = time.Now().UTC()
	dbModel, err := dbFeatureFlagFromDomain(flag)
	if err != nil {
		return err
	}

	// Upsert logic
	query, args, err := r.sq.Insert("feature_flags").
		Columns("key", "name", "description", "type", "is_active", "variations", "targeting_rules", "default_variation_id", "off_variation_id", "updated_at", "updated_by").
		Values(dbModel.Key, dbModel.Name, dbModel.Description, dbModel.Type, dbModel.IsActive, dbModel.Variations, dbModel.TargetingRules, dbModel.DefaultVariationID, dbModel.OffVariationID, dbModel.UpdatedAt, dbModel.UpdatedBy).
		Suffix(`
			ON CONFLICT (key) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			type = EXCLUDED.type,
			is_active = EXCLUDED.is_active,
			variations = EXCLUDED.variations,
			targeting_rules = EXCLUDED.targeting_rules,
			default_variation_id = EXCLUDED.default_variation_id,
			off_variation_id = EXCLUDED.off_variation_id,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by
		`).ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to upsert feature flag: %w", err)
	}

	return nil
}

func (r *FeatureRepository) Delete(ctx context.Context, key string) error {
	query, args, _ := r.sq.Delete("feature_flags").Where(squirrel.Eq{"key": key}).ToSql()

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete feature flag: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return feature.ErrFlagNotFound
	}
	return nil
}

// --- DB Models & Mappers ---

type dbFeatureFlag struct {
	Key                string    `db:"key"`
	Name               string    `db:"name"`
	Description        string    `db:"description"`
	Type               string    `db:"type"`
	IsActive           bool      `db:"is_active"`
	Variations         string    `db:"variations"`      // JSONB as string
	TargetingRules     string    `db:"targeting_rules"` // JSONB as string
	DefaultVariationID string    `db:"default_variation_id"`
	OffVariationID     string    `db:"off_variation_id"`
	UpdatedAt          time.Time `db:"updated_at"`
	UpdatedBy          string    `db:"updated_by"` // UUID as string if preferred, or uuid.UUID
}

func (d *dbFeatureFlag) toDomain() (*feature.Flag, error) {
	updatedByUUID, err := uuidFromStr(d.UpdatedBy)
	if err != nil {
		return nil, err
	}

	f := &feature.Flag{
		Key:                d.Key,
		Name:               d.Name,
		Description:        d.Description,
		Type:               feature.FlagType(d.Type),
		IsActive:           d.IsActive,
		DefaultVariationID: d.DefaultVariationID,
		OffVariationID:     d.OffVariationID,
		UpdatedAt:          d.UpdatedAt,
		UpdatedBy:          updatedByUUID,
	}

	if err := json.Unmarshal([]byte(d.Variations), &f.Variations); err != nil {
		return nil, fmt.Errorf("failed to unmarshal variations: %w", err)
	}

	if err := json.Unmarshal([]byte(d.TargetingRules), &f.TargetingRules); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rules: %w", err)
	}

	return f, nil
}

func dbFeatureFlagFromDomain(f feature.Flag) (*dbFeatureFlag, error) {
	varsBytes, err := json.Marshal(f.Variations)
	if err != nil {
		return nil, err
	}
	// Fallback for nil slices to ensure empty JSON array in DB
	if f.Variations == nil {
		varsBytes = []byte("[]")
	}

	rulesBytes, err := json.Marshal(f.TargetingRules)
	if err != nil {
		return nil, err
	}
	if f.TargetingRules == nil {
		rulesBytes = []byte("[]")
	}

	return &dbFeatureFlag{
		Key:                f.Key,
		Name:               f.Name,
		Description:        f.Description,
		Type:               string(f.Type),
		IsActive:           f.IsActive,
		Variations:         string(varsBytes),
		TargetingRules:     string(rulesBytes),
		DefaultVariationID: f.DefaultVariationID,
		OffVariationID:     f.OffVariationID,
		UpdatedAt:          f.UpdatedAt,
		UpdatedBy:          f.UpdatedBy.String(),
	}, nil
}

func uuidFromStr(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
