# Management Server Release Notes

Version: `v1.2.4.8`  
Date: `2026-06-10`

## Release Positioning

`v1.2.4.8` is the customer integration stable release for API, iframe embed, IoT gateway integration, and security hardening.

## Main Changes

- API manual updated to `v1.2.4.8`.
- Defined `Ready`, `Bridge Ready`, and `Planned`.
- `/api/v1/integrations/network-snapshot` is the customer dashboard bootstrap API.
- iframe embed tokens support create, list, revoke, and expiry checks.
- IoT direct Modbus TCP and REST ingest are `Ready`.
- MQTT, OPC-UA, and BACnet are `Bridge Ready`.

## Security Hardening

- Devices and PDU APIs no longer expose `snmp_community`.
- Blank SNMP community updates preserve existing values.
- Camera RTSP URLs redact credentials and sensitive query parameters.
- Audit/log details redact password, token, API key, secret, authorization, and SNMP community fields.
- iframe and IoT UI now include guidance for token scope, HTTPS, allowlists, and OT network isolation.

## UI

- Fixed long-string overflow in audit, admin, embed, and IoT tables.
- Mobile and tablet tables use controlled horizontal scrolling.
- Monitor mode name is confirmed as `電視牆模式`.

## Packages

- Windows amd64: `artifacts/windows/v1.2.4.8.zip`
- Linux amd64 + arm64: `artifacts/linux/v1.2.4.8.zip`
- Linux arm64: `artifacts/linux-arm64/v1.2.4.8.zip`

## Verified

- Backend `go test ./...` passed.
- Frontend `node --check` passed.
- Static mirrors synced.
- Windows package starts through `start_nms.bat` and reports `v1.2.4.8`.
- Linux arm64 binary validated as AArch64 ELF.

