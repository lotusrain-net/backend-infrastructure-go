param(
    [string]$EnvFile = ".env.example",
    [string]$ProjectName = "backend-infrastructure-go-smoke"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$compose = Join-Path $root "deployments/compose.yml"
$environment = Join-Path $root $EnvFile
$arguments = @("compose", "--project-name", $ProjectName, "--env-file", $environment, "-f", $compose)
$overriddenEnvironment = @(
    "POSTGRES_PASSWORD", "JWT_SECRET", "AUTHENTICATION_KEY", "AUTHENTICATION_PEPPER",
    "ADMIN_PASSWORD", "SCHEDULER_REFRESH_INTERVAL",
    "E2E_BASE_URL", "E2E_ADMIN_EMAIL", "E2E_ADMIN_PASSWORD"
)
$previousEnvironment = @{}
foreach ($name in $overriddenEnvironment) {
    $previousEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
}

function New-RandomHex([int]$ByteCount) {
    $bytes = New-Object byte[] $ByteCount
    $generator = [Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $generator.GetBytes($bytes)
    }
    finally {
        $generator.Dispose()
    }
    return [BitConverter]::ToString($bytes).Replace("-", "").ToLowerInvariant()
}

$env:POSTGRES_PASSWORD = "smoke-" + (New-RandomHex 24)
$env:JWT_SECRET = New-RandomHex 32
$env:AUTHENTICATION_KEY = New-RandomHex 32
$env:AUTHENTICATION_PEPPER = New-RandomHex 32
$env:ADMIN_PASSWORD = "smoke-" + (New-RandomHex 16)
$env:SCHEDULER_REFRESH_INTERVAL = "1s"

function Invoke-Compose([string[]]$Command) {
    & docker @arguments @Command
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose $($Command -join ' ') failed with exit code $LASTEXITCODE"
    }
}

function Invoke-ComposeWithRetry(
    [string[]]$Command,
    [int]$MaxAttempts = 5
) {
    for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
        & docker @arguments @Command
        if ($LASTEXITCODE -eq 0) {
            return
        }

        $exitCode = $LASTEXITCODE
        if ($attempt -eq $MaxAttempts) {
            throw "docker compose $($Command -join ' ') failed after $MaxAttempts attempts with exit code $exitCode"
        }

        $backoff = [Math]::Min(60, 5 * [Math]::Pow(2, $attempt - 1))
        $delaySeconds = [int]$backoff + (Get-Random -Minimum 0 -Maximum 4)
        Write-Warning "docker compose $($Command -join ' ') failed with exit code $exitCode; retrying in $delaySeconds seconds (attempt $attempt of $MaxAttempts)"
        Start-Sleep -Seconds $delaySeconds
    }
}

function Get-HTTPStatus([string]$URL) {
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $URL -TimeoutSec 15
        return [int]$response.StatusCode
    }
    catch {
        if ($null -ne $_.Exception.Response) {
            return [int]$_.Exception.Response.StatusCode
        }
        throw "HTTP request to $URL failed: $($_.Exception.Message)"
    }
}

