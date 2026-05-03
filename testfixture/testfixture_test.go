//go:build integration

package testfixture_test

import (
	"context"
	"embed"
	"testing"

	"github.com/paper-board/sdk/testfixture"
)

//go:embed testdata/*.up.sql
var schemaFS embed.FS

func TestPostgresContainerLoadAndTruncate(t *testing.T) {
	pool := testfixture.PostgresContainer(t, "fixtures")
	testfixture.LoadSchema(t, pool, schemaFS, "testdata")

	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO fixtures.widgets (name) VALUES ('a'), ('b'), ('c')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM fixtures.widgets`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 3 {
		t.Fatalf("after insert: %d, want 3", n)
	}

	testfixture.Truncate(t, pool, "fixtures")
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM fixtures.widgets`).Scan(&n); err != nil {
		t.Fatalf("count2: %v", err)
	}
	if n != 0 {
		t.Fatalf("after truncate: %d, want 0", n)
	}
}

func TestPostgresContainerReusesPool(t *testing.T) {
	a := testfixture.PostgresContainer(t, "fixtures")
	b := testfixture.PostgresContainer(t, "fixtures")
	if a != b {
		t.Fatal("subsequent calls should return the same pool")
	}
	if testfixture.ConnectionString() == "" {
		t.Fatal("ConnectionString should be set after first call")
	}
}

func TestTruncateEmptyNoOp(t *testing.T) {
	pool := testfixture.PostgresContainer(t, "fixtures_empty")
	testfixture.Truncate(t, pool, "fixtures_empty")
}
