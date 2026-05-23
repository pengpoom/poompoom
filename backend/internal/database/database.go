package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"imagestudio/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

const sqliteBusyTimeoutMS = 5000

func Open(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	if driver == "" {
		driver = "postgres"
	}
	dsn := strings.TrimSpace(cfg.Database.DSN)
	if dsn == "" {
		return nil, fmt.Errorf("database dsn is required")
	}

	var (
		db  *sql.DB
		err error
	)
	switch driver {
	case "postgres":
		db, err = sql.Open("pgx", dsn)
	case "sqlite":
		path := cfg.ResolvePath(dsn)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		db, err = sql.Open("sqlite", path)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
	if err != nil {
		return nil, err
	}

	applyPoolSettings(db, cfg)
	if driver == "sqlite" {
		if err := configureSQLite(db); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func applyPoolSettings(db *sql.DB, cfg *config.Config) {
	maxOpen := cfg.Database.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 20
	}
	maxIdle := cfg.Database.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = min(maxOpen, 10)
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetimeSeconds) * time.Second)
}

func configureSQLite(db *sql.DB) error {
	pragmas := []string{
		fmt.Sprintf("PRAGMA busy_timeout = %d;", sqliteBusyTimeoutMS),
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA foreign_keys = ON;",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return err
		}
	}
	return nil
}

func IsPostgres(driver string) bool {
	return strings.EqualFold(strings.TrimSpace(driver), "postgres")
}

func Rebind(driver string, query string) string {
	if !IsPostgres(driver) {
		return query
	}
	var builder strings.Builder
	index := 1
	for _, r := range query {
		if r == '?' {
			builder.WriteString(fmt.Sprintf("$%d", index))
			index++
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}
