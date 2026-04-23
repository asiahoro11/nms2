# v1.2.4 Scope Definition

## Version Identity
- Version: `v1.2.4`
- Theme: `Logs And Audit Hardening`
- Status: `Defined`

## Goal
`v1.2.4` is defined as the release dedicated to rebuilding the current logging and audit capability into a formal, exportable, traceable, and compliance-oriented management module.

This release is not primarily a UI refresh release and not primarily a device-feature release.

Its main purpose is to improve:

- auditability
- traceability
- maintenance visibility
- exportability
- operational accountability

## Primary Scope

### 1. New Main Module
Introduce a dedicated primary navigation entry:

- `日誌與稽核`

Recommended sub-tabs:

- `稽核日誌`
- `系統日誌`
- `設備日誌`
- `設定變更`

### 2. Audit Log Expansion
Expand audit coverage to include at least:

- login
- logout
- login failure
- license activation
- trial activation
- device create
- device update
- device delete
- camera create
- camera update
- camera delete
- topology update
- PoE on
- PoE off
- PoE recycle
- device reboot
- system setting update

Each audit row should preserve:

- actor
- source IP
- source MAC when available
- resource type
- resource id
- resource name
- resource IP
- resource MAC
- status
- detail JSON
- recordset id
- correlation id

### 3. Config Change Tracking
Add dedicated configuration change logging for:

- devices
- cameras
- topology
- license settings
- system settings

Each change record should preserve:

- target type
- target id
- target name
- old values
- new values
- change source
- approval status when relevant

### 4. System Log Separation
Separate platform runtime events from human audit events.

System log should include:

- service start and stop
- scheduler events
- stream restart or failure
- background worker errors
- license validation failures
- database and migration errors
- cache or integration errors

### 5. Device Log Separation
Separate equipment-generated events from human audit events.

Device log should be designed to hold:

- syslog
- SNMP trap
- vendor alarm
- normalized device events

### 6. Display And Localization
Frontend display must:

- use Chinese as the default visible wording
- use i18n for alternate languages
- stop showing raw backend keys such as `login_failed`
- stop showing raw detail strings like `reason: wrong_password`

### 7. Export
Each log class must support independent export.

Minimum formats:

- `CSV`
- `JSON`

Each export should support:

- time range
- user filter
- module filter
- device filter
- status or severity filter
- keyword search

## Compliance-Oriented Objective
`v1.2.4` should improve the system's ability to support audit and maintenance evidence under:

- Taiwan smart-building operational expectations
- privacy and personal-data governance expectations
- cybersecurity governance expectations
- ISO/IEC 27001 evidence needs
- ISO/IEC 27701 privacy-accountability needs
- ISO 9001 traceability and documented-information needs

This version should be described as:

- `audit-ready improvement`
- `evidence-oriented logging hardening`

It should not be described as:

- `ISO certified`
- `fully legally compliant by software alone`

## Non-Goals
The following are not the primary scope of `v1.2.4` unless they directly support logs and audit:

- major topology redesign
- camera preview architecture rewrite
- mobile app delivery
- large licensing redesign
- new NVR feature work

## Done Criteria
`v1.2.4` should only be considered complete when:

- a dedicated `日誌與稽核` module exists
- audit coverage is materially expanded
- config changes preserve before/after state
- system logs are separated from audit logs
- device logs are separated from audit logs
- all log classes support search and export
- visible text is localized and human-readable
- release notes and acceptance files are updated

## Dependencies
Implementation should follow:

- `docs/AUDIT_AND_LOGGING_SPEC.md`
- `docs/FINAL_ACCEPTANCE_v1.2.3.md`

## Suggested Delivery Order
1. Expand and standardize `audit_logs`
2. Add `config_change_logs`
3. Add `system_logs`
4. Add `device_logs`
5. Build frontend `日誌與稽核`
6. Add export and evidence metadata

## Release Statement
`v1.2.4` is formally defined as the release for logging, audit, and evidence hardening.
