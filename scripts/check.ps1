Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$webDir = Join-Path $repoRoot "web"
$backendDir = Join-Path $repoRoot "backend"

function Assert-LastExitCode {
  param(
    [string]$CommandName
  )

  if ($LASTEXITCODE -ne 0) {
    throw "$CommandName failed with exit code $LASTEXITCODE"
  }
}

function Ensure-FrontendDependencies {
  $requiredBins = @(
    "node_modules/.bin/tsc",
    "node_modules/.bin/eslint",
    "node_modules/.bin/vite"
  )

  $missing = @($requiredBins | Where-Object { -not (Test-Path $_) })
  if ($missing.Count -gt 0) {
    npm ci
    Assert-LastExitCode "npm ci"
  }
}

Write-Host "[1/5] Running backend tests..."
Push-Location $backendDir
go test ./...
Assert-LastExitCode "go test ./..."

$stepOffset = 0
if ($env:RUN_IMAGE_MODE_COMPAT_TESTS -eq "1") {
  Write-Host "[2/6] Running optional image mode compatibility tests..."
  go test ./api -run TestImageModeCompatibilityBlackBox -count=1
  Assert-LastExitCode "go test ./api -run TestImageModeCompatibilityBlackBox -count=1"
  $stepOffset = 1
}
Pop-Location

Write-Host "[$(2 + $stepOffset)/$(5 + $stepOffset)] Ensuring frontend dependencies..."
Push-Location $webDir
Ensure-FrontendDependencies

Write-Host "[$(3 + $stepOffset)/$(5 + $stepOffset)] Running frontend type check..."
npx tsc --noEmit
Assert-LastExitCode "npx tsc --noEmit"

Write-Host "[$(4 + $stepOffset)/$(5 + $stepOffset)] Running frontend lint..."
npm run lint
Assert-LastExitCode "npm run lint"

Write-Host "[$(5 + $stepOffset)/$(5 + $stepOffset)] Running frontend production build..."
npm run build
Assert-LastExitCode "npm run build"
Pop-Location

Write-Host "Checks complete."
