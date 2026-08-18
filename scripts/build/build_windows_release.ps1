$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$superAdminOneTimeGuidePath = Join-Path $repoRoot 'docs\WINDOWS_SUPERADMIN_ONE_TIME_SETUP.md'
$updateUpgradeGuidePath = Join-Path $repoRoot 'docs\UPDATE_AND_UPGRADE_GUIDE_ZH-TW.md'
$backendSource = Join-Path $repoRoot "apps\\backend"
$frontendSource = Join-Path $repoRoot "apps\\frontend"
$runtimeBin = Join-Path $repoRoot "runtime\\bin"
$releaseNotesPath = Join-Path $repoRoot "docs\\RELEASE_NOTES.md"
$apiManualPath = Join-Path $repoRoot "docs\\API_MANUAL.md"
$apiManualTwPath = Join-Path $repoRoot "docs\\API_MANUAL_TW.md"
$integrityGuidePath = Join-Path $repoRoot "docs\\SECURITY_INTEGRITY_LOCKDOWN.md"
$iotRoadmapPath = Join-Path $repoRoot "docs\\IOT_UNIVERSAL_INTERFACE_ROADMAP.md"
$superAdminGuidePath = Join-Path $repoRoot "docs\\SUPERADMIN_LOCAL_MAINTENANCE.md"
$licenseMigrationGuidePath = Join-Path $repoRoot "docs\\LICENSE_ED25519_MIGRATION.md"
$examplesPath = Join-Path $repoRoot "docs\\examples"
$tmpRoot = Join-Path $repoRoot "artifacts\\.tmp"
$configPath = Join-Path $backendSource "config\\config.go"
$releaseProfile = [Environment]::GetEnvironmentVariable("NMS_RELEASE_PROFILE")
if ([string]::IsNullOrWhiteSpace($releaseProfile)) {
    $releaseProfile = "protected-release"
}
$releaseProfile = $releaseProfile.Trim().ToLowerInvariant()
$validProfiles = @("protected-internal", "protected-release", "clean")
if (-not ($validProfiles -contains $releaseProfile)) {
    throw "Unsupported NMS_RELEASE_PROFILE '$releaseProfile'. Expected one of: $($validProfiles -join ', ')"
}

$binaryObfuscationOverride = [Environment]::GetEnvironmentVariable("NMS_ENABLE_BINARY_OBFUSCATION")
if ($null -ne $binaryObfuscationOverride) {
    $enableBinaryObfuscation = $binaryObfuscationOverride -ne "0"
}
else {
    $enableBinaryObfuscation = $false
}

$jsObfuscationOverride = [Environment]::GetEnvironmentVariable("NMS_ENABLE_JS_OBFUSCATION")
$legacyJsObfuscationOverride = [Environment]::GetEnvironmentVariable("NMS_ENABLE_OBFUSCATION")
if ($null -ne $jsObfuscationOverride) {
    $enableJsObfuscation = $jsObfuscationOverride -eq "1"
}
elseif ($null -ne $legacyJsObfuscationOverride) {
    $enableJsObfuscation = $legacyJsObfuscationOverride -eq "1"
}
else {
    $enableJsObfuscation = $false
}

$enableJsMinification = [Environment]::GetEnvironmentVariable("NMS_ENABLE_JS_MINIFICATION") -eq "1"
$enableHtmlMinification = [Environment]::GetEnvironmentVariable("NMS_ENABLE_HTML_MINIFICATION") -eq "1"
$licensePublicKeyB64 = [Environment]::GetEnvironmentVariable("NMS_LICENSE_PUBLIC_KEY_B64")
if ($releaseProfile -eq "protected-release") {
    if ([string]::IsNullOrWhiteSpace($licensePublicKeyB64)) {
        throw "protected-release requires NMS_LICENSE_PUBLIC_KEY_B64"
    }
    try {
        $licensePublicKeyBytes = [Convert]::FromBase64String($licensePublicKeyB64.Trim())
    }
    catch {
        throw "NMS_LICENSE_PUBLIC_KEY_B64 must be valid Base64"
    }
    if ($licensePublicKeyBytes.Length -ne 32) {
        throw "NMS_LICENSE_PUBLIC_KEY_B64 must decode to a 32-byte Ed25519 public key"
    }
}
$licenseLdflags = "-s -w"
if (-not [string]::IsNullOrWhiteSpace($licensePublicKeyB64)) {
    $licenseLdflags += " -X management-server/services/license.BuildPublicKeyB64=$licensePublicKeyB64"
}

