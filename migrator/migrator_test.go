package migrator_test

import (
	"context"
	"embed"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/paper-board/sdk/migrator"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

//go:embed testdata/schema/*.sql
var testSchemaFS embed.FS

func setupPostgres(t *testing.T) (string, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	url, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	return url, func() {
		_ = pg.Terminate(context.Background())
	}
}

func TestRun_MissingDBURL(t *testing.T) {
	cfg := migrator.Config{
		Schema:         "test_schema",
		AdvisoryLockID: 99,
		EmbedFS:        testSchemaFS,
		EmbedRoot:      "testdata/schema",
	}
	err := migrator.Run(context.Background(), cfg, []string{"version"})
	if err == nil {
		t.Fatal("expected error for missing DBURL, got nil")
	}
}

func TestRun_UpEmptyDB(t *testing.T) {
	url, cleanup := setupPostgres(t)
	defer cleanup()

	cfg := migrator.Config{
		DBURL:          url,
		Schema:         "test_schema",
		AdvisoryLockID: 99,
		EmbedFS:        testSchemaFS,
		EmbedRoot:      "testdata/schema",
	}
	if err := migrator.Run(context.Background(), cfg, []string{"up"}); err != nil {
		t.Fatalf("migrator up failed: %v", err)
	}
}

func TestRun_DownRoundTrip(t *testing.T) {
	url, cleanup := setupPostgres(t)
	defer cleanup()

	cfg := migrator.Config{
		DBURL:          url,
		Schema:         "test_schema",
		AdvisoryLockID: 99,
		EmbedFS:        testSchemaFS,
		EmbedRoot:      "testdata/schema",
	}
	ctx := context.Background()

	if err := migrator.Run(ctx, cfg, []string{"up"}); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := migrator.Run(ctx, cfg, []string{"down", "1"}); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := migrator.Run(ctx, cfg, []string{"up"}); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

func TestRun_DropRequiresDevEnv(t *testing.T) {
	url, cleanup := setupPostgres(t)
	defer cleanup()

	t.Setenv("MIGRATOR_ENV", "prod")
	cfg := migrator.Config{
		DBURL:          url,
		Schema:         "test_schema",
		AdvisoryLockID: 99,
		EmbedFS:        testSchemaFS,
		EmbedRoot:      "testdata/schema",
	}
	err := migrator.Run(context.Background(), cfg, []string{"drop"})
	if err == nil {
		t.Fatal("expected drop to fail in non-dev env")
	}
}

func TestRun_AdvisoryLockBlocks(t *testing.T) {
	url, cleanup := setupPostgres(t)
	defer cleanup()

	cfg := migrator.Config{
		DBURL:          url,
		Schema:         "test_schema",
		AdvisoryLockID: 99,
		EmbedFS:        testSchemaFS,
		EmbedRoot:      "testdata/schema",
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			errs <- migrator.Run(ctx, cfg, []string{"up"})
		}()
	}
	wg.Wait()
	close(errs)

	// Both should complete (one applies, the other waits then no-ops).
	for err := range errs {
		if err != nil {
			t.Fatalf("parallel migrator failed: %v", err)
		}
	}

	// Sanity: subsequent up returns no error
	if err := migrator.Run(context.Background(), cfg, []string{"up"}); err != nil {
		t.Fatalf("post-parallel up: %v", err)
	}

	_ = os.Stderr // silence unused if test conditions skip
}
