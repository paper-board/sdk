package migrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
)

type runner struct {
	cfg    Config
	logger *slog.Logger
}

func newRunner(cfg Config) *runner {
	return &runner{
		cfg:    cfg,
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)).With("service", cfg.Schema, "lock_id", cfg.AdvisoryLockID),
	}
}

func (r *runner) execute(ctx context.Context, args []string) error {
	root := &cobra.Command{
		Use:   "migrator",
		Short: fmt.Sprintf("Schema migrations for %s service", r.cfg.Schema),
	}
	var dryRun bool
	root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "list pending migrations without applying")

	root.AddCommand(r.cmdUp(&dryRun))
	root.AddCommand(r.cmdDown())
	root.AddCommand(r.cmdForce())
	root.AddCommand(r.cmdVersion())
	root.AddCommand(r.cmdDrop())

	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func (r *runner) cmdUp(dryRun *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := r.openMigrate()
			if err != nil {
				return err
			}
			defer m.Close()

			if *dryRun {
				return r.runDryRun(m)
			}

			r.logger.Info("starting migrate up")
			before, _, _ := m.Version()
			if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				return r.handleMigrateErr(err)
			}
			after, dirty, _ := m.Version()
			r.logger.Info("migrate up complete", "from", before, "to", after, "dirty", dirty)
			return nil
		},
	}
}

func (r *runner) cmdDown() *cobra.Command {
	return &cobra.Command{
		Use:   "down [N]",
		Short: "Revert last N migrations",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var n int
			if _, err := fmt.Sscanf(args[0], "%d", &n); err != nil || n < 1 {
				return fmt.Errorf("invalid N: %s (must be positive integer)", args[0])
			}
			m, err := r.openMigrate()
			if err != nil {
				return err
			}
			defer m.Close()

			r.logger.Warn("starting migrate down", "steps", n)
			if err := m.Steps(-n); err != nil {
				return r.handleMigrateErr(err)
			}
			version, dirty, _ := m.Version()
			r.logger.Info("migrate down complete", "version", version, "dirty", dirty)
			return nil
		},
	}
}

func (r *runner) cmdForce() *cobra.Command {
	return &cobra.Command{
		Use:   "force VERSION",
		Short: "Set version, ignore current state (dirty recovery)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var v int
			if _, err := fmt.Sscanf(args[0], "%d", &v); err != nil {
				return fmt.Errorf("invalid VERSION: %s", args[0])
			}
			m, err := r.openMigrate()
			if err != nil {
				return err
			}
			defer m.Close()

			r.logger.Warn("forcing version (dirty recovery)", "version", v)
			return m.Force(v)
		},
	}
}

func (r *runner) cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print current (version, dirty) state",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := r.openMigrate()
			if err != nil {
				return err
			}
			defer m.Close()
			version, dirty, err := m.Version()
			if errors.Is(err, migrate.ErrNilVersion) {
				fmt.Println("no migrations applied")
				return nil
			}
			if err != nil {
				return err
			}
			fmt.Printf("version=%d dirty=%v\n", version, dirty)
			return nil
		},
	}
}

func (r *runner) cmdDrop() *cobra.Command {
	return &cobra.Command{
		Use:   "drop",
		Short: "Drop all tables in schema (DEV ONLY — MIGRATOR_ENV=dev required)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Getenv("MIGRATOR_ENV") != "dev" {
				return ErrDropProduction
			}
			m, err := r.openMigrate()
			if err != nil {
				return err
			}
			defer m.Close()

			r.logger.Warn("DROPPING all tables in schema", "schema", r.cfg.Schema)
			return m.Drop()
		},
	}
}

func (r *runner) runDryRun(m *migrate.Migrate) error {
	current, _, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return err
	}
	r.logger.Info("dry-run: current version", "version", current)
	// golang-migrate doesn't expose pending list directly; users see version diff post-Up
	fmt.Printf("dry-run: current=%d (run without --dry-run to apply)\n", current)
	return nil
}

func (r *runner) openMigrate() (*migrate.Migrate, error) {
	src, err := iofs.New(r.cfg.EmbedFS, r.cfg.EmbedRoot)
	if err != nil {
		return nil, fmt.Errorf("iofs source: %w", err)
	}

	db, err := openPGXDB(r.cfg.DBURL, r.cfg.Schema)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	driver, err := pgxv5.WithInstance(db, &pgxv5.Config{
		SchemaName:    r.cfg.Schema,
		StatementTimeout: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("pgx driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		return nil, fmt.Errorf("migrate init: %w", err)
	}
	m.LockTimeout = 60 * 1_000_000_000 // 60s in ns; type is time.Duration
	return m, nil
}

func (r *runner) handleMigrateErr(err error) error {
	var derr migrate.ErrDirty
	if errors.As(err, &derr) {
		r.logger.Error("schema is dirty", "err", err, "hint", "run 'migrator force <prev_version>' after fixing")
		return ErrDirty
	}
	return err
}

// openPGXDB opens *sql.DB via pgx stdlib with search_path set to <schema>,public.
// Schema is created if missing (CREATE SCHEMA IF NOT EXISTS) since migrations
// can't bootstrap their own namespace.
func openPGXDB(url, schema string) (dbHandle, error) {
	conn := stdlib.OpenDB(*mustParseConfig(url))
	if _, err := conn.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	if _, err := conn.Exec(fmt.Sprintf("SET search_path TO %s, public", schema)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set search_path: %w", err)
	}
	return conn, nil
}
