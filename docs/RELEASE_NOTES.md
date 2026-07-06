# Management Server Release Notes

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

---

---
## v1.2.4.8sp0007 | 2026-06-24

**Build purpose:** IoT Modbus temperature/humidity value decoding patch.

### IoT Sensor Reading Accuracy
- Fixed 16-bit RS485 temperature/humidity sensors that were being interpreted with the default scale instead of the manual's 0.1 resolution.
- Temperature, humidity, and combined temperature+humidity sensor types now apply the expected 0.1 scale when still configured with the default scale value.
- Combined temperature+humidity devices now read two 16-bit registers by default.
- Sensor cards now decode both values from raw Modbus register bytes and support word-order swapping when a device reports humidity and temperature in the opposite order.
- Added tests for signed temperature compensation, manual register defaults, and register-pair word-order handling.

---
## v1.2.4.8sp0006 | 2026-06-22

**Build purpose:** System Management hang prevention and global layout containment patch.

### System Management Stability
- Stopped automatic admin API loading when entering the System Management page.
- Admin tab data now loads only after the user explicitly selects the tab.
- Changed host-status to a lightweight response without blocking CPU, memory, disk, or GPU system probes.
- Kept request timeouts for host status, license status, and machine ID calls so a single abnormal request cannot leave other pages loading indefinitely.

### Layout Stability
- Added global page and main-content containment so oversized tables or controls cannot expand the application frame.
- Added flexible section headers so action buttons wrap inside the available frame instead of pushing content wider.
- Added an internal scroll frame for the Audit Logs table so it no longer expands the System Management page width.
- Updated responsive table containment so narrow screens keep wide tables inside local scroll regions.

---
## v1.2.4.8sp0005 | 2026-06-22

**Build purpose:** System Management request isolation patch.

### System Management
- Changed the admin page to lazy-load only the active System Management tab instead of firing every admin API at once.
- Added frontend API request timeouts so a stuck admin request cannot leave other pages permanently loading.
- Reduced host-status, license-status, and machine-id request timeouts to fail fast in abnormal environments.

---
## v1.2.4.8sp0004 | 2026-06-22

**Build purpose:** System management stability patch and cross-platform service-pack release.

### System Management
- Prevented the admin System Management page from blocking the server while loading host status.
- Changed host status ffmpeg lookup to use local binaries only, avoiding implicit network downloads during page load.
- Added timeouts around ffmpeg download and hardware-acceleration probing paths.
- Skipped heavyweight GPU probing in the host-status API so other page requests are not blocked after opening System Management.
- Added timeout and caching for machine UUID detection used by the license page.

### Release Packaging
- Built release artifacts for Windows amd64, Linux amd64, and Linux arm64.

---
## v1.2.4.8sp0002 | 2026-06-18

**Build purpose:** IoT feature patch — Modbus RTU / RS485 support and sensor reading UI.

### IoT — Modbus RTU / RS485 Active Polling
- Added native Modbus RTU direct polling over serial port (COM port on Windows, /dev/ttyUSB0 on Linux).
- Added Modbus RS485 half-duplex support (rtu+rs485 scheme with DE/RE pin toggling).
- Serial port parameters configurable per device: baud rate, data bits, parity (N/E/O), stop bits.
- Supports same FC03 / FC04 function codes and all data types (uint16/int16/uint32/int32/float32) as Modbus TCP.
- Background poller now covers modbus_tcp, modbus_rtu, and modbus_rs485 protocols.

### IoT — Sensor Display UI
- Added sensor type classification per device: temperature, humidity, temperature+humidity, power, voltage, current, pressure, CO₂, PM2.5.
- Added sensor reading card grid on IoT page — shows current value with unit (°C, %, W, V, etc.) and online/offline/error status.
- Device list table now shows sensor type and target (IP:port for TCP, serial port for RTU/RS485).
- Measurement values in tables now display with unit auto-detected from metric name.
- Relative timestamps (e.g. "3m ago") on sensor cards for quick freshness check.

### IoT — Add Device Modal
- Replaced inline form with modal dialog and protocol tabs (Modbus TCP / Modbus RTU / Modbus RS485).
- TCP tab shows Host IP and Port fields; RTU/RS485 tab shows serial port, baud rate, data bits, parity, stop bits.
- All Modbus register fields (unit ID, address, FC, data type, scale, offset, endian, poll interval) shared across tabs.

### IoT — Protocol Capabilities
- Updated protocol profile list to mark Modbus RTU and Modbus RS485 as Ready (active poll).
- MQTT, OPC-UA, BACnet remain Bridge Ready (push ingest via REST API).

### UI Layout
- IoT page restructured with clear section titles: Sensor Readings → Device List → Recent Measurements → Configuration → Embed Tokens.
- Configuration cards (Forward Queue, Gateway Ingest Test, Embed Token, Allowlist) moved to a consistent grid layout with labeled form fields.

