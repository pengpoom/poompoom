Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

function Assert-Command {
  param([string]$CommandName)
  if (-not (Get-Command $CommandName -ErrorAction SilentlyContinue)) {
    throw "Missing required command: $CommandName"
  }
}

Assert-Command "docker"

Write-Host "[1/4] Checking Docker Compose..."
docker compose version | Out-Null

if (Test-Path ".git") {
  $trackedChanges = git status --porcelain --untracked-files=no
  if ($trackedChanges) {
    throw "Repository has tracked local changes. Commit or stash them before running docker update."
  }

  Write-Host "[2/4] Pulling latest repository changes..."
  git pull --ff-only origin main
} else {
  Write-Host "[2/4] Git metadata not found, skipping repository pull."
}

Write-Host "[3/4] Pulling latest Docker image from GitHub Container Registry..."
docker compose pull studio

Write-Host "[4/4] Recreating container..."
docker compose up -d --remove-orphans studio
docker compose ps
