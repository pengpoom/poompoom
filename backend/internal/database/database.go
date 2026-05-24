package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"imagestudio/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

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
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
	if err != nil {
		return nil, err
	}

	applyPoolSettings(db, cfg)
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