try {
    Invoke-Compose @("build", "api", "web")
    foreach ($dependency in @("postgres", "redis")) {
        Invoke-ComposeWithRetry @("pull", "--policy", "missing", $dependency)
    }
    Invoke-Compose @("up", "-d", "--no-build", "--pull", "never", "--wait", "--wait-timeout", "180")

    $postgresUser = (& docker @arguments exec -T postgres printenv POSTGRES_USER | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($postgresUser)) {
        throw "Could not read POSTGRES_USER from the PostgreSQL container"
    }
    $postgresDatabase = (& docker @arguments exec -T postgres printenv POSTGRES_DB | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($postgresDatabase)) {
        throw "Could not read POSTGRES_DB from the PostgreSQL container"
    }

    Invoke-Compose @("exec", "-T", "postgres", "pg_isready", "-U", $postgresUser, "-d", $postgresDatabase)
    Invoke-Compose @("exec", "-T", "redis", "redis-cli", "ping")

    $migrationCountRaw = & docker @arguments exec -T postgres psql -U $postgresUser -d $postgresDatabase -Atc "SELECT count(*) FROM schema_migrations;"
    $migrationCount = 0
    if ($LASTEXITCODE -ne 0 -or -not [int]::TryParse(($migrationCountRaw | Out-String).Trim(), [ref]$migrationCount) -or $migrationCount -lt 1) {
        throw "Database migrations were not applied"
    }

    $apiEndpoint = (& docker @arguments port api 8080 | Out-String).Trim()
    $apiEndpointMatch = [regex]::Match($apiEndpoint, ':(\d+)$')
    if ($LASTEXITCODE -ne 0 -or -not $apiEndpointMatch.Success) {
        throw "Could not determine the published API port"
    }
    $webEndpoint = (& docker @arguments port web 3000 | Out-String).Trim()
    $webEndpointMatch = [regex]::Match($webEndpoint, ':(\d+)$')
    if ($LASTEXITCODE -ne 0 -or -not $webEndpointMatch.Success) {
        throw "Could not determine the published web port"
    }
    $resolvedComposeJSON = & docker @arguments config --format json
    if ($LASTEXITCODE -ne 0) {
        throw "Could not resolve the Compose configuration"
    }
    $resolvedCompose = $resolvedComposeJSON | ConvertFrom-Json
    $adminEmail = [string]$resolvedCompose.services.'seed-admin'.environment.ADMIN_EMAIL
    $adminPassword = $env:ADMIN_PASSWORD
    if ([string]::IsNullOrWhiteSpace($adminEmail) -or [string]::IsNullOrWhiteSpace($adminPassword)) {
        throw "Could not resolve E2E administrator credentials"
    }
    $env:E2E_BASE_URL = "http://127.0.0.1:$($apiEndpointMatch.Groups[1].Value)"
    $env:E2E_ADMIN_EMAIL = $adminEmail
    $env:E2E_ADMIN_PASSWORD = $adminPassword
    $webBaseURL = "http://127.0.0.1:$($webEndpointMatch.Groups[1].Value)"
    if ((Get-HTTPStatus "$webBaseURL/api/healthz") -ne 200) {
        throw "Web proxy health endpoint did not return 200"
    }
    $webLoginBody = @{ email = $adminEmail; password = $adminPassword } | ConvertTo-Json -Compress
    $webSession = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    $webLogin = Invoke-WebRequest -UseBasicParsing -WebSession $webSession -Uri "$webBaseURL/api/v1/auth/login" -Method Post -ContentType "application/json" -Body $webLoginBody -TimeoutSec 15
    if ($webLogin.StatusCode -ne 200) {
        throw "Web same-origin login did not return 200"
    }
    $webLoginCookies = @($webLogin.Headers["Set-Cookie"]) -join "`n"
    if ($webLoginCookies -notmatch "access_token=") {
        throw "Web same-origin login did not return an access cookie"
    }
    if ($webLoginCookies -notmatch "refresh_token=") {
        throw "Web same-origin login did not return a refresh cookie"
    }
    $webCurrentUser = Invoke-WebRequest -UseBasicParsing -WebSession $webSession -Uri "$webBaseURL/api/v1/users/me" -TimeoutSec 15
    if ($webCurrentUser.StatusCode -ne 200) {
        throw "Web same-origin session cookies were not replayed to the authenticated user endpoint"
    }
    Push-Location $root
    try {
        & go run ./scripts/verification
        if ($LASTEXITCODE -ne 0) {
            throw "Live E2E verification failed with exit code $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }

    $cronSQL = @'
WITH definition AS (
    INSERT INTO task_definitions (name, task_type, description, default_payload, max_retries, timeout_seconds)
    VALUES ('smoke-cron', 'system.test', 'Docker smoke Cron', jsonb_build_object('processed_rows', 2), 1, 30)
    RETURNING id
), schedule AS (
    INSERT INTO task_schedules (definition_id, cron_expression, timezone, payload, is_enabled)
    SELECT id, '@every 2s', 'UTC', jsonb_build_object('processed_rows', 2), TRUE FROM definition
    RETURNING definition_id
)
SELECT definition_id FROM schedule;
'@
    $cronOutput = (& docker @arguments exec -T postgres psql -U $postgresUser -d $postgresDatabase -Atc $cronSQL | Out-String).Trim()
    $cronDefinitionID = if ($cronOutput -match '(?im)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$') { $Matches[0] } else { "" }
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($cronDefinitionID)) {
        throw "Could not create the Cron verification schedule"
    }
    $cronSucceeded = 0
    $cronDeadline = [DateTime]::UtcNow.AddSeconds(30)
    do {
        $cronCountRaw = & docker @arguments exec -T postgres psql -U $postgresUser -d $postgresDatabase -Atc "SELECT count(*) FROM task_executions WHERE definition_id = '$cronDefinitionID'::uuid AND status = 'succeeded';"
        if ($LASTEXITCODE -eq 0) {
            [void][int]::TryParse(($cronCountRaw | Out-String).Trim(), [ref]$cronSucceeded)
        }
        if ($cronSucceeded -gt 0) {
            break
        }
        Start-Sleep -Seconds 1
    } while ([DateTime]::UtcNow -lt $cronDeadline)
    if ($cronSucceeded -lt 1) {
        throw "Cron dispatcher did not create an execution processed by the worker"
    }

    $workerMetrics = & docker @arguments exec -T postgres wget -qO- http://worker:9090/metrics
    if ($LASTEXITCODE -ne 0 -or ($workerMetrics -join "`n") -notmatch 'backend_task_executions_total\{[^}]*status="succeeded"[^}]*task_type="system\.test"') {
        throw "Worker metrics endpoint did not expose a succeeded task execution"
    }
    $schedulingAuditRaw = & docker @arguments exec -T postgres psql -U $postgresUser -d $postgresDatabase -Atc "SELECT count(*) FROM audit_logs WHERE action = 'scheduling.refresh' AND result = 'success';"
    $schedulingAuditCount = 0
    if ($LASTEXITCODE -ne 0 -or -not [int]::TryParse(($schedulingAuditRaw | Out-String).Trim(), [ref]$schedulingAuditCount) -or $schedulingAuditCount -lt 1) {
        throw "Scheduler did not persist a successful scheduling.refresh audit record"
    }

    Invoke-Compose @("stop", "-t", "30", "worker", "scheduler")
    $readinessURL = "$env:E2E_BASE_URL/health/ready"
    foreach ($dependency in @("redis", "postgres")) {
        Invoke-Compose @("stop", $dependency)
        if ((Get-HTTPStatus $readinessURL) -ne 503) {
            throw "Readiness did not return 503 during $dependency outage"
        }
        Invoke-Compose @("up", "-d", "--no-build", "--pull", "never", "--wait", "--wait-timeout", "60", $dependency)
        $ready = $false
        $readyDeadline = [DateTime]::UtcNow.AddSeconds(30)
        do {
            if ((Get-HTTPStatus $readinessURL) -eq 200) {
                $ready = $true
                break
            }
            Start-Sleep -Seconds 1
        } while ([DateTime]::UtcNow -lt $readyDeadline)
        if (-not $ready) {
            throw "Readiness did not recover after $dependency restart"
        }
    }
    Invoke-Compose @("stop", "-t", "30", "api")
    foreach ($service in @("api", "worker", "scheduler")) {
        $logs = & docker @arguments logs $service 2>&1
        if ($LASTEXITCODE -ne 0 -or ($logs -join "`n") -notmatch 'service stopped') {
            throw "$service did not log graceful shutdown"
        }
        if (($logs -join "`n") -match '(?i)(panic:|fatal|redis connection is shared)') {
            throw "$service logged a fatal shutdown error"
        }
        $containerID = (& docker @arguments ps -a -q $service | Out-String).Trim()
        $exitCode = (& docker inspect --format '{{.State.ExitCode}}' $containerID | Out-String).Trim()
        if ($LASTEXITCODE -ne 0 -or $exitCode -ne "0") {
            throw "$service exited with code $exitCode"
        }
    }
    Write-Host "Docker Compose smoke test passed."
}
finally {
    & docker @arguments down --volumes --remove-orphans
    foreach ($name in $overriddenEnvironment) {
        [Environment]::SetEnvironmentVariable($name, $previousEnvironment[$name], "Process")
    }
}