---
## v1.2.4.8 | 2026-06-10

**Build purpose:** Customer integration stable release for API, iframe embed, and IoT gateway integration.

### Customer Integration Stable Contract
- Promoted the customer integration documentation to `v1.2.4.8` as the current stable contract.
- Clarified integration readiness statuses as `Ready`, `Bridge Ready`, and `Planned` so customer-facing API scope is explicit.
- Kept `/api/v1/integrations/network-snapshot` as the preferred customer dashboard bootstrap API for dashboard summary, device inventory, and topology data.
- Kept iframe embed integration based on scoped embed tokens, server-side token revocation, and runtime `frame-ancestors` allowlist management.
- Kept IoT direct Modbus TCP polling and REST ingest as `Ready` integration paths.
- Marked MQTT, OPC-UA, and BACnet gateway ingest as `Bridge Ready`, with native protocol clients reserved as `Planned`.

### IoT Edge Buffer Update
- Kept the product version at `v1.2.4.8` while adding Modbus TCP background collection.
- Enabled automatic polling for configured `modbus_tcp` signal points with FC03 Holding Register and FC04 Input Register support.
- Added metric, polling interval, scale, offset, byte order, and word order fields for Modbus TCP points.
- Added durable local SQLite store-and-forward behavior for Modbus TCP readings and REST ingest measurements.
- Added HTTP forwarder settings and queue APIs so customer systems can receive NMS-normalized IoT values without connecting to each Modbus TCP device.
- Added offline retry with backoff. Failed forwards stay on the NMS host until the customer endpoint is reachable again.
- Added post-success retention: forwarded IoT records are marked `sent`, kept locally for 10 minutes, then dropped by cleanup.
- Added queue visibility for pending, failed, and sent-hold records.

### Security Hardening
- Redacted device and PDU SNMP community values from normal API JSON responses while preserving internal polling behavior.
- Device and PDU updates now preserve the stored SNMP community when the update request leaves the community field blank.
- Camera API responses redact credentials and sensitive query parameters from RTSP URLs.
- Expanded audit and log detail redaction for password, token, API key, secret, authorization, and SNMP community fields.
- iframe and IoT integration UI now shows security guidance for scoped tokens, HTTPS, allowlists, short token lifetime, and OT network isolation.

### UI and Layout
- Hardened audit, admin, embed, and IoT table layouts against long JSON, token, URL, IP, and identifier strings.
- Mobile and tablet table containers now keep content inside the page boundary with controlled horizontal scrolling.
- Confirmed monitor mode naming remains `電視牆模式`.

### Release Packaging
- Added a dedicated Linux ARM64 packaging path for customer delivery when the standard Linux build already emitted `nms_server_linux_arm64`.
- The dedicated ARM64 package is intended to contain only the ARM64 server binary, ARM64 go2rtc helper, Linux start script, release notes, API manual, and a package manifest.

### Documentation
- Updated `docs/API_MANUAL.md` for `v1.2.4.8` customer integration readiness.
- Added customer security validation gates for credential redaction, iframe token handling, OT network segmentation, and gateway ingest controls.
- Added smart-building 2016 / 2024 alignment notes with official Ministry of the Interior / Architecture and Building Research Institute reference links.
- Updated `docs/STABLE_RELEASE_ACCEPTANCE_CHECKLIST.md` to track the current `v1.2.4.8` acceptance state.
- Bumped backend runtime version, frontend app version, static asset version, and service-worker cache to `v1.2.4.8`.

---
## v1.2.4.7 | 2026-05-31

**Build purpose:** Harden customer iframe integration management and expand IoT gateway integration readiness.

### Customer Integration
- Added server-side embed token records so admins can list and revoke iframe tokens after issuing them.
- Embed snapshot requests now reject revoked or expired embed tokens through the token `jti` record.
- Added runtime iframe `frame-ancestors` allowlist settings through `/api/v1/integrations/settings`; CSP reads the DB-backed allowlist on each request and does not require restart.
- Added admin UI controls on the IoT / Modbus page for iframe allowlist management and embed token revocation.
- Added IoT protocol capability profiles for direct Modbus TCP plus REST, MQTT bridge, OPC-UA gateway, and BACnet gateway ingest paths.

### Documentation and Versioning
- Updated `docs/API_MANUAL.md` to `v1.2.4.7`, including embed token management, iframe allowlist management, and IoT protocol profile guidance.
- Bumped backend, frontend app version, static asset version, and service-worker cache to `v1.2.4.7`.

---
## v1.2.4.6 | 2026-05-25

**Build purpose:** Add customer integration API optimization and publish the API manual.

