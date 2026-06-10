# Plugin Module Definition

This document defines the target plugin-module architecture for `new_nms_sync`.
It is the product and engineering contract for splitting the system into
license-gated, importable capabilities while preserving the current deployed
behavior.

This is not a rewrite plan. It is the rulebook for making every future split
predictable, testable, and commercially controllable.

## Positioning

`new_nms_sync` should become a modular NMS platform:

- the platform core owns identity, runtime, data access, release flow, and the
  common shell
- product modules own business capabilities such as devices, topology, camera,
  PDU, reports, access control, and WebSSH
- integration adapters own outbound or inbound contracts such as JSON export,
  sync API, webhooks, or embedding surfaces
- license entitlement decides which modules are enabled, visible, limited, or
  blocked

The user-facing product can call these "skills" if that language is useful, but
the engineering name is `plugin module`.

## Goals

- Keep current routes, payloads, and UI behavior stable unless a change is
  explicitly versioned.
- Make each capability importable from `apps/backend/modules/<module>`.
- Keep `api/handlers/*` thin and route-compatible.
- Move business logic, canonical models, validation, and persistence ownership
  into modules.
- Make every commercial capability visible through a manifest and license gate.
- Let frontend navigation and module cards render from module state instead of
  hardcoded assumptions.
- Keep canonical internal models separate from UI payloads and external export
  payloads.
- Make module status observable for operators and support staff.
- Preserve the release workflow: source edits, static sync when needed, tests,
  then release artifacts.

## Non-Goals

- Do not split into microservices first.
- Do not make one repository per module first.
- Do not break existing API routes only to make the internals cleaner.
- Do not move generated static mirrors by hand.
- Do not let feature modules own shared platform policy such as authentication,
  global database bootstrap, release packaging, or license validation.
- Do not make the UI payload the canonical model for a module.

## Terms

### Platform Core

The platform core is always present. It owns the process, router bootstrap,
database handle, authentication middleware, license runtime, settings, static
embedding, logs needed for operation, and release workflow.

Core code may expose modules, but it is not sold as an optional add-on.

### Core Foundation Module

A core foundation module is implemented with the same module contract, but it is
required by the platform. Examples:

- `auth`
- `license`
- `admin`
- `dashboard`
- `logs`
- `notifications`

These can be importable and manifest-registered, but they should not disappear
from a running system just because an add-on license is missing.

### Product Module

A product module is a sellable or separately controllable capability. Examples:

- `devices`
- `topology`
- `camera`
- `access_control`
- `pdu`
- `reports`
- `backup`
- `tools`
- `webssh`

Product modules must be manifest-registered and license-aware.

### Integration Adapter

An integration adapter translates canonical module data into an external or
cross-system contract. Examples:

- `export_json`
- `api_sync`
- `webhook_events`
- `embedded_widget`
- `external_inventory_import`

Adapters must not replace the module's canonical internal model.

### Shell

The shell is the common frontend/backend frame that decides:

- which modules are available
- which modules are enabled
- which navigation items and panels are visible
- which modules show disabled, expired, limited, or setup-required states
- which module entry files should be loaded

The shell consumes module manifests and entitlement state. It should not encode
commercial rules directly in page-specific JavaScript.

## Module Classes

| Class | Meaning | Examples | License Behavior |
| --- | --- | --- | --- |
| `platform_core` | Process and shared runtime foundation | router, DB bootstrap, static embed, release flow | Always present |
| `core_foundation` | Required platform feature implemented as a module | auth, license, admin, dashboard, logs, notifications | Always visible to allowed admins; may expose sub-gates |
| `product_module` | Sellable capability | devices, topology, camera, PDU, access control, reports, WebSSH | Controlled by feature gate, capacity, and dependencies |
| `integration_adapter` | Import/export/sync/embed surface | JSON export, API sync, webhooks, embedded topology | Controlled by parent module and adapter gate |
| `runtime_extension` | Runtime helper or engine bound to a module | ffmpeg, go2rtc, SNMP poller, SSH relay | Enabled only when parent module and dependency checks pass |

