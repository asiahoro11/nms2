# Release Note

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