### API Integration
- Added `GET /api/v1/integrations/network-snapshot` as a customer-facing read API that combines dashboard summary, device inventory, and topology into one response.
- Added `include`, `page`, `limit`, `search`, `type`, and `status` query support so clients can trim payload size without returning to multiple bootstrap calls.
- The integration payload uses schema version `nms.integration.network_snapshot.v1` and omits sensitive fields such as SNMP communities, passwords, license keys, and RTSP URLs.
- The endpoint stays behind the existing JWT, license lock, and `device_management` gates.
- Added scoped iframe embed tokens and the read-only `/embed.html` shell for customer portals that need dashboard, topology, inventory, alerts, or IoT views inside an iframe.
- Added IoT/Modbus integration APIs for Modbus TCP polling and REST/webhook ingest, with device and measurement status surfaced in both API and UI.

### Documentation and Versioning
- Added `docs/API_MANUAL.md` for customer integration, including authentication, 2FA, the optimized snapshot API, iframe embed, IoT/Modbus APIs, existing read APIs, write API boundaries, status codes, and integration recommendations.
- Bumped backend, frontend, static asset, and service-worker versions to `v1.2.4.6`.
- Renamed the Traditional Chinese monitor page label from `監控畫面` / `監控模式` to `電視牆模式`, and bumped the static asset cache to `v1.2.4.6-api2`.

---
## v1.2.4.5 | 2026-05-15

**Build purpose:** Fix report export format handling and expand compliance/operations export coverage.

### Reports and Exports
- Fixed report export format handling so CSV/PDF requests no longer fall back to JSON output.
- Added richer device and inventory export fields, including sys name, firmware, location, SNMP uptime, interface counts, PoE count, and aggregate traffic counters.
- Added disk usage to the device health report and SNMP uptime to the availability report.
- Added new export reports for interfaces/PoE/traffic, health trend summaries, SLA evidence, audit/change records, license capacity, camera inventory, PDU/UPS inventory, and access-control inventory.
- Standardized report UI labels and CSV/PDF export titles, headers, and common status values through the report i18n path, defaulting to Traditional Chinese and honoring the selected UI language.
- Added CSV/PDF buttons for the new reports and synced frontend assets into both backend static trees.

### Security and Data Handling
- Report exports intentionally omit sensitive values such as passwords, SNMP communities, full license keys, and RTSP URLs.
- License capacity export masks license keys to a short prefix.

### Build and Versioning
- Bumped backend, frontend, static asset, and service-worker versions to `v1.2.4.5`.
- Added `duration_days` binding to the license generation input so the modular license generator compiles cleanly.
- Added report-module tests covering PDF format routing, expanded CSV fields, and the new report SQL paths.

---
## v1.2.4.3 | 2026-04-28

**Build purpose:** Promote the validated encoder-camera preview and recording fixes to an obfuscated release build.

### Camera Streaming
- MJPEG remains the default low-latency preview path for both management mode and monitor mode.
- WebRTC/MSE remains opt-in through `localStorage.nms-camera-preview-transport = "webrtc"`.
- Failed WebRTC sessions in monitor mode now fall back to MJPEG instead of leaving black camera tiles.
- Preview defaults keep high-quality MJPEG output with `NMS_CAMERA_PREVIEW_QUALITY=2`.
- Camera debug logging is disabled by default in this release; set `NMS_CAMERA_DEBUG=1` only when field diagnostics are needed.

### Recording
- Recording uses FFmpeg direct RTSP by default.
- Recording no longer passes the unsupported `-rw_timeout` input option, fixing FFmpeg startup failures on affected builds.
- Recording writes MPEG-TS first and remuxes after FFmpeg exits, preserving the `.ts` file if MP4 remux fails.

### UI
- Restored local SVG icon rendering for legacy icon placeholders.
- Service worker monitor-page fallback now returns a valid response instead of surfacing `Failed to convert value to 'Response'`.

---
## v1.2.4.4.1-debug1 | 2026-04-27

**Build purpose:** Add camera latency debug logging for encoder RTSP preview tuning.

### Camera Streaming
- Camera debug logging is enabled by default for this debug build and can be disabled with `NMS_CAMERA_DEBUG=0`.
- Added MJPEG pipeline timing logs for subscription setup, FFmpeg start, first frame arrival, frame interval, HTTP flush timing, client count, and stream source.
- MJPEG preview now defaults to direct RTSP -> FFmpeg to remove the extra go2rtc preview hop; set `NMS_CAMERA_PREVIEW_SOURCE=go2rtc` or `auto` to re-enable go2rtc fanout.
- Preview FPS can be tuned without rebuilding through `NMS_CAMERA_PREVIEW_FPS`; this debug build defaults to 15 FPS and clamps values to 1-30.
- Preview MJPEG quality can be tuned through `NMS_CAMERA_PREVIEW_QUALITY`; this debug build defaults to high quality `2` and clamps values to 2-15 (lower is better).
- The default MJPEG path no longer forces the 1200 kbps preview cap when the camera row does not set a bandwidth limit.
- Bitrate-limited preview profiles now use less aggressive JPEG compression, improving image clarity when a camera row still has a bandwidth limit.
- Added optional HLS preview endpoint and frontend switch path through `localStorage.nms-camera-preview-transport = "hls"`; MJPEG remains the default low-latency split-window player.
- New viewers only receive cached MJPEG frames when the cache is fresh, reducing the chance of starting playback from a stale frame.