## Extraction Levels

Every module should move through these levels. The level is more precise than
the current `ExtractionStatus`.

| Level | Name | Definition |
| --- | --- | --- |
| L0 | Folder Split | Code has a module folder, but handlers still own most behavior |
| L1 | Service Owned | Business logic and validation live in the module service |
| L2 | Manifest Registered | Module returns a manifest and appears in the catalog |
| L3 | License Gated | Module status is decided by entitlement and dependency checks |
| L4 | Shell Importable | Shell can render navigation, status, and entrypoints from manifest data |
| L5 | Adapter Ready | Module has canonical import/export or sync adapters where relevant |
| L6 | Package Ready | Module can be reused by another shell without scraping UI payloads |

`StatusExtracted` should mean at least L2. A mature commercial module should be
L4 or higher.

## Current Module Inventory

| Module | Class | Current Role | Target Level |
| --- | --- | --- | --- |
| `auth` | core_foundation | login, current user, password, JWT, two-factor flow | L4 |
| `license` | core_foundation | entitlement, activation, feature gates, device limits | L5 |
| `admin` | core_foundation | users, branding, security settings, system config | L4 |
| `dashboard` | core_foundation | summary projections and top widgets | L4 |
| `logs` | core_foundation | audit, system, device, config-change log center | L5 |
| `notifications` | core_foundation | channels, tests, inbox, email config lookup | L4 |
| `devices` | product_module | inventory, interfaces, metrics, bulk mutation | L5 |
| `topology` | product_module | canonical graph, layout, change tracking, legacy projection | L5 |
| `camera` | product_module | inventory, discovery, health, snapshots, MJPEG, recording | L5 |
| `access_control` | product_module | doors, cards, events, schedules, approvals, access checks | L4 |
| `pdu` | product_module | PDU/UPS inventory, status, SNMP polling | L4 |
| `reports` | product_module | device, log, traffic, health, availability, inventory exports | L5 |
| `backup` | product_module | system, encrypted, and device config backup workflows | L4 |
| `tools` | product_module | ping, traceroute, command execution helpers | L4 |
| `webssh` | product_module | SSH-over-WebSocket device terminal relay | L4 |
| `export_json` | integration_adapter | outbound canonical JSON export | L3 target |
| `api_sync` | integration_adapter | external sync API adapter | L3 target |

## Platform Core Boundary

The platform core owns:

- process startup and shutdown
- global configuration load
- database open, bootstrap, and shared transaction helper
- router bootstrap
- authentication middleware
- license runtime and entitlement evaluation entrypoint
- service-worker and static embedding behavior
- common audit/event envelope
- release scripts and artifact layout
- frontend shell and module loader
- generated static mirror sync

The platform core must not own:

- camera discovery rules
- topology graph truth
- report rendering logic
- PDU polling semantics
- access-control approval semantics
- WebSSH relay lifecycle
- product-specific export payloads

## Module Ownership Boundary

A module owns:

- domain types
- canonical internal model
- business rules
- validation
- persistence helpers for its tables
- schema migration steps for its tables
- service entrypoints
- runtime dependency checks
- module manifest
- status projection
- import/export adapters when relevant
- tests for its own business behavior

A module may expose:

- service methods for handlers
- manifest metadata for the catalog
- status checks for the shell
- adapter interfaces for export/sync
- event names for audit and monitoring

A module must not own:

- global router setup
- authentication middleware
- unrelated HTTP response formatting
- unrelated frontend layout
- unrelated database tables
- hardcoded commercial policy outside its own declared feature gates
- generated static mirrors

## Handler Boundary

`api/handlers/*` should remain as compatibility wrappers while extraction is in
progress.

Handlers are allowed to:

- bind HTTP input
- call auth and license middleware helpers
- call module services
- preserve existing route names and response shapes
- attach audit/event side effects during transition
- translate service errors into existing HTTP responses

