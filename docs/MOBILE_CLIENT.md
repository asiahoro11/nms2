# NMS Mobile Client

## Definition

`NMS Mobile Client` is a native iOS and Android client for the existing
`new_nms_sync` server.

It is not:

- a replacement NMS server
- a WebView wrapper around `apps/frontend`
- a mobile SNMP/ONVIF scanner
- a mobile NVR recorder

The NMS server keeps running on Windows or Linux. The app connects to that
server through the existing API.

```text
Windows/Linux NMS host
  - Go backend
  - SQLite runtime data
  - SNMP / ONVIF / RTSP / ffmpeg / go2rtc
  - license, reports, topology, camera, logs
        ^
        | HTTPS over LAN, VPN, reverse proxy, or tunnel
        v
iOS / Android native app
  - login
  - monitoring
  - alert review
  - camera preview
  - read-only topology
  - report/log lookup
```

## Login Model

The first release uses direct NMS host login:

```text
Site name: optional friendly label
NMS host: domain, IP, and optional port
Username
Password
2FA code if enabled
```

The app normalizes a host without scheme to HTTPS. Examples:

```text
nms.example.com        -> https://nms.example.com
192.168.1.50:8080     -> https://192.168.1.50:8080
http://10.0.0.12:8080 -> http://10.0.0.12:8080
```

## API Contract

Initial app scaffold:

| Capability | Endpoint |
| --- | --- |
| Host probe | `GET /api/v1/system/info` |
| Login | `POST /api/v1/auth/login` |
| 2FA login | `POST /api/v1/auth/verify-2fa` |
| Current user | `GET /api/v1/auth/me` |
| Dashboard | `GET /api/v1/dashboard` |
| Logout | `POST /api/v1/auth/logout` |

Authenticated requests use:

```http
Authorization: Bearer <jwt>
```

The server currently returns the standard envelope:

```json
{
  "success": true,
  "data": {}
}
```

## Release Scope

### v0.1 Mobile

- native app scaffold under `apps/mobile`
- direct host login
- JWT session stored in OS secure storage
- 2FA challenge handling
- dashboard summary
- recent events
- manual refresh and logout

### v0.2 Mobile

- device list and device detail
- unread notification count and notification inbox
- camera snapshot / preview entry points
- topology read-only view
- logs and reports query/download

### Later

- push notifications through APNs and FCM
- multi-site switching
- certificate pinning or managed trust profile option
- remote control actions after RBAC, confirmation, and audit requirements are
  finalized

## Build Direction

Android APK can be built on Windows after Flutter and Android SDK are installed.
Linux is not required.

iOS build requires macOS and Xcode. Formal distribution should use App Store
Unlisted App, with installation through direct link or QR Code after Apple
review.

## Current Repository Location

Mobile source lives in:

```text
apps/mobile
```

The current checkout intentionally does not include generated Android/iOS
platform folders yet. Generate them after installing Flutter:

```powershell
cd apps/mobile
flutter create --project-name nms_mobile_client --org com.yoyo.nms --platforms=android,ios .
flutter pub get
```

Android build helper:

```powershell
scripts\mobile\build_android_apk.ps1 -BootstrapPlatforms
```
