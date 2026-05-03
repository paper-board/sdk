# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.1.2] — 2026-05-03

### Added

- `errors.ToHTTPStatus(err) int` — maps wrapped sentinels to HTTP status codes
  (404/409/401/403/400/429/503/500); mirrors `errors.ToGRPCStatus`. Used as the
  central HTTP error boundary by `sdk/httpmw.HandleErr` (Phase 1.5+).
- `config` package — env-only loader. `MustBind[T]() *T` reads env vars onto a
  service `Config` struct via `env` / `default` / `validate` struct tags; runs
  `validator/v10`; logs to stderr and exits non-zero on validation failure.
  Supports `string`, `int`/`int64`, `bool`, `time.Duration`, `[]string`
  (comma-split). Flags and config files are intentionally unsupported.
- `clock` package — injectable time source. `Clock interface { Now() time.Time }`;
  `Real{}` returns wall-clock UTC; `Fake` is settable for tests
  (`NewFake(t)`, `Set(t)`, `Advance(d)`). All `Now()` returns are normalised to UTC.

### Changed

- `go.mod`: add `github.com/go-playground/validator/v10` (used by `config`).

## [v0.1.1] — 2026-05-01

### Fixed

- `migrator`: use `errors.As` for `migrate.ErrDirty` (struct type, not value).
- `migrator`: drop unused `err` from `pgx5/stdlib.OpenDB` (returns `*sql.DB` only).

## [v0.1.0] — 2026-04-30

### Added

- Initial: `migrator`, `log`, `obs` (placeholder), `errors`.