Handlers should not:

- implement product business logic
- define canonical module models
- run product-specific migrations
- reach into another module's private tables when a service exists
- build export/sync payloads directly

## Manifest Contract

The current code already has:

```go
type ModuleManifest struct {
    ID                  string
    DisplayName         string
    Domain              string
    OwnerPackage        string
    ExtractionStatus    ExtractionStatus
    FeatureGate         string
    ImportableIntoShell bool
    ShellMount          *ShellMount
    DependsOn           []string
    IntegrationSurfaces []IntegrationSurface
    Notes               []string
}
```

This remains the minimum manifest contract.

### Required Fields

Every manifest must define:

- `ID`: stable machine id, lower snake case
- `DisplayName`: English fallback display name
- `Domain`: business domain, such as `video`, `graph`, `entitlement`
- `OwnerPackage`: Go import path
- `ExtractionStatus`: current extraction state
- `FeatureGate`: entitlement key, when gated
- `DependsOn`: module ids that must be ready first
- `IntegrationSurfaces`: owned service, event, adapter, or UI surfaces

### ShellMount Fields

`ShellMount` is required when `ImportableIntoShell` is true.

- `Route`: frontend route or stable page anchor
- `NavLabel`: fallback label only
- `NavI18nKey`: preferred UI label key
- `FrontendEntry`: source-of-truth frontend entry under `apps/frontend`
- `BackendEntry`: backend module package

Because this repository has had text encoding issues, `NavI18nKey` must be the
primary UI label reference. `NavLabel` is only a fallback and must not be the
source of truth for localized UI.

### Target Manifest Expansion

Future contract expansion should add these fields after the current manifest
catalog is stable:

```go
type ModuleClass string

const (
    ClassCoreFoundation    ModuleClass = "core_foundation"
    ClassProductModule     ModuleClass = "product_module"
    ClassIntegrationAdapter ModuleClass = "integration_adapter"
)

type ModuleManifest struct {
    ID                  string
    DisplayName         string
    Domain              string
    Class               ModuleClass
    OwnerPackage        string
    ExtractionStatus    ExtractionStatus
    ContractLevel       string
    FeatureGate         string
    RequiredEntitlements []string
    CapacityKeys        []string
    ImportableIntoShell bool
    ShellMount          *ShellMount
    DependsOn           []string
    DataScopes          []DataScope
    ApiSurfaces         []ApiSurface
    EventSurfaces       []EventSurface
    IntegrationSurfaces []IntegrationSurface
    RuntimeDependencies []RuntimeDependency
    Compatibility       CompatibilityPolicy
    Notes               []string
}
```

Do not add all fields at once unless the implementation actually consumes them.
The first useful additions are `Class`, `ContractLevel`, `RequiredEntitlements`,
`CapacityKeys`, and `RuntimeDependencies`.

## Module Status Contract

The shell should receive a normalized status for each module.

Recommended states:

| State | Meaning |
| --- | --- |
| `available` | Module exists and can be enabled by license/config |
| `enabled` | Module is licensed, dependencies are ready, and routes/UI can be used |
| `disabled` | Module exists but is disabled by configuration or license |
| `unlicensed` | No valid entitlement for the module |
| `expired` | Entitlement existed but is expired |
| `over_limit` | Entitlement exists but capacity has been exceeded |
| `dependency_missing` | Required module or runtime dependency is unavailable |
| `setup_required` | Licensed, but required configuration is incomplete |
| `degraded` | Running with a missing optional runtime dependency |
| `hidden` | Module should not appear for this user or license tier |

Status response shape:

