#!/usr/bin/env bash
# Checks that unit test coverage meets the minimum threshold.
# NOTE: outbox and inbox packages have integration-only DB/Redis code paths.
# Unit-only run (no RUN_INTEGRATION=1) yields ~43% total. Set RUN_INTEGRATION=1
# and use -tags integration to get meaningful numbers for those packages.
# Usage: ./scripts/cover-check.sh
set -euo pipefail

MIN="${COVERAGE_MIN:-80}"

TAGS=""
if [ "${RUN_INTEGRATION:-0}" = "1" ]; then
  TAGS="-tags integration"
fi

# shellcheck disable=SC2086
go test ${TAGS} -coverprofile=cover.out ./...

total=$(go tool cover -func=cover.out | grep '^total:' | awk '{print $3}' | tr -d '%')

echo "Total coverage: ${total}% (min: ${MIN}%)"

if awk "BEGIN { exit !( ${total} < ${MIN} ) }"; then
  echo "FAIL: coverage ${total}% < ${MIN}%" >&2
  exit 1
fi

echo "PASS"
