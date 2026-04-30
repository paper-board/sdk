package migrator

import (
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// dbHandle abstracts *sql.DB for testability.
type dbHandle = *sql.DB

// mustParseConfig parses a Postgres connection URL into pgx config.
// Panics if invalid (caller responsibility to validate before).
func mustParseConfig(url string) *pgx.ConnConfig {
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		panic(fmt.Sprintf("invalid MIGRATION_DB_URL: %v", err))
	}
	return cfg
}
