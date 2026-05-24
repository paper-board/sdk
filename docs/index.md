# paper-board/sdk

`github.com/paper-board/sdk` is the shared Go library that all seven
paper-board backend services import. It provides common infrastructure
primitives so each service can stay thin and consistent.

## What is in the SDK

| Package       | What it does                                                          |
| ------------- | --------------------------------------------------------------------- |
| `migrator`    | golang-migrate wrapper + cobra CLI; per-service schema migrations     |
| `log`         | slog-based structured logging; context-aware helpers                  |
| `obs`         | OpenTelemetry SDK setup (OTLP exporter + Prometheus registration)     |
| `auth`        | JWT + API-key HTTP middleware backed by identity gRPC                 |
| `errors`      | Sentinel errors + gRPC/HTTP status mapping                            |
| `config`      | Env-only config loader (`MustBind[T]`)                                |
| `healthcheck` | Composable readiness probe registry; built-in DB + gRPC checkers      |
| `idempotency` | HTTP middleware for Idempotency-Key replay (Stripe-style, 24h TTL)    |
| `httpmw`      | Shared HTTP middleware: request-id, logger, recover, body-limit, OTLP |
| `httpclient`  | Pre-configured `*http.Client` with OTLP transport and timeouts        |
| `store`       | Generic `WithTx` helper for pgx transaction management                |
| `clock`       | `Clock` interface + `Real` / `Fake` implementations for testing       |
| `testfixture` | Testcontainers-based Postgres helpers for integration tests           |

## Who uses it

Every backend service pins a released SDK version:

| Service  | Phase | Primary packages used                                 |
| -------- | ----- | ----------------------------------------------------- |
| agents   | 1.1   | migrator, log, obs, auth, errors, config, healthcheck |
| identity | 2     | migrator, log, obs, errors, config, healthcheck       |
| runtime  | 3     | migrator, log, obs, auth, errors, config, healthcheck |
| compute  | 3     | log, obs, errors, config, healthcheck                 |
| billing  | 5     | migrator, log, obs, auth, errors, config, healthcheck |
| platform | 4     | migrator, log, obs, auth, errors, config, healthcheck |
| gateway  | 7     | log, obs, auth, errors, config, healthcheck, httpmw   |

## Package map

```
github.com/paper-board/sdk
├── migrator/        ← schema migration CLI (ADR-0004)
├── log/             ← structured logging
│   └── redact.go    ← secret-scrubbing slog handler
├── obs/             ← OpenTelemetry setup
├── auth/            ← JWT + API-key HTTP middleware
│   └── mock/        ← test double for identity AuthServiceClient
├── errors/          ← sentinels + gRPC/HTTP status mapping
├── config/          ← env-only loader
├── healthcheck/     ← readiness probe registry
├── idempotency/     ← Idempotency-Key middleware
├── httpmw/          ← shared HTTP middleware chain
├── httpclient/      ← pre-configured *http.Client
├── store/           ← pgx transaction helper
├── clock/           ← Clock interface
└── testfixture/     ← integration test helpers
```

## Quickstart

### Add the dependency

```bash
go get github.com/paper-board/sdk@latest
```

The module is private. Ensure your environment has `GOPRIVATE=github.com/paper-board/*`
and a GitHub PAT configured (see CI setup in `paper-board/.github/go-ci.yml`).

### Wire up a migrator (service cmd)

```go
// cmd/migrator/main.go — ~30 lines, same in every service
package main

import (
    "context"
    "log"
    "os"

    "github.com/paper-board/sdk/migrator"
    "github.com/paper-board/myservice/migrations" // embed.FS lives here
)

func main() {
    cfg := migrator.Config{
        DBURL:     os.Getenv("MIGRATION_DB_URL"),
        Schema:    "myservice",
        EmbedFS:   migrations.SchemaFS,
        EmbedRoot: "schema",
    }
    if err := migrator.Run(context.Background(), cfg, os.Args[1:]); err != nil {
        log.Fatal(err)
    }
}
```

`MIGRATION_DB_URL` must point to direct Postgres (port 5432), never PgBouncer.
Advisory locking requires a session-mode connection.

### Configure structured logging

```go
import "github.com/paper-board/sdk/log"

logger := log.New(os.Stdout, "myservice", log.LevelFromEnv())
log.SetDefault(logger)

// In handlers, enrich ctx and log:
ctx = log.WithRequestID(ctx, requestID)
log.Pkg(ctx).Info("request started", "path", r.URL.Path)
```

### Bootstrap OpenTelemetry

```go
import "github.com/paper-board/sdk/obs"

shutdown, err := obs.SetupOTel("myservice", version)
if err != nil {
    log.Fatal(err)
}
defer shutdown(context.Background())
```

Set `OTEL_EXPORTER_OTLP_ENDPOINT` to your collector; unset = OTLP disabled.

### Run the tests

```bash
go test -race -count=1 ./...
```

Integration tests (packages `migrator`, `testfixture`) spin up a
testcontainers-managed Postgres instance. Docker must be running.
CI runs the same command via the reusable workflow in `paper-board/.github`.
