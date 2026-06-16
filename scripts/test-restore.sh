#!/bin/bash
set -euo pipefail

if [ "${DBB_TEST_BACKUP_ID:-}" = "" ] || [ "${DBB_TEST_DATABASE:-}" = "" ] || [ "${DBB_TEST_TARGET_DB:-}" = "" ]; then
  echo "请设置 DBB_TEST_BACKUP_ID、DBB_TEST_DATABASE 和 DBB_TEST_TARGET_DB。"
  exit 2
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [ ! -x bin/dbbackupctl ]; then
  go build -o bin/dbbackupctl ./cmd/dbbackupctl
fi

case "${DBB_TEST_DB_TYPE:-mysql}" in
  mysql)
    bin/dbbackupctl mysql restore --id "$DBB_TEST_BACKUP_ID" --database "$DBB_TEST_DATABASE" --target-db "$DBB_TEST_TARGET_DB"
    if [ "${1:-}" = "--execute" ]; then
      bin/dbbackupctl mysql restore --id "$DBB_TEST_BACKUP_ID" --database "$DBB_TEST_DATABASE" --target-db "$DBB_TEST_TARGET_DB" --execute
    fi
    ;;
  postgresql)
    bin/dbbackupctl postgresql restore --id "$DBB_TEST_BACKUP_ID" --database "$DBB_TEST_DATABASE" --target-db "$DBB_TEST_TARGET_DB"
    if [ "${1:-}" = "--execute" ]; then
      bin/dbbackupctl postgresql restore --id "$DBB_TEST_BACKUP_ID" --database "$DBB_TEST_DATABASE" --target-db "$DBB_TEST_TARGET_DB" --execute
    fi
    ;;
  *)
    echo "DBB_TEST_DB_TYPE 必须是 mysql 或 postgresql"
    exit 2
    ;;
esac
