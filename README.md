# paper-board/sdk

**Phase:** 2 — `migrator/`, `log/`, `errors/`, `auth/` shipped at v0.3.0. `obs/` placeholder; real OTLP exporter Phase 1.5.

Shared Go library for paper-board services. SemVer 2.0 disciplined.

## Packages

- **`migrator/`** — golang-migrate wrapper with cobra CLI; per-service advisory lock; embed.FS source. See [ADR-0004](https://github.com/paper-board/.github/blob/main/docs/adr/0004-migrator-shared-library.md).
- **`log/`** — slog-based structured logging; ctx-aware `Pkg(ctx)` helper auto-injects request_id, trace_id, org_id, user_id.
- **`auth/`** — JWT + API-key middleware backed by identity gRPC. Drop-in for any paper-board service post-Phase 2.
- **`obs/`** — OpenTelemetry SDK setup; Phase 1.0 placeholder, Phase 1.5 real OTLP exporter.
- **`errors/`** — sentinel errors + gRPC code mapping (`ToGRPCStatus`, `FromGRPCStatus`).

## Install

```bash
go get github.com/paper-board/sdk@latest
```

Each minor/patch tag triggers a Renovate auto-PR in every dependent service repo.

## Usage

```go
// cmd/migrator/main.go (in your service repo, ~30 lines)
package main

import (
    "context"
    "log"
    "os"

    "github.com/paper-board/sdk/migrator"
    "github.com/paper-board/identity/migrations"  // your service's embed
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

## Versioning

SemVer strict:
- **major** — breaking API change
- **minor** — additive (new function, new field)
- **patch** — bug fix, doc fix

Each minor/patch bump triggers Renovate auto-PR in dependent service repos.

## Development

```bash
go mod tidy
go test -race -count=1 ./...
```

Integration tests use [testcontainers-go](https://golang.testcontainers.org/) for Postgres. Requires Docker.

## Standards

Code follows [go-coding-conventions.md](https://github.com/paper-board/.github/blob/main/docs/standards/go-coding-conventions.md). Test discipline follows [testing.md](https://github.com/paper-board/.github/blob/main/docs/standards/testing.md). All public APIs SemVer-versioned per [ADR-0008](https://github.com/paper-board/.github/blob/main/docs/adr/0008-license-coc-commit-conventions.md).

## Further Reading

- [ADR-0004](https://github.com/paper-board/.github/blob/main/docs/adr/0004-migrator-shared-library.md) — migrator as shared library
- [ADR-0008](https://github.com/paper-board/.github/blob/main/docs/adr/0008-license-coc-commit-conventions.md) — license + versioning

## Contributing

Issues + PRs welcome. Open issues for breaking proposals before coding; the SemVer-major bump cost is high (Renovate-driven cascade across all services). See [paper-board/.github](https://github.com/paper-board/.github) for community files.

## License

MIT — see [ADR-0008](https://github.com/paper-board/.github/blob/main/docs/adr/0008-license-coc-commit-conventions.md) for licensing policy. `LICENSE` file lands Phase 5.
