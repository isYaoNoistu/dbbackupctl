#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

go test ./...
go vet ./...
go build -o bin/dbbackupctl ./cmd/dbbackupctl

scripts/test-mysql.sh --dry-run-only
scripts/test-postgresql.sh --dry-run-only
scripts/test-prune.sh --dry-run-only

bin/dbbackupctl index verify