### Recording
- Recording now defaults to direct RTSP -> FFmpeg instead of first routing through go2rtc local RTSP; set `NMS_CAMERA_RECORDING_SOURCE=go2rtc` or `auto` to re-enable fanout.
- Recording now maps video only by default and skips the separate audio probe to avoid extra RTSP sessions against encoder sources; set `NMS_CAMERA_RECORD_AUDIO=1` to include audio.
- Recording writes to a temporary MPEG-TS file first and remuxes to MP4 after FFmpeg exits; if remux fails, NMS keeps the `.ts` file and serves it with the correct media type instead of saving TS data as a fake MP4.
- Removed the unsupported `-rw_timeout` input option from the NVR recording FFmpeg command after field logs showed this FFmpeg build rejects it with `Option rw_timeout not found`.

### UI
- Restored local SVG icon rendering for legacy `fas fa-*` and `icon-*` placeholders, and made inline SVG icons self-contained so they do not render as black filled labels when cached CSS is stale.
- Monitor mode now follows the same preview default as the management camera page: MJPEG is the default, WebRTC is opt-in through `localStorage.nms-camera-preview-transport = "webrtc"`, and failed WebRTC sessions fall back to MJPEG instead of leaving black tiles.
- Service worker navigation fallback now always returns a valid response for monitor pages, avoiding `Failed to convert value to 'Response'` console errors when a cached page is missing.

---
## v1.2.4.4.1 | 2026-04-27

**Build purpose:** Test low-latency FFmpeg/MJPEG preview with go2rtc RTSP fanout.

### Camera Streaming
- Default browser preview is back to the FFmpeg/MJPEG low-latency path because field testing showed it is currently faster than WebRTC on the encoder stream.
- WebRTC/MSE remains available as an opt-in test path through `localStorage.nms-camera-preview-transport = "webrtc"`.
- MJPEG preview first tries to pull from go2rtc's local RTSP fanout, then falls back to direct RTSP if go2rtc is unavailable.
- Recording continues to use FFmpeg and can pull from the same go2rtc local RTSP stream, reducing duplicate upstream pulls against encoder sources.

---
## v1.2.4.8-camera-timeout-test | 2026-04-27

**Build purpose:** Avoid premature MSE preview shutdown for slow-starting RTSP encoder streams.

### Camera Streaming
- Increased the initial MSE preview startup timeout from 7 seconds to 20 seconds.
- After go2rtc returns the MSE codec/MIME response, the browser now waits up to 30 seconds for the first media fragment before closing the WebSocket.
- This keeps preview behavior simple: MSE remains the default preview player, while FFmpeg remains reserved for recording and legacy snapshot/MJPEG paths.
- This is intended for public/NAT encoder streams where go2rtc registration succeeds but the first live media segment arrives slower than local camera streams.

---
## v1.2.4.6 | 2026-04-27

**Build purpose:** Restore go2rtc MSE preview after v1.2.4.6 registration and MIME negotiation issues.

### Camera Streaming
- Fixed go2rtc dynamic stream registration to use `PUT /api/streams?name=...&src=...`, matching the go2rtc 1.9.x Web UI API.
- Fixed the MSE client to consume go2rtc's full `video/mp4; codecs="..."` MIME response instead of wrapping it as a codec string.
- MSE codec negotiation now sends only browser-supported MP4 codecs, reducing blank previews when the source offers unsupported tracks.
- Added server-side logging when go2rtc stream preparation fails before WebSocket upgrade, so future 503 failures include the upstream reason in `server.log`.

---
## v1.2.4.6 | 2026-04-27

**Build purpose:** Simplify camera preview by separating live playback from FFmpeg recording.

### Camera Streaming
- Browser preview now uses go2rtc MSE direct playback as the default path.
- Automatic preview probing no longer tries WebRTC first and no longer falls through to FFmpeg-backed preview transcode.
- go2rtc preview config no longer attaches an FFmpeg binary, preventing preview startup from triggering FFmpeg lookup or download.
- FFmpeg remains reserved for the existing recording pipeline and legacy MJPEG/snapshot compatibility paths.
- If an encoder sends H.265 and the browser cannot decode it through MSE, set the preview/substream to H.264 while keeping the recording stream on the desired quality/codec.

