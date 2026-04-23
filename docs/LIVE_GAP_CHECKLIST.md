# Live Gap Checklist

This document tracks the current gap between the live site at `http://10.100.100.11:8080`
and the intended `new_nms_sync` cutover target.

## Scope

- Source of truth: `apps/frontend`
- Runtime target: replace the mixed live runtime with a single `new_nms_sync` release
- Priority: restore product usability first, then finish product polish

## Current Live Findings

### 1. Version Drift

- The live site exposes mixed versions across pages:
  - login page shows `v1.2.1`
  - mode-selection shows `v1.2.0`
  - dashboard/header shows `v1.2.2alpha2`
- This indicates the live runtime is not serving one clean, consistent artifact.

### 2. Global Translation Regression

- Large parts of the live UI show raw i18n keys instead of human-readable text.
- Examples observed on the live site:
  - `login.forgot_password`
  - `common.theme`
  - `nav.cameras`
  - `nav.access_control`
  - `nav.pdu`
  - `dashboard.reload_page`
  - `dashboard.system_info`
  - `dashboard.online_devices`
  - `dashboard.events.title`
  - `admin.tabs.*`
  - `ac.*`
  - `pdu.*`
  - `cameras.*`
- Required direction:
  - restore all visible UI strings back to real translated text
  - do not ship pages that still expose fallback keys
  - verify language switching per page, not just locale JSON presence

### 3. Branding Drift

- The product naming is inconsistent across the live site:
  - `NMS-Lite`
  - `NMS-Sync`
  - `Management System`
- Required direction:
  - choose one final product label
  - apply it consistently to login, mode-selection, dashboard, monitor, admin, release notes, and launcher text

### 4. Event Text / Formatting Errors

- Monitor/event stream currently shows broken formatted strings such as:
  - `%!(EXTRA string=10.100.100.38)`
- Some event messages also show broken Chinese output.
- Required direction:
  - audit backend event string formatting
  - align placeholder count and argument count
  - verify monitor event rendering under all languages

### 5. Topology / Monitor Regression

- Topology is still visible, but the overall experience is not yet back to a stable product state.
- Required direction:
  - restore human-readable labels in topology controls and legends
  - re-validate force/tree mode switching
  - re-validate collapse/expand behavior
  - re-validate abnormal/offline branch indication in monitor mode
  - re-validate node/link labels, status colors, and empty states
  - verify monitor layout with topology + camera split view as a product surface, not only as a technical demo

### 6. Module Surface Incompleteness

- `Camera` page:
  - module opens
  - many labels and hints still show raw keys
  - live/NVR controls need final text validation
- `Access Control` page:
  - mostly skeleton state
  - tabs and actions still expose raw keys
  - empty state needs product-quality guidance text
- `PDU` page:
  - mostly empty state
  - raw key text still visible
  - license prompt and empty-state messaging must be finalized
- `Admin` page:
  - content exists
  - tabs and many labels still expose raw keys
  - backup, reports, licenses, alerts, and tools need language restoration and smoke validation

### 7. Deployment / Static Drift

- Live behavior strongly suggests asset drift between:
  - current source
  - embedded backend static
  - deployed runtime
- Required direction:
  - keep `apps/frontend` as source of truth
  - sync both backend static mirrors before building
  - replace the live runtime with one clean artifact instead of patching mixed old assets in place

## Repair Workstreams

### Workstream A: Live Runtime Unification

- Remove version drift.
- Deploy one release only.
- Confirm login, mode-selection, dashboard, monitor, and admin all report the same version.

### Workstream B: Translation Restoration

- Restore all currently exposed i18n keys to real text.
- Validate:
  - `zh-TW`
  - `en-US`
  - `zh-CN`
  - `ja-JP`
  - `ko-KR`
- Required rule:
  - if a translation is missing, fallback may be used internally
  - but raw key names must not remain visible in the UI

### Workstream C: Product Naming and Branding Cleanup

- Finalize one product name.
- Remove old mixed labels from:
  - login
  - mode-selection
  - dashboard
  - monitor
  - admin
  - launcher messages
  - release metadata

### Workstream D: Topology and Monitor Recovery

- Restore readable UI controls and legends.
- Re-verify node grouping and tree/force behavior.
- Re-verify monitor topology integration.
- Re-verify camera wall + topology split layout.
- Re-verify abnormal/offline indicators and branch warning behavior.

### Workstream E: Module-by-Module Product Recovery

- Dashboard
- Devices
- Topology
- Cameras
- Access Control
- PDU
- Logs
- Admin

Each module needs:

- restored translated text
- stable empty states
- license-aware presentation
- smoke-tested navigation

## Recommended Execution Order

1. Unify live runtime and version output.
2. Restore global translations and remove visible raw keys.
3. Fix event message formatting errors.
4. Recover topology and monitor behavior.
5. Recover camera/access/pdu/admin module surfaces.
6. Run cross-language smoke validation.
7. Cut over to one clean `new_nms_sync` release.

## Definition of Done

- Live site reports one version only.
- No visible raw i18n keys remain on major pages.
- No `%!(EXTRA ...)` or broken event message formatting remains.
- Topology controls, legends, and states are readable and stable.
- Monitor mode works as an operational page, not a half-broken demo.
- Camera, Access Control, PDU, and Admin all show product-quality labels and empty states.
- One clean release artifact fully replaces the mixed live deployment.
