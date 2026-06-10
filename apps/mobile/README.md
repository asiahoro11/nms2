# NMS Mobile Client

Native mobile client for `new_nms_sync`.

This app does not replace the NMS server and does not embed the existing web UI.
The Windows/Linux NMS deployment remains the source of truth. The mobile app
connects to a configured NMS host through the existing HTTPS API.

## Product Definition

- Android: native APK first, Play Store or MDM later.
- iOS: native app distributed as an App Store Unlisted App after Apple review.
- Login: site name, NMS host/domain/IP, username, password, optional 2FA.
- First release: monitoring, dashboard, device status, alerts, camera preview,
  topology read-only view, logs and reports.
- Later release: remote control actions after RBAC, confirmation, and audit
  paths are hardened.

## Current Scaffold

This repository currently includes the Flutter source scaffold:

- `lib/main.dart`
- `lib/src/api/nms_api_client.dart`
- `lib/src/models/nms_models.dart`
- `lib/src/storage/session_store.dart`
- `lib/src/screens/login_screen.dart`
- `lib/src/screens/dashboard_screen.dart`

The local Windows machine still needs Flutter and Android SDK installed before
it can build an APK.

## Bootstrap Platform Files

After Flutter is installed, generate Android and iOS platform folders from this
directory:

```powershell
cd apps/mobile
flutter create --project-name nms_mobile_client --org com.yoyo.nms --platforms=android,ios .
flutter pub get
```

If `flutter create` changes `lib/main.dart`, restore this app source from git
before building.

## Android APK Build

Windows can build the Android APK. Linux is not required.

Prerequisites:

- Flutter SDK
- Android Studio or Android SDK command-line tools
- Android SDK Platform and Build Tools
- Java JDK

Build:

```powershell
cd apps/mobile
flutter doctor
flutter pub get
flutter build apk --release
```

Or from the repository root:

```powershell
scripts\mobile\build_android_apk.ps1 -BootstrapPlatforms
```

Output:

```text
apps/mobile/build/app/outputs/flutter-apk/app-release.apk
```

## iOS Build and Unlisted App

iOS requires macOS and Xcode.

```bash
cd apps/mobile
flutter doctor
flutter pub get
flutter build ios --release
```

Distribution direction:

1. Archive and upload through Xcode or Transporter.
2. Submit to App Review in App Store Connect.
3. Configure or request Unlisted App distribution.
4. Provide customers the direct App Store link or a QR Code.

## NMS API Contract Used

The first scaffold uses:

- `GET /api/v1/system/info`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/verify-2fa`
- `GET /api/v1/auth/me`
- `GET /api/v1/dashboard`

Authenticated requests use:

```http
Authorization: Bearer <jwt>
```
