# UTF-8 編碼設�?
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ConfirmPreference = "None"
# ============================================================================
# Quick Check Script - Verify Release Files
# ============================================================================

$ErrorActionPreference = "Stop"

function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

Write-ColorOutput Cyan "================================================================"
Write-ColorOutput Cyan " Management Server Release Files Quick Check                            "
Write-ColorOutput Cyan "================================================================"
Write-Output ""

$baseDir = $PSScriptRoot
$versions = @("v1.0.8sp1", "v1.0.8sp999")

function Check-Version {
    param(
        [string]$Platform,
        [string]$Version,
        [string]$ReleaseDir
    )
    
    $versionPath = Join-Path $ReleaseDir $Version
    
    Write-ColorOutput Yellow "$Platform - $Version"
    
    if (-not (Test-Path $versionPath)) {
        Write-ColorOutput Red "  ERROR: Version directory not found!"
        return
    }
    
    # Check server file
    $serverFile = if ($Platform -eq "Linux") { "nms_server" } else { "nms_server.exe" }
    $serverPath = Join-Path $versionPath $serverFile
    
    if (Test-Path $serverPath) {
        $info = Get-Item $serverPath
        $sizeMB = [math]::Round($info.Length / 1MB, 2)
        Write-ColorOutput Green "  OK: $serverFile ($sizeMB MB) - Modified: $($info.LastWriteTime)"
    }
    else {
        Write-ColorOutput Red "  ERROR: $serverFile not found!"
    }
    
    # Check frontend files
    $frontendPath = Join-Path $versionPath "frontend"
    if (Test-Path $frontendPath) {
        $htmlFiles = Get-ChildItem $frontendPath -Filter "*.html"
        $cssFiles = Get-ChildItem (Join-Path $frontendPath "css") -File -ErrorAction SilentlyContinue
        $jsFiles = Get-ChildItem (Join-Path $frontendPath "js") -File -ErrorAction SilentlyContinue
        
        Write-Output "  Frontend:"
        Write-Output "    - HTML: $($htmlFiles.Count) files"
        Write-Output "    - CSS:  $($cssFiles.Count) files"
        Write-Output "    - JS:   $($jsFiles.Count) files"
        
        # Check key files
        $keyFiles = @("index.html", "login.html", "js\admin.js", "js\topology.js")
        foreach ($file in $keyFiles) {
            $filePath = Join-Path $frontendPath $file
            if (Test-Path $filePath) {
                Write-ColorOutput Green "    OK: $file"
            }
            else {
                Write-ColorOutput Red "    MISSING: $file"
            }
        }
    }
    else {
        Write-ColorOutput Red "  ERROR: frontend directory not found!"
    }
    
    # Check data directory
    $dataPath = Join-Path $versionPath "data"
    if (Test-Path $dataPath) {
        Write-ColorOutput Green "  OK: data directory exists"
    }
    else {
        Write-ColorOutput Yellow "  WARNING: data directory not found"
    }
    
    Write-Output ""
}

# Check Linux releases
Write-ColorOutput Cyan "Linux Releases:"
Write-ColorOutput Cyan "----------------------------------------------------------------"
$linuxReleaseDir = Join-Path $baseDir "linux_release"
foreach ($version in $versions) {
    Check-Version -Platform "Linux" -Version $version -ReleaseDir $linuxReleaseDir
}

# Check Windows releases
Write-ColorOutput Cyan "Windows Releases:"
Write-ColorOutput Cyan "----------------------------------------------------------------"
$windowsReleaseDir = Join-Path $baseDir "windows_release"
foreach ($version in $versions) {
    Check-Version -Platform "Windows" -Version $version -ReleaseDir $windowsReleaseDir
}

Write-ColorOutput Green "================================================================"
Write-ColorOutput Green " Check completed!                                               "
Write-ColorOutput Green "================================================================"