---
## v1.2.4.5 | 2026-04-27

**Build purpose:** Lower-latency encoder preview path when WebRTC cannot establish clean playback.

### Camera Streaming
- Live preview now tries direct WebRTC first, then go2rtc MSE over the existing authenticated WebSocket route, then go2rtc FFmpeg H.264 transcode over MSE, and only then falls back to MJPEG.
- MSE fallback avoids the browser ICE/8555 path, which helps encoder streams that play quickly in VLC but drift when the UI falls back to MJPEG.
- The transcode fallback is used only when direct playback codecs are not accepted by the browser, for example H.265 sources on clients without HEVC browser support.
- Direct go2rtc RTSP registration no longer forces TCP or UDP in the source URL, letting go2rtc negotiate the RTSP transport itself while the existing FFmpeg MJPEG/recording paths still honor configured TCP/UDP and UDP port range settings.
- MSE playback keeps the browser close to the live edge by dropping queued fragments and correcting playback lag.

---
## v1.2.4.4 | 2026-04-27

**Build purpose:** Optional go2rtc/WebRTC live-preview integration for lower-latency camera monitoring.

### Camera Streaming
- Added a go2rtc-backed WebRTC preview path for camera live view, monitor mode, and dashboard camera tiles.
- NMS starts go2rtc on demand, keeps the go2rtc HTTP API bound to `127.0.0.1`, and proxies WebRTC signaling through authenticated NMS routes.
- Browser playback now prefers WebRTC `<video>` and automatically falls back to the existing MJPEG stream when WebRTC/go2rtc is unavailable.
- RTSP URLs and credentials remain managed by NMS; the browser does not receive direct RTSP credentials.
- Request logging now redacts sensitive query parameters such as `token`, `password`, `username`, and `auth`.
- Windows packages include `go2rtc.exe`; Linux packages include both `go2rtc_linux_amd64` and `go2rtc_linux_arm64`.
- Existing FFmpeg recording, RTSP preview/recording split, TCP/UDP transport settings, UDP port ranges, DB, and License behavior are preserved.

---
## v1.2.4.3.3 | 2026-04-27

**Build purpose:** More aggressive low-latency MJPEG preview tuning.

### Camera Streaming
- Live-preview FFmpeg now reduces RTSP input buffering, packet reordering, probe delay, and output mux delay more aggressively.
- Preview FPS limiting is now handled in the video filter chain instead of output frame-rate synchronization, reducing frame scheduling delay.
- MJPEG HTTP responses now send `X-Accel-Buffering: no` and stricter no-cache headers so reverse proxies are less likely to buffer live frames.
- This remains an MJPEG compatibility path; sub-second latency depends on the source encoder GOP/B-frame settings and network/proxy path.

---
## v1.2.4.3.2 | 2026-04-27

**Build purpose:** Low-latency live-preview hotfix for RTSP encoder/camera streams.

### Camera Streaming
- MJPEG preview now keeps only the newest frame per browser client, preventing slow viewers from accumulating old frames and drifting behind real time.
- FFmpeg live-preview input now uses a tighter low-latency probe/buffer profile and disables audio handling for preview-only MJPEG output.
- No database migration is required; existing preview/recording RTSP URLs, transport settings, UDP port ranges, and license state are preserved.

---
## v1.2.4.3.1 | 2026-04-27

**Build purpose:** Hotfix for slow/unstable camera preview after v1.2.4.3 encoder compatibility changes.

### Camera Streaming
- New and migrated cameras now default RTSP transport to `TCP`; `Auto (TCP -> UDP)` remains available only when the site needs fallback probing.
- Live preview now applies a default 720p-class MJPEG preview profile when no preview bitrate limit is configured, reducing browser and CPU load.
- Hardware decode fallback timeout was shortened so unsupported GPU decode switches to CPU faster.
- Background offline-camera recovery no longer opens FFmpeg snapshot probes; cameras are marked online only after an actual snapshot or live preview frame succeeds.
- MJPEG FFmpeg input now uses low-latency flags to reduce buffering delay.

---
## v1.2.4.3 | 2026-04-27

**Build purpose:** Camera/encoder RTSP compatibility build for PoC sites using mixed camera sources.

### Camera Streaming
- Preview and snapshot decoding keep GPU-first behavior and fall back to CPU when hardware decode fails, times out, or produces no first frame.
- RTSP preview and recording remain separate fields so low-bitrate substreams can be used for preview while recording uses the main stream.
- RTSP transport can now be set per camera: `Auto`, `TCP`, or `UDP`.
- `Auto` tries TCP first and falls back to UDP for encoder/NAT/firewall cases where RTP media does not arrive over TCP.
- UDP RTP port range can be configured per camera, for example `30000-30200` for encoder deployments.
- The same transport settings are applied to snapshots, live MJPEG preview, and NVR recording.
- Multiple channels from one encoder IP are supported as separate camera rows when their RTSP URLs differ.

