//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	pbstore "github.com/paper-board/sdk/store"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	url, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("conn string: %v", err)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `CREATE TABLE counters (n int NOT NULL DEFAULT 0)`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO counters (n) VALUES (0)`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return pool
}

func TestWithTxCommit(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()

	got, err := pbstore.WithTx(ctx, pool, func(tx pgx.Tx) (int, error) {
		_, err := tx.Exec(ctx, `UPDATE counters SET n = n + 1`)
		return 42, err
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got != 42 {
		t.Fatalf("got %d", got)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT n FROM counters`).Scan(&n); err != nil {
		t.Fatalf("read: %v", err)
	}
	if n != 1 {
		t.Fatalf("counter: %d", n)
	}
}

func TestWithTxRollbackOnError(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	wantErr := errors.New("boom")

	_, err := pbstore.WithTx(ctx, pool, func(tx pgx.Tx) (int, error) {
		_, _ = tx.Exec(ctx, `UPDATE counters SET n = n + 99`)
		return 0, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT n FROM counters`).Scan(&n); err != nil {
		t.Fatalf("read: %v", err)
	}
	if n != 0 {
		t.Fatalf("rollback failed, counter=%d", n)
	}
}

func TestWithTxRollbackOnCtxCancel(t *testing.T) {
	pool := newPool(t)
	ctx, cancel := context.WithCancel(context.Background())

	_, err := pbstore.WithTx(ctx, pool, func(tx pgx.Tx) (int, error) {
		_, _ = tx.Exec(ctx, `UPDATE counters SET n = n + 7`)
		cancel()
		return 0, ctx.Err()
	})
	if err == nil {
		t.Fatal("expected ctx err")
	}
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT n FROM counters`).Scan(&n); err != nil {
		t.Fatalf("read: %v", err)
	}
	if n != 0 {
		t.Fatalf("counter: %d (rollback should leave 0)", n)
	}
}
