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
foreach ($binary in @("api", "worker", "scheduler", "migrate", "seed-admin")) {
    Assert-True ($dockerfile -match ("/app/" + [regex]::Escape($binary))) "Dockerfile must install /app/$binary"
}
Assert-True ($dockerfile -match '(?m)-o\s+/out/migrate\s+\./cmd/migrate') "Dockerfile must build the repository cmd/migrate command"
Assert-True ($dockerfile -match '(?m)-o\s+/out/seed-admin\s+\./cmd/seed-admin') "Dockerfile must build the repository cmd/seed-admin command"
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
$requiredServices = @("postgres", "redis", "migrate", "seed-admin", "api", "worker", "scheduler")
foreach ($service in $requiredServices) {
    Assert-True ($null -ne $config.services.$service) "Compose service missing: $service"
}
Assert-True ($null -eq $config.services.postgres.ports) "PostgreSQL must not publish host ports"
Assert-True ($null -eq $config.services.redis.ports) "Redis must not publish host ports"
Assert-True ($config.services.postgres.image -match '^postgres:15(?:-|$)') "Compose must use PostgreSQL 15"
Assert-True ($config.services.redis.image -match '^redis:7(?:-|$)') "Compose must use Redis 7"
Assert-True (($config.services.redis.command -join " ") -match '(?i)--appendonly\s+yes') "Redis must enable AOF"
Assert-True ($config.networks.backend.internal -eq $true) "Backend service network must be private/internal"
Assert-True ($null -ne $config.networks.edge) "Compose must define an edge network for the published API port"
Assert-True ($config.networks.edge.internal -ne $true) "API edge network must permit host port publishing"
Assert-True ($config.services.api.networks.PSObject.Properties.Name -contains "backend") "API must remain connected to the private backend network"
Assert-True ($config.services.api.networks.PSObject.Properties.Name -contains "edge") "API must connect to the edge network for host access"
foreach ($service in @("postgres", "redis", "migrate", "seed-admin", "worker", "scheduler")) {
    Assert-True ($config.services.$service.networks.PSObject.Properties.Name -notcontains "edge") "$service must not connect to the edge network"
}
Assert-True ($config.services.migrate.depends_on.postgres.condition -eq "service_healthy") "Migrate must wait for PostgreSQL health"
Assert-True ($config.services.migrate.entrypoint[0] -eq "/app/migrate") "Migrate service must execute the repository migration binary"
Assert-True ($config.services.migrate.command[0] -eq "up") "Migrate service must run the up command"
Assert-True ($config.services.migrate.environment.MIGRATIONS_SOURCE -eq "file:///app/migrations") "Migrate service must read migrations from the container image path"
Assert-True ($config.services.'seed-admin'.depends_on.migrate.condition -eq "service_completed_successfully") "Admin seed must wait for successful migrations"
Assert-True ($config.services.'seed-admin'.entrypoint[0] -eq "/app/seed-admin") "Admin seed service must execute the repository seed binary"
Assert-True ($config.services.api.depends_on.'seed-admin'.condition -eq "service_completed_successfully") "API must wait for successful administrator seeding"
Assert-True ($config.services.worker.environment.WORKER_METRICS_ADDR -eq ":9090") "Worker must listen on the internal metrics port"
Assert-True ($config.services.worker.expose -contains "9090") "Worker metrics port must be exposed to the Compose network"
foreach ($service in @("api", "worker", "scheduler")) {
    Assert-True ($config.services.$service.depends_on.postgres.condition -eq "service_healthy") "$service must wait for PostgreSQL health"
    Assert-True ($config.services.$service.depends_on.redis.condition -eq "service_healthy") "$service must wait for Redis health"
    Assert-True ($config.services.$service.user -eq "10001:10001") "$service must run as UID/GID 10001"
    Assert-True ($config.services.$service.read_only -eq $true) "$service root filesystem must be read-only"
    Assert-True ($config.services.$service.security_opt -contains "no-new-privileges:true") "$service must disable privilege escalation"
}
Assert-True ($null -ne $config.services.api.build) "API service must own the single application image build"
foreach ($service in @("migrate", "seed-admin", "worker", "scheduler")) {
    Assert-True ($null -eq $config.services.$service.build) "$service must reuse the application image without declaring another build"
    Assert-True ($config.services.$service.image -eq $config.services.api.image) "$service must reuse the API application image"
}
foreach ($name in @("ADMIN_EMAIL", "ADMIN_USERNAME", "ADMIN_PASSWORD")) {
    Assert-True (-not [string]::IsNullOrWhiteSpace($config.services.'seed-admin'.environment.$name)) "Admin seed environment missing: $name"
    Assert-True ($config.services.api.environment.PSObject.Properties.Name -notcontains $name) "API must not retain bootstrap secret $name"
}
Assert-True (-not [string]::IsNullOrWhiteSpace($config.services.api.environment.JWT_SECRET)) "API environment missing: JWT_SECRET"
foreach ($service in @("migrate", "worker", "scheduler")) {
    foreach ($name in @("JWT_SECRET", "ADMIN_EMAIL", "ADMIN_USERNAME", "ADMIN_PASSWORD")) {
        Assert-True ($config.services.$service.environment.PSObject.Properties.Name -notcontains $name) "$service must not receive API secret $name"
    }
}
Assert-True ($config.services.'seed-admin'.environment.PSObject.Properties.Name -notcontains "JWT_SECRET") "Admin seed must not receive the API JWT secret"

