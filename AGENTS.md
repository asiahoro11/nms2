# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## What This Project Is

A Network Management System (NMS) — a multi-protocol device monitoring and management platform. Monitors network devices (routers, switches, PDUs, UPS, cameras, IoT, access control) via SNMP, MQTT, Modbus, SSH, RTSP. Provides dashboards, alerts, topology visualization, reporting, audit logging, and optional cloud integration via EdgeCore/MQTT.

**Stack:** Go 1.25 backend (Gin, SQLite) + embedded vanilla JS/HTML frontend + Flutter mobile companion app.

---

## Commands

### Running Locally

```bash
go run ./apps/backend
# Starts server at http://localhost:8080
# Requires runtime/data/ directory to exist (created automatically on first run)
```

### Tests

```bash
go test ./...                          # All backend tests
go test ./apps/backend/modules/...    # Specific module tests
go test ./apps/backend/api/...        # Handler tests
```

### Build (Release)

```bash
# Linux/Mac
bash ./scripts/build/build_release.sh          # Both Win + Linux targets

# Windows
powershell -ExecutionPolicy Bypass -File .\scripts\build\build_both_releases.ps1

# Optional env vars:
# NMS_ENABLE_BINARY_OBFUSCATION=1  → obfuscate Go binary for protected core code paths (requires garble)
# NMS_ENABLE_JS_OBFUSCATION=1      → obfuscate web JS only when explicitly required
# NMS_ENABLE_JS_MINIFICATION=1     → minify web JS only when explicitly required
# NMS_ENABLE_HTML_MINIFICATION=1   → minify HTML only when explicitly required
```

Artifacts are output to `artifacts/windows/<version>/` and `artifacts/linux/<version>/`.

### Frontend Sync

The frontend source of truth is `apps/frontend/`. **Never edit** `apps/backend/static/` or `apps/backend/cmd/agent/static/` directly.

```bash
bash ./scripts/sync-static.sh      # Linux/Mac
powershell .\scripts\sync-static.ps1  # Windows
```

---

## Architecture

### Backend Layer Structure

```
api/router.go          → Route definitions only
api/handlers/<name>.go → Thin HTTP wrappers (parse request, call module, return JSON)
modules/<name>/        → Business logic, domain types, persistence
services/<name>/       → Background goroutines (SNMP collector, pinger, alerts, etc.)
database/db.go         → SQLite schema & migrations
config/config.go       → YAML config loader
```

**The module extraction pattern:** when adding features, keep handlers thin and push logic into `modules/<feature>/`. Preserve external API payload compatibility unless intentionally versioning.

### Key Background Services

| Service | Role |
|---------|------|
| `services/snmp` | Polls devices for CPU/mem/interface metrics |
| `services/scheduler` | Orchestrates polling intervals |
| `services/pinger` | Heartbeat health checks |
| `services/alert` | Dispatches alerts (email, webhook, syslog) |
| `services/cloud` | MQTT sync with EdgeCore cloud platform |
| `services/syslog` | Inbound syslog UDP receiver |
| `services/dbworker` | Serialized DB write queue (prevents SQLite race conditions) |
| `services/license` | License validation and feature entitlement enforcement |

### Middleware Stack (in order)

`CORS → Logger → Secure → AuthRequired → EnforceLicenseLock → role checks (RequireAdmin / RequireEditor / RequireDeviceManagement)`

### Authentication

JWT (HS256) with TOTP 2FA support. Roles: Admin, Editor, Viewer. Routes under `/api/v1/` are grouped by privilege in `router.go`.

### Frontend (Vanilla JS SPA)

- `app.js` — route dispatching and view rendering
- `api.js` — fetch() wrapper with token injection and error handling
- `i18n.js` — English/Chinese translation support
- No build step; files are copied directly into Go embed

### Embedded Assets

Frontend files are embedded into the Go binary via `go:embed`. The server falls through to `index.html` for client-side routing. `assets.go` in the backend root wires this up.

### Database

SQLite with optional field-level encryption (`services/dbencryption`). All writes go through `services/dbworker` to serialize access. Schema and migrations are in `database/db.go`.

### Go Workspace

`go.work` at the root points to `./apps/backend`. Run `go` commands from the repo root or from `apps/backend/`.

---

## Configuration

Runtime config lives at `runtime/data/config.yaml` (generated on first run, not committed). Key sections: `server`, `database`, `security` (JWT secret, TLS, CORS origins), `logging`, `snmp`.

`runtime/data/` is the writable runtime directory for the database, logs, and uploads. It is not committed.
