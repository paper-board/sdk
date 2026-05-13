# auth — integration tests

## Overview

`middleware_integration_test.go` contains 8 integration tests (build tag `integration`)
that exercise the `sdk/auth` middleware against a real identity gRPC server backed by
Postgres.

## Orchestration choice: in-process gRPC + testcontainers (Option A variant)

**Why not subprocess?**
`identity.Config.Env` validates as `oneof=dev|staging|prod` while API-key env is `live|test`.
Running the identity binary with `ENV=dev` means gRPC `VerifyAPIKey` always returns
`wrong_environment` — the config ENV value never equals the api-key env value. This is
a known identity design mismatch, tracked for Phase 2 follow-up.

**What we do instead:**
- Spin Postgres via testcontainers-go (no sibling-repo path required, no port conflicts)
- Apply the identity schema inline (mirrors `000001_init.up.sql`)
- Start an in-process gRPC `AuthServiceServer` (`dbAuthServer`) over `bufconn`
  backed by the testcontainer DB
- `dbAuthServer` implements real argon2id verification, real RS256 key management —
  the actual logic, not stubs

This is not mocking: real SQL queries, real crypto, real schema.

## Running locally

```bash
# From the repository root:
RUN_INTEGRATION=1 go test -tags=integration -race -count=1 -p 1 ./auth/...
```

Docker must be running (testcontainers needs it). First run pulls `postgres:16-alpine`.
Expected output: 8 `TestIntegration_*` cases PASS (~3-4 s).

## CI

Integration tests skip by default unless `RUN_INTEGRATION=1` is set (`TestMain`
exits 0). Full CI wiring deferred to Task 14 (cross-service integration).

## 8 cases

| Test                                    | What it verifies                                      |
| --------------------------------------- | ----------------------------------------------------- |
| `TestIntegration_valid_apikey`          | Live-env api key → 200                               |
| `TestIntegration_expired_apikey_401`    | Expired (soft-deleted) api key → 401                 |
| `TestIntegration_deleted_apikey_401`    | Manually revoked (soft-deleted) api key → 401        |
| `TestIntegration_malformed_header_401`  | `Bearer garbage-token` → 401                         |
| `TestIntegration_missing_header_401`    | No Authorization header → 401                        |
| `TestIntegration_valid_jwt`             | JWT signed by active key → 200, correct AuthCtx      |
| `TestIntegration_expired_jwt_401`       | JWT with exp in past → 401                           |
| `TestIntegration_invalid_jwt_signature_401` | JWT with tampered signature bytes → 401          |
