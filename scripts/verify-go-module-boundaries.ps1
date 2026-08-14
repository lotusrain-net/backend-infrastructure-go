[CmdletBinding()]
param()

$packages = @(go list ./...)
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

$webPackages = @($packages | Where-Object {
    $_ -eq "github.com/jyysy/backend-infrastructure-go/web" -or $_ -like "github.com/jyysy/backend-infrastructure-go/web/*"
})
if ($webPackages.Count -gt 0) {
    Write-Error "Root Go module must not discover web packages: $($webPackages -join ', ')"
    exit 1
}
