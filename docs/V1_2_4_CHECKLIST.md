# v1.2.4 Remaining Checklist

## Status
- Version: `v1.2.4`
- Theme: `Logs And Audit Hardening`
- Current Progress: `In Progress`

## Summary
This checklist tracks the remaining work for `v1.2.4` after the initial logging model, API routes, export baseline, evidence bundle flow, review workflow, and core write coverage were completed.

Completed baseline:
- `audit_logs / system_logs / device_logs / config_change_logs` data model
- log center API and frontend four-tab skeleton
- search, time range, scope filter, guided filters, CSV and JSON export
- evidence bundle export
- structured audit writes and config change capture
- retention policy configuration and summary display
- primary write coverage for `auth / license / poe / reboot / backup / camera / topology / device CRUD / bulk scan / topology discovery`

## P0
- [x] Normalize the frontend module naming to `日誌與稽核` and remove remaining garbled labels in the log center UI
- [x] Replace remaining runtime fallback strings in `logs.js` with clean source strings and stable i18n hooks
- [x] Add more high-value write points for periodic license validation and expiry / lock events
- [x] Add more high-value write points for camera background jobs, stream recycle, and scheduled recovery events
- [ ] Verify the log center end-to-end in browser for:
  - [ ] audit log tab
  - [ ] system log tab
  - [ ] device log tab
  - [ ] config change tab
  - [ ] CSV export
  - [ ] JSON export

## P1
- [x] Add review workflow fields and actions:
  - [x] review status
  - [x] reviewed by
  - [x] reviewed at
  - [x] review note
- [x] Add acknowledge workflow for device logs
- [x] Add export package workflow for audit evidence bundles
- [x] Add richer frontend filters:
  - [x] actor filter
  - [x] device filter
  - [x] status filter
  - [x] module filter as guided selector instead of free-text only
- [x] Improve audit detail rendering so structured JSON is shown as human-readable fields instead of raw payloads

## P2
- [x] Expand i18n coverage for the `日誌與稽核` module across all supported locales
- [x] Add retention policy configuration and retention metadata display
- [x] Add evidence metadata:
  - [x] recordset id visibility
  - [x] correlation id visibility
  - [x] export manifest / checksum
- [ ] Add more device-side sources:
  - [x] discovery follow-up events
  - [x] backup restore completion events
- [x] future syslog / trap normalization pipeline hooks

## Suggested Execution Order
1. Finish `P0` browser smoke validation
2. Finish `P2` i18n coverage for `日誌與稽核`
3. Expand device-side ingestion beyond internal events into future syslog / trap normalization
4. Run a final end-to-end validation and release packaging pass for `v1.2.4`

## Current Working Note
The next active tranche should focus on:
- validating the `日誌與稽核` module end-to-end in browser
- confirming retention save flow against live system config updates
- expanding device-side normalization hooks for future syslog / trap ingestion
