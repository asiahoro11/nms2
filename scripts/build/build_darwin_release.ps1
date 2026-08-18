$OutputEncoding = [System.Text.Encoding]::UTF8
$ConfirmPreference = "None"
$ErrorActionPreference = "Stop"

# Set encoding to UTF-8
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

$workspaceRoot = $PSScriptRoot
$backendDir = Join-Path $workspaceRoot "backend"
$frontendDir = Join-Path $workspaceRoot "frontend"
$staticEmbedDir = Join-Path $backendDir "static"

$configPath = Join-Path $backendDir "config\config.go"
$version = "unknown"

# Extract version from config.go
if (Test-Path $configPath) {
    $content = Get-Content $configPath -Raw
    if ($content -match 'var Version = "([^"]+)"') {
        $version = $matches[1]
        Write-Host "Detected Version: $version" -ForegroundColor Cyan
    }
}

if ($version -eq "unknown") {
    Write-Warning "Could not detect version from config.go. Using 'latest'."
    $version = "latest"
}

$releaseRoot = Join-Path $workspaceRoot "darwin_release"
$targetDir = Join-Path $releaseRoot $version

Write-Host ">>> Starting macOS (Darwin) Release Build for version: $version" -ForegroundColor Green

# 1. Clean and Create Target Directory
if (Test-Path $targetDir) {
    Write-Host "Cleaning existing release directory: $targetDir"
    Get-ChildItem -Path $targetDir -Recurse | Remove-Item -Force -Recurse
}
else {
    New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
}

# 2. Use existing static assets if they were already prepared by linux build
# This saves time on obfuscation
if (-not (Test-Path $staticEmbedDir)) {
    Write-Host "2. Preparing Embedded Assets..."
    New-Item -ItemType Directory -Path $staticEmbedDir -Force | Out-Null
    Copy-Item -Recurse -Path (Join-Path $frontendDir "*") -Destination $staticEmbedDir
    
    # Simple obfuscation if linux build hasn't run yet
    $jsFiles = Get-ChildItem -Path (Join-Path $staticEmbedDir "js") -Filter "*.js"
    foreach ($file in $jsFiles) {
        Write-Host "   Securing: $($file.Name)"
        cmd /c "npx -y javascript-obfuscator `"$($file.FullName)`" --output `"$($file.FullName)`" --compact true"
    }
}
else {
    Write-Host "2. Using already prepared static assets (for performance)" -ForegroundColor Yellow
}

# 3. Build Backend (Darwin amd64)
Write-Host "3. Building for macOS Intel (amd64)..."
Push-Location $backendDir
try {
    $env:GOOS = "darwin"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"
    go build -ldflags="-s -w" -o (Join-Path $targetDir "nms_server_darwin_amd64") .
    Write-Host "   macOS Intel built." -ForegroundColor Green
}
catch {
    Write-Error "macOS Intel build failed: $_"
}
finally {
    $env:GOOS = $null
    $env:GOARCH = $null
}

# 4. Build Backend (Darwin arm64 - M1/M2/M3)
Write-Host "4. Building for macOS Apple Silicon (arm64)..."
try {
    $env:GOOS = "darwin"
    $env:GOARCH = "arm64"
    $env:CGO_ENABLED = "0"
    go build -ldflags="-s -w" -o (Join-Path $targetDir "nms_server_darwin_arm64") .
    Write-Host "   macOS Apple Silicon built." -ForegroundColor Green
}
catch {
    Write-Error "macOS Apple Silicon build failed: $_"
}
finally {
    $env:GOOS = $null
    $env:GOARCH = $null
    Pop-Location
}

# 5. Create Data Directory
New-Item -ItemType Directory -Force -Path (Join-Path $targetDir "data") | Out-Null

# 6. Include start script
$startSh = Join-Path $workspaceRoot "start_nms_utf8.sh"
if (Test-Path $startSh) {
    Copy-Item -Path $startSh -Destination (Join-Path $targetDir "start_nms.sh") -Force
}

Write-Host ">>> macOS Build Complete!" -ForegroundColor Green
Write-Host ">>> Artifacts are in: $targetDir" -ForegroundColor White