### Operations
- Anonymous RTSP URLs such as `rtsp://host:port/stream1` stay credential-free when username is blank.
- If a camera is saved without a username, stale encrypted camera passwords are cleared to avoid misleading credential state in diagnostics.
- Stream logs and audit/config-change details redact URL credentials, password fields, tokens, and SNMP/CLI secrets.

---
## v1.2.4.1 | 2026-04-26

**Build purpose:** Inspection build for validating corrected translations and stable local SVG icons.

### Frontend
- Corrected Logs & Audit translations across zh-TW, zh-CN, ja-JP, and ko-KR.
- Added a local SVG icon layer for navigation, status cards, header actions, and mobile navigation.
- Bumped static cache version to v1.2.4.1-i18n-icons1.


---
## v1.2.4-PoC | 2026-04-24

**本次版本定位:** 可直接交付客戶測試的 PoC 版，並保留同套程式轉正式 UUID 授權的出貨路徑。  
**主要目標:** 預設以 `0` 授權啟動；由 PoC License 解鎖測試範圍；未來切換成標準 UUID 授權時，不需要重新更換軟體。

### PoC Runtime
- 版本封裝調整為 `v1.2.4-PoC`，方便與現行 `v1.2.4` 主線對齊。
- PoC 版預設 `default_device_limit = 0`，裝置管理維持鎖定狀態。
- 客戶測試時可另外匯入 `PoC License`，由授權內容決定可用裝置數與模組能力。

### License Promotion
- `/api/v1/system/info` 與 Dashboard 版本字樣改為依實際授權狀態動態切換。
- 當系統存在有效的標準 UUID 授權時，版本字樣自動從 `v1.2.4-PoC` 轉為 `v1.2.4`。
- PoC License 與 Trial 不會移除 `PoC` 字樣，避免與正式出貨授權混淆。

### Frontend Alignment
- 登入頁與模式選擇頁會在載入後同步後端 runtime 版本字樣。
- 靜態資產版號維持 `v1.2.4`，後續正式授權切換時不需要更換整套軟體。

---
## v1.2.4 | Candidate

**本次版本定位:** 日誌與稽核強化版。  
**主要目標:** 將既有零散的系統日誌與稽核紀錄，重構為可搜尋、可匯出、可追溯、可支援後續維運與合規審查的正式模組。

### Logs & Audit
- 新增獨立主欄位：`日誌與稽核`
- 拆分四類紀錄：
  - `稽核日誌`
  - `系統日誌`
  - `設備日誌`
  - `設定變更`
- 各類日誌支援獨立查詢、篩選與個別匯出

### Audit Coverage
- 補齊高價值操作稽核：
  - 使用者登入 / 登出 / 登入失敗
  - 授權新增 / 啟用 / 試用授權
  - 設備新增 / 修改 / 刪除
  - 攝影機新增 / 修改 / 刪除
  - 拓樸變更
  - PoE 開 / 關 / 重啟
  - 設備重啟
  - 系統設定變更
- 稽核 detail 改為結構化 JSON，而非鬆散字串
- 補入 `recordset_id` 與 `correlation_id` 以串聯同一批操作

### Config Change
- 新增設定變更紀錄
- 針對設備、攝影機、授權、拓樸、系統設定保留 `before / after` 差異
- 區分變更來源：
  - `manual`
  - `batch`
  - `auto_discovery`
  - `system_sync`

### Presentation And Export
- 前端顯示統一中文化，再由 i18n 載入其他語系
- 不再直接顯示 raw action key、英文 reason 或未格式化 detail
- 匯出格式至少支援：
  - `CSV`
  - `JSON`
- 匯出內容包含篩選條件、時間範圍、匯出時間與筆數

### Compliance Direction
- 對齊智慧建築制度演進需求與管理維護可追溯性
- 強化對 `ISO/IEC 27001`、`ISO/IEC 27701`、`ISO 9001` 的證據支援能力
- 版本重點為「更接近可稽核」，不是直接宣稱完成認證

### Reference
- 詳細規格請參考：
  - `docs/AUDIT_AND_LOGGING_SPEC.md`
  - `docs/V1_2_4_SCOPE.md`

---
## v1.2.1-PoC | 2026-04-20

**Summary:** Introduces the dedicated PoC licensing line, locks device management by default, and adds a dual-mode generator for Formal and PoC licenses.

### License / PoC
- New `v1.2.1-PoC` line with `default_device_limit = 0` by default.
- Device management stays locked until a valid license enables `device_management` or grants licensed device capacity.
- PoC licenses are not bound to Machine ID and support custom device count and exact expiry time.
- Formal licenses remain Machine ID bound.

