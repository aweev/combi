// internal/persistence/postgres/localization_repository.go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/localization"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ localization.Repository = (*LocalizationRepository)(nil)

type LocalizationRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewLocalizationRepository(db *sqlx.DB, logger log.Logger) *LocalizationRepository {
	return &LocalizationRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "localization_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// --- Locale Management ---

func (r *LocalizationRepository) GetLocales(ctx context.Context) ([]localization.Locale, error) {
	query, args, _ := r.sq.Select("*").From("locales").OrderBy("code ASC").ToSql()

	var dbl []dbLocale
	if err := sqlx.SelectContext(ctx, r.runner, &dbl, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list locales: %w", err)
	}

	result := make([]localization.Locale, len(dbl))
	for i, l := range dbl {
		result[i] = *l.toDomain()
	}
	return result, nil
}

func (r *LocalizationRepository) GetLocale(ctx context.Context, code localization.LocaleCode) (*localization.Locale, error) {
	query, args, _ := r.sq.Select("*").From("locales").Where(squirrel.Eq{"code": code}).ToSql()

	var dbl dbLocale
	if err := sqlx.GetContext(ctx, r.runner, &dbl, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, localization.ErrLocaleNotFound
		}
		return nil, err
	}
	return dbl.toDomain(), nil
}

// --- Key Management ---

func (r *LocalizationRepository) CreateKey(ctx context.Context, key *localization.TranslationKey) error {
	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}
	key.CreatedAt = time.Now().UTC()

	dbModel := dbKeyFromDomain(key)

	query, args, err := r.sq.Insert("translation_keys").
		Columns("id", "namespace", "key", "description", "created_at").
		Values(dbModel.ID, dbModel.Namespace, dbModel.Key, dbModel.Description, dbModel.CreatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return localization.ErrDuplicateKey
		}
		return err
	}
	return nil
}

func (r *LocalizationRepository) GetKey(ctx context.Context, namespace, key string) (*localization.TranslationKey, error) {
	query, args, _ := r.sq.Select("*").From("translation_keys").
		Where(squirrel.Eq{"namespace": namespace, "key": key}).ToSql()

	var dbk dbTranslationKey
	if err := sqlx.GetContext(ctx, r.runner, &dbk, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, localization.ErrKeyNotFound
		}
		return nil, err
	}
	return dbk.toDomain(), nil
}

func (r *LocalizationRepository) ListKeys(ctx context.Context, namespace string, limit, offset int) ([]localization.TranslationKey, int, error) {
	// 1. Get total count
	countQuery, countArgs, _ := r.sq.Select("COUNT(*)").From("translation_keys").Where(squirrel.Eq{"namespace": namespace}).ToSql()
	var total int
	if err := r.runner.QueryRowxContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 2. Get data
	query, args, _ := r.sq.Select("*").From("translation_keys").
		Where(squirrel.Eq{"namespace": namespace}).
		OrderBy("key ASC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).
		ToSql()

	var dbks []dbTranslationKey
	if err := sqlx.SelectContext(ctx, r.runner, &dbks, query, args...); err != nil {
		return nil, 0, err
	}

	result := make([]localization.TranslationKey, len(dbks))
	for i, k := range dbks {
		result[i] = *k.toDomain()
	}
	return result, total, nil
}

func (r *LocalizationRepository) DeleteKey(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("translation_keys").Where(squirrel.Eq{"id": id}).ToSql()

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return localization.ErrKeyNotFound
	}
	return nil
}

// --- Value Management ---

func (r *LocalizationRepository) UpsertValue(ctx context.Context, val *localization.TranslationValue) error {
	val.UpdatedAt = time.Now().UTC()
	dbModel := dbValueFromDomain(val)

	query, args, err := r.sq.Insert("translation_values").
		Columns("key_id", "locale", "value", "is_verified", "updated_at").
		Values(dbModel.KeyID, dbModel.Locale, dbModel.Value, dbModel.IsVerified, dbModel.UpdatedAt).
		Suffix(`
			ON CONFLICT (key_id, locale) DO UPDATE SET
			value = EXCLUDED.value,
			is_verified = EXCLUDED.is_verified,
			updated_at = EXCLUDED.updated_at
		`).ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		// Check foreign key constraint for locale
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return localization.ErrLocaleNotFound
		}
		return err
	}
	return nil
}

