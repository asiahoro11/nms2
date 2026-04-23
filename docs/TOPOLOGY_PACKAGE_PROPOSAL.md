# Topology Package — Standalone Distribution Proposal

**Author:** Claude
**Date:** 2026-04-20
**Status:** Draft — pending Codex review
**Target Release:** `v1.3.0` (new product line, separate from NMS core)

---

## 1. Objective

Extract the existing NMS topology capability (backend engine + frontend viewer) into a standalone, licensable product that third-party platforms can embed or integrate without adopting the full NMS stack.

The package is delivered in three forms:

- **Engine** — backend binary / library that ingests device and link data and serves topology JSON
- **Viewer** — frontend JS bundle that renders topology from a JSON source
- **Contract** — frozen, versioned API schema that both sides honor

`nms_sync` remains the first consumer of this package; the package is not a fork.

---

## 2. Product Boundary

### 2.1 What Ships

| Artifact                         | Form                  | Platforms                     |
|----------------------------------|-----------------------|-------------------------------|
| `topology-engine`                | Standalone binary     | Windows amd64, Linux amd64/arm64 |
| `topology-engine-lib`            | Go library (optional) | Go projects embedding engine  |
| `topology-viewer.min.js`         | UMD bundle            | Any browser ≥ ES2020          |
| `topology-viewer.css`            | Stylesheet            | Any browser                   |
| `topology-schema.json`           | JSON Schema           | Language-neutral contract     |
| `topology-openapi.yaml`          | OpenAPI 3.1 spec      | API clients / codegen         |

### 2.2 What Does Not Ship

- NMS device management UI
- Camera, PoE, license-gen, audit modules
- SQLite migrations or schema files unrelated to topology

### 2.3 Naming

- Product name: **`TopoKit`** (placeholder, to be finalized)
- Module path: `github.com/premiumnms/topokit` (or internal path if closed)
- NPM-like distribution: as static asset download; not published to public npm

---

## 3. Data Feed Modes

The engine supports three modes for receiving device/link data. A deployment picks one; they are mutually compatible at the schema level.

### 3.1 Pull Mode
Engine polls a consumer-provided endpoint on a configurable interval.

```yaml
feed:
  mode: pull
  url: https://customer.example.com/api/devices
  interval_seconds: 60
  auth:
    type: bearer
    token_env: CUSTOMER_TOKEN
```

### 3.2 Push Mode
Consumer posts data to the engine.

```
POST /api/v1/topology/import
Content-Type: application/json
Authorization: Bearer <api_key>

{ "schema_version": "1.0", "nodes": [...], "links": [...] }
```

### 3.3 Embed Mode
Engine is bypassed; frontend viewer receives JSON directly from host application.

```html
<script src="topology-viewer.min.js"></script>
<script>
  TopoKit.mount('#topo-root', {
    mode: 'embed',
    data: { schema_version: '1.0', nodes: [...], links: [...] }
  });
</script>
```

---

## 4. API Contract

### 4.1 Contract Principles

1. **Versioned schema.** Every response carries `schema_version`. Breaking changes bump the major version.
2. **Stable field names.** External field names differ from internal DB columns on purpose. Internal renames never leak.
3. **Additive evolution.** New fields may be added at the same major version; consumers ignore unknown fields.
4. **No internal types.** IDs are opaque strings, not DB integers. Timestamps are ISO 8601 UTC strings.
5. **Nullable is explicit.** Optional fields are `null` when absent, not omitted.

### 4.2 Base URL And Authentication

```
Base URL:  https://<host>/api/v1/topokit
Auth:      Authorization: ApiKey <api_key>
```

- `ApiKey` scheme is separate from user JWT. Keys are issued by license-gen, revocable, rate-limited, and optionally IP-scoped.
- All endpoints return `application/json; charset=utf-8`.
- All endpoints accept `Accept-Language` and echo localized labels where applicable.

### 4.3 Common Response Envelope