if (-not (Test-Path $configPath)) {
    throw "config.go not found: $configPath"
}

$configContent = Get-Content $configPath -Raw
$version = if ($configContent -match 'var Version = "([^"]+)"') { $matches[1] } else { "latest" }
$windowsVersionString = $version.TrimStart("v", "V")
$versionNumbers = [regex]::Matches($version, '\d+') | ForEach-Object { [int]$_.Value }
$versionMajor = if ($versionNumbers.Count -gt 0) { $versionNumbers[0] } else { 0 }
$versionMinor = if ($versionNumbers.Count -gt 1) { $versionNumbers[1] } else { 0 }
$versionPatch = if ($versionNumbers.Count -gt 2) { $versionNumbers[2] } else { 0 }
$versionBuild = if ($versionNumbers.Count -gt 3) { $versionNumbers[3] } else { 0 }

$artifactRoot = Join-Path $repoRoot "artifacts\\windows"
$targetDir = Join-Path $artifactRoot $version
$tempDir = Join-Path $tmpRoot ("windows-" + [guid]::NewGuid().ToString("N"))
$tempBackend = Join-Path $tempDir "backend"
$windowsVersionMetadata = $false

New-Item -ItemType Directory -Force -Path $artifactRoot | Out-Null
New-Item -ItemType Directory -Force -Path $tmpRoot | Out-Null
if (Test-Path $targetDir) {
    Get-ChildItem -Force $targetDir | Remove-Item -Recurse -Force
}
else {
    New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
}

