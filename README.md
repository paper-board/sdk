# paper-board/sdk

[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/paper-board/.github/blob/main/docs/adr/0008-license-coc-commit-conventions.md)
[![Go Reference](https://pkg.go.dev/badge/github.com/paper-board/sdk.svg)](https://pkg.go.dev/github.com/paper-board/sdk)

Shared Go library for paper-board backend services. Phase 4 substrate (current: v0.5.0).

## Packages

| Package           | Added  | Description                                                                                                 |
| ----------------- | ------ | ----------------------------------------------------------------------------------------------------------- |
| `migrator/`       | v0.1.0 | Schema-per-service migrations with advisory locks; golang-migrate + cobra CLI wrapper                       |
| `log/`            | v0.1.0 | Structured logging via slog; ctx-aware `Pkg(ctx)` helper auto-injects request_id, trace_id, org_id, user_id |
| `obs/`            | v0.1.0 | OpenTelemetry SDK setup (OTLP exporter pending)                                                             |
| `errors/`         | v0.1.0 | Sentinel errors + gRPC code mapping (`ToGRPCStatus`, `FromGRPCStatus`)                                      |
| `errors/classify` | v0.5.0 | Dependency-free error classification primitives (retryable, permanent, transient)                           |
| `auth/`           | v0.3.0 | JWT + API-key middleware backed by identity gRPC; drop-in for any paper-board service                       |
| `healthcheck/`    | v0.4.0 | Liveness/readiness HTTP primitives                                                                          |
| `idempotency/`    | v0.4.0 | Idempotency-key middleware                                                                                  |
| `retry/`          | v0.5.0 | Dependency-free retry primitives with configurable backoff schedule                                         |
| `outbox/`         | v0.5.0 | Transactional outbox writer (Phase 4 cross-service eventing substrate)                                      |
| `inbox/`          | v0.5.0 | Inbox consumer pattern with atomic deduplication (Phase 4 cross-service eventing substrate)                 |

Additional packages: `clock/`, `config/`, `httpclient/`, `httpmw/`, `store/`, `testfixture/`.

## Phase 4 substrate role

`outbox/` and `inbox/` implement the transactional outbox + inbox pattern used for cross-service eventing in Phase 4. All six Phase 4 services (audit, metering, notifications, onboarding, environments, vaults) consume these packages. `retry/` and `errors/classify` are dependency-free primitives shared across all drain loops and gRPC interceptors.

See [ADR-0005](https://github.com/paper-board/.github/blob/main/docs/adr/0005-communication-patterns.md) for the cross-service communication design.

## Install

```bash
go get github.com/paper-board/sdk@v0.5.0
```

Requires Go 1.26+.

## Usage

```go
// cmd/migrator/main.go (~30 lines per service)
package main

import (
    "context"
    "log"
    "os"

    "github.com/paper-board/sdk/migrator"
    "github.com/paper-board/identity/migrations"
)

func main() {
    cfg := migrator.Config{
        DBURL:     os.Getenv("MIGRATION_DB_URL"),
        Schema:    "identity",
        EmbedFS:   migrations.SchemaFS,
        EmbedRoot: "schema",
    }
    if err := migrator.Run(context.Background(), cfg, os.Args[1:]); err != nil {
        log.Fatal(err)
    }
}
```

## Versioning and releases

SemVer 2.0:

- **major** — breaking API change
- **minor** — additive (new package, new exported symbol)
- **patch** — bug fix, doc fix

Releases are automated via [release-please](https://github.com/googleapis/release-please). Merging the release-please PR cuts the tag. Manual `git tag` is NOT used here (unlike `paper-board/proto`).

Each minor/patch bump triggers a Renovate auto-PR in every dependent service repo.

## Compatibility

| sdk    | proto     |
| ------ | --------- |
| v0.5.x | >= v0.4.0 |
| v0.4.x | >= v0.3.0 |
| v0.3.x | >= v0.2.0 |

## Development

```bash
go mod tidy
go test -race -count=1 ./...
```

Integration tests use [testcontainers-go](https://golang.testcontainers.org/) for Postgres and Redis. Requires Docker.

## Standards

Code follows [go-coding-conventions.md](https://github.com/paper-board/.github/blob/main/docs/standards/go-coding-conventions.md). Test discipline follows [testing.md](https://github.com/paper-board/.github/blob/main/docs/standards/testing.md). All public APIs SemVer-versioned per [ADR-0008](https://github.com/paper-board/.github/blob/main/docs/adr/0008-license-coc-commit-conventions.md).

## Further reading

- [ADR-0004](https://github.com/paper-board/.github/blob/main/docs/adr/0004-migrator-shared-library.md) — migrator as shared library
- [ADR-0005](https://github.com/paper-board/.github/blob/main/docs/adr/0005-communication-patterns.md) — communication patterns
- [ADR-0008](https://github.com/paper-board/.github/blob/main/docs/adr/0008-license-coc-commit-conventions.md) — license + versioning

## Contributing

Issues and PRs welcome. Open an issue for breaking proposals before coding; the SemVer-major bump cost is high (Renovate-driven cascade across all services). See [Code of Conduct](https://github.com/paper-board/.github/blob/main/CODE_OF_CONDUCT.md) and [paper-board/.github](https://github.com/paper-board/.github) for community files.

## License

MIT. `LICENSE` file lands Phase 5. See [ADR-0008](https://github.com/paper-board/.github/blob/main/docs/adr/0008-license-coc-commit-conventions.md) for licensing policy.
