#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DATA_DIR="${IMAGE_STUDIO_DATA_DIR:-$ROOT_DIR/backend/data}"
BACKUP_DIR="${IMAGE_STUDIO_BACKUP_DIR:-$ROOT_DIR/backups}"
TIMESTAMP="$(date +'%Y%m%d-%H%M%S')"
WORK_DIR="$(mktemp -d)"
ARCHIVE="$BACKUP_DIR/image-studio-data-$TIMESTAMP.tar.gz"

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

if [ ! -d "$DATA_DIR" ]; then
  echo "data directory not found: $DATA_DIR" >&2
  exit 1
fi

mkdir -p "$BACKUP_DIR" "$WORK_DIR/data"

copy_if_exists() {
  local source="$1"
  local target="$2"
  if [ -e "$source" ]; then
    mkdir -p "$(dirname "$target")"
    cp -a "$source" "$target"
  fi
}

copy_if_exists "$DATA_DIR/config.toml" "$WORK_DIR/data/config.toml"
copy_if_exists "$DATA_DIR/config.example.toml" "$WORK_DIR/data/config.example.toml"
copy_if_exists "$DATA_DIR/accounts_state.json" "$WORK_DIR/data/accounts_state.json"
copy_if_exists "$DATA_DIR/auths" "$WORK_DIR/data/auths"
copy_if_exists "$DATA_DIR/sync_state" "$WORK_DIR/data/sync_state"
copy_if_exists "$DATA_DIR/business-images" "$WORK_DIR/data/business-images"
copy_if_exists "$DATA_DIR/tmp/image" "$WORK_DIR/data/tmp/image"
copy_if_exists "$DATA_DIR/last-startup-error.txt" "$WORK_DIR/data/last-startup-error.txt"

tar -C "$WORK_DIR" -czf "$ARCHIVE" data

echo "backup created: $ARCHIVE"
