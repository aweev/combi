package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/combi/pkg/domain/secret"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var _ secret.Repository = (*SecretRepository)(nil)

type SecretRepository struct {
	db             *sqlx.DB
	runner         sqlx.ExtContext
	squirrelRunner squirrel.BaseRunner
}

func NewSecretRepository(db *sqlx.DB) *SecretRepository {
	return &SecretRepository{
		db:             db,
		runner:         db,
		squirrelRunner: db,
	}
}

func (r *SecretRepository) builder() squirrel.StatementBuilderType {
	return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).RunWith(r.squirrelRunner)
}

// Get retrieves a secret by name.
func (r *SecretRepository) Get(ctx context.Context, name string) (*secret.Secret, error) {
	var dbSec dbSecret
	query, args, _ := r.builder().Select("*").From("secrets").Where(squirrel.Eq{"name": name}).ToSql()

	if err := sqlx.GetContext(ctx, r.runner, &dbSec, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, secret.ErrSecretNotFound
		}
		return nil, &secret.BackendError{Err: err, Backend: "postgres", Name: name}
	}
	return dbSec.ToDomain()
}

// Set creates or updates a secret.
func (r *SecretRepository) Set(ctx context.Context, s *secret.Secret) error {
	policyBytes, _ := json.Marshal(s.RotationPolicy)
	metadataBytes, _ := json.Marshal(s.Metadata)

	// Upsert logic (Insert or Update on conflict)
	query := `
		INSERT INTO secrets (
			id, name, encrypted_value, encryption_context, version, status,
			created_at, updated_at, rotation_policy, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (name) DO UPDATE SET
			encrypted_value = EXCLUDED.encrypted_value,
			encryption_context = EXCLUDED.encryption_context,
			version = EXCLUDED.version,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at,
			rotation_policy = EXCLUDED.rotation_policy,
			metadata = EXCLUDED.metadata
	`

	_, err := r.runner.ExecContext(ctx, query,
		s.ID, s.Name, s.EncryptedValue, s.EncryptionContext, s.Version, s.Status,
		s.CreatedAt, s.UpdatedAt, policyBytes, metadataBytes,
	)

	if err != nil {
		return &secret.BackendError{Err: err, Backend: "postgres", Name: s.Name}
	}
	return nil
}

// List returns names matching a prefix.
func (r *SecretRepository) List(ctx context.Context, prefix string) ([]string, error) {
	var names []string
	query, args, _ := r.builder().Select("name").From("secrets").
		Where("name LIKE ?", prefix+"%").
		OrderBy("name ASC").ToSql()

	if err := sqlx.SelectContext(ctx, r.runner, &names, query, args...); err != nil {
		return nil, &secret.BackendError{Err: err, Backend: "postgres", Name: "list"}
	}
	return names, nil
}

// Delete removes a secret.
func (r *SecretRepository) Delete(ctx context.Context, name string) error {
	res, err := r.builder().Delete("secrets").Where(squirrel.Eq{"name": name}).ExecContext(ctx)
	if err != nil {
		return &secret.BackendError{Err: err, Backend: "postgres", Name: name}
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return secret.ErrSecretNotFound
	}
	return nil
}

// FindSecretsDueForRotation uses JSONB queries to find secrets where rotation is enabled and due.
func (r *SecretRepository) FindSecretsDueForRotation(ctx context.Context, now time.Time) ([]*secret.Secret, error) {
	// Postgres JSONB query: rotation_policy->'enabled' is true AND rotation_policy->'next_rotation_at' < now
	query := `
		SELECT * FROM secrets 
		WHERE status = 'ACTIVE'
		AND (rotation_policy->>'enabled')::boolean = true 
		AND (rotation_policy->>'next_rotation_at')::timestamptz < $1
	`

	var dbSecs []dbSecret
	if err := sqlx.SelectContext(ctx, r.runner, &dbSecs, query, now); err != nil {
		return nil, &secret.BackendError{Err: err, Backend: "postgres", Name: "rotation_check"}
	}

	secrets := make([]*secret.Secret, len(dbSecs))
	for i, dbs := range dbSecs {
		s, err := dbs.ToDomain()
		if err != nil {
			return nil, err
		}
		secrets[i] = s
	}
	return secrets, nil
}

// --- DB Models ---

type dbSecret struct {
	ID                uuid.UUID           `db:"id"`
	Name              string              `db:"name"`
	EncryptedValue    []byte              `db:"encrypted_value"`
	EncryptionContext []byte              `db:"encryption_context"`
	Version           int                 `db:"version"`
	Status            secret.SecretStatus `db:"status"`
	CreatedAt         time.Time           `db:"created_at"`
	UpdatedAt         time.Time           `db:"updated_at"`
	RotationPolicy    []byte              `db:"rotation_policy"` // JSONB
	Metadata          []byte              `db:"metadata"`        // JSONB
}

func (d *dbSecret) ToDomain() (*secret.Secret, error) {
	var policy secret.RotationPolicy
	if len(d.RotationPolicy) > 0 {
		if err := json.Unmarshal(d.RotationPolicy, &policy); err != nil {
			return nil, err
		}
	}
	var meta map[string]string
	if len(d.Metadata) > 0 {
		if err := json.Unmarshal(d.Metadata, &meta); err != nil {
			return nil, err
		}
	}

	return &secret.Secret{
		ID:                d.ID,
		Name:              d.Name,
		EncryptedValue:    d.EncryptedValue,
		EncryptionContext: d.EncryptionContext,
		Version:           d.Version,
		Status:            d.Status,
		CreatedAt:         d.CreatedAt,
		UpdatedAt:         d.UpdatedAt,
		RotationPolicy:    &policy,
		Metadata:          meta,
	}, nil
}

func (r *SecretRepository) GetVersion(ctx context.Context, name string, version int) (*secret.Secret, error) {
	var dbSec dbSecret
	// We enforce both name AND version match
	query, args, _ := r.builder().Select("*").From("secrets").
		Where(squirrel.Eq{"name": name, "version": version}).ToSql()

	if err := sqlx.GetContext(ctx, r.runner, &dbSec, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, secret.ErrSecretNotFound
		}
		return nil, &secret.BackendError{Err: err, Backend: "postgres", Name: name}
	}
	return dbSec.ToDomain()
}
