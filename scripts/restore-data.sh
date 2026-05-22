#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DATA_DIR="${IMAGE_STUDIO_DATA_DIR:-$ROOT_DIR/backend/data}"
BACKUP_DIR="${IMAGE_STUDIO_BACKUP_DIR:-$ROOT_DIR/backups}"
ARCHIVE="${1:-}"
TIMESTAMP="$(date +'%Y%m%d-%H%M%S')"
SAFETY_ARCHIVE="$BACKUP_DIR/pre-restore-data-$TIMESTAMP.tar.gz"
WORK_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

if [ -z "$ARCHIVE" ]; then
  echo "usage: $0 <backup.tar.gz>" >&2
  exit 1
fi

if [ ! -f "$ARCHIVE" ]; then
  echo "backup archive not found: $ARCHIVE" >&2
  exit 1
fi

echo "Restore target: $DATA_DIR"
echo "Backup archive: $ARCHIVE"
echo
echo "Stop the service before restoring, for example: docker compose down"
read -r -p "Type RESTORE to overwrite backend/data: " confirmation
if [ "$confirmation" != "RESTORE" ]; then
  echo "restore cancelled"
  exit 1
fi

mkdir -p "$BACKUP_DIR"
if [ -d "$DATA_DIR" ]; then
  tar -C "$(dirname "$DATA_DIR")" -czf "$SAFETY_ARCHIVE" "$(basename "$DATA_DIR")"
  echo "current data saved before restore: $SAFETY_ARCHIVE"
fi

tar -C "$WORK_DIR" -xzf "$ARCHIVE"
if [ ! -d "$WORK_DIR/data" ]; then
  echo "invalid backup archive: missing data directory" >&2
  exit 1
fi

rm -rf "$DATA_DIR"
mkdir -p "$(dirname "$DATA_DIR")"
cp -a "$WORK_DIR/data" "$DATA_DIR"

echo "restore complete: $DATA_DIR"
echo "Start the service again, for example: docker compose up -d"