try {
    Copy-Item -Recurse -Force $backendSource $tempBackend

    $embedTargets = @(
        (Join-Path $tempBackend "static"),
        (Join-Path $tempBackend "cmd\\agent\\static")
    )

    foreach ($target in $embedTargets) {
        if (Test-Path $target) {
            Remove-Item -Recurse -Force $target
        }
        Copy-Item -Recurse -Force $frontendSource $target
    }

    $npx = Get-Command npx -ErrorAction SilentlyContinue
    if (($enableJsObfuscation -or $enableJsMinification -or $enableHtmlMinification) -and $null -eq $npx) {
        throw "npx was not found. Refusing to build a release that requires frontend processing."
    }

    if ($enableJsObfuscation) {
        Write-Host "Building with heavy JS obfuscation enabled by explicit request." -ForegroundColor Yellow
        foreach ($target in $embedTargets) {
            $jsDir = Join-Path $target "js"
            if (Test-Path $jsDir) {
                $jsFiles = Get-ChildItem -Path $jsDir -Filter "*.js" -Recurse -File
                foreach ($file in $jsFiles) {
                    & $npx.Source -y javascript-obfuscator $file.FullName --output $file.FullName --compact true --string-array true --string-array-encoding base64 --identifier-names-generator hexadecimal --rename-globals false | Out-Null
                }
            }

        }
    }
    elseif ($enableJsMinification) {
        Write-Host "Minifying external JS without heavy obfuscation." -ForegroundColor Yellow
        foreach ($target in $embedTargets) {
            $jsDir = Join-Path $target "js"
            if (Test-Path $jsDir) {
                $jsFiles = Get-ChildItem -Path $jsDir -Filter "*.js" -Recurse -File
                foreach ($file in $jsFiles) {
                    & $npx.Source -y terser $file.FullName --compress --output $file.FullName | Out-Null
                }
            }
        }
    }

    if ($enableHtmlMinification) {
        Write-Host "Minifying HTML and inline CSS/JS." -ForegroundColor Yellow
        foreach ($target in $embedTargets) {
            foreach ($htmlName in @("index.html", "login.html", "monitor.html", "mode-selection.html")) {
                $htmlPath = Join-Path $target $htmlName
                if (Test-Path $htmlPath) {
                    & $npx.Source -y html-minifier-terser $htmlPath -o $htmlPath --collapse-whitespace --remove-comments --remove-redundant-attributes --remove-script-type-attributes --use-short-doctype --minify-css true --minify-js true | Out-Null
                }
            }
        }
    }

    if (-not $enableJsObfuscation) {
        Write-Host "Web JS obfuscation disabled by default for lower antivirus false-positive risk." -ForegroundColor Yellow
    }

    $skipWindowsVersionMetadata = [Environment]::GetEnvironmentVariable("NMS_SKIP_WINDOWS_VERSION_METADATA") -eq "1"
    if ($skipWindowsVersionMetadata) {
        Write-Host "Skipping Windows version metadata by explicit build override." -ForegroundColor Yellow
    }
    else {
    $versionInfoTool = Join-Path $repoRoot "runtime\\tools\\goversioninfo-build.exe"
    if (-not (Test-Path $versionInfoTool)) {
        $versionInfoTool = Join-Path $repoRoot "runtime\\tools\\goversioninfo-local.exe"
    }
    if (-not (Test-Path $versionInfoTool)) {
        $versionInfoTool = Join-Path $repoRoot "runtime\\tools\\goversioninfo.exe"
    }
    if (-not (Test-Path $versionInfoTool)) {
        $versionInfoCommand = Get-Command goversioninfo -ErrorAction SilentlyContinue
        if ($null -ne $versionInfoCommand) {
            $versionInfoTool = $versionInfoCommand.Source
        }
    }
    if (-not (Test-Path $versionInfoTool)) {
        throw "goversioninfo was not found. Install it with: go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest"
    }

    $versionInfoPath = Join-Path $tempBackend "versioninfo.json"
    $resourcePath = Join-Path $tempBackend "resource_windows_amd64.syso"
    $versionInfo = [ordered]@{
        FixedFileInfo = [ordered]@{
            FileVersion = [ordered]@{
                Major = $versionMajor
                Minor = $versionMinor
                Patch = $versionPatch
                Build = $versionBuild
            }
            ProductVersion = [ordered]@{
                Major = $versionMajor
                Minor = $versionMinor
                Patch = $versionPatch
                Build = $versionBuild
            }
            FileFlagsMask = "3f"
            "FileFlags " = "00"
            FileOS = "040004"
            FileType = "01"
            FileSubType = "00"
        }
        StringFileInfo = [ordered]@{
            Comments = "Network Management System server runtime."
            CompanyName = "YTSworks"
            FileDescription = "Management System Server"
            FileVersion = $windowsVersionString
            InternalName = "nms_server"
            LegalCopyright = "Copyright (c) 2026 YTSworks. All rights reserved."
            LegalTrademarks = ""
            OriginalFilename = "nms_server.exe"
            PrivateBuild = ""
            ProductName = "Management System"
            ProductVersion = $windowsVersionString
            SpecialBuild = ""
        }
        VarFileInfo = [ordered]@{
            Translation = [ordered]@{
                LangID = "0404"
                CharsetID = "04B0"
            }
        }
        IconPath = ""
        ManifestPath = ""
    }
    $versionInfoJson = $versionInfo | ConvertTo-Json -Depth 8
    [System.IO.File]::WriteAllText($versionInfoPath, $versionInfoJson, [System.Text.UTF8Encoding]::new($false))
    $prebuiltVersionResource = [Environment]::GetEnvironmentVariable("NMS_WINDOWS_VERSION_RESOURCE")
    if (-not [string]::IsNullOrWhiteSpace($prebuiltVersionResource)) {
        $prebuiltVersionResource = [System.IO.Path]::GetFullPath($prebuiltVersionResource)
        if (-not (Test-Path -LiteralPath $prebuiltVersionResource -PathType Leaf)) {
            throw "NMS_WINDOWS_VERSION_RESOURCE does not exist: $prebuiltVersionResource"
        }
        Copy-Item -LiteralPath $prebuiltVersionResource -Destination $resourcePath -Force
        Write-Host "Using pre-generated Windows version metadata resource." -ForegroundColor Yellow
    }
    else {
        & $versionInfoTool -64 -o $resourcePath $versionInfoPath
        if ($LASTEXITCODE -ne 0 -or -not (Test-Path $resourcePath)) {
            throw "failed to generate Windows version metadata resource"
        }
    }
    $windowsVersionMetadata = $true
    Write-Host "Embedded Windows version metadata: YTSworks / Management System $version" -ForegroundColor Yellow
    }

    Push-Location $tempBackend
    try {
        $env:GOOS = "windows"
        $env:GOARCH = "amd64"
        $env:GOWORK = "off"
        $binaryOutput = Join-Path $targetDir "nms_server.exe"
        $garble = Get-Command garble -ErrorAction SilentlyContinue
        if ($enableBinaryObfuscation -and $null -ne $garble) {
            Write-Host "Building with Go binary obfuscation enabled via garble." -ForegroundColor Yellow
            & $garble.Source build -trimpath -ldflags $licenseLdflags -o $binaryOutput .
        }
        elseif ($enableBinaryObfuscation) {
            throw "NMS_ENABLE_BINARY_OBFUSCATION is enabled but garble was not found. Refusing to build a non-obfuscated binary."
        }
        else {
            go build -trimpath -ldflags $licenseLdflags -o $binaryOutput .
        }
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for windows-amd64"
        }
        $localTools = Join-Path $targetDir "tools"
        New-Item -ItemType Directory -Force -Path $localTools | Out-Null
        go build -trimpath -ldflags $licenseLdflags -o (Join-Path $localTools "superadmin-local.exe") ./cmd/superadmin-local
        if ($LASTEXITCODE -ne 0) { throw "go build failed for superadmin-local.exe" }
    }
    finally {
        $env:GOOS = $null
        $env:GOARCH = $null
        $env:GOWORK = $null
        Pop-Location
    }

    if (-not (Test-Path (Join-Path $targetDir "nms_server.exe"))) {
        throw "missing output binary: nms_server.exe"
    }

    # Optional code signing (opt-in). An unsigned executable from a low-reputation
    # publisher is the single biggest driver of Windows Defender/SmartScreen false
    # positives; sign here once a certificate is available.
    $signThumbprint = [Environment]::GetEnvironmentVariable("NMS_CODE_SIGN_THUMBPRINT")
    $signPfxPath = [Environment]::GetEnvironmentVariable("NMS_CODE_SIGN_PFX")
    $signPfxPassword = [Environment]::GetEnvironmentVariable("NMS_CODE_SIGN_PFX_PASSWORD")
    $signTimestampUrl = [Environment]::GetEnvironmentVariable("NMS_CODE_SIGN_TIMESTAMP_URL")
    if ([string]::IsNullOrWhiteSpace($signTimestampUrl)) {
        $signTimestampUrl = "http://timestamp.digicert.com"
    }
    $codeSigned = $false

    if (-not [string]::IsNullOrWhiteSpace($signThumbprint) -or -not [string]::IsNullOrWhiteSpace($signPfxPath)) {
        $signtool = Get-Command signtool -ErrorAction SilentlyContinue
        if ($null -eq $signtool) {
            $sdkBinRoot = "${env:ProgramFiles(x86)}\Windows Kits\10\bin"
            $sdkCandidates = Get-ChildItem $sdkBinRoot -Directory -ErrorAction SilentlyContinue | Sort-Object Name -Descending
            foreach ($sdkDir in $sdkCandidates) {
                $candidate = Join-Path $sdkDir.FullName "x64\signtool.exe"
                if (Test-Path $candidate) {
                    $signtool = Get-Item $candidate
                    break
                }
            }
        }
        if ($null -eq $signtool) {
            throw "Code signing was requested (NMS_CODE_SIGN_THUMBPRINT/NMS_CODE_SIGN_PFX set) but signtool.exe was not found. Install the Windows SDK or add signtool to PATH."
        }

        $signArgs = @("sign", "/fd", "sha256", "/td", "sha256", "/tr", $signTimestampUrl)
        if (-not [string]::IsNullOrWhiteSpace($signThumbprint)) {
            $signArgs += @("/sha1", $signThumbprint)
        }
        else {
            $signArgs += @("/f", $signPfxPath)
            if (-not [string]::IsNullOrWhiteSpace($signPfxPassword)) {
                $signArgs += @("/p", $signPfxPassword)
            }
        }
        $signArgs += (Join-Path $targetDir "nms_server.exe")

        Write-Host "Signing nms_server.exe with signtool..." -ForegroundColor Yellow
        & $signtool.Source @signArgs
        if ($LASTEXITCODE -ne 0) {
            throw "signtool failed to sign nms_server.exe"
        }
        $codeSigned = $true
        Write-Host "nms_server.exe signed successfully." -ForegroundColor Green
    }
    else {
        Write-Host "Code signing skipped (set NMS_CODE_SIGN_THUMBPRINT or NMS_CODE_SIGN_PFX to enable). Unsigned Windows binaries are the most common cause of Defender/SmartScreen false positives." -ForegroundColor Yellow
    }

    New-Item -ItemType Directory -Force -Path (Join-Path $targetDir "data") | Out-Null
    $targetBin = Join-Path $targetDir "bin"
    New-Item -ItemType Directory -Force -Path $targetBin | Out-Null
    foreach ($helperName in @("ffmpeg.exe", "go2rtc.exe")) {
        $helperSrc = Join-Path $runtimeBin $helperName
        if (Test-Path $helperSrc) {
            Copy-Item -Force $helperSrc (Join-Path $targetBin $helperName)
            Write-Host "Bundled ${helperName}: $('{0:N0}' -f (Get-Item $helperSrc).Length) bytes"
        }
        else {
            Write-Host "WARNING: $helperName not found in runtime/bin - skipping bundle"
        }
    }

    if (Test-Path $releaseNotesPath) {
        Copy-Item -Force $releaseNotesPath (Join-Path $targetDir "RELEASE_NOTES.md")
    }
    if (Test-Path $apiManualPath) {
        Copy-Item -Force $apiManualPath (Join-Path $targetDir "API_MANUAL.md")
    }
    if (Test-Path $apiManualTwPath) {
        Copy-Item -Force $apiManualTwPath (Join-Path $targetDir "API_MANUAL_TW.md")
    }
    if (Test-Path $integrityGuidePath) {
        Copy-Item -Force $integrityGuidePath (Join-Path $targetDir "SECURITY_INTEGRITY_LOCKDOWN.md")
    }
    if (Test-Path $iotRoadmapPath) {
        Copy-Item -Force $iotRoadmapPath (Join-Path $targetDir "IOT_UNIVERSAL_INTERFACE_ROADMAP.md")
    }
    if (Test-Path $superAdminGuidePath) {
        Copy-Item -Force $superAdminGuidePath (Join-Path $targetDir "SUPERADMIN_LOCAL_MAINTENANCE.md")
    }
    if (Test-Path $licenseMigrationGuidePath) {
        Copy-Item -Force $superAdminOneTimeGuidePath (Join-Path $targetDir 'WINDOWS_SUPERADMIN_ONE_TIME_SETUP.md')
        Copy-Item -Force $licenseMigrationGuidePath (Join-Path $targetDir "LICENSE_ED25519_MIGRATION.md")
    }
    if (Test-Path $updateUpgradeGuidePath) {
        Copy-Item -Force $updateUpgradeGuidePath (Join-Path $targetDir 'UPDATE_AND_UPGRADE_GUIDE_ZH-TW.md')
    }
    if (Test-Path $examplesPath) {
        $targetExamples = Join-Path $targetDir "docs\\examples"
        New-Item -ItemType Directory -Force -Path $targetExamples | Out-Null
        Copy-Item -Recurse -Force (Join-Path $examplesPath "*") $targetExamples
    }

    @(
        "Product: Management System"
        "Version: $version"
        "Platform: windows-amd64"
        "ReleaseProfile: $releaseProfile"
        "GoBinaryObfuscation: $enableBinaryObfuscation"
        "HeavyJSObfuscation: $enableJsObfuscation"
        "JSMinification: $enableJsMinification"
        "HTMLMinification: $enableHtmlMinification"
        "WindowsVersionMetadata: $windowsVersionMetadata"
        "CodeSigned: $codeSigned"
        "LicenseSignature: Ed25519-V1"
        "LegacyLicenseEnabledByDefault: False"
        "WindowsCompanyName: YTSworks"
        "WindowsProductName: Management System"
        "BuildDate: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz')"
        "Artifact: nms_server.exe"
        "ReleaseNotes: RELEASE_NOTES.md"
        "APIDocumentation: API_MANUAL.md"
        "APIDocumentationTW: API_MANUAL_TW.md"
        "IntegrityRecoveryGuide: SECURITY_INTEGRITY_LOCKDOWN.md"
        "UniversalIoTRoadmap: IOT_UNIVERSAL_INTERFACE_ROADMAP.md"
        "SuperAdminGuide: SUPERADMIN_LOCAL_MAINTENANCE.md"
        'WindowsSuperAdminOneTimeGuide: WINDOWS_SUPERADMIN_ONE_TIME_SETUP.md'
        'UpdateUpgradeGuideTW: UPDATE_AND_UPGRADE_GUIDE_ZH-TW.md'
        "LicenseMigrationGuide: LICENSE_ED25519_MIGRATION.md"
        "IntegrationExamples: docs/examples"
    ) | Set-Content -Path (Join-Path $targetDir "RELEASE_NOTE.txt") -Encoding UTF8

    $startBat = Join-Path $repoRoot "scripts\\run\\start_nms_utf8.bat"
    $startPs1 = Join-Path $repoRoot "scripts\\run\\start_nms_utf8.ps1"
	$manageSuperAdmin = Join-Path $repoRoot "scripts\\run\\manage_superadmin.bat"
    if (Test-Path $startBat) {
        Copy-Item -Force $startBat (Join-Path $targetDir "start_nms.bat")
    }
    if (Test-Path $startPs1) {
        Copy-Item -Force $startPs1 (Join-Path $targetDir "start_nms.ps1")
    }
	if (Test-Path $manageSuperAdmin) {
		Copy-Item -Force $manageSuperAdmin (Join-Path $targetDir "manage_superadmin.bat")
	}

    $targetPrefix = (Resolve-Path -LiteralPath $targetDir).Path.TrimEnd([char[]]@(92, 47)) + [System.IO.Path]::DirectorySeparatorChar
    $manifest = Get-ChildItem -Path $targetDir -Recurse -File | Sort-Object FullName | ForEach-Object {
        [ordered]@{
            path = $_.FullName.Substring($targetPrefix.Length).Replace('\', '/')
            size = $_.Length
            sha256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        }
    }
    [System.IO.File]::WriteAllText((Join-Path $targetDir "SHA256_MANIFEST.json"), ($manifest | ConvertTo-Json -Depth 4), [System.Text.UTF8Encoding]::new($false))
}
finally {
    if (Test-Path $tempDir) {
        Remove-Item -Recurse -Force $tempDir
    }
}

Write-Host "Windows artifact ready: $targetDir"
