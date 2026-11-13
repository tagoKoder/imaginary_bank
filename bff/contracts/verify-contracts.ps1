$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

Push-Location $root

try {
    Write-Host "==> Checking bank-contracts dependency"

    $module = "github.com/tagoKoder/bank-contracts"

    go list -m $module

    if ($LASTEXITCODE -ne 0) {
        throw "bank-contracts module is missing"
    }

    Write-Host "==> Checking generated protobuf packages"

    $packages = @(
        "$module/gen/go/bank/accounts/v1",
        "$module/gen/go/bank/identity/v1",
        "$module/gen/go/bank/ledgerpayments/v1"
    )

    foreach ($package in $packages) {
        Write-Host "Checking $package"

        go list $package

        if ($LASTEXITCODE -ne 0) {
            throw "Missing protobuf package: $package"
        }
    }

    Write-Host "==> Contract packages are available"
}
finally {
    Pop-Location
}