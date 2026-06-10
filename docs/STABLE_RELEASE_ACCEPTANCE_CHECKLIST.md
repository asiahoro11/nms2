# Stable Release Acceptance Checklist

This checklist tracks release-readiness for the current `new_nms_sync` stable line.

Current target:

- release version: `v1.2.4.8`
- release purpose: customer integration stable release
- default port: `8080`
- frontend source of truth: `apps/frontend`
- backend runtime version source: `apps/backend/config/config.go`
- customer API document: `docs/API_MANUAL.md`
- release notes: `docs/RELEASE_NOTES.md`

## Acceptance Status Legend

- `PASS`: verified in current workspace/runtime
- `PENDING`: not yet verified end-to-end
- `BLOCKED`: known issue prevents sign-off
- `N/A`: outside this release scope

## A. Version and Documentation Consistency

- `PASS` `docs/API_MANUAL.md` is updated to `v1.2.4.8`.
- `PASS` `docs/RELEASE_NOTES.md` includes the `v1.2.4.8` customer integration stable release notes.
- `PASS` API readiness states are explicitly defined as `Ready`, `Bridge Ready`, and `Planned`.
- `PASS` Backend version source reports `v1.2.4.8` from `apps/backend/config/config.go`.
- `PASS` Frontend visible version and service-worker cache report `v1.2.4.8`.
- `PASS` Runtime `/api/v1/system/info` reports `v1.2.4.8` in the Windows package smoke test.

Sign-off rule:

- customer-facing docs, backend runtime version, frontend visible version, and packaged manifests must all report `v1.2.4.8`
- this docs/build pass does not modify Go or frontend application code

## B. Customer Integration API

- `PENDING` `POST /api/v1/auth/login` returns JWT and expected user envelope.
- `PENDING` `POST /api/v1/auth/verify-2fa` handles enabled 2FA challenge flow.
- `PENDING` `GET /api/v1/integrations/network-snapshot` returns dashboard, devices, topology, and schema version `nms.integration.network_snapshot.v1`.
- `PENDING` `include`, `page`, `limit`, `search`, `type`, and `status` filters behave as documented.
- `PASS` Snapshot payload omits sensitive fields such as SNMP communities, passwords, license keys, and RTSP URLs.
- `PENDING` Existing smaller read APIs remain available for drill-down clients.

Sign-off rule:

- a customer dashboard can bootstrap from one snapshot request after authentication
- sensitive secrets must not appear in the customer integration payload

## C. iframe Embed Integration

- `PENDING` Admin can issue scoped embed tokens through `/api/v1/integrations/embed-tokens`.
- `PENDING` Embed token list and revoke APIs work after token creation.
- `PENDING` Revoked and expired embed tokens are rejected by embed snapshot requests.
- `PENDING` `/api/v1/integrations/settings` can read and update `frame_ancestors`.
- `PENDING` Runtime CSP reflects DB-backed iframe allowlist without requiring restart.
- `PENDING` `/embed.html` loads supported views with read-only data.

Sign-off rule:

- iframe customers must use scoped embed tokens instead of admin JWTs in iframe URLs
- customer origins must be controlled by `frame_ancestors`

## D. IoT and Gateway Integration

- `PENDING` Modbus TCP device create, update, delete, and poll APIs work for admin/editor roles as documented.
- `PENDING` REST ingest upserts by `external_id` and stores measurements.
- `PENDING` `/api/v1/iot/capabilities` returns documented readiness profiles.
- `PASS` Readiness terminology separates `Ready`, `Bridge Ready`, and `Planned` paths in the API manual.
- `PENDING` MQTT bridge ingest test posts normalized payloads to `/api/v1/iot/ingest`.
- `PENDING` OPC-UA bridge ingest test posts normalized payloads to `/api/v1/iot/ingest`.
- `PENDING` BACnet bridge ingest test posts normalized payloads to `/api/v1/iot/ingest`.

Sign-off rule:

- direct Modbus TCP and REST ingest are the production-ready runtime paths
- MQTT, OPC-UA, and BACnet are customer gateway integrations until native clients are implemented

## E. Security and Access Control

- `PENDING` Integration write APIs require editor or admin role.
- `PENDING` Admin-only integration settings and embed-token APIs reject non-admin roles.
- `PENDING` License and feature gates block disabled modules consistently.
- `PENDING` Audit logs record integration settings and write operations.
- `PENDING` JWT and embed-token failures return safe error bodies without leaking secrets.

Sign-off rule:

- no customer integration path may bypass RBAC, license gates, or audit expectations

## F. Packaging

- `PASS` Windows release build produces `artifacts/windows/v1.2.4.8.zip`.
- `PASS` Standard Linux release build produces `artifacts/linux/v1.2.4.8.zip`.
- `PASS` Standard Linux artifact includes `nms_server_linux_amd64` and `nms_server_linux_arm64`.
- `PASS` Dedicated Linux ARM64 package produces `artifacts/linux-arm64/v1.2.4.8.zip`.
- `PASS` Linux ARM64 package includes `nms_server_linux_arm64`, `start_nms.sh`, `bin/go2rtc_linux_arm64`, `API_MANUAL.md`, `RELEASE_NOTES.md`, and `RELEASE_NOTE.txt`.
- `PASS` ARM64 binary validates as AArch64 ELF.
- `PENDING` Linux package launches on Ubuntu ARM64.
- `PASS` Windows package rebuild and launch smoke test through `start_nms.bat`.

Packaging policy:

- default release builds should stay clean and readable
- JS obfuscation and HTML minification are disabled by default to reduce false-positive AV risk
- only enable obfuscation explicitly when needed through `NMS_ENABLE_OBFUSCATION=1`
- Go binary obfuscation is optional and only activates when `NMS_ENABLE_BINARY_OBFUSCATION=1`
- protected builds prefer `garble build`; when `garble` is missing the scripts warn and fall back to normal `go build`

## G. Runtime Smoke Test

- `PENDING` Login, mode-selection, dashboard, and monitor routes load without 404.
- `PENDING` Dashboard, devices, topology, cameras, logs, and admin pages open without fatal JS errors.
- `PENDING` Camera preview and monitor layout remain stable during page switching.
- `PENDING` Topology remains readable and interactive after repeated refreshes.
- `PENDING` Event and notification text contains no malformed `%!(EXTRA ...)` formatting.
- `PENDING` Cross-language smoke test for `zh-TW`, `en-US`, `zh-CN`, `ja-JP`, and `ko-KR`.

Sign-off rule:

- no current stable release sign-off until runtime smoke, packaging smoke, and version consistency are all verified

## Recommended Execution Order

1. Bump runtime and frontend version sources to `v1.2.4.8`.
2. Run backend and frontend sanity checks.
3. Run customer integration API smoke tests.
4. Run iframe embed and token revocation smoke tests.
5. Run IoT direct and bridge-ingest smoke tests.
6. Build standard Linux and Windows release artifacts.
7. Build dedicated Linux ARM64 package.
8. Validate packaged runtime on Windows and Ubuntu ARM64.

## Current Release Readiness Summary

- ready for continued verification: yes
- customer integration documentation updated: yes
- dedicated ARM64 packaging path available: yes
- version consistency stabilized: yes
- backend/frontend static syntax checks passed: yes
- backend Go tests passed: yes
- stable release sign-off complete: no

Main remaining risks:

- Windows package smoke test passed through `start_nms.bat`
- customer integration APIs need end-to-end runtime smoke tests
- iframe token revoke and CSP allowlist behavior need runtime verification
- Linux ARM64 package build output is complete  Ubuntu ARM64 launch validation still needs target hardware