### Generator
- Added a dedicated generator with separate `Formal License` and `PoC License` interfaces.
- Formal licenses require Machine ID and generate machine-bound keys.
- PoC licenses can define custom expiry time and device count without UUID binding.

### Frontend / Gate
- Device and topology entry points now rely on `/api/v1/license/features` and `device_management`.
- Unauthorized device and topology read/write APIs are blocked by the backend.

---
## v1.2.3 | 2026-04-20

**本次更新摘要:** 完成監控模式與拓樸圖穩定化、攝影機預覽切換修復、錄影命名修正，以及版本與發布資訊收斂。

### Monitor / Topology
- 修正監控模式拓樸圖與 tooltip 殘留的 `??` / 亂碼顯示問題。
- 監控模式樹狀收合後，底下若有離線設備，父節點會正常顯示紅色呼吸燈提示。
- 管理模式後台拓樸圖同步補上相同的收合離線提示，不再只在 Monitor 生效。
- 拓樸統計欄位與速率文字顯示已統一收斂，避免再出現殘留的開發字串。

### Camera / Recording
- 修正從 `Monitor` 切回管理模式後，攝影機預覽可能卡住不再更新的 backend stale stream 問題。
- 攝影機管理頁的預覽流程已回到穩定模式，避免 fullscreen 或頁面切換後長時間停住。
- 錄影清單與匯出檔名改為優先使用 `camera_recordings.camera_name`，不再退回 `cam1/cam2/cam4` 類型命名。
- 新錄影會保留當下攝影機名稱快照，後續即使設備 ID 改動，錄影名稱仍可正確對應。

### UI / Release
- 清理前後台攝影機模組、錄影管理、刪除按鈕、監控提示中的可見亂碼與 raw key 顯示。
- 版本顯示與靜態資產 cache-busting 版號已統一收斂到 `v1.2.3`。
- 本版 release artifact 會同步帶入最新 `RELEASE_NOTES.md` 與 `RELEASE_NOTE.txt`。

### License
- 正式授權新增永久授權規則：當年限達到 `50` 年以上時，系統自動判定為 `永久授權`。
- NMS 授權頁會針對永久授權統一顯示 `永久授權`，不再顯示一般到期日期。

---

## v1.2.2alpha4 — 2026-04-17

**核心更新:** 頻寬超限告警通知、攝影機密碼解密容錯、拓樸樹狀圖層次修正

### 🔔 告警通知 (Bandwidth Alerts)
- **攝影機頻寬超限告警：** 攝影機 MJPEG 串流超過頻寬限制時，寫入鈴鐺通知（改自 events 表 → notifications 表）
- **網路設備頻寬超限告警：** SNMP 輪詢時若介面流量超過 link speed 的 80%，自動寫入 notifications，5 分鐘冷卻
- 告警通知可在右上角鈴鐺面板查看、標記已讀、清除

### 🌐 拓樸圖修正 (Topology Fix)
- **樹狀圖層次修正：** 移除「同類型 switch 直連視為 HA pair」的錯誤推斷，只有明確 `link_type=ha/vrf/spine_leaf` 才合併同層，IPCAM 等下游設備現在正確掛在其上游 switch 下

### 📷 攝影機模組 (Camera Module)
- **密碼解密容錯：** 舊版加密密碼無法以當前 AES-GCM key 解密時，自動 fallback 解析 RTSP URL 中的明文密碼（`rtsp://user:pass@host/`），修復 401 Unauthorized 問題

---

## v1.2.2alpha3 — 2026-04-16

**核心更新:** 拓樸樹狀圖節點收折、IPCAM Bitrate 設定、監控模式拓樸修正

### 🌐 拓樸圖優化 (Topology Enhancements)
- **樹狀拓樸節點收折/展開：** 點選有子設備的節點可收折/展開；收折節點右下角顯示數字視覺提示
- **收折狀態持久化：** 節點收折狀態儲存在瀏覽器 `localStorage`，重新整理頁面可自動恢復
- **統一渲染引擎：** 主頁面 (Monitor) 與管理模式共享同一套拓樸渲染邏輯，確保視覺與設定一致性

### 📷 攝影機模組改善 (Camera Module)
- **攝影 Bitrate 設定：** 攝影機設定中新增「錄影 Bitrate (kbps)」欄位
- **轉碼錄影：** 若設定 Bitrate > 0，錄影程序啟用 `libx264` 轉碼以符合頻寬需求；設為 0 則維持 `Stream Copy` 高效模式
- **ONVIF 相容性：** 錄影來源支援 ONVIF 協定連接，確保錄影流能正確更新

### 🔧 系統改善
- **編譯版本更新：** `build_both_releases.ps1` 支援 v1.2.2alpha3 版本，Windows 與 Linux 同步編譯
- **快取清除：** 版本更新後自動 append `?v=v1.2.2alpha3` 至資源檔案，防止舊版 JS 緩衝衝突

