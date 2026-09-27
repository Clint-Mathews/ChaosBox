package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql seeds/*.sql
var sqlFiles embed.FS

type DB struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

func (db *DB) Prepare(ctx context.Context) error {
	if _, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	migrations, err := fs.Glob(sqlFiles, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	for _, path := range migrations {
		if err := db.applyMigration(ctx, path); err != nil {
			return err
		}
	}

	seeds, err := fs.Glob(sqlFiles, "seeds/*.sql")
	if err != nil {
		return fmt.Errorf("list seeds: %w", err)
	}
	for _, path := range seeds {
		contents, err := sqlFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read seed %s: %w", path, err)
		}
		if _, err := db.pool.Exec(ctx, string(contents)); err != nil {
			return fmt.Errorf("apply seed %s: %w", path, err)
		}
	}

	return nil
}

func (db *DB) applyMigration(ctx context.Context, path string) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", path, err)
	}
	defer tx.Rollback(ctx)

	version := filepath.Base(path)
	var applied bool
	if err := tx.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)",
		version,
	).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %s: %w", version, err)
	}
	if applied {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration check %s: %w", version, err)
		}
		return nil
	}

	contents, err := sqlFiles.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", path, err)
	}
	if _, err := tx.Exec(ctx, string(contents)); err != nil {
		return fmt.Errorf("apply migration %s: %w", version, err)
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO schema_migrations (version) VALUES ($1)",
		version,
	); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}

	return nil
}
