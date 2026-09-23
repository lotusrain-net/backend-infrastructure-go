$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$tempBase = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
$tempRoot = Join-Path $tempBase ("backend-infrastructure-go-sqlc-" + [guid]::NewGuid().ToString("N"))
$tempRoot = [System.IO.Path]::GetFullPath($tempRoot)
if (-not $tempRoot.StartsWith($tempBase, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to create SQLC verification directory outside the system temp directory"
}

function Get-GeneratedHashes([string]$Root) {
    $resolvedRoot = [System.IO.Path]::GetFullPath($Root)
    $hashes = @{}
    $sha256 = [System.Security.Cryptography.SHA256]::Create()
    Get-ChildItem -LiteralPath $resolvedRoot -Filter "*.go" -File -Recurse | ForEach-Object {
        $relative = $_.FullName.Substring($resolvedRoot.Length).TrimStart('\', '/')
        $normalized = [System.IO.File]::ReadAllText($_.FullName).Replace("`r`n", "`n")
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($normalized)
        $hashes[$relative] = [BitConverter]::ToString($sha256.ComputeHash($bytes)).Replace("-", "")
    }
    $sha256.Dispose()
    return $hashes
}

try {
    $tempDB = Join-Path $tempRoot "db"
    $tempMigrations = Join-Path $tempRoot "pkg\migrations"
    $tempQueries = Join-Path $tempDB "queries"
    New-Item -ItemType Directory -Path $tempMigrations, $tempQueries -Force | Out-Null
    Get-ChildItem -LiteralPath (Join-Path $repoRoot "pkg\migrations") -Filter "*.sql" -File |
        Copy-Item -Destination $tempMigrations
    Get-ChildItem -LiteralPath (Join-Path $PSScriptRoot "queries") -Filter "*.sql" -File |
        Copy-Item -Destination $tempQueries
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot "sqlc.yaml") -Destination $tempDB

    & go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate -f (Join-Path $tempDB "sqlc.yaml")
    if ($LASTEXITCODE -ne 0) {
        throw "SQLC generation failed with exit code $LASTEXITCODE"
    }

    $committedRoot = Join-Path $repoRoot "pkg\platform\database\dbgen"
    $generatedRoot = Join-Path $tempRoot "pkg\platform\database\dbgen"
    $committed = Get-GeneratedHashes $committedRoot
    $generated = Get-GeneratedHashes $generatedRoot
    $allFiles = @($committed.Keys + $generated.Keys | Sort-Object -Unique)
    $drift = @($allFiles | Where-Object {
        -not $committed.ContainsKey($_) -or
        -not $generated.ContainsKey($_) -or
        $committed[$_] -ne $generated[$_]
    })
    if ($drift.Count -gt 0) {
        throw "SQLC generated output is stale: $($drift -join ', ')"
    }
}
finally {
    if (Test-Path -LiteralPath $tempRoot) {
        $resolved = [System.IO.Path]::GetFullPath($tempRoot)
        if (-not $resolved.StartsWith($tempBase, [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "Refusing to remove SQLC verification directory outside the system temp directory"
        }
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
