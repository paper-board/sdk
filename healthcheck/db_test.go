//go:build integration

package healthcheck_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/paper-board/sdk/healthcheck"
	"github.com/paper-board/sdk/testfixture"
)

func TestDBChecker_Live(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use the shared testfixture container; we ignore the returned pool
	// because we need a dedicated pool whose lifecycle we control (closing
	// it must produce a failed Check without affecting the shared one).
	_ = testfixture.PostgresContainer(t, "healthcheck_test")
	dsn := testfixture.ConnectionString()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}

	c := healthcheck.DB(pool)
	if err := c.Check(ctx); err != nil {
		t.Fatalf("expected ok, got: %v", err)
	}

	pool.Close()
	if err := c.Check(ctx); err == nil {
		t.Fatal("expected error after pool close, got nil")
	}
}
