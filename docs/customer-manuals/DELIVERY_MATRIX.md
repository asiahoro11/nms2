# Customer Manual Delivery Matrix

Version: `v1.2.4.8`

## Languages

| Language | Folder | Status |
| --- | --- | --- |
| Traditional Chinese | `zh-TW` | PASS |
| Simplified Chinese | `zh-CN` | PASS |
| English | `en-US` | PASS |
| Japanese | `ja-JP` | PASS |
| Korean | `ko-KR` | PASS |

## Files

Each language folder contains the same customer-facing manual set:

| File | Purpose | Status |
| --- | --- | --- |
| `api-manual.md` | Customer API, iframe, IoT, gateway, security, smart-building alignment | PASS |
| `deployment-manual.md` | Windows/Linux deployment and upgrade | PASS |
| `database-maintenance-guide.md` | Backup, restore, health check, security maintenance | PASS |
| `database-encryption-guide.md` | Encrypted backup and retention guidance | PASS |
| `release-notes.md` | v1.2.4.8 customer release summary | PASS |
| `iot-edge-buffer-update.md` | v1.2.4.8 Modbus TCP background polling and store-and-forward update | PASS |

## Customer Package Recommendation

For customer delivery, include the full `docs/customer-manuals/` folder beside:

- `artifacts/windows/v1.2.4.8.zip`
- `artifacts/linux/v1.2.4.8.zip`
- `artifacts/linux-arm64/v1.2.4.8.zip`

Do not treat internal engineering specs, old acceptance checklists, roadmap documents, or proposal drafts as customer manuals.
