$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
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

$artifactRoot = Join-Path $repoRoot "artifacts\\linux"
$targetDir = Join-Path $artifactRoot $version
$tempDir = Join-Path $tmpRoot ("linux-" + [guid]::NewGuid().ToString("N"))
$tempBackend = Join-Path $tempDir "backend"

New-Item -ItemType Directory -Force -Path $artifactRoot | Out-Null
New-Item -ItemType Directory -Force -Path $tmpRoot | Out-Null
if (Test-Path $targetDir) {
    Remove-Item -Recurse -Force $targetDir
}
New-Item -ItemType Directory -Force -Path $targetDir | Out-Null

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

    Push-Location $tempBackend
    try {
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        $env:GOWORK = "off"
        $garble = Get-Command garble -ErrorAction SilentlyContinue
        $linuxAmd64Output = Join-Path $targetDir "nms_server_linux_amd64"
        $linuxArm64Output = Join-Path $targetDir "nms_server_linux_arm64"
        if ($enableBinaryObfuscation -and $null -ne $garble) {
            Write-Host "Building with Go binary obfuscation enabled via garble." -ForegroundColor Yellow
            & $garble.Source build -trimpath -ldflags $licenseLdflags -o $linuxAmd64Output .
        }
        elseif ($enableBinaryObfuscation) {
            throw "NMS_ENABLE_BINARY_OBFUSCATION is enabled but garble was not found. Refusing to build a non-obfuscated binary."
        }
        else {
            go build -trimpath -ldflags $licenseLdflags -o $linuxAmd64Output .
        }
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for linux-amd64"
        }

        $env:GOARCH = "arm64"
        if ($enableBinaryObfuscation -and $null -ne $garble) {
            & $garble.Source build -trimpath -ldflags $licenseLdflags -o $linuxArm64Output .
        }
        elseif ($enableBinaryObfuscation) {
            throw "NMS_ENABLE_BINARY_OBFUSCATION is enabled but garble was not found. Refusing to build a non-obfuscated binary."
        }
        else {
            go build -trimpath -ldflags $licenseLdflags -o $linuxArm64Output .
        }
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for linux-arm64"
        }
        $localTools = Join-Path $targetDir "tools"
        New-Item -ItemType Directory -Force -Path $localTools | Out-Null
        $env:GOARCH = "amd64"
        go build -trimpath -ldflags $licenseLdflags -o (Join-Path $localTools "superadmin-local_linux_amd64") ./cmd/superadmin-local
        if ($LASTEXITCODE -ne 0) { throw "go build failed for superadmin-local_linux_amd64" }
        $env:GOARCH = "arm64"
        go build -trimpath -ldflags $licenseLdflags -o (Join-Path $localTools "superadmin-local_linux_arm64") ./cmd/superadmin-local
        if ($LASTEXITCODE -ne 0) { throw "go build failed for superadmin-local_linux_arm64" }
    }
    finally {
        $env:GOOS = $null
        $env:GOARCH = $null
        $env:GOWORK = $null
        Pop-Location
    }

    if (-not (Test-Path (Join-Path $targetDir "nms_server_linux_amd64"))) {
        throw "missing output binary: nms_server_linux_amd64"
    }
    if (-not (Test-Path (Join-Path $targetDir "nms_server_linux_arm64"))) {
        throw "missing output binary: nms_server_linux_arm64"
    }

    New-Item -ItemType Directory -Force -Path (Join-Path $targetDir "data") | Out-Null
    $targetBin = Join-Path $targetDir "bin"
    New-Item -ItemType Directory -Force -Path $targetBin | Out-Null
    foreach ($helperName in @("ffmpeg_linux_amd64", "go2rtc_linux_amd64")) {
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
        Copy-Item -Force $updateUpgradeGuidePath (Join-Path $targetDir 'UPDATE_AND_UPGRADE_GUIDE_ZH-TW.md')
        Copy-Item -Force $licenseMigrationGuidePath (Join-Path $targetDir "LICENSE_ED25519_MIGRATION.md")
    }
    if (Test-Path $examplesPath) {
        $targetExamples = Join-Path $targetDir "docs\\examples"
        New-Item -ItemType Directory -Force -Path $targetExamples | Out-Null
        Copy-Item -Recurse -Force (Join-Path $examplesPath "*") $targetExamples
    }

    @(
        "Product: Management System"
        "Version: $version"
        "Platform: linux-amd64,linux-arm64"
        "ReleaseProfile: $releaseProfile"
        "GoBinaryObfuscation: $enableBinaryObfuscation"
        "HeavyJSObfuscation: $enableJsObfuscation"
        "JSMinification: $enableJsMinification"
        "HTMLMinification: $enableHtmlMinification"
        "LicenseSignature: Ed25519-V1"
        "LegacyLicenseEnabledByDefault: False"
        "BuildDate: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz')"
        "Artifacts: nms_server_linux_amd64, nms_server_linux_arm64"
        "ReleaseNotes: RELEASE_NOTES.md"
        "APIDocumentation: API_MANUAL.md"
        "APIDocumentationTW: API_MANUAL_TW.md"
        "IntegrityRecoveryGuide: SECURITY_INTEGRITY_LOCKDOWN.md"
        "UniversalIoTRoadmap: IOT_UNIVERSAL_INTERFACE_ROADMAP.md"
        "SuperAdminGuide: SUPERADMIN_LOCAL_MAINTENANCE.md"
        'UpdateUpgradeGuideTW: UPDATE_AND_UPGRADE_GUIDE_ZH-TW.md'
        "LicenseMigrationGuide: LICENSE_ED25519_MIGRATION.md"
        "IntegrationExamples: docs/examples"
    ) | Set-Content -Path (Join-Path $targetDir "RELEASE_NOTE.txt") -Encoding UTF8

    $startSh = Join-Path $repoRoot "scripts\\run\\start_nms_utf8.sh"
    if (Test-Path $startSh) {
        Copy-Item -Force $startSh (Join-Path $targetDir "start_nms.sh")
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

    $arm64ArtifactRoot = Join-Path $repoRoot "artifacts\\linux-arm64"
    $arm64TargetDir = Join-Path $arm64ArtifactRoot $version
    $arm64ZipPath = Join-Path $arm64ArtifactRoot ($version + ".zip")
    $arm64Server = Join-Path $targetDir "nms_server_linux_arm64"
    $arm64Go2rtc = Join-Path $runtimeBin "go2rtc_linux_arm64"

    if (-not (Test-Path $arm64Server)) {
        throw "missing ARM64 server binary for dedicated package: $arm64Server"
    }
    if (-not (Test-Path $startSh)) {
        throw "missing Linux start script for dedicated ARM64 package: $startSh"
    }
    if (-not (Test-Path $arm64Go2rtc)) {
        throw "missing ARM64 go2rtc helper for dedicated package: $arm64Go2rtc"
    }

    New-Item -ItemType Directory -Force -Path $arm64ArtifactRoot | Out-Null
    if (Test-Path $arm64TargetDir) {
        Remove-Item -Recurse -Force $arm64TargetDir
    }
    New-Item -ItemType Directory -Force -Path $arm64TargetDir | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $arm64TargetDir "bin") | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $arm64TargetDir "data") | Out-Null

    Copy-Item -Force $arm64Server (Join-Path $arm64TargetDir "nms_server_linux_arm64")
    Copy-Item -Force (Join-Path $targetDir "tools\superadmin-local_linux_arm64") (Join-Path $arm64TargetDir "superadmin-local_linux_arm64")
    Copy-Item -Force $arm64Go2rtc (Join-Path $arm64TargetDir "bin\\go2rtc_linux_arm64")
    Copy-Item -Force $startSh (Join-Path $arm64TargetDir "start_nms.sh")

    $ffmpegArm64Src = Join-Path $runtimeBin "ffmpeg_linux_arm64"
    if (Test-Path $ffmpegArm64Src) {
        Copy-Item -Force $ffmpegArm64Src (Join-Path $arm64TargetDir "bin\ffmpeg_linux_arm64")
        Write-Host "Bundled ffmpeg_linux_arm64: $('{0:N0}' -f (Get-Item $ffmpegArm64Src).Length) bytes"
    } else {
        Write-Host "WARNING: ffmpeg_linux_arm64 not found in runtime/bin — skipping bundle"
    }
    if (Test-Path $releaseNotesPath) {
        Copy-Item -Force $releaseNotesPath (Join-Path $arm64TargetDir "RELEASE_NOTES.md")
    }
    if (Test-Path $apiManualPath) {
        Copy-Item -Force $apiManualPath (Join-Path $arm64TargetDir "API_MANUAL.md")
    }
    if (Test-Path $apiManualTwPath) {
        Copy-Item -Force $apiManualTwPath (Join-Path $arm64TargetDir "API_MANUAL_TW.md")
    }
    if (Test-Path $integrityGuidePath) {
        Copy-Item -Force $integrityGuidePath (Join-Path $arm64TargetDir "SECURITY_INTEGRITY_LOCKDOWN.md")
    }
    if (Test-Path $iotRoadmapPath) {
        Copy-Item -Force $iotRoadmapPath (Join-Path $arm64TargetDir "IOT_UNIVERSAL_INTERFACE_ROADMAP.md")
    }
    if (Test-Path $superAdminGuidePath) {
        Copy-Item -Force $superAdminGuidePath (Join-Path $arm64TargetDir "SUPERADMIN_LOCAL_MAINTENANCE.md")
    }
    if (Test-Path $licenseMigrationGuidePath) {
        Copy-Item -Force $updateUpgradeGuidePath (Join-Path $arm64TargetDir 'UPDATE_AND_UPGRADE_GUIDE_ZH-TW.md')
        Copy-Item -Force $licenseMigrationGuidePath (Join-Path $arm64TargetDir "LICENSE_ED25519_MIGRATION.md")
    }
    if (Test-Path $examplesPath) {
        $arm64TargetExamples = Join-Path $arm64TargetDir "docs\\examples"
        New-Item -ItemType Directory -Force -Path $arm64TargetExamples | Out-Null
        Copy-Item -Recurse -Force (Join-Path $examplesPath "*") $arm64TargetExamples
    }

    @(
        "Product: Management System"
        "Version: $version"
        "Platform: linux-arm64"
        "ReleaseProfile: $releaseProfile"
        "GoBinaryObfuscation: $enableBinaryObfuscation"
        "HeavyJSObfuscation: $enableJsObfuscation"
        "JSMinification: $enableJsMinification"
        "HTMLMinification: $enableHtmlMinification"
        "LicenseSignature: Ed25519-V1"
        "LegacyLicenseEnabledByDefault: False"
        "BuildDate: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz')"
        "Artifacts: nms_server_linux_arm64"
        "RuntimeHelpers: bin/ffmpeg_linux_arm64, bin/go2rtc_linux_arm64"
        "StartScript: start_nms.sh"
        "ReleaseNotes: RELEASE_NOTES.md"
        "APIDocumentation: API_MANUAL.md"
        "APIDocumentationTW: API_MANUAL_TW.md"
        "IntegrityRecoveryGuide: SECURITY_INTEGRITY_LOCKDOWN.md"
        "UniversalIoTRoadmap: IOT_UNIVERSAL_INTERFACE_ROADMAP.md"
        "SuperAdminGuide: SUPERADMIN_LOCAL_MAINTENANCE.md"
        'UpdateUpgradeGuideTW: UPDATE_AND_UPGRADE_GUIDE_ZH-TW.md'
        "LicenseMigrationGuide: LICENSE_ED25519_MIGRATION.md"
        "IntegrationExamples: docs/examples"
        "SourceArtifact: artifacts/linux/$version"
    ) | Set-Content -Path (Join-Path $arm64TargetDir "RELEASE_NOTE.txt") -Encoding UTF8

    $arm64TargetPrefix = (Resolve-Path -LiteralPath $arm64TargetDir).Path.TrimEnd([char[]]@(92, 47)) + [System.IO.Path]::DirectorySeparatorChar
    $armManifest = Get-ChildItem -Path $arm64TargetDir -Recurse -File | Sort-Object FullName | ForEach-Object {
        [ordered]@{
            path = $_.FullName.Substring($arm64TargetPrefix.Length).Replace('\', '/')
            size = $_.Length
            sha256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        }
    }
    [System.IO.File]::WriteAllText((Join-Path $arm64TargetDir "SHA256_MANIFEST.json"), ($armManifest | ConvertTo-Json -Depth 4), [System.Text.UTF8Encoding]::new($false))

    if (Test-Path $arm64ZipPath) {
        Remove-Item -Force $arm64ZipPath
    }
    Compress-Archive -Path (Join-Path $arm64TargetDir "*") -DestinationPath $arm64ZipPath -Force
}
finally {
    if (Test-Path $tempDir) {
        Remove-Item -Recurse -Force $tempDir
    }
}

Write-Host "Linux artifact ready: $targetDir"
Write-Host "Linux ARM64 artifact ready: $arm64TargetDir"
Write-Host "Linux ARM64 zip ready: $arm64ZipPath"