```json
{
  "schema_version": "1.0",
  "generated_at": "2026-04-20T08:15:30Z",
  "request_id": "req_01HW...",
  "data": { ... },
  "error": null
}
```

Errors:

```json
{
  "schema_version": "1.0",
  "generated_at": "2026-04-20T08:15:30Z",
  "request_id": "req_01HW...",
  "data": null,
  "error": {
    "code": "invalid_api_key",
    "message": "API key rejected or expired",
    "details": { "key_prefix": "tk_abc..." }
  }
}
```

### 4.4 Canonical Topology Schema

```jsonc
{
  "schema_version": "1.0",
  "generated_at": "2026-04-20T08:15:30Z",
  "site": {
    "id": "site_hq",
    "name": "HQ Main Campus",
    "timezone": "Asia/Taipei"
  },
  "nodes": [
    {
      "id": "node_001",                    // opaque string, stable across restarts
      "name": "core-sw-01",
      "display_name": "Core Switch 01",    // human-facing, localized
      "device_type": "switch",             // enum: router|switch|firewall|server|ipcam|ap|codec|pdu|ups|other
      "vendor": "EdgeCore",                // optional
      "model": "ECS4150-28P",              // optional
      "ip_address": "10.0.0.1",
      "mac_address": "aa:bb:cc:dd:ee:ff",  // optional
      "status": "online",                  // enum: online|offline|unknown|degraded
      "status_since": "2026-04-20T07:00:00Z",
      "position": { "x": 120.5, "y": 340.0 },  // optional; null = auto-layout
      "metrics": {                         // optional block; null if not collected
        "uptime_seconds": 864000,
        "cpu_percent": 12.4,
        "memory_percent": 45.1,
        "temperature_c": 38.0
      },
      "tags": ["edge", "poe-capable"],     // optional
      "image_url": null,                   // null or absolute URL
      "metadata": {}                       // vendor/site extension dict, opaque to viewer
    }
  ],
  "links": [
    {
      "id": "link_001",
      "source_node_id": "node_001",
      "target_node_id": "node_002",
      "source_interface": {                // nullable
        "id": "if_30",
        "name": "GigabitEthernet0/30",
        "index": 30
      },
      "target_interface": null,
      "link_type": "ethernet",             // enum: ethernet|fiber|wireless|ha|vrf|spine_leaf|logical|unknown
      "link_speed_bps": 1000000000,        // int64; 0 = unknown
      "bandwidth_in_bps": 23898,           // int64; 0 = no traffic data
      "bandwidth_out_bps": 5904,
      "bandwidth_usage_bps": 29802,        // convenience: in+out or total
      "label": null,                       // short human tag
      "status": "up",                      // enum: up|down|unknown
      "is_manual": false,                  // was this link user-asserted or auto-discovered
      "discovery_source": "lldp",          // enum: lldp|cdp|manual|import|inferred
      "metadata": {}
    }
  ],
  "layout_hint": {                         // optional; viewer may ignore
    "algorithm": "tree",                   // enum: tree|force|grid|manual
    "root_node_id": "node_001",
    "direction": "top-down"
  }
}
```

### 4.5 Endpoints

#### 4.5.1 `GET /api/v1/topokit/topology`
Returns the current topology snapshot.

| Query param       | Type     | Default | Notes                                   |
|-------------------|----------|---------|-----------------------------------------|
| `include_metrics` | bool     | `true`  | Set `false` to omit per-node metrics    |
| `include_offline` | bool     | `true`  | Filter out offline nodes when `false`   |
| `site_id`         | string   | —       | Multi-site deployments                  |
| `since`           | ISO 8601 | —       | Only return changes since this time (delta mode) |
| `format`          | enum     | `json`  | `json` \| `graphml` \| `cytoscape`      |

Response: Canonical Topology Schema (§4.4) wrapped in envelope.

Rate limit: 60 req/min per API key (default, configurable).

