# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0](https://github.com/paper-board/sdk/compare/v0.2.0...v0.3.0) (2026-05-13)


### Features

* **auth:** sdk/auth package (Phase 2) ([#1](https://github.com/paper-board/sdk/issues/1)) ([ca05f5a](https://github.com/paper-board/sdk/commit/ca05f5aa74f5ae83295b8637f4f65337d269ba25))

## [Unreleased] — Phase 2 Identity Integration

### Added

- `auth/` package: per-route middleware `Require(modes...)` / `RequireRole(roles...)` decorators
  backed by identity gRPC AuthService. Format-detects `pbk_(live|test)_*` (APIKey path) vs JWT
  (RS256). Includes `Keystore` with 5min TTL cache + `Refresh(kid)`. Mock client under
  `auth/mock/` for downstream template-validate smoke. Coverage ≥85% on `auth/`.
  - `auth.New(opts...)` returns `*Config`; options: `WithClient`, `WithKeystore`, `WithKeystoreTTL`.
  - `cfg.Require(JWT, APIKey)` — middleware factory, OR-set of accepted modes.
  - `cfg.RequireRole(Owner)` — middleware factory, OR-set of required roles; run after `Require`.
  - `auth.FromContext(ctx)` / `auth.WithContext(ctx, ac)` — typed AuthCtx accessor.
  - `auth/mock.AuthClient` — function-field stub for `identityv1.AuthServiceClient`.

## [v0.2.0] — 2026-05-03

### Added

- `httpmw` package — canonical HTTP middleware suite for paper-board services:
  - `RequestID` — read `X-Request-Id` or mint UUIDv7; echo on response.
  - `TrustHeaders` — gateway-issued `X-Org-Id`/`X-User-Id`/`X-Roles` to ctx
    (Phase 7+; pre-Phase-7 silent no-op).
  - `OtelHTTP(svc)` — `otelhttp.NewHandler` factory.
  - `Recover(svc)` — panic-to-500 with structured `error.kind=panic` log,
    canonical envelope via `HandleErr`.
  - `Logger` — one INFO record per request: method, route, status,
    duration_ms, bytes_in, bytes_out. SSE-compatible (forwards Flush).
  - `BodyLimit(n)` — `http.MaxBytesReader` wrapper.
  - `AuthStub(orgID)` — Phase 1.x bridge stamping a fixed org_id; identity
    Phase 2 swaps for real Auth.
  - `HandleErr(w, r, svc, err)` — central HTTP error boundary. Maps via
    `errors.ToHTTPStatus`; emits `{error:{code:"<svc>.<short>", message,
    details?}}` with sentinel-only message (wrapped chain stays in logs).
    `validator.ValidationErrors` surfaces under `details` with HTTP 400.
  - `WriteError(w, status, code, message)` — minimal envelope writer.
- `testfixture` package — Postgres test infra:
  - `PostgresContainer(t, schema)` — process-shared testcontainers Postgres
    via `sync.Once`; installs pgcrypto + citext; ensures schema.
  - `LoadSchema(t, pool, fs.FS, root)` — applies `*.up.sql` lexically.
  - `Truncate(t, pool, schemas...)` — TRUNCATE … RESTART IDENTITY CASCADE.
- `obs.SetupOTel(svc, version)` — real OTLP/HTTP exporter (traces + metrics)
  via `OTEL_EXPORTER_OTLP_ENDPOINT`; W3C TraceContext + Baggage propagators
  always set; `OTEL_EXPORTER_OTLP_INSECURE=true` for plain HTTP. No-endpoint
  path installs propagator only and returns no-op Shutdown so services boot
  without an OTel collector.
- `httpclient.Default(timeout)` — `*http.Client` wrapped via
  `otelhttp.NewTransport`; connect 5s, idle 90s.
- `store.WithTx[T](ctx, pool, fn)` — generic transaction wrapper; commits on
  success, rolls back on err or panic, propagates ctx cancellation.
- `log.RedactingHandler` — wraps any `slog.Handler`. Default key denylist
  (`password`, `secret`, `api_key`, `authorization`, `token`, `cookie`,
  `set-cookie`) + regex set (bearer, JWT-shape, sk-keys, email). Extensible
  via `RedactOpts`.
- `log.ErrorAttrs(err) []any` + `log.ErrorAttrsAttr(err) []slog.Attr` —
  emit `error.kind` + `error` for any `errors.Wrap`-anchored sentinel.
- `log.LevelFromEnv()` — parse `LOG_LEVEL` into `slog.Level`; defaults INFO.
- `log.WithRoles(ctx, []string)` plus `RequestIDFromContext`,
  `TraceIDFromContext`, `OrgFromContext`, `UserFromContext`,
  `RolesFromContext` getters.
- `errors.Kind(err) string` + `errors.Message(err) string` — sentinel slug
  + canonical user-facing message; used by httpmw + log.

### Changed

- `migrator/migrator_test.go` — gated `//go:build integration`; default
  `go test ./...` no longer requires Docker.
- `migrator.openPGXDB` — treat SQLSTATE `42P06` and `23505` from
  `CREATE SCHEMA IF NOT EXISTS` as benign (Postgres' implementation races
  on the system catalog when two callers hit it simultaneously).

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
