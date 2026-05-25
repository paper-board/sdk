#!/usr/bin/env bash
# Checks that unit test coverage meets the minimum threshold.
# Integration-only packages (outbox, inbox) require RUN_INTEGRATION=1 to hit >=80%.
# Usage: ./scripts/cover-check.sh [min_pct]
set -euo pipefail

MIN="${COVERAGE_MIN:-80}"

go test -coverprofile=cover.out ./...

total=$(go tool cover -func=cover.out | grep '^total:' | awk '{print $3}' | tr -d '%')

echo "Total coverage: ${total}% (min: ${MIN}%)"

# Use awk for float comparison (bash can't do floats).
if awk "BEGIN { exit !( ${total} < ${MIN} ) }"; then
  echo "FAIL: coverage ${total}% < ${MIN}%" >&2
  exit 1
fi

echo "PASS"