#### 4.5.2 `GET /api/v1/topokit/topology/nodes/{id}`
Single-node detail. Includes extended fields not present in the aggregate topology call (interface list, neighbor history, recent status transitions).

#### 4.5.3 `GET /api/v1/topokit/topology/links/{id}`
Single-link detail with traffic history (last N samples).

#### 4.5.4 `POST /api/v1/topokit/topology/import`
Push-mode ingestion. Replaces or merges the current snapshot.

Body:
```jsonc
{
  "schema_version": "1.0",
  "mode": "replace",           // enum: replace|merge|upsert
  "nodes": [...],
  "links": [...]
}
```

Response: `{ "accepted_nodes": N, "accepted_links": M, "rejected": [...] }`

Rate limit: 10 req/min per API key.

#### 4.5.5 `POST /api/v1/topokit/topology/discover`
Trigger engine-side LLDP/CDP discovery (only if engine is running in auto-discovery mode).

Response: `{ "job_id": "job_abc", "status": "started" }`

#### 4.5.6 `GET /api/v1/topokit/topology/stream` (SSE)
Server-Sent Events stream of topology deltas. Emits:
- `node.added`, `node.updated`, `node.removed`
- `link.added`, `link.updated`, `link.removed`
- `snapshot.refreshed`

Event payload matches the single-entity shape from §4.4.

#### 4.5.7 `GET /api/v1/topokit/health`
Liveness. Returns `{ "status": "ok", "version": "1.3.0", "license": { "valid": true, "expires_at": "..." } }`

#### 4.5.8 `GET /api/v1/topokit/schema`
Returns the JSON Schema document for the current `schema_version`. Lets consumers validate without shipping schema files.

### 4.6 Export Formats

- `json` (default) — canonical schema
- `graphml` — XML graph format, Gephi/yEd compatible
- `cytoscape` — Cytoscape.js `{ elements: { nodes: [], edges: [] } }` shape

`GET /api/v1/topokit/topology?format=graphml` returns `Content-Type: application/graphml+xml`.

### 4.7 Error Codes

| Code                      | HTTP | Meaning                                   |
|---------------------------|------|-------------------------------------------|
| `invalid_api_key`         | 401  | Missing, malformed, or revoked key        |
| `api_key_expired`         | 401  | Key past expiry                           |
| `license_invalid`         | 403  | Engine license not valid                  |
| `license_expired`         | 403  | Engine license past expiry                |
| `rate_limited`            | 429  | Too many requests; honor `Retry-After`    |
| `schema_version_mismatch` | 400  | Incoming payload schema not supported     |
| `node_not_found`          | 404  | Single-node lookup failed                 |
| `import_validation_failed`| 422  | Push payload failed schema validation     |
| `internal_error`          | 500  | Unexpected server failure                 |

---

## 5. Frontend Viewer SDK

### 5.1 Mount API

```js
const viewer = TopoKit.mount(element, {
  mode: 'pull' | 'push' | 'embed',
  apiUrl:   'https://host/api/v1/topokit',   // pull/push mode
  apiKey:   'tk_...',                         // pull mode
  data:     { ... },                          // embed mode
  theme:    'dark' | 'light' | 'auto',
  layout:   'tree' | 'force' | 'grid' | 'manual',
  locale:   'zh-TW',
  interactive: true,
  onNodeClick: (node) => { ... },
  onLinkClick: (link) => { ... },
  onError:     (err) => { ... }
});
```

### 5.2 Instance Methods

```js
viewer.refresh();                // force re-fetch
viewer.setData(json);            // embed mode: replace data
viewer.focusNode(nodeId);
viewer.exportPNG();              // returns Blob
viewer.exportSVG();              // returns string
viewer.destroy();                // cleanup, remove listeners
```

### 5.3 Events

Emitted on the returned instance via `viewer.on(event, handler)`:
- `ready`
- `data-loaded`
- `node-selected`
- `link-selected`
- `error`

### 5.4 CSS Scoping

All viewer styles are scoped under `.topokit-root` to avoid host page conflicts.

---

