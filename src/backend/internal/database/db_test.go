package database

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewPool_DefaultDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := NewPool(context.Background())
	if err == nil {
		t.Log("NewPool succeeded (database available)")
	} else {
		t.Logf("NewPool failed as expected (no database): %v", err)
	}
}

func TestNewPool_CustomDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/testdb?sslmode=disable")
	_, err := NewPool(context.Background())
	if err == nil {
		t.Log("NewPool succeeded with custom DSN")
	} else {
		t.Logf("NewPool failed as expected: %v", err)
	}
}

func TestNewPool_InvalidDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "not-a-valid-dsn")
	_, err := NewPool(context.Background())
	if err == nil {
		t.Fatal("should fail with invalid DSN")
	}
}

func TestNewPool_NewWithConfigError(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/testdb?sslmode=disable")

	orig := newPoolWithConfig
	newPoolWithConfig = func(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, error) {
		return nil, context.DeadlineExceeded
	}
	defer func() { newPoolWithConfig = orig }()

	_, err := NewPool(context.Background())
	if err == nil {
		t.Fatal("should fail when newPoolWithConfig fails")
	}
}

func TestNewPool_PingError(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/testdb?sslmode=disable")

	// Mock newPoolWithConfig to return a dummy pool successfully
	origNewPool := newPoolWithConfig
	newPoolWithConfig = func(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, error) {
		// Just returning nil pool since pingPool will be mocked anyway and won't use it
		return nil, nil
	}
	defer func() { newPoolWithConfig = origNewPool }()

	// Mock pingPool to return an error
	origPing := pingPool
	pingPool = func(ctx context.Context, pool *pgxpool.Pool) error {
		return context.DeadlineExceeded
	}
	defer func() { pingPool = origPing }()

	_, err := NewPool(context.Background())
	if err == nil {
		t.Fatal("should fail when pingPool fails")
	}
}

func TestNewPool_Success(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/testdb?sslmode=disable")

	// Mock newPoolWithConfig to return a dummy pool successfully
	origNewPool := newPoolWithConfig
	newPoolWithConfig = func(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, error) {
		return &pgxpool.Pool{}, nil
	}
	defer func() { newPoolWithConfig = origNewPool }()

	// Mock pingPool to succeed
	origPing := pingPool
	pingPool = func(ctx context.Context, pool *pgxpool.Pool) error {
		return nil
	}
	defer func() { pingPool = origPing }()

	pool, err := NewPool(context.Background())
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
}
