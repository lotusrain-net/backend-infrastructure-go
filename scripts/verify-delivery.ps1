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
Assert-True ($dockerfile -match '(?im)^ARG\s+IMAGE_REGISTRY=public\.ecr\.aws/docker\s*$') "Dockerfile must define the default container registry"
Assert-True ($dockerfile -match '(?im)^FROM\s+\$\{IMAGE_REGISTRY\}/library/golang:1\.26\.6\s+AS\s+build\s*$') "Dockerfile must use IMAGE_REGISTRY for the Go build image"
Assert-True ($dockerfile -match '(?im)^ARG\s+GOPROXY=https://goproxy\.cn,direct\s*$') "Dockerfile must define the Go module mirror"
Assert-True ($dockerfile -match '(?im)^ARG\s+GOSUMDB=sum\.golang\.google\.cn\s*$') "Dockerfile must define the Go checksum database"
Assert-True ($dockerfile -match '(?im)^USER\s+10001:10001\s*$') "Dockerfile runtime must use UID/GID 10001"
foreach ($binary in @("api", "worker", "scheduler", "migrate", "seed-admin")) {
    Assert-True ($dockerfile -match ("/app/" + [regex]::Escape($binary))) "Dockerfile must install /app/$binary"
}
Assert-True ($dockerfile -match '(?m)-o\s+/out/migrate\s+\./cmd/migrate') "Dockerfile must build the repository cmd/migrate command"
Assert-True ($dockerfile -match '(?m)-o\s+/out/seed-admin\s+\./cmd/seed-admin') "Dockerfile must build the repository cmd/seed-admin command"
Assert-True ($dockerfile -notmatch 'golang-migrate/migrate/.*/cmd/migrate') "Runtime migration must not be replaced by an external CLI build"
Assert-True ($dockerfile -match '(?im)^COPY\s+.*db/migrations/\*\.sql\s+/app/migrations/?\s*$') "Dockerfile must copy only SQL migration artifacts"
Assert-True ($dockerfile -notmatch '(?im)^USER\s+(root|0)(:0)?\s*$') "Dockerfile must not switch the runtime back to root"

$webDockerfile = Read-Required "web/Dockerfile"
Assert-True ($webDockerfile -match '(?im)^ARG\s+IMAGE_REGISTRY=public\.ecr\.aws/docker\s*$') "Web Dockerfile must define the default container registry"
Assert-True ($webDockerfile -match '(?im)^FROM\s+\$\{IMAGE_REGISTRY\}/library/node:20-alpine\s+AS\s+deps\s*$') "Web Dockerfile must use Node 20 through IMAGE_REGISTRY"
Assert-True ($webDockerfile -notmatch '(?im)^FROM\s+node:22') "Web Dockerfile must not use Node 22"
Assert-True ($webDockerfile -match '(?im)^ARG\s+API_PROXY_TARGET\s*$') "Web Dockerfile must accept API_PROXY_TARGET at build time"
Assert-True ($webDockerfile -match '(?im)^ENV\s+API_PROXY_TARGET=\$API_PROXY_TARGET\s*$') "Web Dockerfile must expose the build proxy target to Next.js"
Assert-True ($webDockerfile -match '(?im)^USER\s+10001:10001\s*$') "Web Dockerfile runtime must use UID/GID 10001"
Assert-True ($webDockerfile -notmatch '(?im)^COPY\s+--from=builder\s+/app/public\s+') "Web Dockerfile must not copy a missing public directory"

$webHealthRoute = Read-Required "web/src/app/api/healthz/route.ts"
Assert-True ($webHealthRoute -match 'health/ready') "Web health route must check API readiness"
Assert-True ($webHealthRoute -match 'status:\s*["'']ok["'']') "Web health route must return an OK response"

