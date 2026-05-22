#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB_DIR="$REPO_ROOT/web"
BACKEND_DIR="$REPO_ROOT/backend"

echo "[1/5] Running backend tests..."
cd "$BACKEND_DIR"
go test ./...

if [ "${RUN_IMAGE_MODE_COMPAT_TESTS:-}" = "1" ]; then
  echo "[2/6] Running optional image mode compatibility tests..."
  go test ./api -run TestImageModeCompatibilityBlackBox -count=1
  FRONTEND_STEP_OFFSET=1
else
  FRONTEND_STEP_OFFSET=0
fi

echo "[$((2 + FRONTEND_STEP_OFFSET))/$((
 5 + FRONTEND_STEP_OFFSET
))] Ensuring frontend dependencies..."
cd "$WEB_DIR"
if [ ! -x node_modules/.bin/tsc ] || [ ! -x node_modules/.bin/eslint ] || [ ! -x node_modules/.bin/vite ]; then
  npm ci
fi

echo "[$((3 + FRONTEND_STEP_OFFSET))/$((
 5 + FRONTEND_STEP_OFFSET
))] Running frontend type check..."
npx tsc --noEmit

echo "[$((4 + FRONTEND_STEP_OFFSET))/$((
 5 + FRONTEND_STEP_OFFSET
))] Running frontend lint..."
npm run lint

echo "[$((5 + FRONTEND_STEP_OFFSET))/$((
 5 + FRONTEND_STEP_OFFSET
))] Running frontend production build..."
npm run build

echo "Checks complete."
