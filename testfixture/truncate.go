package testfixture

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Truncate issues TRUNCATE ... RESTART IDENTITY CASCADE on every base table
// within the given schemas. Cheaper than DROP+recreate, preserves indexes,
// resets sequence counters. Skips schemas with no tables.
//
// Call after each test (or in t.Cleanup) to give every test a clean slate
// against the shared container.
func Truncate(t *testing.T, pool *pgxpool.Pool, schemas ...string) {
	t.Helper()
	ctx := context.Background()
	for _, schema := range schemas {
		rows, err := pool.Query(ctx,
			`SELECT tablename FROM pg_tables WHERE schemaname = $1`,
			schema,
		)
		if err != nil {
			t.Fatalf("testfixture: list tables in %q: %v", schema, err)
		}
		var tables []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				t.Fatalf("testfixture: scan: %v", err)
			}
			tables = append(tables, schema+"."+name)
		}
		rows.Close()
		if len(tables) == 0 {
			continue
		}
		sql := "TRUNCATE " + strings.Join(tables, ", ") + " RESTART IDENTITY CASCADE"
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatalf("testfixture: truncate %q: %v", schema, err)
		}
	}
}