$nextConfig = Read-Required "web/next.config.ts"
Assert-True ($nextConfig -match 'NODE_ENV\s*===\s*["'']production["'']') "Next.js config must distinguish production builds from local development"
Assert-True ($nextConfig -match 'throw new Error\([^\r\n]*API_PROXY_TARGET') "Next.js config must reject a missing production API proxy target"
Assert-True ($nextConfig -match '127\.0\.0\.1:8080') "Next.js config must retain a local development proxy default"

$eslintConfig = Read-Required "web/eslint.config.mjs"
Assert-True ($eslintConfig -match 'globalIgnores') "ESLint config must use global ignores for generated directories"
foreach ($ignoredPath in @(".next/**", "coverage/**", "node_modules_stale_*/**", ".node_modules_stale_*/**")) {
    Assert-True ($eslintConfig.Contains($ignoredPath)) "ESLint config must ignore $ignoredPath"
}
Assert-True ($eslintConfig -notmatch '(?im)["'']src(?:/|\*|["''])') "ESLint config must not hide application source files"

$webRoot = Join-Path $root "web"
$generatedWebDirectoryPattern = '^(?:\.?node_modules[^\\/]*|\.next|coverage|dist)$'
function Get-WebFiles([string]$Directory) {
    foreach ($item in Get-ChildItem -LiteralPath $Directory -Force) {
        if ($item.PSIsContainer) {
            if ($item.Name -match $generatedWebDirectoryPattern -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
                continue
            }
            Get-WebFiles -Directory $item.FullName
            continue
        }
        $item
    }
}
$webFiles = @(Get-WebFiles -Directory $webRoot)
$webRelativePaths = @($webFiles | ForEach-Object {
    $_.FullName.Substring($webRoot.Length) -replace '^[\\/]+' , ''
})
$forbiddenFrontendPatterns = @(
    @{ Label = "Douyin branding"; Pattern = '(?i)(douyin|抖音|tiktok)' },
    @{ Label = "store business domain"; Pattern = '(?i)(\bshop\b|店铺)' },
    @{ Label = "collection business domain"; Pattern = '(?i)(/collect(?:ion)?(?:/|["''`?])|\b(?:data[-_ ]?collection|collection[-_ ]?(?:task|record|source))\b|采集|crawler|crawl)' },
    @{ Label = "Agent business domain"; Pattern = '(?i)(/agents?(?:/|["''`?])|\bagent\b|智能体)' }
)
$webSourceFiles = $webFiles | Where-Object {
    $_.Name -ne "package-lock.json" -and $_.Extension -in @(".ts", ".tsx", ".js", ".mjs", ".cjs", ".css", ".json", ".md", ".yml", ".yaml")
}
foreach ($entry in $forbiddenFrontendPatterns) {
    $pathMatches = $webRelativePaths | Where-Object { $_ -match $entry.Pattern }
    Assert-True ($pathMatches.Count -eq 0) "Frontend contains forbidden $($entry.Label) path or asset: $($pathMatches -join ', ')"

    $contentMatches = $webSourceFiles | Select-String -Pattern $entry.Pattern -List
    Assert-True ($contentMatches.Count -eq 0) "Frontend contains forbidden $($entry.Label) term or API path: $($contentMatches.Path -join ', ')"
}

$composePath = Join-Path $root "deployments/compose.yml"
Assert-True (Test-Path -LiteralPath $composePath -PathType Leaf) "Missing required file: deployments/compose.yml"
$composeSource = Read-Required "deployments/compose.yml"
Assert-True ($composeSource -match 'COOKIE_SECURE:\s*\$\{COOKIE_SECURE:-true\}') "Compose must retain secure-cookie defaults when no environment profile is supplied"
$envPath = Join-Path $root $EnvFile
Assert-True (Test-Path -LiteralPath $envPath -PathType Leaf) "Missing environment template: $EnvFile"

