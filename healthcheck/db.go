package healthcheck

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type dbChecker struct {
	pool *pgxpool.Pool
}

// DB returns a Checker that runs `SELECT 1` against the given pgx pool.
// Use this for /readyz to verify the database is reachable through the
// pool used by request handlers (e.g., pgbouncer in production).
func DB(pool *pgxpool.Pool) Checker {
	return &dbChecker{pool: pool}
}

func (d *dbChecker) Name() string { return "db" }

func (d *dbChecker) Check(ctx context.Context) error {
	var one int
	if err := d.pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}
	return nil
}
