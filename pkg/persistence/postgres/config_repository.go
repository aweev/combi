package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/configuration"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Statically ensure interface compliance.
var _ configuration.Repository = (*ConfigRepository)(nil)

type ConfigRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	baseSq squirrel.StatementBuilderType
}

func NewConfigRepository(db *sqlx.DB, logger log.Logger) *ConfigRepository {
	return &ConfigRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "config_repository")),
		baseSq: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// builder helper
func (r *ConfigRepository) builder() squirrel.StatementBuilderType {
	switch v := r.runner.(type) {
	case *sqlx.Tx:
		return r.baseSq.RunWith(v)
	case *sqlx.DB:
		return r.baseSq.RunWith(v)
	default:
		if runner, ok := r.runner.(squirrel.BaseRunner); ok {
			return r.baseSq.RunWith(runner)
		}
		panic("config_repository: runner does not implement squirrel.BaseRunner")
	}
}

// --- SETTINGS ---

func (r *ConfigRepository) GetSetting(ctx context.Context, key string) (*configuration.SystemSetting, error) {
	query, args, _ := r.builder().Select("*").From("system_settings").Where(squirrel.Eq{"key": key}).ToSql()

	var dbs dbSystemSetting
	if err := sqlx.GetContext(ctx, r.runner, &dbs, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, configuration.ErrSettingNotFound
		}
		return nil, fmt.Errorf("failed to get setting: %w", err)
	}
	return dbs.toDomain()
}

func (r *ConfigRepository) ListSettings(ctx context.Context) ([]configuration.SystemSetting, error) {
	query, args, _ := r.builder().Select("*").From("system_settings").OrderBy("key ASC").ToSql()

	var dbs []dbSystemSetting
	if err := sqlx.SelectContext(ctx, r.runner, &dbs, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list settings: %w", err)
	}

	result := make([]configuration.SystemSetting, len(dbs))
	for i, s := range dbs {
		val, err := s.toDomain()
		if err != nil {
			r.logger.Error("failed to unmarshal setting", err, log.String("key", s.Key))
			continue
		}
		result[i] = *val
	}
	return result, nil
}

func (r *ConfigRepository) UpdateSetting(ctx context.Context, s *configuration.SystemSetting) error {
	s.UpdatedAt = time.Now().UTC()
	dbModel, err := dbSystemSettingFromDomain(s)
	if err != nil {
		return err
	}

	// We use direct ExecContext here with the SQL string because squirrel's Suffix syntax
	// can sometimes be tricky with complex ON CONFLICT clauses.
	query := `
		INSERT INTO system_settings (key, value, type, description, is_public, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			type = EXCLUDED.type,
			description = EXCLUDED.description,
			is_public = EXCLUDED.is_public,
			updated_at = EXCLUDED.updated_at
	`

	if _, err := r.runner.ExecContext(ctx, query,
		dbModel.Key, dbModel.Value, dbModel.Type, dbModel.Description, dbModel.IsPublic, dbModel.UpdatedAt,
	); err != nil {
		return fmt.Errorf("failed to upsert setting: %w", err)
	}
	return nil
}

// --- THEMES ---