$smoke = Read-Required "scripts/docker-smoke.ps1"
Assert-True ($smoke -match 'Invoke-Compose\s+@\("build",\s*"api"\)') "Docker smoke test must build the application image exactly once through the API service"
Assert-True ($smoke -match 'Invoke-Compose\s+@\("up",\s*"-d",\s*"--no-build"') "Docker smoke test must start services without triggering duplicate image builds"
Assert-True ($smoke -notmatch 'Invoke-Compose\s+@\("up"[^\r\n]*"--build"') "Docker smoke test must not build every service during compose up"
Assert-True ($smoke -match '@arguments\s+exec\s+-T\s+postgres\s+printenv\s+POSTGRES_USER') "Docker smoke test must read the configured PostgreSQL user without shell interpolation"
Assert-True ($smoke -match '@arguments\s+exec\s+-T\s+postgres\s+printenv\s+POSTGRES_DB') "Docker smoke test must read the configured PostgreSQL database without shell interpolation"
Assert-True ($smoke -notmatch 'exec[^\r\n]*postgres[^\r\n]*sh[^\r\n]*-ec') "Docker smoke PostgreSQL checks must avoid nested shell quoting"
Assert-True ($smoke -match 'go\s+run\s+\./scripts/verification') "Docker smoke test must run the live login/RBAC/task/audit workflow"
Assert-True ($smoke -match 'INSERT\s+INTO\s+task_schedules') "Docker smoke test must create a real Cron schedule"
Assert-True ($smoke -match "task_executions[^\r\n]+status[^\r\n]+succeeded") "Docker smoke test must prove the Cron execution was processed by the worker"
Assert-True ($smoke -match 'backend_task_executions_total[^\r\n]+succeeded') "Docker smoke test must scrape a succeeded task metric from the worker"
Assert-True ($smoke -match "audit_logs[^\r\n]+scheduling\.refresh") "Docker smoke test must prove Scheduler audit records are persisted"
Assert-True ($smoke -match 'foreach\s*\(\$dependency\s+in\s+@\("redis",\s*"postgres"\)\)') "Docker smoke test must exercise Redis and PostgreSQL outage behavior"
Assert-True ($smoke -match 'Invoke-Compose\s+@\("stop",\s*\$dependency\)') "Docker smoke test must stop each dependency during outage verification"
Assert-True ($smoke -match 'health/ready') "Docker smoke test must probe readiness during dependency outages"
Assert-True ($smoke -match 'StatusServiceUnavailable|503') "Docker smoke test must require readiness to fail closed"
Assert-True ($smoke -match 'foreach\s*\(\$service\s+in\s+@\("api",\s*"worker",\s*"scheduler"\)\)') "Docker smoke test must verify graceful shutdown for every service"
Assert-True ($smoke -match 'State\.ExitCode') "Docker smoke test must verify every service exits successfully"

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
