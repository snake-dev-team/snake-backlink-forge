// Package migrator wraps goose v3 to run embedded SQL migrations against a pgxpool.
// Migrations are embedded at compile time via internal/migrations.FS, so no external
// migration directory is needed at runtime.
package migrator

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/kekuta/snake-backlink-forge/services/api/internal/migrations"
)

const migrationsDir = "."

func newDB(pool *pgxpool.Pool) *sql.DB {
	// stdlib.OpenDBFromPool wraps the pgxpool in a database/sql adapter that
	// goose requires; it does not create new connections — it reuses the pool.
	return stdlib.OpenDBFromPool(pool)
}

func setup() error {
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("migrator: set dialect: %w", err)
	}
	return nil
}

// Up applies all pending migrations.
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	if err := setup(); err != nil {
		return err
	}
	db := newDB(pool)
	defer db.Close() //nolint:errcheck // stdlib wrapper; close is best-effort
	if err := goose.UpContext(ctx, db, migrationsDir); err != nil {
		return fmt.Errorf("migrator: up: %w", err)
	}
	return nil
}

// Down rolls back the last applied migration.
func Down(ctx context.Context, pool *pgxpool.Pool) error {
	if err := setup(); err != nil {
		return err
	}
	db := newDB(pool)
	defer db.Close() //nolint:errcheck
	if err := goose.DownContext(ctx, db, migrationsDir); err != nil {
		return fmt.Errorf("migrator: down: %w", err)
	}
	return nil
}

// Status prints the current migration status to stdout (goose handles formatting).
func Status(ctx context.Context, pool *pgxpool.Pool) error {
	if err := setup(); err != nil {
		return err
	}
	db := newDB(pool)
	defer db.Close() //nolint:errcheck
	if err := goose.StatusContext(ctx, db, migrationsDir); err != nil {
		return fmt.Errorf("migrator: status: %w", err)
	}
	return nil
}

// DownTo rolls back all migrations down to (but not including) the given version.
// Pass 0 to roll back everything.
func DownTo(ctx context.Context, pool *pgxpool.Pool, version int64) error {
	if err := setup(); err != nil {
		return err
	}
	db := newDB(pool)
	defer db.Close() //nolint:errcheck
	if err := goose.DownToContext(ctx, db, migrationsDir, version); err != nil {
		return fmt.Errorf("migrator: down-to %d: %w", version, err)
	}
	return nil
}
