# Audit And Logging Spec

## Purpose
This document defines the target architecture for `new_nms_sync` logging, auditability, exportability, and compliance-oriented evidence retention.

The goal is not only to improve troubleshooting, but to make the system materially more useful for:

- operational management
- security investigation
- maintenance traceability
- vendor accountability
- internal and external audit preparation

This document is also the baseline for aligning the product with Taiwan smart-building evaluation requirements and management-system style audit expectations such as ISO/IEC 27001, ISO/IEC 27701, and ISO 9001.

## Important Boundary
This specification is designed to make the product more audit-ready and evidence-friendly.

It does **not** by itself guarantee certification or legal approval. Actual ISO certification and legal compliance still depend on:

- organization policy
- retention policy
- role assignment
- review workflow
- operating procedures
- training records
- supplier governance
- formal audit execution

## Regulatory Baseline

### Taiwan Smart Building Baseline
For Taiwan smart-building requirements, the system should be aligned to the public regulatory and evaluation path maintained by the Ministry of the Interior Architecture and Building Research Institute and related public-sector implementation rules.

The public version path relevant to the 2006-2024 window is:

- `2003` smart building evaluation handbook established the original 7-indicator system
- `2011` smart building handbook introduced 8 indicators and graded evaluation
- `2016` smart building handbook moved toward total-score based evaluation and clearer review documents
- `2024` smart building handbook significantly updated the evaluation framework to reflect AI, IoT, data integration, and newer building operation needs

For practical product design, the current target baseline should be:

- `智慧建築標章申請審核認可及使用作業要點`
- `智慧建築評估手冊 2024 年版`
- public-sector smart green building implementation rules for applicable government projects
- related building technical regulations where building operation, safety, and system evidence are required

### Related Taiwan Legal/Policy Domains
The logging architecture should also be designed to support adjacent legal and governance obligations commonly touched by an NMS platform:

- personal data protection
- cybersecurity governance
- operational traceability
- maintenance accountability
- configuration change history

The most relevant legal/policy families to consider are:

- `個人資料保護法`
- `資通安全管理法`
- `建築法 / 建築技術規則`
- smart-building label operation rules and handbooks

### ISO-Oriented Governance Baseline
The platform should be designed to support evidence collection and traceability that map well to:

- `ISO/IEC 27001`
- `ISO/IEC 27701`
- `ISO 9001`

The system should therefore support:

- access accountability
- change traceability
- structured incident records
- documented information retention
- exportable evidence
- reviewable historical records

## Design Principle
Do not treat every event as one flat log stream.

The product should expose a dedicated primary navigation section:

- `Logs & Audit`

Under that section, the system should separate records into four distinct classes:

1. `Audit Log`
2. `System Log`
3. `Device Log`
4. `Config Change Log`

Each class serves a different purpose and should be independently searchable and exportable.

## Log Classes

### 1. Audit Log
Purpose:

- answer who did what, when, from where, to which object, and with what result

Typical sources:

- login and logout
- failed login
- license activation
- trial activation
- user management
- device CRUD
- camera CRUD
- topology modification
- PoE control
- device reboot
- privilege-sensitive system operations

Required fields:

- `id`
- `time`
- `actor_id`
- `actor_name`
- `actor_role`
- `source_ip`
- `source_mac`
- `module`
- `action`
- `resource_type`
- `resource_id`
- `resource_name`
- `resource_ip`
- `resource_mac`
- `status`
- `detail_json`
- `recordset_id`
- `correlation_id`

### 2. System Log
Purpose:

- record platform-side runtime events for maintenance and diagnostics

Typical sources:

- service startup
- service shutdown
- background task failure
- stream rebuild
- scheduler error
- database migration
- license validation failure
- cache error
- API exception

Required fields:

- `id`
- `time`
- `service`
- `level`
- `event_code`
- `message`
- `context_json`
- `node_name`
- `build_version`

### 3. Device Log
Purpose:

- record events emitted by managed devices rather than by human operators

Typical sources:

- syslog
- SNMP trap
- vendor alarm
- PoE fault
- port state change
- temperature or power alert
- camera event

Required fields:

- `id`
- `time`
- `device_id`
- `device_name`
- `device_ip`
- `device_mac`
- `facility`
- `severity`
- `raw_message`
- `normalized_message`
- `matched_rule`
- `ack_status`
- `context_json`

### 4. Config Change Log
Purpose:

- preserve before/after evidence for configuration changes

Typical sources:

- device config edit
- camera config edit
- license config change
- topology change
- system setting change
- bulk operation
- discovery-driven updates

Required fields:

