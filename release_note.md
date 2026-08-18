# Release Note

## v1.2.4.10 | 2026-08-12

- Fixed SNMP v1/v2c/v3 devices being displayed as Ping because the frontend depended on a community field that is not returned by the API.
- Ping-only devices now remain `snmp_version = 0` with an empty SNMP community.
- SNMP defaults are applied only when the create form selects SNMP.
- Synchronized frontend and embedded static assets.

---

## v1.2.4.9sp00022 | 2026-07-23

**Build purpose:** IoT multi-register device setup and resilient offline store-and-forward release.

- Enforces mutually exclusive connection fields: TCP transports show only Host/IP and Port, while RS485 shows only serial-port settings.
- Normalizes legacy protocol aliases and removes stale fields from the other transport.
- Supports multiple register/tag definitions for one physical IoT device.
- Converts readable `40001`/`30001` reference addresses to zero-based Modbus wire offsets.
- Stores unsent measurements in NMS SQLite and retries them after connectivity returns.
- Uses a millisecond forwarding interval and sends `sendTime` as a Unix epoch millisecond integer.
- Forwards `deviceId`, `sendTime`, and multi-value `tagData` JSON payloads.
- Prevents already delivered samples from being resent when a later sample in the same batch fails.
- Adds CSV, PDF, and JSON exports for topology, events, Syslog, notifications, IoT, recordings, access-control operations, and configuration backup history.
- Adds ready-to-fill forwarding settings and outbound payload JSON examples under `docs/examples/`.
- Updates the Chinese and English API manuals with the complete categorized route catalog and role matrix.
- Adds a reversible integrity lockdown: SQLite corruption or an optional executable SHA-256 mismatch blocks application APIs with HTTP 423, opens the database read-only on subsequent startup, and disables background writers without deleting data.
- Keeps static files and a local integrity-status API available; unlocking requires an offline, audited recovery.
- Adds the universal IoT interface roadmap and `/api/v2/iot/*` contract plan for transport-independent devices, points, and samples while preserving v1 compatibility.

---

## v1.2.4.9 | 2026-06-26

### 繁體中文
- IoT / Modbus 模組已加入 License 授權控制，未授權時會顯示授權提示並停用操作。
- IoT API、背景輪詢與 iframe embed 均會檢查授權狀態，避免繞過模組授權。
- 修正 IoT 授權提示與狀態標記的編碼，避免前端顯示亂碼。
- Windows packages include only ffmpeg.exe and go2rtc.exe; Linux amd64 packages include only ffmpeg_linux_amd64 and go2rtc_linux_amd64; Linux arm64 packages include only ffmpeg_linux_arm64 and go2rtc_linux_arm64.

### 简体中文
- IoT / Modbus module now requires a License; unavailable features show a license notice and are disabled.
- IoT API, background polling, and iframe embed snapshots now check license status.
- Fixed IoT license notice/status text encoding to avoid mojibake in the UI.
- Windows packages include only ffmpeg.exe and go2rtc.exe; Linux amd64 packages include only ffmpeg_linux_amd64 and go2rtc_linux_amd64; Linux arm64 packages include only ffmpeg_linux_arm64 and go2rtc_linux_arm64.

### English
- IoT / Modbus module now requires a License; unavailable features show a license notice and are disabled.
- IoT API, background polling, and iframe embed snapshots now check license status.
- Fixed IoT license notice and status marker encoding to avoid mojibake in the UI.
- Windows packages include only ffmpeg.exe and go2rtc.exe; Linux amd64 packages include only ffmpeg_linux_amd64 and go2rtc_linux_amd64; Linux arm64 packages include only ffmpeg_linux_arm64 and go2rtc_linux_arm64.

### 日本語
- IoT / Modbus module now requires a License, with disabled UI controls when not licensed.
- IoT API, polling jobs, and iframe embed snapshots now respect license state.
- Fixed IoT license notice encoding to avoid garbled text.
- Windows packages include only ffmpeg.exe and go2rtc.exe; Linux amd64 packages include only ffmpeg_linux_amd64 and go2rtc_linux_amd64; Linux arm64 packages include only ffmpeg_linux_arm64 and go2rtc_linux_arm64.

### 한국어
- IoT / Modbus module now requires a License, and unlicensed controls are disabled.
- IoT API, polling jobs, and iframe embed snapshots now validate license state.
- Fixed IoT license notice encoding to prevent garbled UI text.
- Windows packages include only ffmpeg.exe and go2rtc.exe; Linux amd64 packages include only ffmpeg_linux_amd64 and go2rtc_linux_amd64; Linux arm64 packages include only ffmpeg_linux_arm64 and go2rtc_linux_arm64.

---

## Artifact Notes
- Version: v1.2.4.9
- Database policy: packaged builds do not remove an existing nms.db; no backup copy is created for this test build.
- Windows runtime binaries: ffmpeg.exe, go2rtc.exe.
- Linux amd64 runtime binaries: ffmpeg_linux_amd64, go2rtc_linux_amd64.
- Linux arm64 runtime binaries: ffmpeg_linux_arm64, go2rtc_linux_arm64.
## v1.2.4.11

- 隱藏管理入口改由後端驗證管理員帳號密碼。
- 已啟用 TOTP 的帳號才會要求一次性驗證碼。
- 移除前端硬編碼管理員密碼及永久解鎖狀態。
