#!/usr/bin/env bash
set -euo pipefail

GITHUB_REPO="${GITHUB_REPO:-pengpoom/poomimage}"
BRANCH="${BRANCH:-main}"
INSTALL_DIR="${INSTALL_DIR:-$PWD}"
COMPOSE_URL="${COMPOSE_URL:-https://raw.githubusercontent.com/$GITHUB_REPO/$BRANCH/docker-compose.yml}"
ENV_URL="${ENV_URL:-https://raw.githubusercontent.com/$GITHUB_REPO/$BRANCH/.env.example}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

log() {
  printf '[image-studio] %s\n' "$*"
}

warn() {
  printf '[image-studio] %s\n' "$*" >&2
}

require_command() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    warn "missing required command: $name"
    exit 1
  fi
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 24
    return
  fi
  if [ -r /dev/urandom ]; then
    LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 48
    printf '\n'
    return
  fi
  date +%s | sha256sum | awk '{print $1}'
}

copy_or_download() {
  local source_path="$1"
  local url="$2"
  local target_path="$3"

  if [ -f "$source_path" ]; then
    cp "$source_path" "$target_path"
    return
  fi

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$target_path"
    return
  fi

  if command -v wget >/dev/null 2>&1; then
    wget -qO "$target_path" "$url"
    return
  fi

  warn "missing required command: curl or wget"
  exit 1
}

replace_env_value() {
  local file="$1"
  local key="$2"
  local value="$3"
  if grep -q "^${key}=" "$file"; then
    sed -i "s#^${key}=.*#${key}=${value}#" "$file"
  else
    printf '%s=%s\n' "$key" "$value" >>"$file"
  fi
}

require_command docker
docker compose version >/dev/null

mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

log "install directory: $INSTALL_DIR"

if [ -f "docker-compose.yml" ]; then
  log "docker-compose.yml already exists, leaving it unchanged"
else
  log "writing docker-compose.yml"
  copy_or_download "$REPO_ROOT/docker-compose.yml" "$COMPOSE_URL" "docker-compose.yml"
fi

if [ -f ".env" ]; then
  log ".env already exists, leaving it unchanged"
else
  log "writing .env"
  copy_or_download "$REPO_ROOT/.env.example" "$ENV_URL" ".env"
  replace_env_value ".env" "ADMIN_PASSWORD" "change-this-admin-password-$(random_secret)"
  replace_env_value ".env" "TEST_PASSWORD" "change-this-user-password-$(random_secret)"
  replace_env_value ".env" "IMAGE_STUDIO_UPDATER_TOKEN" "image-studio-updater-$(random_secret)"
  replace_env_value ".env" "EXTERNAL_API_SIGNING_SECRET" "$(random_secret)"
  replace_env_value ".env" "IMAGE_STUDIO_DEPLOY_DIR" "$INSTALL_DIR"
  if [ "$(id -u)" = "0" ]; then
    replace_env_value ".env" "DOCKER_CONFIG_DIR" "/root/.docker"
  else
    replace_env_value ".env" "DOCKER_CONFIG_DIR" "$HOME/.docker"
  fi
fi

mkdir -p "backend/data" "backups"

log "deployment files are ready"
cat <<'EOF'

Next steps:
  1. Edit .env and set ADMIN_PASSWORD / TEST_PASSWORD to values you will keep.
  2. If GHCR is private, run docker login ghcr.io before pulling images.
  3. Start ImageStudio:

       docker compose pull
       docker compose up -d
       docker compose logs -f studio

  4. Open:

       http://SERVER_IP:7000/

Notes:
  - Web one-click update requires IMAGE_STUDIO_UPDATER_TOKEN and Docker socket access.
  - DOCKER_CONFIG_DIR must point to the Docker config directory that can pull the GHCR image.
  - Keep backend/data backed up before upgrades or migration.

EOF