$json = & docker compose --env-file $envPath -f $composePath config --format json
if ($LASTEXITCODE -ne 0) {
    throw "docker compose config failed with exit code $LASTEXITCODE"
}
$config = $json | ConvertFrom-Json
$requiredServices = @("postgres", "redis", "migrate", "seed-admin", "api", "worker", "scheduler", "web")
foreach ($service in $requiredServices) {
    Assert-True ($null -ne $config.services.$service) "Compose service missing: $service"
}
Assert-True ($null -eq $config.services.postgres.ports) "PostgreSQL must not publish host ports"
Assert-True ($null -eq $config.services.redis.ports) "Redis must not publish host ports"
Assert-True ($config.services.postgres.image -match '/library/postgres:15(?:-|$)') "Compose must use PostgreSQL 15 through the configured registry"
Assert-True ($config.services.redis.image -match '/library/redis:7(?:-|$)') "Compose must use Redis 7 through the configured registry"
Assert-True (($config.services.redis.command -join " ") -match '(?i)--appendonly\s+yes') "Redis must enable AOF"
Assert-True ($config.networks.backend.internal -eq $true) "Backend service network must be private/internal"
Assert-True ($null -ne $config.networks.edge) "Compose must define an edge network for the published API port"
Assert-True ($config.networks.edge.internal -ne $true) "API edge network must permit host port publishing"
Assert-True ($config.services.api.networks.PSObject.Properties.Name -contains "backend") "API must remain connected to the private backend network"
Assert-True ($config.services.api.networks.PSObject.Properties.Name -contains "edge") "API must connect to the edge network for host access"
Assert-True ($config.services.web.networks.PSObject.Properties.Name -contains "edge") "Web must connect to the edge network for host access"
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
Assert-True ($null -ne $config.services.api.build) "API service must inherit the shared application image build"
Assert-True (-not [string]::IsNullOrWhiteSpace([string]$config.services.api.build.args.IMAGE_REGISTRY)) "API build must receive a container registry"
Assert-True ($config.services.api.build.args.GOPROXY -eq "https://goproxy.cn,direct") "API build must receive the Go module mirror"
Assert-True ($config.services.api.build.args.GOSUMDB -eq "sum.golang.google.cn") "API build must receive the Go checksum database"
Assert-True ($null -ne $config.services.web.build) "Web service must declare its own frontend build"
Assert-True (-not [string]::IsNullOrWhiteSpace([string]$config.services.web.build.args.IMAGE_REGISTRY)) "Web build must receive a container registry"
Assert-True ($config.services.web.build.context -match '[/\\]web$') "Web service build context must resolve to the repository web directory"
$webBuildProxyTarget = [string]$config.services.web.build.args.API_PROXY_TARGET
$webRuntimeProxyTarget = [string]$config.services.web.environment.API_PROXY_TARGET
Assert-True (-not [string]::IsNullOrWhiteSpace($webBuildProxyTarget)) "Web build must inject an API proxy target"
Assert-True (-not [string]::IsNullOrWhiteSpace($webRuntimeProxyTarget)) "Web runtime must receive an API proxy target"
Assert-True ($webBuildProxyTarget -eq $webRuntimeProxyTarget) "Web build and runtime API proxy targets must match"
Assert-True ($config.services.web.depends_on.api.condition -eq "service_started") "Web must wait for the API container to start"
Assert-True ($config.services.web.user -eq "10001:10001") "Web must run as UID/GID 10001"
Assert-True ($config.services.web.read_only -eq $true) "Web root filesystem must be read-only"
Assert-True ($config.services.web.security_opt -contains "no-new-privileges:true") "Web must disable privilege escalation"
Assert-True ($config.services.web.healthcheck.test -join " " -match '/api/healthz') "Web healthcheck must probe the proxied /api/healthz endpoint"
$webPorts = @($config.services.web.ports)
Assert-True ($webPorts.Count -eq 1) "Web must publish exactly one host port"
Assert-True ($webPorts[0].host_ip -eq "127.0.0.1") "Web must bind only to localhost"
Assert-True ($webPorts[0].target -eq 3000) "Web container must listen on port 3000"
Assert-True ($webPorts[0].published -eq "3000") "Web default published port must be 3000"
Assert-True ($config.services.api.environment.COOKIE_SECURE -eq "false") "The HTTP localhost Compose profile must set COOKIE_SECURE=false so browser sessions can retain authentication cookies"
Assert-True ($config.services.api.environment.APP_ENV -eq "development") "The HTTP localhost Compose profile must identify itself as development"
foreach ($service in @("migrate", "seed-admin", "worker", "scheduler")) {
    Assert-True ($null -ne $config.services.$service.build) "$service must inherit the shared application image build"
    Assert-True ($config.services.$service.build.context -eq $config.services.api.build.context) "$service must use the API build context"
    Assert-True ($config.services.$service.build.dockerfile -eq $config.services.api.build.dockerfile) "$service must use the API Dockerfile"
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
Assert-True ($smoke -match 'Invoke-Compose\s+@\("build",\s*"api",\s*"web"\)') "Docker smoke test must build the API and web images before startup"
Assert-True ($smoke -match 'Invoke-Compose\s+@\("up",\s*"-d",\s*"--no-build"') "Docker smoke test must start services without triggering duplicate image builds"
Assert-True ($smoke -notmatch 'Invoke-Compose\s+@\("up"[^\r\n]*"--build"') "Docker smoke test must not build every service during compose up"
Assert-True ($smoke -match 'foreach\s*\(\$dependency\s+in\s+@\("postgres",\s*"redis"\)\)') "Docker smoke test must pull stateful dependencies explicitly"
Assert-True ($smoke -match 'Invoke-ComposeWithRetry\s+@\("pull",\s*"--policy",\s*"missing",\s*\$dependency\)') "Docker smoke dependency pulls must use bounded retries"
Assert-True ($smoke -match 'Get-Random[^\r\n]+Start-Sleep|Start-Sleep[^\r\n]+\$delaySeconds') "Docker smoke pull retries must use delayed backoff"
Assert-True ($smoke -match 'Invoke-Compose\s+@\("up"[^\r\n]+"--pull",\s*"never"') "Docker smoke startup must not bypass explicit image pulls"
Assert-True ($smoke -match '/api/healthz') "Docker smoke test must verify the web proxy health endpoint"
Assert-True ($smoke -match '\$webBaseURL/api/v1/auth/login') "Docker smoke test must verify same-origin login through the web proxy"
Assert-True ($smoke -match 'access_token=') "Docker smoke test must verify the proxied login returns an access cookie"
Assert-True ($smoke -match 'refresh_token=') "Docker smoke test must verify the proxied login returns a refresh cookie"
Assert-True ($smoke -match 'WebRequestSession') "Docker smoke test must retain same-origin login cookies for an authenticated web request"
Assert-True ($smoke -match '\$webSession') "Docker smoke test must replay the retained same-origin login cookies"
Assert-True ($smoke -match '\$webBaseURL/api/v1/users/me') "Docker smoke test must use the retained session for an authenticated same-origin request"
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
Assert-True ($workflow -match '(?s)docker-e2e:.*?actions/setup-go@v5') "Docker E2E job must install Go before running the smoke verifier"
Assert-True ($workflow -match '(?s)- name: Build\s+env:\s+API_PROXY_TARGET:\s+http://127\.0\.0\.1:8080\s+run: npm run build') "Web CI build must explicitly configure a local API proxy target"
foreach ($gate in @(
    'go test -count=1 ./...',
    'go test -race -count=1 ./...',
    'go vet ./...',
    'go build ./...',
    'npm ci',
    'npm run lint',
    'npm run typecheck',
    'npm run test',
    'npm run build',
    'staticcheck@v0.7.0 ./...',
    'govulncheck@v1.6.0 ./...',
    './db/verify-sqlc.ps1',
    'go test -count=1 ./db/migrations',
    './scripts/docker-smoke.ps1'
)) {
    Assert-True ($workflow.Contains($gate)) "CI gate missing: $gate"
}

Write-Host "Delivery configuration verification passed."
