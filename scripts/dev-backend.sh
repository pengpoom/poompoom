#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"

export API_ONLY="${API_ONLY:-true}"
export SERVER_HOST="${SERVER_HOST:-0.0.0.0}"
export SERVER_PORT="${SERVER_PORT:-7070}"
export CORS_ALLOWED_ORIGINS="${CORS_ALLOWED_ORIGINS:-http://localhost:5270,http://127.0.0.1:5270}"
export DATABASE_DRIVER="${DATABASE_DRIVER:-postgres}"
export DATABASE_DSN="${DATABASE_DSN:-postgres://image_studio:image_studio@127.0.0.1:5432/image_studio?sslmode=disable}"
export JOB_QUEUE_BACKEND="${JOB_QUEUE_BACKEND:-redis}"
export REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"
export REDIS_PASSWORD="${REDIS_PASSWORD:-}"
export REDIS_DB="${REDIS_DB:-0}"
export REDIS_PREFIX="${REDIS_PREFIX:-imagestudio:studio}"
export GOCACHE="${GOCACHE:-/tmp/poomimage-renew-go-cache}"

echo "Starting backend API on ${SERVER_HOST}:${SERVER_PORT}"
echo "Database: ${DATABASE_DRIVER}"
echo "Queue: ${JOB_QUEUE_BACKEND}"

cd "$BACKEND_DIR"
exec go run .
