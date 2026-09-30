package postgres

import (
	"fmt"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/log"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// NewConnection opens the shared platform Postgres connection pool.
//
// Consolidated here (pkg/persistence/postgres) in the F1 chassis cleanup —
// the former pkg/database package was deleted as a duplicate entry point.
// Behaviour is unchanged: schema-per-service search_path, pool settings
// from config, and a post-connect ping.
func NewConnection(cfg config.PostgresConfig, logger log.Logger) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	// Dynamic schema-per-service isolation configuration
	_, err = db.Exec(`SET search_path TO "user", auth, audit, secret, queue, rbac, integration, org, notification, feature, storage, localization, config, consent, tagging, catalog, event, content, form, monitoring, analytics, search, financial, workflow, project, reporting, matchmaking, messaging, achievement, impact, automation, sync, public;`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to configure search_path: %w", err)
	}

	// Apply connection pool settings from config
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	logger.Info("PostgreSQL connection pool established", log.String("host", cfg.Host), log.Int("port", cfg.Port))
	return db, nil
}