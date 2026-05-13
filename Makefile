.PHONY: all test integration-test lint cover clean

all: test

# Unit tests — no build tag, no external deps.
test:
	go test -race -count=1 ./...

# Integration tests for the auth/ package. Spins Postgres via testcontainers-go
# + in-process gRPC AuthServer. Docker must be running.
integration-test:
	RUN_INTEGRATION=1 go test -tags=integration -race -count=1 -p 1 ./auth/...

# Local lint (mirrors the reusable CI workflow's lint job).
lint:
	golangci-lint run ./...

# Coverage profile + summary.
cover:
	go test -race -count=1 -coverprofile=cover.out ./...
	go tool cover -func=cover.out | tail -1

clean:
	rm -f cover.out
