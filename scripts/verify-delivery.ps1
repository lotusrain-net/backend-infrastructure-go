param(
    [string]$EnvFile = ".env.example"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

function Assert-True([bool]$Condition, [string]$Message) {
    if (-not $Condition) {
        throw $Message
    }
}

function Read-Required([string]$RelativePath) {
    $path = Join-Path $root $RelativePath
    Assert-True (Test-Path -LiteralPath $path -PathType Leaf) "Missing required file: $RelativePath"
    return [System.IO.File]::ReadAllText($path)
}

$dockerfile = Read-Required "Dockerfile"
Assert-True ($dockerfile -match '(?im)^FROM\s+\S+\s+AS\s+build\s*$') "Dockerfile must use a named build stage"
Assert-True ($dockerfile -match '(?im)^USER\s+10001:10001\s*$') "Dockerfile runtime must use UID/GID 10001"
foreach ($binary in @("api", "worker", "scheduler", "migrate")) {
    Assert-True ($dockerfile -match ("/app/" + [regex]::Escape($binary))) "Dockerfile must install /app/$binary"
}
Assert-True ($dockerfile -match '(?m)-o\s+/out/migrate\s+\./cmd/migrate') "Dockerfile must build the repository cmd/migrate command"
Assert-True ($dockerfile -notmatch 'golang-migrate/migrate/.*/cmd/migrate') "Runtime migration must not be replaced by an external CLI build"
Assert-True ($dockerfile -match '(?im)^COPY\s+.*db/migrations/\*\.sql\s+/app/migrations/?\s*$') "Dockerfile must copy only SQL migration artifacts"
Assert-True ($dockerfile -notmatch '(?im)^USER\s+(root|0)(:0)?\s*$') "Dockerfile must not switch the runtime back to root"

$composePath = Join-Path $root "deployments/compose.yml"
Assert-True (Test-Path -LiteralPath $composePath -PathType Leaf) "Missing required file: deployments/compose.yml"
$envPath = Join-Path $root $EnvFile
Assert-True (Test-Path -LiteralPath $envPath -PathType Leaf) "Missing environment template: $EnvFile"

$json = & docker compose --env-file $envPath -f $composePath config --format json
if ($LASTEXITCODE -ne 0) {
    throw "docker compose config failed with exit code $LASTEXITCODE"
}
$config = $json | ConvertFrom-Json
$requiredServices = @("postgres", "redis", "migrate", "api", "worker", "scheduler")
foreach ($service in $requiredServices) {
    Assert-True ($null -ne $config.services.$service) "Compose service missing: $service"
}
Assert-True ($null -eq $config.services.postgres.ports) "PostgreSQL must not publish host ports"
Assert-True ($null -eq $config.services.redis.ports) "Redis must not publish host ports"
Assert-True ($config.services.postgres.image -match '^postgres:15(?:-|$)') "Compose must use PostgreSQL 15"
Assert-True ($config.services.redis.image -match '^redis:7(?:-|$)') "Compose must use Redis 7"
Assert-True (($config.services.redis.command -join " ") -match '(?i)--appendonly\s+yes') "Redis must enable AOF"
Assert-True ($config.networks.backend.internal -eq $true) "Backend service network must be private/internal"
Assert-True ($config.services.migrate.depends_on.postgres.condition -eq "service_healthy") "Migrate must wait for PostgreSQL health"
Assert-True ($config.services.migrate.entrypoint[0] -eq "/app/migrate") "Migrate service must execute the repository migration binary"
Assert-True ($config.services.migrate.command[0] -eq "up") "Migrate service must run the up command"
foreach ($service in @("api", "worker", "scheduler")) {
    Assert-True ($config.services.$service.depends_on.migrate.condition -eq "service_completed_successfully") "$service must wait for successful migrations"
    Assert-True ($config.services.$service.depends_on.postgres.condition -eq "service_healthy") "$service must wait for PostgreSQL health"
    Assert-True ($config.services.$service.depends_on.redis.condition -eq "service_healthy") "$service must wait for Redis health"
    Assert-True ($config.services.$service.user -eq "10001:10001") "$service must run as UID/GID 10001"
    Assert-True ($config.services.$service.read_only -eq $true) "$service root filesystem must be read-only"
    Assert-True ($config.services.$service.security_opt -contains "no-new-privileges:true") "$service must disable privilege escalation"
}
foreach ($name in @("ADMIN_EMAIL", "ADMIN_USERNAME", "ADMIN_PASSWORD")) {
    Assert-True (-not [string]::IsNullOrWhiteSpace($config.services.api.environment.$name)) "API environment missing: $name"
}

$deliveryFiles = @(
    "Dockerfile",
    "deployments/compose.yml",
    ".env.example",
    ".github/workflows/ci.yml"
)
$secretPatterns = @(
    ('-----BEGIN ' + 'PRIVATE KEY-----'),
    ('gh' + 'p_[A-Za-z0-9]{30,}'),
    ('AK' + 'IA[0-9A-Z]{16}'),
    ('(?i)(password|secret|token)\s*[:=]\s*["'']?(?!replace-|example-|\$\{)[A-Za-z0-9+/=_-]{24,}')
)
foreach ($relative in $deliveryFiles) {
    $content = Read-Required $relative
    foreach ($pattern in $secretPatterns) {
        Assert-True ($content -notmatch $pattern) "Potential secret found in $relative"
    }
}

$workflow = Read-Required ".github/workflows/ci.yml"
foreach ($gate in @(
    'go test -count=1 ./...',
    'go test -race -count=1 ./...',
    'go vet ./...',
    'go build ./...',
    'staticcheck@v0.7.0 ./...',
    'govulncheck@v1.6.0 ./...',
    './db/verify-sqlc.ps1',
    'go test -count=1 ./db/migrations',
    './scripts/docker-smoke.ps1'
)) {
    Assert-True ($workflow.Contains($gate)) "CI gate missing: $gate"
}

Write-Host "Delivery configuration verification passed."
