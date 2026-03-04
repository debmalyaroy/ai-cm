package database

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var newPoolWithConfig = pgxpool.NewWithConfig

// poolPinger allows us to mock the Ping method for testing
var pingPool = func(ctx context.Context, pool *pgxpool.Pool) error {
	return pool.Ping(ctx)
}

// closePool allows us to mock the Close method for testing
var closePool = func(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}

// NewPool creates a new PostgreSQL connection pool.
func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://aicm:aicm_secret@localhost:5432/aicm?sslmode=disable"
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	config.MaxConns = 20
	config.MinConns = 2

	pool, err := newPoolWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	// Verify connection
	if err := pingPool(ctx, pool); err != nil {
		closePool(pool)
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
