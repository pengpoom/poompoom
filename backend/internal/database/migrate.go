package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/postgres/*.sql
var postgresMigrations embed.FS

const postgresMigrationLockKey int64 = 9128674217001

type Migration struct {
	Version string
	SQL     string
}

func Migrate(ctx context.Context, db *sql.DB, driver string) error {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver != "postgres" {
		return nil
	}
	if err := lockPostgresMigrations(ctx, db); err != nil {
		return err
	}
	defer unlockPostgresMigrations(context.Background(), db)

	if err := ensureMigrationTable(ctx, db); err != nil {
		return err
	}
	migrations, err := loadPostgresMigrations()
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		if err := applyMigration(ctx, db, migration); err != nil {
			return err
		}
	}
	return nil
}

func lockPostgresMigrations(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, postgresMigrationLockKey)
	return err
}

func unlockPostgresMigrations(ctx context.Context, db *sql.DB) {
	_, _ = db.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, postgresMigrationLockKey)
}

func ensureMigrationTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL
		);
	`)
	return err
}

func loadPostgresMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(postgresMigrations, "migrations/postgres")
	if err != nil {
		return nil, err
	}
	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		content, err := postgresMigrations.ReadFile(path.Join("migrations/postgres", entry.Name()))
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, Migration{
			Version: strings.TrimSuffix(entry.Name(), ".sql"),
			SQL:     string(content),
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

func applyMigration(ctx context.Context, db *sql.DB, migration Migration) error {
	var applied string
	err := db.QueryRowContext(ctx, `SELECT version FROM schema_migrations WHERE version = $1`, migration.Version).Scan(&applied)
	if err == nil {
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("apply migration %s: %w", migration.Version, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES($1, $2)`, migration.Version, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}