---

## v1.2.0 — 2026-04-08

**核心更新:** 監控模式正式化、雙模式登入架構、角色導向主控台、多語系 i18n 補完

### 📺 監控模式 (Demo → Monitor 正式化)
- `demo.html` 正式更名為 `monitor.html`，路由從 `/demo` 改為 `/monitor`
- 頁面標題、Badge、退出按鈕文字全面更新為「監控模式」
- Badge 與標題後綴支援 5 語系即時切換（繁中 / 簡中 / EN / 日本語 / 한국어）
- 系統名稱顯示修正：優先顯示 sysName，其次自定義名稱，fallback IP
- 拓樸連線消失問題修復：自動刷新時比對 node/link ID，結構有變化則觸發完整 rebuild

### 🎛️ 雙模式主控台 (新架構)
- 新增 `mode-selection.html`：登入後統一跳至此頁選擇模式
- **角色導向顯示規則：**
  - `admin` / `editor`：顯示管理模式 + 監控模式兩張卡片
  - `viewer`：僅顯示監控模式，無法進入管理介面
- 主控台支援語系切換、深色/淺色主題、登出
- 管理模式右上角「監控模式」按鈕改為「🏠 主控台」，跳回模式選擇頁
- 監控模式「離開監控」跳回主控台（保留 token，不需重新登入）

### 🔐 登入流程重構
- 登入成功後統一跳至 `mode-selection.html`，不再在登入頁選模式
- 登入與變更密碼後強制 cache-bust（等同 Ctrl+F5），確保載入最新 JS/CSS
- 已登入狀態開啟登入頁，自動跳至主控台選擇頁（而非管理模式）

### 🌐 i18n 補完
- 修復登入頁「登入模式」下拉選單未隨語系切換的問題
- 各語言檔補充 `login.mode_label` / `login.mode_admin` / `login.mode_monitor`
- 修復管理模式「資源使用率」標題硬碼問題，改用 `data-i18n="admin.resource_usage"`
- 修復攝影機編輯 Modal 所有標籤、placeholder、按鈕改用 `t()` i18n 呼叫
- 刪除確認對話框、操作成功/失敗 Toast 全面 i18n 化

### 🔧 Build 流程改善
- `build_both_releases.ps1` 同時 Windows + Linux 雙平台版本一鍵編譯
- 新增 sp999 防呆機制：偵測到 sp999 版本時要求輸入 YES 才繼續
- 版本號統一讀取 `config.go`，腳本無需手動修改版本號

---

## v1.1.0sp2 — 2026-04-07

**核心更新:** Demo 展示模式完善、攝影機 RWD 改善、Viewer 權限強化、UI Bug 修復

### 📺 Demo 展示模式（更新）
- 完整實裝 Demo 頁面（`/demo`）：無需登出，無需重新登入
- 左側：互動式網路拓樸圖（D3.js，可縮放/拖曳）
- 右側：設備統計卡片 + 即時事件 Log
- 支援 5 種語系：繁中 / 簡中 / EN / 日本語 / 한국어
- 支援深色 / 淺色主題切換（持久化至 localStorage）
- 5 秒自動刷新，更新設備 on/off 狀態 + 線路流量（不重置 zoom/pan）
- 自動更新時樣保持 zoom/pan 位置（修復首次載入 bug）
- 流量格式統一使用 SI bps（修復與管理模式數值不一致問題）

---

## v1.1.0 — 2026-04-03

**核心更新:** EdgeCore SSH 整合、PoE 控制、攝影機 GPU 加速、ffmpeg 自動下載

### 🔌 EdgeCore SSH 整合
- 支援 ECS2100-10P、ECS4150、ECS2220 系列
- SSH 指令序列：備份、重啟、PoE ON/OFF/Recycle
- isPrompt 修正：`!<stackingDB>` 不再誤判為 prompt

### ⚡ PoE 控制
- PoE ON/OFF/Recycle（SSH 優先，SNMP fallback）
- 每 port 即時功耗（EdgeCore private OID）
- PoE 功率摘要卡片（總功率 / 已使用 / 進度條 >85% 紅 >60% 橙）

### 🎥 攝影機強化
- GPU 加速串流（NVIDIA CUDA / Intel QSV / AMD VAAPI / D3D11VA）
- 攝影機分頁（16ch/頁，最多 64ch）
- 攝影機 status 修正（第一幀成功後才寫 online）
- ffmpeg 自動下載（找不到時從 GitHub BtbN builds 下載）

### 🔧 其他
- 設備離線時介面速率 tab 灰化 + 警告 banner
- 系統狀態頁 GPU 規格顯示
- PoE 0W 顯示「非 PoE 設備 / 0W」