```json
{
  "id": "camera",
  "class": "product_module",
  "display_name": "Camera",
  "status": "enabled",
  "feature_gate": "camera_viewer_enabled",
  "limits": {
    "licensed": 16,
    "used": 8
  },
  "dependencies": [
    {"id": "license", "status": "enabled"},
    {"id": "logs", "status": "enabled"}
  ],
  "runtime": [
    {"name": "ffmpeg", "status": "available"},
    {"name": "go2rtc", "status": "optional_missing"}
  ],
  "shell_mount": {
    "route": "/cameras",
    "nav_i18n_key": "nav.cameras",
    "frontend_entry": "apps/frontend/js/camera-module.js"
  }
}
```

## License Rules

License gating is a platform decision using module-declared inputs.

Rules:

- Each product module must declare a `FeatureGate`.
- Each capacity-limited module must declare a capacity key.
- Missing entitlement means disabled or hidden, not implicitly free.
- Capacity defaults to zero unless license or configuration explicitly grants
  capacity.
- A module should fail closed for mutating operations and capacity expansion.
- Read-only visibility may be allowed only when the product decision explicitly
  defines it.
- UI state must follow backend module status, not duplicate license logic.
- Handlers may keep compatibility checks during transition, but the final source
  of truth should be module status plus license runtime.

Recommended capability checks:

| Check | Example |
| --- | --- |
| feature enabled | `camera_viewer_enabled` |
| capacity limit | `max_camera_channels`, `max_devices` |
| operation scope | `topology_edit`, `reports_export`, `webssh_connect` |
| adapter scope | `topology_export`, `api_sync_push` |
| runtime dependency | `ffmpeg`, `go2rtc`, `snmp`, `ssh` |

## Dependency Rules

Module dependencies must be explicit.

Rules:

- A module can depend on a core foundation module.
- A product module can depend on another product module only through a public
  service interface or adapter.
- Circular dependencies are not allowed.
- A module must not import `api/handlers`.
- A module should not import another module's private table helpers directly.
- Shared contracts belong under `apps/backend/modules/contracts`.
- Shared catalog assembly belongs under `apps/backend/modules/catalog`.

Examples:

- `topology` depends on `devices`
- `camera` depends on `license` and `logs`
- `reports` can depend on `devices`, `logs`, `camera`, and `topology` through
  read-only service interfaces
- `export_json` depends on canonical adapters, not UI payloads

## Backend Directory Rules

Target backend layout:

```text
apps/backend/
  api/
    handlers/
      camera.go
      reports.go
      topology.go
  modules/
    contracts/
      manifest.go
      status.go
      events.go
    catalog/
      catalog.go
    camera/
      manifest.go
      service.go
      types.go
      migrations.go
      adapters.go
      runtime_service.go
      service_test.go
    topology/
      manifest.go
      service.go
      canonical.go
      adapters.go
      layout_service.go
      service_test.go
```

Minimum files for a mature module:

- `manifest.go`
- `types.go`
- `service.go`
- `migrations.go`, when the module owns tables
- `adapters.go`, when import/export/sync exists
- `runtime_service.go`, when external processes or protocols are involved
- `service_test.go`

Do not force empty files into small modules. The rule is ownership clarity, not
file count.

## Frontend Directory Rules

The editable frontend source of truth stays under `apps/frontend`.

Recommended layout:

```text
apps/frontend/
  js/
    app.js
    module-shell.js
    module-registry.js
    camera-module.js
    topology.js
    reports.js
  css/
    app.css
    module-shell.css
    camera.css
    topology.css
```

Frontend rules:

- The shell asks the backend for module status.
- Navigation uses `NavI18nKey` first.
- Disabled modules show a clear disabled or upgrade state.
- Hidden modules do not render navigation.
- A module entry file owns module-specific interactions.
- Shared UI patterns belong in shell utilities, not copied per module.
- Frontend changes must be synced into backend static mirrors before packaging.

Generated targets remain:

- `apps/backend/static`
- `apps/backend/cmd/agent/static`

Do not hand-edit generated targets.

## Canonical Model Rules

Each module should define one canonical internal model.

Rules:

- Canonical model is not the current UI JSON unless explicitly designed that
  way.
