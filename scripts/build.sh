#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB_DIR="$REPO_ROOT/web"
BACKEND_DIR="$REPO_ROOT/backend"
DIST_DIR="$REPO_ROOT/dist"
BACKEND_STATIC_DIR="$BACKEND_DIR/static"
PACKAGE_DIR="$DIST_DIR/package"
PACKAGE_DATA_DIR="$PACKAGE_DIR/data"
PACKAGE_STATIC_DIR="$PACKAGE_DIR/static"
VERSION="$(git -C "$REPO_ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)"
COMMIT="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo none)"
BUILD_TIME="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
LDFLAGS="-s -w -X imagestudio/internal/buildinfo.Version=$VERSION -X imagestudio/internal/buildinfo.Commit=$COMMIT -X imagestudio/internal/buildinfo.BuildTime=$BUILD_TIME"

echo "[1/4] Building frontend..."
cd "$WEB_DIR"
npm ci
export VITE_APP_VERSION="$VERSION"
npm run build

echo "[2/4] Syncing frontend assets..."
rm -rf "$BACKEND_STATIC_DIR"
mkdir -p "$BACKEND_STATIC_DIR"
cp -R "$WEB_DIR"/dist/. "$BACKEND_STATIC_DIR"/

echo "[3/4] Building backend..."
mkdir -p "$DIST_DIR"
cd "$BACKEND_DIR"
go build -ldflags "$LDFLAGS" -o "$DIST_DIR/image-studio" .

echo "[4/4] Preparing local release package..."
rm -rf "$PACKAGE_DIR"
mkdir -p "$PACKAGE_DATA_DIR" "$PACKAGE_STATIC_DIR"
cp "$DIST_DIR/image-studio" "$PACKAGE_DIR/image-studio"
cp "$BACKEND_DIR/internal/config/config.defaults.toml" "$PACKAGE_DATA_DIR/config.example.toml"
cp -R "$WEB_DIR"/dist/. "$PACKAGE_STATIC_DIR"/
cp "$REPO_ROOT/packaging/README.txt" "$PACKAGE_DIR/README.txt"

echo "Build complete: $DIST_DIR"