func (r *LocalizationRepository) GetValue(ctx context.Context, keyID uuid.UUID, locale localization.LocaleCode) (*localization.TranslationValue, error) {
	query, args, _ := r.sq.Select("*").From("translation_values").
		Where(squirrel.Eq{"key_id": keyID, "locale": locale}).ToSql()

	var dbv dbTranslationValue
	if err := sqlx.GetContext(ctx, r.runner, &dbv, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Instead of error, we return nil (missing translation)
			return nil, nil
		}
		return nil, err
	}
	return dbv.toDomain(), nil
}

// GetValuesForNamespace implements the high-performance bundle fetch.
// Returns a map of "key" -> "value".
func (r *LocalizationRepository) GetValuesForNamespace(ctx context.Context, locale localization.LocaleCode, namespace string) (map[string]string, error) {
	query, args, _ := r.sq.Select("k.key", "v.value").
		From("translation_keys k").
		Join("translation_values v ON k.id = v.key_id").
		Where(squirrel.Eq{
			"k.namespace": namespace,
			"v.locale":    locale,
		}).ToSql()

	rows, err := r.runner.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err != nil {
			continue
		}
		result[key] = val
	}
	return result, nil
}

func (r *LocalizationRepository) GetKeysMissingValue(ctx context.Context, locale localization.LocaleCode) ([]localization.TranslationKey, error) {
	// Find keys that do NOT exist in values table for this locale
	subQuery, subArgs, _ := r.sq.Select("1").From("translation_values v").
		Where("v.key_id = k.id").
		Where(squirrel.Eq{"v.locale": locale}).ToSql()

	query, args, _ := r.sq.Select("*").From("translation_keys k").
		Where(fmt.Sprintf("NOT EXISTS (%s)", subQuery), subArgs...).ToSql()

	var dbks []dbTranslationKey
	if err := sqlx.SelectContext(ctx, r.runner, &dbks, query, args...); err != nil {
		return nil, err
	}

	result := make([]localization.TranslationKey, len(dbks))
	for i, k := range dbks {
		result[i] = *k.toDomain()
	}
	return result, nil
}

// --- DB Models ---

type dbLocale struct {
	Code       string    `db:"code"`
	Name       string    `db:"name"`
	IsDefault  bool      `db:"is_default"`
	IsActive   bool      `db:"is_active"`
	Direction  string    `db:"direction"`
	DateFormat string    `db:"date_format"`
	CreatedAt  time.Time `db:"created_at"`
}

func (d *dbLocale) toDomain() *localization.Locale {
	return &localization.Locale{
		Code:       localization.LocaleCode(d.Code),
		Name:       d.Name,
		IsDefault:  d.IsDefault,
		IsActive:   d.IsActive,
		Direction:  d.Direction,
		DateFormat: d.DateFormat,
		CreatedAt:  d.CreatedAt,
	}
}

type dbTranslationKey struct {
	ID          uuid.UUID `db:"id"`
	Namespace   string    `db:"namespace"`
	Key         string    `db:"key"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
}

func (d *dbTranslationKey) toDomain() *localization.TranslationKey {
	return &localization.TranslationKey{
		ID:          d.ID,
		Namespace:   d.Namespace,
		Key:         d.Key,
		Description: d.Description,
		CreatedAt:   d.CreatedAt,
	}
}

func dbKeyFromDomain(k *localization.TranslationKey) *dbTranslationKey {
	return &dbTranslationKey{
		ID:          k.ID,
		Namespace:   k.Namespace,
		Key:         k.Key,
		Description: k.Description,
		CreatedAt:   k.CreatedAt,
	}
}

type dbTranslationValue struct {
	KeyID      uuid.UUID `db:"key_id"`
	Locale     string    `db:"locale"`
	Value      string    `db:"value"`
	IsVerified bool      `db:"is_verified"`
	UpdatedAt  time.Time `db:"updated_at"`
}

func (d *dbTranslationValue) toDomain() *localization.TranslationValue {
	return &localization.TranslationValue{
		KeyID:      d.KeyID,
		Locale:     localization.LocaleCode(d.Locale),
		Value:      d.Value,
		IsVerified: d.IsVerified,
		UpdatedAt:  d.UpdatedAt,
	}
}

func dbValueFromDomain(v *localization.TranslationValue) *dbTranslationValue {
	return &dbTranslationValue{
		KeyID:      v.KeyID,
		Locale:     string(v.Locale),
		Value:      v.Value,
		IsVerified: v.IsVerified,
		UpdatedAt:  v.UpdatedAt,
	}
}