- `id`
- `time`
- `actor_id`
- `actor_name`
- `target_type`
- `target_id`
- `target_name`
- `change_scope`
- `change_source`
- `approval_status`
- `old_values_json`
- `new_values_json`
- `recordset_id`
- `correlation_id`

## Action Taxonomy
Action values must be canonical keys in the backend and rendered to localized text in the frontend via i18n.

Recommended initial action set:

- `login`
- `logout`
- `login_failed`
- `activate_license`
- `activate_trial_license`
- `create_user`
- `update_user`
- `delete_user`
- `create_device`
- `update_device`
- `delete_device`
- `reboot_device`
- `control_poe_port`
- `control_port_status`
- `create_camera`
- `update_camera`
- `delete_camera`
- `update_topology`
- `update_system_settings`
- `export_logs`
- `backup_system`
- `restore_system`

## Display Rule
Backend stores structured keys.

Frontend displays localized human-readable text.

Example:

- backend action: `login_failed`
- frontend text in zh-TW: `登入失敗`
- frontend text in en-US: `Login failed`

Raw strings like the following should not be shown directly in the UI:

- `login_failed`
- `wrong_password`
- `activate_license`
- `role: admin`

Instead they should be rendered as localized labels and values.

## Export Requirements
Every log class must support export independently.

Minimum export formats:

- `CSV`
- `JSON`

Recommended additional format:

- `PDF` summary report

Each export should support:

- time range
- module filter
- actor filter
- device filter
- severity or status filter
- keyword search

Each export package should include:

- export timestamp
- version
- filter conditions
- row count
- checksum or manifest when possible

## Retention And Evidence
Recommended default retention strategy:

- `Audit Log`: 3 years
- `System Log`: 1 year
- `Device Log`: 180 days to 1 year depending on scale
- `Config Change Log`: 3 years

Recommended evidence enhancements:

- append-only write model where feasible
- export manifest with checksum
- `recordset_id` for grouped actions
- `correlation_id` for multi-step operations
- review status for audit records

## ISO/Compliance Mapping
This is a practical mapping target, not a certification claim.

### ISO/IEC 27001 Support Goals
The system should support:

- access accountability
- privileged operation traceability
- incident evidence
- security-relevant event retention
- role-based visibility

### ISO/IEC 27701 Support Goals
The system should support:

- PII-related access accountability
- privacy-sensitive operation traceability
- exportable privacy-relevant evidence
- minimization of overexposed personal data in log presentation

### ISO 9001 Support Goals
The system should support:

- documented information
- traceable operational history
- change control evidence
- maintenance accountability
- exportable records for review

## Smart Building Alignment Goals
To better align with Taiwan smart-building evaluation expectations, the platform should be able to demonstrate:

- event monitoring capability
- historical operation records
- maintenance traceability
- integrated management visibility
- security and fault evidence
- control and operation records for building-related devices

For this product, that especially means preserving reliable records for:

- device online/offline state changes
- alarm events
- PoE control actions
- equipment restart actions
- camera configuration changes
- topology structure changes
- license enablement of building-management functions

## UI Structure
Recommended primary navigation entry:

- `日誌與稽核`

Recommended sub-tabs:

- `稽核日誌`
- `系統日誌`
- `設備日誌`
- `設定變更`

Recommended table capabilities:

- time filter
- user filter
- module filter
- device filter
- severity filter
- keyword search
- export button
- detail drawer

## Implementation Order

### Phase 1
- standardize `audit_logs`
- add `recordset_id`
- add `correlation_id`
- convert detail payloads to structured JSON

### Phase 2
- add `config_change_logs`
- persist before/after changes for device, camera, topology, license, and system settings

### Phase 3
- add `system_logs`
- add structured runtime event codes

### Phase 4
- add `device_logs`
- normalize syslog or vendor events

### Phase 5
- build unified frontend `Logs & Audit` module
- enable independent export per log class

### Phase 6
- add evidence-strengthening functions
- add review workflow
- add retention management

## Definition Of Done
This work is considered complete only when:

- all four log classes exist
- high-value actions are recorded with structured detail
- frontend can search and export each class independently
- visible text is localized through i18n
- before/after changes are available for major configuration edits
- retention behavior is documented
- export evidence is reproducible

## Current Gap Summary For new_nms_sync
As of the current state of the project:

- audit logging has started but is still partial
- some high-value actions are already wired
- license activation and PoE control are partially covered
- device reboot attempts are recorded but reboot execution is still stubbed
- system logs and device logs are not yet first-class modules
- config change logs are not yet fully implemented
- export is not yet separated by log class

This document is the target baseline for the next implementation stage.