- Legacy UI payloads are projections from the canonical model.
- Export payloads are adapters from the canonical model.
- Sync payloads are adapters from the canonical model.
- Database rows are persistence details, not necessarily the canonical API.

Examples:

- `topology` canonical truth is the graph model, not D3 layout JSON.
- `camera` canonical truth is camera inventory plus stream/runtime state, not
  one monitor page payload.
- `reports` canonical truth is report request/result metadata plus renderer
  outputs, not a specific CSV or HTML table.

## Data Ownership Rules

Each table should have one owning module.

Rules:

- Table names should make ownership clear.
- Migrations must be idempotent.
- Modules must not mutate another module's table without a service interface or
  a clearly documented shared contract.
- Cross-module read models should be explicit.
- Audit and event records can reference resources from any module through a
  normalized envelope.
- Runtime files must stay under runtime data paths, not source directories.

Recommended table ownership examples:

| Table or Data | Owner |
| --- | --- |
| license inventory and activation state | `license` |
| device inventory and interfaces | `devices` |
| topology graph, links, layouts, change logs | `topology` |
| camera inventory, recordings, stream metadata | `camera` |
| audit/system/device/config logs | `logs` |
| report history and export metadata | `reports` |
| PDU/UPS inventory and poll state | `pdu` |

## Event And Audit Contract

Every significant module action should be expressible through a common event
envelope.

Recommended fields:

```json
{
  "event_id": "uuid",
  "module_id": "camera",
  "resource_type": "camera",
  "resource_id": "cam-001",
  "action": "recording.started",
  "severity": "info",
  "actor_id": "user-001",
  "correlation_id": "request-001",
  "occurred_at": "2026-04-30T00:00:00Z",
  "summary": "Recording started",
  "details": {}
}
```

Rules:

- Modules emit domain events.
- The platform or `logs` module persists and projects event records.
- Event names use `module.resource.action` or `resource.action` consistently.
- Events must not expose secrets.
- Export/sync adapters can subscribe to the event contract later.

## Integration Adapter Rules

Adapters are first-class, but they are not the module core.

Adapter types:

- `json_export`
- `sync_api`
- `webhook`
- `external_import`
- `embedded_widget`
- `report_renderer`

Rules:

- Adapter code consumes canonical module data.
- Adapter code must declare its parent module.
- Adapter gates can be separate from parent feature gates.
- Adapter payloads must have schema versions.
- Adapter failures must not corrupt canonical state.
- Adapters must be testable without launching the full UI.

## API Versioning Rules

Existing API behavior should remain stable while internals move.

Rules:

- Keep existing routes unless intentionally versioned.
- New integration contracts should use explicit versions.
- Legacy payloads should be named as projections in code and docs.
- Public or partner-facing payloads require schema version fields.
- Breaking changes require a compatibility plan.

Recommended route groups:

```text
/api/v1/modules
/api/v1/modules/:id/status
/api/v1/modules/:id/capabilities
/api/v1/modules/:id/export
/api/v1/modules/:id/sync
```

Do not expose these until the backend has a stable status provider.

## Runtime Dependency Rules

Some modules rely on external tools or protocols.

Examples:

- `camera`: ffmpeg, go2rtc, ONVIF, RTSP, MJPEG runtime
- `pdu`: SNMP polling
- `webssh`: SSH relay and WebSocket lifecycle
- `tools`: ping and traceroute executables

Rules:

- Runtime dependency checks belong to the owning module.
- The shell should display dependency-missing or degraded state.
- Optional dependencies should not block unrelated modules.
- Dependency probing should be cached or rate-limited.
- Runtime tool paths must be resolved from supported runtime locations.

## Security Rules

Module boundaries must not weaken security.

Rules:

- Authentication stays platform-owned.
- Authorization decisions use platform identity plus module capability checks.
- Modules must not parse or trust user identity from raw request data.
- Secrets must stay encrypted or in supported config stores.
- Export and sync adapters must redact secrets by default.
- Runtime helpers must validate command arguments.
- WebSocket modules must enforce the same user/session policy as HTTP routes.