func (r *ConfigRepository) UpsertTheme(ctx context.Context, t *configuration.Theme) error {
	dbModel, err := dbThemeFromDomain(t)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO theme_configs (id, name, is_system, variables, active_from, active_until, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			variables = EXCLUDED.variables,
			active_from = EXCLUDED.active_from,
			active_until = EXCLUDED.active_until
	`

	if _, err := r.runner.ExecContext(ctx, query,
		dbModel.ID, dbModel.Name, dbModel.IsSystem, dbModel.Variables, dbModel.ActiveFrom, dbModel.ActiveUntil, dbModel.CreatedAt,
	); err != nil {
		return fmt.Errorf("failed to upsert theme: %w", err)
	}
	return nil
}

func (r *ConfigRepository) GetTheme(ctx context.Context, id uuid.UUID) (*configuration.Theme, error) {
	query, args, _ := r.builder().Select("*").From("theme_configs").Where(squirrel.Eq{"id": id}).ToSql()

	var dbt dbTheme
	if err := sqlx.GetContext(ctx, r.runner, &dbt, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, configuration.ErrThemeNotFound
		}
		return nil, fmt.Errorf("failed to get theme: %w", err)
	}
	return dbt.toDomain()
}

func (r *ConfigRepository) ListThemes(ctx context.Context) ([]configuration.Theme, error) {
	query, args, _ := r.builder().Select("*").From("theme_configs").OrderBy("created_at DESC").ToSql()

	var dbt []dbTheme
	if err := sqlx.SelectContext(ctx, r.runner, &dbt, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list themes: %w", err)
	}

	result := make([]configuration.Theme, len(dbt))
	for i, t := range dbt {
		domain, err := t.toDomain()
		if err != nil {
			continue
		}
		result[i] = *domain
	}
	return result, nil
}

func (r *ConfigRepository) DeleteTheme(ctx context.Context, id uuid.UUID) error {
	res, err := r.builder().Delete("theme_configs").
		Where(squirrel.Eq{"id": id, "is_system": false}).
		ExecContext(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete theme: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return configuration.ErrThemeNotFound
	}
	return nil
}

func (r *ConfigRepository) GetSeasonalTheme(ctx context.Context, now time.Time) (*configuration.Theme, error) {
	// Find themes where NOW is between ActiveFrom and ActiveUntil
	query, args, _ := r.builder().Select("*").
		From("theme_configs").
		Where(squirrel.LtOrEq{"active_from": now}).
		Where(squirrel.GtOrEq{"active_until": now}).
		OrderBy("priority DESC", "created_at DESC").
		Limit(1).
		ToSql()

	var dbt dbTheme
	if err := sqlx.GetContext(ctx, r.runner, &dbt, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return dbt.toDomain()
}

// --- BRANDS ---

func (r *ConfigRepository) GetBrand(ctx context.Context, orgID *uuid.UUID, projectID *uuid.UUID) (*configuration.Brand, error) {
	builder := r.builder().Select("*").From("brands")

	// Specificity Logic:
	// We only return ONE brand based on the tightest scope requested.
	if projectID != nil {
		builder = builder.Where(squirrel.Eq{"project_id": *projectID})
	} else if orgID != nil {
		builder = builder.Where(squirrel.Eq{"organization_id": *orgID})
	} else {
		return nil, configuration.ErrBrandNotFound
	}

	query, args, _ := builder.ToSql()
	var dbb dbBrand
	if err := sqlx.GetContext(ctx, r.runner, &dbb, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, configuration.ErrBrandNotFound
		}
		return nil, err
	}
	return dbb.toDomain()
}

func (r *ConfigRepository) GetSystemBrand(ctx context.Context) (*configuration.Brand, error) {
	query, args, _ := r.builder().Select("*").From("brands").
		Where("organization_id IS NULL").
		Where("project_id IS NULL").
		ToSql()

	var dbb dbBrand
	if err := sqlx.GetContext(ctx, r.runner, &dbb, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, configuration.ErrBrandNotFound
		}
		return nil, err
	}
	return dbb.toDomain()
}

func (r *ConfigRepository) UpsertBrand(ctx context.Context, brand configuration.Brand) error {
	brand.UpdatedAt = time.Now().UTC()
	dbModel, err := dbBrandFromDomain(&brand)
	if err != nil {
		return err
	}

	if brand.ID == uuid.Nil {
		brand.ID = uuid.New()
		dbModel.ID = brand.ID
	}

	query := `
		INSERT INTO brands (id, organization_id, project_id, name, logo_url, favicon_url, theme_id, custom_overrides, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			project_id = EXCLUDED.project_id,
			name = EXCLUDED.name,
			logo_url = EXCLUDED.logo_url,
			favicon_url = EXCLUDED.favicon_url,
			theme_id = EXCLUDED.theme_id,
			custom_overrides = EXCLUDED.custom_overrides,
			updated_at = EXCLUDED.updated_at
	`

	if _, err := r.runner.ExecContext(ctx, query,
		dbModel.ID, dbModel.OrganizationID, dbModel.ProjectID, dbModel.Name, dbModel.LogoURL, dbModel.FaviconURL, dbModel.ThemeID, dbModel.CustomOverrides, dbModel.UpdatedAt,
	); err != nil {
		return fmt.Errorf("failed to upsert brand: %w", err)
	}
	return nil
}

func (r *ConfigRepository) DeleteBrand(ctx context.Context, id uuid.UUID) error {
	res, err := r.builder().Delete("brands").Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete brand: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return configuration.ErrBrandNotFound
	}
	return nil
}

// --- DB MODELS & MAPPERS ---

type dbSystemSetting struct {
	Key         string    `db:"key"`
	Value       string    `db:"value"` // JSON encoded string
	Type        string    `db:"type"`
	Description string    `db:"description"`
	IsPublic    bool      `db:"is_public"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (d *dbSystemSetting) toDomain() (*configuration.SystemSetting, error) {
	var val any
	// Robust Unmarshal: DB value is ALWAYS a JSON string (e.g., "true", "123", "{\"foo\":\"bar\"}")
	if err := json.Unmarshal([]byte(d.Value), &val); err != nil {
		// Fallback: If DB contains raw text that isn't valid JSON (shouldn't happen with proper insertions)
		val = d.Value
	}

	return &configuration.SystemSetting{
		Key:         d.Key,
		Value:       val,
		Type:        configuration.SettingType(d.Type),
		Description: d.Description,
		IsPublic:    d.IsPublic,
		UpdatedAt:   d.UpdatedAt,
	}, nil
}

func dbSystemSettingFromDomain(s *configuration.SystemSetting) (*dbSystemSetting, error) {
	valBytes, err := json.Marshal(s.Value)
	if err != nil {
		return nil, err
	}
	return &dbSystemSetting{
		Key:         s.Key,
		Value:       string(valBytes),
		Type:        string(s.Type),
		Description: s.Description,
		IsPublic:    s.IsPublic,
		UpdatedAt:   s.UpdatedAt,
	}, nil
}

type dbTheme struct {
	ID          uuid.UUID  `db:"id"`
	Name        string     `db:"name"`
	Slug        string     `db:"slug"`
	IsSystem    bool       `db:"is_system"`
	Priority    int        `db:"priority"`
	Variables   []byte     `db:"variables"` // JSONB
	ActiveFrom  *time.Time `db:"active_from"`
	ActiveUntil *time.Time `db:"active_until"`
	CreatedAt   time.Time  `db:"created_at"`
}

func (d *dbTheme) toDomain() (*configuration.Theme, error) {
	var vars map[string]string
	if len(d.Variables) > 0 {
		if err := json.Unmarshal(d.Variables, &vars); err != nil {
			return nil, err
		}
	} else {
		vars = make(map[string]string)
	}

	return &configuration.Theme{
		ID:          d.ID,
		Name:        d.Name,
		IsSystem:    d.IsSystem,
		Variables:   vars,
		ActiveFrom:  d.ActiveFrom,
		ActiveUntil: d.ActiveUntil,
		CreatedAt:   d.CreatedAt,
	}, nil
}

func dbThemeFromDomain(t *configuration.Theme) (*dbTheme, error) {
	varsBytes, err := json.Marshal(t.Variables)
	if err != nil {
		return nil, err
	}

	return &dbTheme{
		ID:          t.ID,
		Name:        t.Name,
		IsSystem:    t.IsSystem,
		Variables:   varsBytes,
		ActiveFrom:  t.ActiveFrom,
		ActiveUntil: t.ActiveUntil,
		CreatedAt:   t.CreatedAt,
	}, nil
}

type dbBrand struct {
	ID              uuid.UUID  `db:"id"`
	OrganizationID  *uuid.UUID `db:"organization_id"`
	ProjectID       *uuid.UUID `db:"project_id"`
	Name            string     `db:"name"`
	LogoURL         string     `db:"logo_url"`
	FaviconURL      string     `db:"favicon_url"`
	ThemeID         uuid.UUID  `db:"theme_id"`
	CustomOverrides []byte     `db:"custom_overrides"` // JSONB
	UpdatedAt       time.Time  `db:"updated_at"`
}

func (d *dbBrand) toDomain() (*configuration.Brand, error) {
	var overrides map[string]string
	if len(d.CustomOverrides) > 0 {
		if err := json.Unmarshal(d.CustomOverrides, &overrides); err != nil {
			return nil, err
		}
	} else {
		overrides = make(map[string]string)
	}

	return &configuration.Brand{
		ID:              d.ID,
		OrganizationID:  d.OrganizationID,
		ProjectID:       d.ProjectID,
		Name:            d.Name,
		LogoURL:         d.LogoURL,
		FaviconURL:      d.FaviconURL,
		ThemeID:         d.ThemeID,
		CustomOverrides: overrides,
		UpdatedAt:       d.UpdatedAt,
	}, nil
}

func dbBrandFromDomain(b *configuration.Brand) (*dbBrand, error) {
	overridesBytes, err := json.Marshal(b.CustomOverrides)
	if err != nil {
		return nil, err
	}
	if b.CustomOverrides == nil {
		overridesBytes = []byte("{}")
	}

	return &dbBrand{
		ID:              b.ID,
		OrganizationID:  b.OrganizationID,
		ProjectID:       b.ProjectID,
		Name:            b.Name,
		LogoURL:         b.LogoURL,
		FaviconURL:      b.FaviconURL,
		ThemeID:         b.ThemeID,
		CustomOverrides: overridesBytes,
		UpdatedAt:       b.UpdatedAt,
	}, nil
}
