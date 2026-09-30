package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NewTestDB starts a PostgreSQL container for testing.
func NewTestDB(t *testing.T) (*sqlx.DB, func()) {
	t.Helper()
	db, _, teardown := NewTestDBWithDSN(t)
	return db, teardown
}

// NewTestDBWithDSN starts a PostgreSQL container, runs migrations, and returns the DB connection, DSN, and a teardown function.
func NewTestDBWithDSN(t *testing.T) (*sqlx.DB, string, func()) {
	t.Helper()
	ctx := context.Background()

	t.Log("Starting PostgreSQL container...")

	// Safely catch any panic from testcontainers (like missing Docker daemon) and skip the test
	defer func() {
		if r := recover(); r != nil {
			t.Skipf("Docker daemon is not running or accessible. Skipping test. Recovered panic: %v", r)
		}
	}()

	// FIX: Replaced deprecated RunContainer and WithImage with postgres.Run
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Skipf("failed to start postgres container (skipping test). Is Docker running and accessible? Error: %s", err)
		return nil, "", func() {}
	}
	t.Log("PostgreSQL container started.")

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %s", err)
	}

	db, err := sqlx.ConnectContext(ctx, "postgres", connStr)
	if err != nil {
		t.Fatalf("failed to connect to test database: %s", err)
	}

	// --- MIGRATION PATH FIX ---
	projectRoot, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("failed to find project root: %s", err)
	}

	// Updated path to match your restructuring
	migrationsPath := filepath.Join(projectRoot, "internal", "persistence", "migrations")

	// Verify path exists to fail fast
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		t.Fatalf("migrations directory not found at: %s", migrationsPath)
	}

	// Use "file://" prefix with absolute path
	sourceURL := fmt.Sprintf("file://%s", migrationsPath)

	t.Logf("Running migrations from: %s", sourceURL)
	m, err := migrate.New(sourceURL, connStr)
	if err != nil {
		t.Fatalf("failed to create migration instance: %s", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("failed to run migrations: %s", err)
	}
	sourceErr, dbErr := m.Close()
	if sourceErr != nil {
		t.Logf("Warning: error closing migration source: %s", sourceErr)
	}
	if dbErr != nil {
		t.Logf("Warning: error closing migration db connection: %s", dbErr)
	}

	t.Log("Migrations applied successfully.")

	teardown := func() {
		t.Log("Shutting down test database container...")
		_ = db.Close()
		teardownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := pgContainer.Terminate(teardownCtx); err != nil {
			t.Logf("failed to terminate postgres container gracefully: %s", err)
		}
		t.Log("Test database container shut down.")
	}

	return db, connStr, teardown
}

// FindProjectRoot travels up from the current working directory until it finds a go.mod file.
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir { // Reached the filesystem root
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