## UI State Rules

The frontend shell should render modules from backend state.

Recommended UI states:

- normal navigation item for `enabled`
- muted disabled item for `disabled`, `unlicensed`, or `expired` when product
  wants upsell visibility
- no navigation item for `hidden`
- setup badge for `setup_required`
- warning badge for `degraded`
- limit badge for `over_limit`

Rules:

- UI must not look like logout or exit unless it actually exits.
- Module labels should use i18n keys.
- Module cards should not duplicate backend license calculations.
- Operator workflows should remain dense, professional, and scan-friendly.
- RWD is required for new shell/module surfaces.

## Acceptance Criteria For A Mature Module

A module is mature when:

- it has a manifest in `apps/backend/modules/<module>/manifest.go`
- it appears in `modules/catalog`
- it has a clear class and feature gate
- it owns service logic outside handlers
- handlers are thin wrappers
- it has canonical types or documents its current legacy projection
- it owns its migrations or documents no database ownership
- license/capacity checks fail closed
- dependency checks are visible in status
- frontend entry and i18n key are declared when shell-importable
- tests cover core service behavior
- existing API routes remain compatible
- docs identify the module boundary and adapters

## Definition Of Done For Extraction Work

For each extraction task:

1. Identify current handlers, services, tables, and frontend entrypoints.
2. Add or update module manifest.
3. Move business logic into module service.
4. Keep external route behavior stable.
5. Move or document migration ownership.
6. Add or update module tests.
7. Update docs when the boundary changes.
8. Run `gofmt` on changed Go files.
9. Run `go test ./...` from `apps/backend`.
10. If frontend changed, run `scripts/sync-static.ps1` or
    `scripts/sync-static.sh`.
11. If release-facing, run the supported release build.

## Recommended Implementation Sequence

### Phase 0: Freeze The Contract

- Add this definition document.
- Keep `docs/MODULES.md` as current implementation status.
- Treat `modules/contracts` and `modules/catalog` as the canonical code
  entrypoints.

### Phase 1: Normalize Manifests

- Add `Class` and `ContractLevel` to the manifest contract.
- Normalize all feature gates.
- Replace localized fallback labels with stable i18n keys as primary labels.
- Mark core foundation modules distinctly from product modules.

### Phase 2: Build Module Status

- Add a shared module status contract.
- Add backend status projection from manifest plus license runtime.
- Expose a stable status endpoint for the frontend shell.
- Keep old UI behavior during this phase.

### Phase 3: Shell-Driven Frontend

- Add frontend module registry utilities under `apps/frontend`.
- Render navigation/module cards from backend status.
- Show disabled, hidden, expired, over-limit, setup-required, and degraded
  states.
- Sync static mirrors after frontend updates.

### Phase 4: Adapter-Ready Modules

- Start with `topology`, `camera`, `devices`, `reports`.
- Define canonical export adapters.
- Keep legacy UI projections intact.
- Add schema versions to external payloads.

### Phase 5: Package-Ready Modules

- Make selected modules importable by another shell.
- Stabilize service interfaces.
- Separate optional runtime dependencies.
- Add compatibility tests for imports and adapters.

## First Modules To Harden

Priority order:

1. `license`: must be the consistent entitlement source.
2. `devices`: many modules depend on inventory truth.
3. `topology`: already has a strong canonical-model direction.
4. `camera`: high product value and runtime dependency complexity.
5. `reports`: natural proof that canonical data can feed adapters.
6. `pdu` and `access_control`: good commercial add-on candidates.
7. `webssh`: security-sensitive runtime extension.

## Practical Rule

Do not split for aesthetics. Split when the module boundary gives at least one
of these concrete benefits:

- license control
- importability by another shell
- independent testing
- stable export/sync contract
- clearer runtime dependency ownership
- less handler complexity
- safer release behavior

If a change does not improve one of those outcomes, keep it in the existing
boundary.