## 6. Licensing Integration

### 6.1 Reuse Existing `license-gen`

The existing `v1.2.1-PoC` license infrastructure is extended, not forked:

- New license feature flag: `topokit`
- New sub-flags: `topokit_api`, `topokit_viewer`, `topokit_export`
- New field: `topokit_max_nodes` (integer cap; `0` = unlimited)
- Formal licenses: Machine ID bound, as today
- PoC licenses: time-bound, unbound, as today

### 6.2 API Key Lifecycle

- API keys are separate from user JWTs. Users never authenticate against topokit endpoints.
- Generated via NMS admin UI or license-gen CLI.
- Stored in DB hashed (`argon2id` or `bcrypt`); raw value shown once at creation.
- Revocable individually; bulk-revocable per license.
- Attributes: `name`, `scopes[]`, `expires_at`, `rate_limit_override`, `allowed_ips[]`, `last_used_at`.

### 6.3 Enforcement Points

1. Engine startup: validate license signature + expiry
2. Every API request: validate API key, check scope, check rate limit, check node count cap
3. Viewer load: optional domain whitelist check (best-effort, not security boundary)

---

## 7. Code Protection Strategy

### 7.1 Backend (Go)

| Layer                  | Tool                     | Purpose                              |
|------------------------|--------------------------|--------------------------------------|
| Stripped symbols       | `-ldflags "-s -w"`       | Remove symbol/debug info             |
| Obfuscation            | `garble build`           | Rename identifiers, encrypt strings  |
| Binary packing         | UPX (optional)           | Compression + mild obfuscation       |
| Commercial protection  | VMProtect / Themida      | Windows-only, if budget allows       |
| License signature      | Ed25519 signed license   | Tamper detection                     |

### 7.2 Frontend (JS)

| Layer                  | Tool                        | Purpose                           |
|------------------------|------------------------------|-----------------------------------|
| Bundle + minify        | `esbuild` / `rollup` + terser| Baseline                          |
| Obfuscation            | `javascript-obfuscator`      | Control flow, string encryption   |
| Debugger traps         | built into obfuscator        | Slow down live reversing          |
| Domain lock            | runtime check                | Best-effort; not security         |
| Source maps            | **never distributed**        | —                                 |
| Core algorithm in WASM | Rust/Go → wasm-pack          | Real protection for valuable algo |

### 7.3 Explicit Non-Guarantees

- JS obfuscation **is not encryption**; a motivated attacker will recover behavior.
- Real protection comes from keeping **valuable server-side algorithms (LLDP parsing, topology inference, HA collapse logic) in the Go binary**. The JS viewer only renders JSON it receives.
- If the server goes away, the viewer becomes useless — that is the moat.

---

## 8. Repository Layout

```
topokit/
├── cmd/
│   ├── topokit-engine/        # standalone server binary
│   └── topokit-keygen/        # license/API key CLI
├── internal/
│   ├── discovery/             # LLDP/CDP (extracted from nms_sync)
│   ├── topology/              # inference, HA collapse
│   ├── api/                   # HTTP handlers
│   ├── license/               # reuses license-gen core
│   └── storage/               # pluggable backend: sqlite, postgres, memory
├── pkg/
│   └── topokit/               # public Go API for library consumers
├── web/
│   ├── src/                   # viewer source
│   ├── dist/                  # built artifacts
│   └── examples/              # integration samples
├── schema/
│   ├── topology-1.0.json      # JSON Schema
│   └── openapi-1.0.yaml       # OpenAPI 3.1 spec
├── docs/
│   ├── API.md                 # human-readable API reference
│   ├── INTEGRATION.md         # embed / pull / push guides
│   └── LICENSING.md
└── examples/
    ├── pull-from-custom-nms/
    ├── embed-in-dashboard/
    └── push-from-python/
```

`nms_sync` depends on `topokit` as a Go module; the NMS UI's topology view becomes a `topokit-viewer` consumer.

---

## 9. Migration Path from Current nms_sync

