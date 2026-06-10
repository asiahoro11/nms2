param(
    [switch]$BootstrapPlatforms
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$mobileDir = Join-Path $repoRoot "apps\mobile"

if (!(Test-Path $mobileDir)) {
    throw "Mobile app directory not found: $mobileDir"
}

if (!(Get-Command flutter -ErrorAction SilentlyContinue)) {
    throw "Flutter is not installed or not on PATH. Install Flutter and Android SDK first."
}

Push-Location $mobileDir
try {
    if ($BootstrapPlatforms) {
        flutter create --project-name nms_mobile_client --org com.yoyo.nms --platforms=android,ios .
    }

    flutter doctor
    flutter pub get
    flutter analyze
    flutter build apk --release

    $apk = Join-Path $mobileDir "build\app\outputs\flutter-apk\app-release.apk"
    if (!(Test-Path $apk)) {
        throw "APK build completed without expected output: $apk"
    }

    Write-Host "APK ready: $apk"
}
finally {
    Pop-Location
}
