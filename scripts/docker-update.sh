#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

command -v docker >/dev/null 2>&1 || {
  echo "Missing required command: docker" >&2
  exit 1
}

echo "[1/4] Checking Docker Compose..."
docker compose version >/dev/null

if [ -d ".git" ]; then
  if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
    echo "Repository has tracked local changes. Commit or stash them before running docker update." >&2
    exit 1
  fi

  echo "[2/4] Pulling latest repository changes..."
  git pull --ff-only origin main
else
  echo "[2/4] Git metadata not found, skipping repository pull."
fi

echo "[3/4] Pulling latest Docker image from GitHub Container Registry..."
docker compose pull studio

echo "[4/4] Recreating container..."
docker compose up -d --remove-orphans studio
docker compose ps