1. **Freeze current internal topology types** (`TopologyData`, `TopologyNode`, `TopologyLink`).
2. **Introduce `topokit` canonical schema** alongside; add a translator `internal → canonical`.
3. **Expose `/api/v1/topokit/topology`** on the NMS server as the first consumer. Old `/api/v1/topology` endpoint stays for backward compat; marks as deprecated.
4. **Extract `discovery/`, `topology/` packages** into `topokit/internal/`. `nms_sync` imports them.
5. **Build `topokit-viewer`** from the existing frontend rendering code, strip NMS-specific UI.
6. **Ship `topokit-engine` as a separate binary** once the Go split is clean.
7. **Deprecate internal topology types** in `nms_sync` a release later.

No user-visible disruption at any step if executed in order.

---

## 10. Open Questions (for Codex)

1. **Schema ownership**: should `topokit` schema be the truth, and `nms_sync` internal types derive from it, or vice versa? This proposal assumes topokit-first.
2. **Storage**: should topokit ship with SQLite by default, or be storage-agnostic from day one (require consumer to supply a `Storage` interface)?
3. **Real-time push**: SSE (proposed) vs WebSocket vs MQTT? SSE is simplest, firewall-friendly, one-way — matches the use case.
4. **Multi-site**: is multi-site (§4.5.1 `site_id`) a v1.0 feature or deferred to v1.1?
5. **Licensing granularity**: per-node, per-deployment, per-site — which billing unit?
6. **Public npm publish**: yes (wider adoption) or no (harder to scrape source)?
7. **LLDP/CDP discovery extraction**: is the current `nms_sync` discovery code clean enough to lift, or does it need a refactor pass first?

---

## 11. Explicit Non-Goals (v1.0)

- Device management (add/edit/delete devices from UI)
- PoE control
- Camera / video
- Alerting / notifications
- User management / RBAC (consumer's responsibility)
- On-prem auto-update (consumer redistributes)

---

## 12. Risks

| Risk                                          | Likelihood | Mitigation                                             |
|-----------------------------------------------|------------|--------------------------------------------------------|
| Internal/external schema drift                | High       | Contract tests; CI fails if canonical schema changes without version bump |
| Fork maintenance burden                       | Medium     | Monorepo, `nms_sync` consumes `topokit`, never copies  |
| JS obfuscation reversed                       | High       | Keep value in server; accept viewer is inspectable     |
| License bypass                                | Medium     | Ed25519 signature; server-side enforcement; revocation |
| Performance (large topologies)                | Medium     | Delta mode (`since` param), SSE stream, pagination     |
| Consumer locks to `v1.0` forever              | Low        | Major-version support policy published up front       |

---

## 13. Proposed Milestones

| Milestone        | Deliverable                                      | Gate                                    |
|------------------|--------------------------------------------------|-----------------------------------------|
| M0 — Extraction  | `topokit` repo layout, `pkg/topokit` Go API      | `nms_sync` builds against it            |
| M1 — Contract    | Frozen `topology-1.0.json` schema, OpenAPI spec  | Schema review signed off                |
| M2 — Engine      | `topokit-engine` binary, pull + embed modes      | All §4.5 endpoints pass contract tests  |
| M3 — Viewer      | `topokit-viewer.min.js` distributable            | Renders `topology-1.0` sample correctly |
| M4 — Licensing   | API keys + license-gen integration               | License-gen emits topokit keys          |
| M5 — Protection  | Garble build + JS obfuscation in release pipe    | Binary + JS pass obfuscation check      |
| M6 — GA          | Docs, integration examples, release artifacts    | Third-party pilot integration succeeds  |

---

## 14. Decision Requested

- **Greenlight `topokit` as a v1.3.0 product line?**
- **Confirm schema-first approach (§9.1)?**
- **Confirm license-gen reuse strategy (§6.1)?**
- **Pick one of the open questions in §10 to unblock first?**

---

_End of proposal. Pending Codex review._
