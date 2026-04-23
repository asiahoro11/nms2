# Topology Module Spec

## Purpose
This document defines how topology should be split into a standalone, licensable, externally integrable module inside `new_nms_sync`.

The target is not only to keep the current topology page working, but to turn topology into a reusable product capability that can be:

- used by the NMS itself
- embedded into other systems
- fed by external systems
- exported as a stable contract
- operated independently from the current topology UI implementation

This document is the source of truth for topology modularization.

It is intentionally different from `TOPOLOGY_PACKAGE_PROPOSAL.md`:

- `TOPOLOGY_MODULE_SPEC.md`
  defines the internal product architecture and module boundary
- `TOPOLOGY_PACKAGE_PROPOSAL.md`
  defines one possible later packaging/distribution strategy

## Design Goals

### Primary Goals
- separate topology data truth from topology presentation
- make topology import/export first-class
- make topology usable by other systems without requiring the whole NMS UI
- support license-gated enablement
- preserve compatibility with current monitor/admin topology use cases

### Secondary Goals
- support multi-source discovery
- support external IDs and system correlation
- support change tracking and future audit linkage
- support embeddable frontend rendering

### Non-Goals
- this spec does not redesign camera, logs, or mobile app architecture
- this spec does not require immediate replacement of all current topology UI code
- this spec does not define a public npm package or public OSS release process

## Core Principle
Topology must be split into three layers:

1. `Topology Core`
   Owns the canonical graph model
2. `Topology Adapters`
   Ingests or exports topology data to and from external systems
3. `Topology UI`
   Renders and edits topology, but does not define topology truth

This means:

- the D3 JSON used by the current UI is not the canonical contract
- layout coordinates are not the same thing as topology truth
- monitor mode and admin mode are consumers of the topology module, not the topology module itself

## Proposed Module Boundary

### Backend
Proposed target structure:

```text
apps/backend/
  modules/
    topology/
      module.go
      service.go
      api.go
      license.go
  services/
    topology/
      graph/
      adapters/
      export/
      layout/
      normalize/
  api/
    handlers/
      topology_*.go
```

### Frontend
Proposed target structure:

```text
apps/frontend/
  js/
    topology.js
    topology-api.js
    topology-embed.js
  css/
    topology.css
    topology-widget.css
```

### License Gate
Topology should remain separately gateable.

Minimum feature flags:

- `topology_view`
- `topology_edit`
- `topology_export`
- `topology_embed`
- `topology_external_sync`

## Canonical Domain Model

### Graph
The topology module should expose a canonical graph document:

```json
{
  "schema_version": "1.0",
  "graph_id": "default",
  "generated_at": "2026-04-21T00:00:00Z",
  "nodes": [],
  "ports": [],
  "links": [],
  "groups": [],
  "layouts": [],
  "status_overlays": [],
  "meta": {}
}
```

### Node
Minimum fields:

- `id`
- `external_id`
- `source_of_truth`
- `name`
- `display_name`
- `type`
- `vendor`
- `model`
- `site_id`
- `group_id`
- `ip_address`
- `mac_address`
- `status`
- `role`
- `last_seen`
- `confidence`
- `attributes`

### Port
Minimum fields:

- `id`
- `node_id`
- `external_id`
- `name`
- `index`
- `status`
- `speed`
- `poe`
- `duplex`
- `attributes`

### Link
Minimum fields:

- `id`
- `external_id`
- `source_node_id`
- `source_port_id`
- `target_node_id`
- `target_port_id`
- `link_type`
- `status`
- `speed_limit`
- `traffic_in_bps`
- `traffic_out_bps`
- `discovery_source`
- `confidence`
- `last_seen`
- `attributes`

### Group
Minimum fields:

- `id`
- `name`
- `type`
- `parent_group_id`
- `site_id`
- `attributes`

### Layout
Layout must be separate from the graph relation itself.

Minimum fields:

- `layout_id`
- `graph_id`
- `layout_type`
- `node_positions`
- `viewport`
- `updated_by`
- `updated_at`

### Status Overlay
Used for monitor mode, health state, hidden offline count, and alert emphasis.

Minimum fields:

- `target_type`
- `target_id`
- `status`
- `severity`
- `hidden_offline_count`
- `heartbeat_state`
- `annotations`

## Source Of Truth Rules

### Internal Rule
The topology module owns graph truth.

UI consumers may request:

- tree view
- force graph
- grouped graph
- monitor overlay view

But these are projections, not independent data truths.

### External Rule
Every imported node/link should preserve:

- `source_of_truth`
- `external_id`
- `external_type`
- `correlation_id`
- `confidence`

This allows mapping between:

- NMS internal device IDs
- external CMDB IDs
- BMS / BAS IDs
- DCIM IDs
- third-party NMS IDs

## Discovery And Adapter Layer

### Supported Input Modes
The module should support:

