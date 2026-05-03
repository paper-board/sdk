// Package testfixture provides Postgres test infrastructure for paper-board
// services: a process-shared testcontainers Postgres, schema-bootstrap, and
// per-test truncation.
//
// Lifecycle: PostgresContainer spins one container per test process via
// sync.Once. The container terminates on process exit; testcontainers' Reaper
// reaps any leftovers. Per-test isolation is the caller's responsibility —
// invoke Truncate(t, pool, schema) after each test.
//
// Build tag — package compiles unconditionally; tests using it must be tagged
// `//go:build integration` and run as `go test -tags=integration ./...`.
package testfixture

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

var (
	once       sync.Once
	sharedPool *pgxpool.Pool
	sharedURL  string
	initErr    error
)

// PostgresContainer returns a *pgxpool.Pool connected to a process-shared
// Postgres container. The container installs cluster-wide extensions
// (pgcrypto + citext) on first call, and creates the requested schema.
// Subsequent calls within the same test process reuse the pool.
func PostgresContainer(t *testing.T, schema string) *pgxpool.Pool {
	t.Helper()
	once.Do(func() { initErr = bootContainer() })
	if initErr != nil {
		t.Fatalf("testfixture postgres boot: %v", initErr)
	}
	if err := ensureSchema(context.Background(), sharedPool, schema); err != nil {
		t.Fatalf("testfixture ensure schema %q: %v", schema, err)
	}
	return sharedPool
}

// ConnectionString returns the URL of the shared container. Returns "" when
// PostgresContainer has not been called yet.
func ConnectionString() string { return sharedURL }

func bootContainer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		return fmt.Errorf("run postgres: %w", err)
	}

	url, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pg.Terminate(context.Background())
		return fmt.Errorf("connection string: %w", err)
	}

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		_ = pg.Terminate(context.Background())
		return fmt.Errorf("pool: %w", err)
	}

	if _, err := pool.Exec(ctx, `
		CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;
		CREATE EXTENSION IF NOT EXISTS citext   WITH SCHEMA public;
	`); err != nil {
		_ = pg.Terminate(context.Background())
		return fmt.Errorf("install extensions: %w", err)
	}

	sharedURL = url
	sharedPool = pool
	return nil
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema))
	return err
}
