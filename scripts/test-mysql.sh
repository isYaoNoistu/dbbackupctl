#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [ ! -x bin/dbbackupctl ]; then
  go build -o bin/dbbackupctl ./cmd/dbbackupctl
fi

bin/dbbackupctl check --mysql --job "${DBB_TEST_JOB:-prod}"
bin/dbbackupctl mysql backup --job "${DBB_TEST_JOB:-prod}" --dry-run

if [ "${1:-}" != "--dry-run-only" ]; then
  bin/dbbackupctl mysql backup --job "${DBB_TEST_JOB:-prod}"
  bin/dbbackupctl show mysql
fi
