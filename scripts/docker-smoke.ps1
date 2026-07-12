param(
    [string]$EnvFile = ".env.example",
    [string]$ProjectName = "backend-infrastructure-go-smoke"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$compose = Join-Path $root "deployments/compose.yml"
$environment = Join-Path $root $EnvFile
$arguments = @("compose", "--project-name", $ProjectName, "--env-file", $environment, "-f", $compose)

function Invoke-Compose([string[]]$Command) {
    & docker @arguments @Command
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose $($Command -join ' ') failed with exit code $LASTEXITCODE"
    }
}

try {
    Invoke-Compose @("up", "-d", "--build", "--wait", "--wait-timeout", "180")
    Invoke-Compose @("exec", "-T", "postgres", "sh", "-ec", 'pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"')
    Invoke-Compose @("exec", "-T", "redis", "redis-cli", "ping")

    $migrationCount = & docker @arguments exec -T postgres sh -ec 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "SELECT count(*) FROM schema_migrations;"'
    if ($LASTEXITCODE -ne 0 -or [int]$migrationCount -lt 1) {
        throw "Database migrations were not applied"
    }

    Invoke-Compose @("stop", "-t", "30", "api", "worker", "scheduler")
    $logs = & docker @arguments logs api worker scheduler
    if ($LASTEXITCODE -ne 0 -or ($logs -join "`n") -notmatch 'service stopped') {
        throw "Service containers did not log graceful shutdown"
    }
    Write-Host "Docker Compose smoke test passed."
}
finally {
    & docker @arguments down --volumes --remove-orphans
}