- `manual`
- `lldp`
- `snmp`
- `import_json`
- `webhook_push`
- `api_pull`
- `external_connector`

### Adapter Contract
Each adapter should normalize its output into the canonical graph model before persistence.

Adapters must not write UI-specific fields directly.

### Required Adapters
Phase-1 target:

- current manual topology edit flow
- current NMS device list as a topology source
- LLDP discovery source
- JSON import/export

Phase-2 target:

- webhook push
- API pull
- third-party connector adapters

## Storage Model

### Required Tables
Recommended storage split:

- `topology_graphs`
- `topology_nodes`
- `topology_ports`
- `topology_links`
- `topology_groups`
- `topology_layouts`
- `topology_snapshots`
- `topology_change_logs`
- `topology_external_refs`

### Important Rule
Do not overload the current UI-only layout table or current D3 payload shape as the long-term storage model.

Topology relation data and layout data must remain separate.

## API Contract

### Internal Product API
Recommended initial endpoints:

- `GET /api/v1/topology/graph`
- `GET /api/v1/topology/nodes`
- `GET /api/v1/topology/ports`
- `GET /api/v1/topology/links`
- `GET /api/v1/topology/layouts`
- `POST /api/v1/topology/layouts/:id/apply`
- `POST /api/v1/topology/import`
- `GET /api/v1/topology/export`
- `POST /api/v1/topology/discovery/run`
- `GET /api/v1/topology/change-logs`

### External Integration API
For outside systems, keep a stable, versioned contract:

- opaque IDs, not DB IDs
- RFC3339 timestamps
- additive evolution
- explicit nullability
- schema version on every response

### Export Formats
Minimum:

- `JSON`
- `CSV` for node/link inventory

Later:

- evidence bundle
- signed export manifest
- OpenAPI schema
- JSON schema

## Event Model
Topology should emit typed domain events.

Minimum event family:

- `topology.node.created`
- `topology.node.updated`
- `topology.node.deleted`
- `topology.link.created`
- `topology.link.updated`
- `topology.link.deleted`
- `topology.layout.updated`
- `topology.discovery.started`
- `topology.discovery.completed`
- `topology.discovery.failed`

These events can later feed:

- logs and audit
- webhook integrations
- message queues
- external observers

## Frontend Responsibilities

### Topology UI
The UI should only do:

- render graph
- edit layout
- edit manual links
- show overlays
- request exports

The UI should not:

- invent canonical node IDs
- infer long-term topology truth
- silently mutate graph relationships without server persistence

### Embeddable Viewer
The topology module should later support a thin embeddable viewer:

- given a graph JSON
- render read-only topology
- allow theme and layout config
- optionally show monitor overlays

This is the correct path for third-party system integration, rather than exposing the full NMS admin page.

## Relationship To Existing NMS Screens

### Monitor Mode
Monitor mode should consume:

- canonical graph
- monitor overlays
- read-only camera slot assignment

### Admin Topology
Admin topology should consume:

- canonical graph
- editable layouts
- manual link operations
- discovery controls

Both are consumers of the same module.

## Audit And Change Tracking
Topology must integrate with `v1.2.4` logs and audit design.

Topology actions that should be auditable:

- create manual link
- update manual link
- delete manual link
- move node layout
- import topology
- export topology
- discovery run
- graph merge

Configuration change records should preserve:

- actor
- source
- target
- old values
- new values
- recordset_id
- correlation_id

## Migration Strategy

### Phase 1
Stabilize the current NMS topology behavior behind a clearer module boundary.

Deliverables:

- canonical graph service
- node/link export contract
- current UI adapted to consume the new service

### Phase 2
Separate layout from graph truth.

Deliverables:

- layout persistence model
- graph snapshots
- topology change logs

### Phase 3
Add external integration capability.

Deliverables:

- import/export API
- connector adapter interface
- webhook / pull / push support

### Phase 4
Add embeddable viewer and package strategy.

Deliverables:

- topology widget entrypoint
- packaging alignment with `TOPOLOGY_PACKAGE_PROPOSAL.md`

## Recommended Delivery Order

1. Define and freeze canonical graph schema
2. Introduce backend topology core service
3. Add import/export API
4. Refactor current admin and monitor topology screens to consume the core service
5. Add external adapters
6. Add embeddable viewer

## Acceptance Criteria

This module can be considered correctly split when:

- topology data truth is independent from topology UI rendering
- the graph can be exported without relying on D3/UI payloads
- admin and monitor topology consume the same canonical graph
- manual edit, discovery, and layout are separated concerns
- external systems can import/export graph data without needing the full NMS UI
- topology remains license-gated and module-aware

## Open Questions

- should topology embed mode require a separate license from topology view?
- should external IDs be unique per source or globally unique?
- should topology snapshots be retained for audit-grade diffing by default?
- should topology package distribution remain internal-only or become a separately branded product later?
