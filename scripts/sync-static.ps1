$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$source = Join-Path $root "apps\\frontend"
$targets = @(
    (Join-Path $root "apps\\backend\\static"),
    (Join-Path $root "apps\\backend\\cmd\\agent\\static")
)

foreach ($target in $targets) {
    $parent = Split-Path -Parent $target
    if (-not (Test-Path $parent)) {
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
    }

    if (Test-Path $target) {
        Remove-Item -Recurse -Force $target
    }

    Copy-Item $source $target -Recurse -Force
}

Write-Host "Static assets synced from apps/frontend."
