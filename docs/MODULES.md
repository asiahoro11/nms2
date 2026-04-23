# Backend Module Architecture

This document defines the backend module direction for `new_nms_sync`.

The goal is not to rewrite the system around a new router first. The goal is:

- keep the existing external behavior stable
- move feature logic into `apps/backend/modules/*`
- make each feature importable by other systems such as `nms_sync2`
- preserve license gates and future export/sync boundaries

## Current Rule

Until a module is fully extracted:

- `api/handlers/*` may remain as the HTTP wrapper
- module code becomes the internal business/domain layer
- external routes and payloads should remain unchanged unless intentionally versioned

This is the same pattern already applied to `topology`.

## Current Status

### Extracted

- `topology`
  - lives under `apps/backend/modules/topology`
  - uses canonical graph as internal truth
  - preserves existing `/topology` payload through legacy projection
  - now owns internal topology mutation persistence:
    - `topology_change_logs`
    - `topology_layout_snapshots`
- `license`
  - lives under `apps/backend/modules/license`
  - owns status queries, active entitlement calculations, feature flags, activation, generation, reissue, admin inventory projection, and device-management gating
- `backup`
  - lives under `apps/backend/modules/backup`
  - owns system backup, encrypted backup, and device config backup workflows behind thin handlers
- `admin`
  - lives under `apps/backend/modules/admin`
  - owns user management, branding, security settings, system config orchestration, and system info projection
- `auth`
  - lives under `apps/backend/modules/auth`
  - owns login, current-user lookup, password change, password reset, JWT issuance, and protected two-factor management routes
  - includes TOTP enrollment, confirmation, disable, and recovery-code regeneration, with email OTP reserved as a future fallback
- `devices`
  - lives under `apps/backend/modules/devices`
  - owns inventory CRUD, interface queries, image upload persistence, metrics read path, and bulk mutation support
  - thin handlers still own audit, event insertion, and alert dispatch side effects around the extracted device service
- `camera`
  - lives under `apps/backend/modules/camera`
  - owns schema migration, license-backed inventory CRUD, camera count checks, monitor-display projection, recording metadata/config, batch credential updates, discovery, health probing, ffmpeg helper utilities, RTSP auth helpers, ONVIF probe helpers, snapshot execution, MJPEG runtime, and recording session lifecycle
  - handlers retain temporary legacy compatibility helpers, but active routes now delegate runtime ownership to the module
- `logs`
  - lives under `apps/backend/modules/logs`
  - owns the latest consolidated log-center read path:
    - `audit_logs`
    - `system_logs`
    - `device_logs`
    - `config_change_logs`
  - owns review, acknowledge, export, and evidence-bundle data access
  - legacy `syslogs` and `events` compatibility routes remain available in handlers
- `notifications`
  - lives under `apps/backend/modules/notifications`
  - owns alert channel settings, test dispatch, in-app notification inbox read/ack/delete, and shared email transport config lookup
- `access_control`
  - lives under `apps/backend/modules/accesscontrol`
  - owns door, card, event, and schedule CRUD plus approval and runtime access checks
  - thin handlers still perform HTTP binding and license freshness checks
- `pdu`
  - lives under `apps/backend/modules/pdu`
  - owns PDU/UPS inventory CRUD, module status, and on-demand SNMP polling
- `tools`
  - lives under `apps/backend/modules/tools`
  - owns ping and traceroute validation plus command execution helpers
- `reports`
  - lives under `apps/backend/modules/reports`
  - owns device, log, traffic, health, availability, and inventory export rendering
- `dashboard`
  - lives under `apps/backend/modules/dashboard`
  - owns the main summary projection plus top CPU and top memory widget queries
- `webssh`
  - lives under `apps/backend/modules/webssh`
  - owns the SSH-over-WebSocket terminal relay lifecycle for device-backed sessions
## Target Layout

```text
apps/backend/modules/
  contracts/
  catalog/
  auth/
  dashboard/
  devices/
  topology/
  camera/
  accesscontrol/
  pdu/
  logs/
  license/
  backup/
  notifications/
  reports/
  tools/
  webssh/
  admin/
```

## Module Contract

Every module should eventually own:

- domain types
- business services
- internal persistence helpers
- import/export adapters when relevant
- manifest metadata for downstream integration
- shell-mount metadata so `nms_sync2` can render an outer frame and import modules by manifest

Every module should avoid owning:

- unrelated HTTP glue
- unrelated view formatting
- cross-feature dumping-ground helpers

## Integration Direction

Future reuse in `nms_sync2` or other systems should happen by importing the module package, not by scraping UI-specific payloads.

That means each mature module should move toward:

- a canonical internal model
- stable module manifest metadata
- shell mount metadata:
  - route
  - nav label / i18n key
  - frontend entry
  - backend entry
  - feature gate
- optional outbound adapters:
  - JSON
  - sync API
  - webhook/event stream

## Recommended Extraction Order

1. `devices`
2. `camera`
3. `auth`

This order keeps shared infrastructure moving first:

- inventory
- video
- logs/audit
- entitlement

## Migration Pattern

For each feature:

1. create `apps/backend/modules/<feature>`
2. move domain/service logic there
3. keep `api/handlers/<feature>.go` as thin HTTP wrapper
4. keep router unchanged
5. validate with `go test ./...`

## Canonical Rule For Future Work

When splitting new functionality:

- prefer module-first internal structure
- keep existing outward behavior stable
- defer new integration APIs until the internal module boundary is clean
