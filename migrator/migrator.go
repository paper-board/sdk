// Package migrator provides a thin wrapper around golang-migrate for paper-board
// services. Each service ships a ~30-line cmd/migrator/main.go that calls Run
// with a Config carrying the embedded migrations and service-specific advisory
// lock id.
//
// Usage:
//
//	cfg := migrator.Config{
//	    DBURL:          os.Getenv("MIGRATION_DB_URL"),
//	    Schema:         "identity",
//	    AdvisoryLockID: 1,
//	    EmbedFS:        migrations.SchemaFS,
//	    EmbedRoot:      "schema",
//	}
//	migrator.Run(context.Background(), cfg, os.Args[1:])
package migrator

import (
	"context"
	"embed"
	"errors"
	"fmt"
)

// Config carries everything Run needs. All fields required.
type Config struct {
	// DBURL is MIGRATION_DB_URL — direct Postgres on port 5432, NOT PgBouncer.
	// Advisory locks require session-mode connections; PgBouncer transaction
	// pooling breaks them.
	DBURL string

	// Schema is the Postgres schema this service owns (e.g., "identity").
	// Migrations run with search_path=<Schema>,public.
	Schema string

	// AdvisoryLockID is unique per service. paper-board mapping:
	//   identity = 1
	//   billing  = 2
	//   agents   = 3
	//   platform = 4
	// This allows parallel migrations across services without serialization.
	AdvisoryLockID int

	// EmbedFS is the embedded migration filesystem from the service repo.
	// Convention: //go:embed schema/*.sql
	EmbedFS embed.FS

	// EmbedRoot is the directory inside EmbedFS containing migration files.
	// Typically "schema".
	EmbedRoot string
}

// validate ensures Config has all required fields. Run calls this first.
func (c Config) validate() error {
	var missing []string
	if c.DBURL == "" {
		missing = append(missing, "DBURL (set MIGRATION_DB_URL env)")
	}
	if c.Schema == "" {
		missing = append(missing, "Schema")
	}
	if c.AdvisoryLockID == 0 {
		missing = append(missing, "AdvisoryLockID (must be > 0)")
	}
	if c.EmbedRoot == "" {
		missing = append(missing, "EmbedRoot")
	}
	if len(missing) > 0 {
		return fmt.Errorf("migrator.Config invalid, missing: %v", missing)
	}
	return nil
}

// Sentinel errors. Callers may use errors.Is to detect specific failure modes.
var (
	ErrMissingEnv     = errors.New("MIGRATION_DB_URL env not set")
	ErrDirty          = errors.New("schema is dirty; run 'migrator force <prev_version>' after fixing")
	ErrLockTimeout    = errors.New("advisory lock acquisition timed out (another migrator running?)")
	ErrDropProduction = errors.New("'drop' is dev-only; set MIGRATOR_ENV=dev to enable")
)

// Run is the binary entry point. It parses args, connects to Postgres,
// acquires the advisory lock, and dispatches to the appropriate subcommand.
//
// Returns nil on success, error otherwise. Callers typically:
//
//	if err := migrator.Run(ctx, cfg, os.Args[1:]); err != nil {
//	    log.Fatal(err)
//	}
func Run(ctx context.Context, cfg Config, args []string) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	return newRunner(cfg).execute(ctx, args)
}
