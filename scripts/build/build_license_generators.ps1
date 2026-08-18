$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$backendSource = Join-Path $repoRoot "apps\backend"
$configPath = Join-Path $repoRoot "apps\backend\config\config.go"
$guidePath = Join-Path $repoRoot "docs\LICENSE_GENERATOR_GUIDE_ZH-TW.md"

if (-not (Test-Path $configPath)) {
    throw "config.go not found: $configPath"
}

$configContent = Get-Content $configPath -Raw
$version = if ($configContent -match 'var Version = "([^"]+)"') { $matches[1] } else { "latest" }

$artifactRoot = Join-Path $repoRoot "artifacts\license-generators"
$targetRoot = Join-Path $artifactRoot $version

if (Test-Path $targetRoot) {
    Remove-Item -Recurse -Force $targetRoot
}
New-Item -ItemType Directory -Force -Path $targetRoot | Out-Null

$generator = @{ Name = "license-issuer"; Path = "./cmd/license-issuer" }

$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Dir = "windows-amd64"; Ext = ".exe" },
    @{ GOOS = "linux"; GOARCH = "amd64"; Dir = "linux-amd64"; Ext = "" },
    @{ GOOS = "linux"; GOARCH = "arm64"; Dir = "linux-arm64"; Ext = "" }
)

Push-Location $backendSource
try {
    foreach ($target in $targets) {
        $targetDir = Join-Path $targetRoot $target.Dir
        New-Item -ItemType Directory -Force -Path $targetDir | Out-Null

        $env:GOOS = $target.GOOS
        $env:GOARCH = $target.GOARCH

        $output = Join-Path $targetDir ($generator.Name + $target.Ext)
        go build -trimpath -ldflags="-s -w" -o $output $generator.Path
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $($generator.Name) $($target.GOOS)/$($target.GOARCH)"
        }

        Copy-Item -Force $guidePath (Join-Path $targetDir "LICENSE_GENERATOR_GUIDE_ZH-TW.md")
        if ($target.GOOS -eq "windows") {
            $launcherLines = @(
                "@echo off"
                "chcp 65001 >nul"
                "cd /d `"%~dp0`""
                "license-issuer.exe"
                "echo."
                "pause"
            )
            [System.IO.File]::WriteAllLines(
                (Join-Path $targetDir "start_license_generator.bat"),
                $launcherLines,
                [System.Text.UTF8Encoding]::new($false)
            )
        }
    }
}
finally {
    $env:GOOS = $null
    $env:GOARCH = $null
    Pop-Location
}

@(
    "Product: Management System License Generator"
    "Version: $version"
    "OutputPolicy: Separate artifact; not bundled into NMS server release packages"
    "Platforms: windows-amd64, linux-amd64, linux-arm64"
    "Artifact: license-issuer"
    "Operation: interactive offline wizard; JSON payload automation remains supported"
    "PrivateKeyPolicy: issuer_private.key must be supplied separately and is never packaged"
    "Presets: full, device, camera, iot, pdu, access-control, notification, custom"
    "BuildDate: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz')"
) | Set-Content -Path (Join-Path $targetRoot "LICENSE_GENERATORS_NOTE.txt") -Encoding UTF8

$targetPrefix = (Resolve-Path -LiteralPath $targetRoot).Path.TrimEnd([char[]]@(92, 47)) + [System.IO.Path]::DirectorySeparatorChar
$manifest = Get-ChildItem -Path $targetRoot -Recurse -File | Sort-Object FullName | ForEach-Object {
    [ordered]@{
        path = $_.FullName.Substring($targetPrefix.Length).Replace('\', '/')
        size = $_.Length
        sha256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    }
}
[System.IO.File]::WriteAllText((Join-Path $targetRoot "SHA256_MANIFEST.json"), ($manifest | ConvertTo-Json -Depth 4), [System.Text.UTF8Encoding]::new($false))

$zipPath = Join-Path $artifactRoot ($version + ".zip")
if (Test-Path $zipPath) {
    Remove-Item -Force $zipPath
}
Compress-Archive -Path (Join-Path $targetRoot "*") -DestinationPath $zipPath -CompressionLevel Optimal -Force

Write-Host "License generator artifact ready: $targetRoot"
Write-Host "License generator zip ready: $zipPath"
