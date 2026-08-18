# Management Server API Manual

Version: `v1.2.4.9sp00022`
Audience: customer system integration
Base path: `/api/v1`

## Integration Readiness

`v1.2.4.9sp00022` is the customer integration stable release for the current API contract.

Capability status terms:

| Status | Meaning | Customer use |
| --- | --- | --- |
| `Ready` | Implemented as a direct NMS API or runtime capability. | Safe for customer integration. |
| `Bridge Ready` | NMS provides a stable ingest or embed contract, while the protocol adapter runs outside NMS. | Safe when a gateway normalizes data into the documented API. |
| `Planned` | Reserved roadmap item. No stable runtime contract is exposed yet. | Do not build production integrations against this item. |

## A few practical notes before you integrate

This manual is written for engineers connecting a real system to NMS. These details are worth calling out up front:

- The API root is `/api/v1`. When the document shows `/devices`, the full URL is
  `/api/v1/devices`; do not add another `/api/v1` segment.
- Except for login, public system information, and the integrity status endpoint, requests need a Bearer JWT.
  JWTs expire. Store them on the server side and never put them in browser logs, URLs, or exported files.
- `viewer` is read-only, `editor` can perform normal device and IoT actions, and `admin` can change users,
  licensing, system settings, IoT definitions, and forwarding settings. A correct role does not bypass a missing License;
  those requests can still return `403`.
- Implement pagination for list APIs. Do not assume that one response contains the entire inventory or measurement history.
  Use device, status, and time filters where available.
- Time fields may be ISO 8601 or Unix epoch milliseconds. IoT forwarding `sendTime` is always an integer in milliseconds;
  do not interpret it as seconds.
- Keep the Modbus register address, function code, byte order, word order, scale, and offset together. A single decoded
  value is usually not enough to reproduce how the field device was read.
- `/iot/queue/status` reports NMS's local store-and-forward queue. If the target host is offline, records staying in this
  queue is expected; it does not mean the customer system has received them yet.
- If you receive `423 Locked`, check `/api/v1/system/integrity/status` before retrying write APIs. It means NMS is
  protecting the database and requires the documented offline recovery procedure.

## 1. Authentication

All integration endpoints use JSON and the standard response envelope:

```json
{
  "success": true,
  "data": {}
}
```

Login:

```http
POST /api/v1/auth/login
Content-Type: application/json
```

```json
{
  "username": "admin",
  "password": "your-password"
}
```

Success response:

```json
{
  "success": true,
  "data": {
    "token": "<jwt>",
    "user": {
      "id": 1,
      "username": "admin",
      "role": "admin",
      "is_active": true
    },
    "expires_at": 1770000000,
    "require_password_change": false
  }
}
```

If 2FA is enabled, the login response includes:

```json
{
  "requires_two_factor": true,
  "challenge_token": "<challenge-token>",
  "two_factor_method": "totp"
}
```

Verify 2FA:

```http
POST /api/v1/auth/verify-2fa
Content-Type: application/json
```

```json
{
  "challenge_token": "<challenge-token>",
  "code": "123456",
  "method": "totp"
}
```

Authenticated requests:

```http
Authorization: Bearer <jwt>
Accept: application/json
```

## 2. Optimized Integration API

`v1.2.4.9sp00022` provides a combined read API for customer dashboards and external systems.
Use this first when the client needs inventory, topology, and dashboard summary in one refresh.

```http
GET /api/v1/integrations/network-snapshot
Authorization: Bearer <jwt>
```

This replaces the common multi-call sequence:

```text
GET /api/v1/dashboard
GET /api/v1/devices
GET /api/v1/topology
```

### Query Parameters

| Name | Default | Description |
| --- | --- | --- |
| `include` | `dashboard,devices,topology` | Comma-separated sections. Allowed values are `dashboard`, `devices`, `topology`. |
| `page` | `1` | Device list page. |
| `limit` | `500` | Device list page size. Maximum is `2000`. |
| `search` | empty | Device name or IP search. Applies to the embedded device list. |
| `type` | empty | Device type filter. Example `switch`, `router`, `server`. |
| `status` | empty | Device status filter. Use `online` or `offline`. |

### Example

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://nms.example.com/api/v1/integrations/network-snapshot?include=dashboard,devices,topology&limit=500"
```

### Response

```json
{
  "success": true,
  "data": {
    "schema_version": "nms.integration.network_snapshot.v1",
    "generated_at": "2026-05-25T02:30:00Z",
    "api_version": "v1",
    "system": {
      "name": "System Server",
      "version": "v1.2.4.9sp00022"
    },
    "dashboard": {
      "version": "v1.2.4.9sp00022",
      "system_name": "System Server",
      "total_devices": 120,
      "online_count": 110,
      "offline_count": 10,
      "device_types": {
        "switch": 80,
        "router": 10
      },
      "recent_events": [],
      "top_devices": [],
      "system_stats": {
        "memory_usage": 42.5,
        "go_routines": 32
      }
    },
    "devices": {
      "items": [
        {
          "id": 1,
          "name": "Core Switch",
          "sys_name": "core-sw-01",
          "ip_address": "10.0.0.1",
          "mac_address": "00:11:22:33:44:55",
          "device_type": "switch",
          "snmp_version": 2,
          "vendor": "Edgecore",
          "model": "ECS",
          "firmware": "1.0.0",
          "is_online": true,
          "last_seen": "2026-05-25 10:00:00",
          "pos_x": 10,
          "pos_y": 20,
          "created_at": "2026-05-25 09:00:00",
          "updated_at": "2026-05-25 10:00:00"
        }
      ],
      "total": 120,
      "page": 1,
      "limit": 500
    },
    "topology": {
      "nodes": [],
      "links": []
    },
    "links": {
      "self": "/api/v1/integrations/network-snapshot",
      "dashboard": "/api/v1/dashboard",
      "devices": "/api/v1/devices",
      "topology": "/api/v1/topology"
    }
  }
}
```

The optimized integration payload intentionally omits sensitive fields such as `snmp_community`, passwords, license keys, and RTSP URLs.

## 3. Existing Read APIs

These endpoints remain available for clients that need smaller calls.

| Method | Endpoint | Description |
| --- | --- | --- |
| `GET` | `/api/v1/system/info` | Runtime version and basic system metadata. No token required. |
| `GET` | `/api/v1/auth/me` | Current authenticated user. |
| `GET` | `/api/v1/dashboard` | Dashboard summary. |
| `GET` | `/api/v1/devices?page=1&limit=50` | Paginated device list. |
| `GET` | `/api/v1/devices/:id` | Device detail. |
| `GET` | `/api/v1/devices/:id/metrics` | Latest or historical device metrics. |
| `GET` | `/api/v1/devices/:id/interfaces` | Device interface list. |
| `GET` | `/api/v1/devices/:id/events` | Device event list. |
| `GET` | `/api/v1/topology` | Topology nodes and links. |
| `GET` | `/api/v1/cameras/status` | Camera module status. |
| `GET` | `/api/v1/access-control/status` | Access-control module status. |
| `GET` | `/api/v1/pdu/status` | PDU/UPS module status. |
| `GET` | `/api/v1/iot/status` | IoT/Modbus integration status. |
| `GET` | `/api/v1/iot/capabilities` | Supported direct and gateway IoT protocol profiles. |
| `GET` | `/api/v1/iot/devices` | IoT/Modbus device list. |
| `GET` | `/api/v1/iot/measurements?limit=100` | Recent IoT measurements. |
| `GET` | `/api/v1/iot/queue/status` | Local IoT store-and-forward queue status. |
| `GET` | `/api/v1/integrations/embed-snapshot?view=dashboard&token=<embed-token>` | Read-only payload used by iframe pages. |
| `GET` | `/api/v1/license/features` | Feature flags allowed by the current license. |

## 4. iframe Embed Integration

Create a scoped embed token as an admin:

```http
POST /api/v1/integrations/embed-tokens
Authorization: Bearer <admin-jwt>
Content-Type: application/json
```

```json
{
  "name": "Customer Portal",
  "views": ["dashboard", "topology", "devices", "alerts", "iot"],
  "expires_in_minutes": 1440
}
```

Response:

```json
{
  "success": true,
  "data": {
    "token_id": "9f2e0f8a6c1d4a22a4bda411cbbf0d6e",
    "token": "<embed-token>",
    "expires_at": "2026-05-30T00:00:00Z",
    "url": "/embed.html?view=dashboard&token=<embed-token>",
    "views": ["dashboard", "topology", "devices", "alerts", "iot"]
  }
}
```

Customer system iframe example:

```html
<iframe
  src="https://nms.example.com/embed.html?view=dashboard&token=<embed-token>"
  style="width:100%;height:640px;border:0;"
  loading="lazy">
</iframe>
```

Supported `view` values:

| View | Description |
| --- | --- |
| `dashboard` | Summary counters for dashboard widgets. |
| `topology` | Topology node/link summary and node list. |
| `devices` | Sanitized device inventory. |
| `alerts` | Recent system events. |
| `iot` | IoT/Modbus status and device latest values. |

Embed tokens are read-only and scoped to the requested views. The server CSP allows iframe integration through `security.frame_ancestors` in `config.yaml`.
Newly issued embed tokens are also stored server-side so admins can list and revoke them.

Example:

```yaml
security:
  frame_ancestors:
    - "'self'"
    - "https://customer.example.com"
```

Admin token management:

```http
GET /api/v1/integrations/embed-tokens
Authorization: Bearer <admin-jwt>
```

```http
DELETE /api/v1/integrations/embed-tokens/<token_id>
Authorization: Bearer <admin-jwt>
```

Runtime iframe allowlist management:

```http
GET /api/v1/integrations/settings
Authorization: Bearer <admin-jwt>
```

```http
PUT /api/v1/integrations/settings
Authorization: Bearer <admin-jwt>
Content-Type: application/json
```

```json
{
  "frame_ancestors": ["'self'", "https://customer.example.com"]
}
```

Allowed `frame_ancestors` values are `'self'`, `'none'`, `http:`, `https:`, and exact `http://` or `https://` origins.

## 5. IoT / Modbus Integration APIs

### Modbus TCP Device And Background Collection

Create a Modbus TCP signal point. For a temperature and humidity sensor, create one entry per register or metric, even when both entries point to the same Modbus TCP host.

```http
POST /api/v1/iot/devices
Authorization: Bearer <admin-jwt>
Content-Type: application/json
```

```json
{
  "name": "Power Meter 1",
  "protocol": "modbus_tcp",
  "host": "10.10.10.50",
  "port": 502,
  "unit_id": 1,
  "address": 0,
  "function_code": 3,
  "quantity": 2,
  "data_type": "float32",
  "byte_order": "big",
  "word_order": "big",
  "scale": 1,
  "offset": 0,
  "metric": "temperature",
  "poll_interval_seconds": 60,
  "enabled": true
}
```

NMS automatically polls enabled `modbus_tcp` entries in the background. Manual poll is also available for commissioning and troubleshooting:

```http
POST /api/v1/iot/devices/1/poll
Authorization: Bearer <editor-or-admin-jwt>
```

Supported `data_type` values:

| Type | Registers | Notes |
| --- | --- | --- |
| `uint16` | 1 | Unsigned 16-bit integer. |
| `int16` | 1 | Signed 16-bit integer. |
| `uint32` | 2 | Unsigned 32-bit integer. |
| `int32` | 2 | Signed 32-bit integer. |
| `float32` | 2 | IEEE 754 float. |

Supported function codes:

| Function | Description |
| --- | --- |
| `3` | Holding Register FC03 |
| `4` | Input Register FC04 |

The final value is calculated as:

```text
decoded_register_value * scale + offset
```

### Local Store-And-Forward Queue

Every background Modbus TCP reading and every REST ingest payload is written to local SQLite storage first. Customer systems can integrate with NMS instead of communicating with each Modbus TCP device directly.

When the configured forward endpoint is offline or returns a non-2xx response, NMS keeps the measurements locally and retries later. When the forward endpoint recovers, NMS continues delivery from the local queue. Successful records are marked as `sent`, kept locally for 10 minutes, then dropped by background cleanup.

Read queue status:

```http
GET /api/v1/iot/queue/status
Authorization: Bearer <editor-or-admin-jwt>
```

Configure the HTTP forwarder:

```http
PUT /api/v1/iot/forwarder/settings
Authorization: Bearer <admin-jwt>
Content-Type: application/json
```

```json
{
  "enabled": true,
  "url": "https://customer.example.com/nms/iot",
  "token": "<customer-forward-token>",
  "clear_token": false,
  "batch_size": 50,
  "interval_ms": 30000
}
```

The token is stored server-side and is not returned by the settings API. The forward payload contains `deviceId`, Unix epoch millisecond `sendTime`, and a multi-value `tagData` object. NMS sends a stable sample identifier in the `X-NMS-Idempotency-Key` header.

Force a retry flush:

```http
POST /api/v1/iot/queue/flush
Authorization: Bearer <editor-or-admin-jwt>
```

Force cleanup of already sent and expired records:

```http
POST /api/v1/iot/queue/cleanup
Authorization: Bearer <admin-jwt>
```

### REST / Webhook Ingest

Use this when an IoT gateway, MQTT bridge, or customer system pushes values into NMS:

```http
POST /api/v1/iot/ingest
Authorization: Bearer <editor-or-admin-jwt>
Content-Type: application/json
```

```json
{
  "external_id": "floor-1-temp-01",
  "name": "Floor 1 Temperature",
  "protocol": "rest",
  "metric": "temperature",
  "value": 25.5,
  "raw": {
    "unit": "C",
    "source": "customer-gateway"
  }
}
```

The ingest API upserts a lightweight IoT device by `external_id` and stores the measurement in `iot_measurements`.

Supported IoT profiles:

```http
GET /api/v1/iot/capabilities
Authorization: Bearer <jwt>
```

| Protocol | Mode | Status | Integration path |
| --- | --- | --- | --- |
| `modbus_tcp` | direct poll | `Ready` | Configure `/api/v1/iot/devices`; NMS polls enabled entries in the background and writes local queue records. |
| `rest` | gateway ingest | `Ready` | Push normalized values to `/api/v1/iot/ingest`; NMS writes the same local queue records. |
| `http_forward` | store and forward | `Ready` | NMS forwards queued IoT records to the configured customer webhook with offline retry and 10-minute post-success cleanup. |
| `mqtt` | gateway ingest | `Bridge Ready` | MQTT bridge subscribes externally and posts normalized values to `/api/v1/iot/ingest`. |
| `opcua` | gateway ingest | `Bridge Ready` | OPC-UA gateway posts node values to `/api/v1/iot/ingest`. |
| `bacnet` | gateway ingest | `Bridge Ready` | BACnet/IP collector posts point values to `/api/v1/iot/ingest`. |
| `mqtt_native` | direct broker subscription | `Planned` | No built-in broker client contract in this release. Use bridge ingest. |
| `opcua_native` | direct node polling | `Planned` | No built-in OPC-UA client contract in this release. Use bridge ingest. |
| `bacnet_native` | direct BACnet/IP polling | `Planned` | No built-in BACnet client contract in this release. Use bridge ingest. |

## 6. Write APIs For Managed Integrations

Use write APIs only for trusted integrations with editor or admin role.
Every write is subject to license, RBAC, and audit logging.

| Method | Endpoint | Role | Description |
| --- | --- | --- | --- |
| `POST` | `/api/v1/devices` | editor | Create a device. |
| `PUT` | `/api/v1/devices/:id` | editor | Update a device. |
| `DELETE` | `/api/v1/devices/:id` | editor | Delete a device. |
| `POST` | `/api/v1/devices/bulk-scan` | editor | Scan and add devices. |
| `POST` | `/api/v1/topology/discover` | editor | Start LLDP topology discovery. |
| `PUT` | `/api/v1/topology/positions` | editor | Save topology positions. |
| `POST` | `/api/v1/iot/ingest` | editor | Push an IoT measurement. |
| `POST` | `/api/v1/iot/devices/:id/poll` | editor | Poll a Modbus TCP device. |
| `POST` | `/api/v1/iot/queue/flush` | editor | Retry queued IoT forwarding. |
| `POST` | `/api/v1/iot/devices` | admin | Create an IoT/Modbus device. |
| `PUT` | `/api/v1/iot/devices/:id` | admin | Update an IoT/Modbus device. |
| `DELETE` | `/api/v1/iot/devices/:id` | admin | Delete an IoT/Modbus device. |
| `GET` | `/api/v1/iot/forwarder/settings` | admin | Read IoT forwarder settings without exposing the bearer token. |
| `PUT` | `/api/v1/iot/forwarder/settings` | admin | Update IoT forwarder settings. |
| `POST` | `/api/v1/iot/queue/cleanup` | admin | Drop sent IoT queue records after the 10-minute hold window. |
| `GET` | `/api/v1/integrations/settings` | admin | Read iframe and integration settings. |
| `PUT` | `/api/v1/integrations/settings` | admin | Update iframe allowlist. |
| `GET` | `/api/v1/integrations/embed-tokens` | admin | List issued embed tokens. |
| `DELETE` | `/api/v1/integrations/embed-tokens/:token_id` | admin | Revoke an embed token. |

## 7. Complete Categorized API Catalog

This section lists the API routes currently registered by `router.go`. Except for public
routes and the integrity status route, requests require `Authorization: Bearer <jwt>`.
`:id`, `:tokenId`, `:ifIndex`, and `:backupId` are path parameters. The role column is
the minimum role: `viewer`, `editor`, or `admin`.

### 7.1 Public, authentication, and system

| Method | Path | Minimum role | Purpose |
|---|---|---|---|
| POST | `/auth/login` | public | Login and receive a JWT or 2FA challenge. |
| POST | `/auth/verify-2fa` | public | Complete TOTP login. |
| POST | `/auth/forgot-password` | public | Request a password reset. |
| POST | `/auth/reset-password` | public | Set a password with a reset token. |
| GET | `/system/info` | public | Version and basic system information. |
| GET | `/branding` | public | Read public branding settings. |
| GET | `/system/config` | public | Read public system configuration. |
| POST | `/license/debug` | public/debug | License debug query; disable in production. |
| GET | `/system/integrity/status` | public | Read-only integrity lockdown status. |

### 7.2 Users, dashboard, license, alerts, and logs

| Method | Path | Minimum role | Purpose |
|---|---|---|---|
| POST | `/auth/logout` | viewer | Log out. |
| GET | `/auth/me` | viewer | Current user. |
| POST | `/auth/change-password` | viewer | Change the current password. |
| GET | `/auth/2fa/status` | viewer | Read 2FA status. |
| POST | `/auth/2fa/enroll` | viewer | Begin TOTP enrollment. |
| POST | `/auth/2fa/confirm` | viewer | Confirm TOTP enrollment. |
| POST | `/auth/2fa/disable` | viewer | Disable the current user's 2FA. |
| POST | `/auth/2fa/recovery-codes/regenerate` | viewer | Regenerate recovery codes. |
| GET | `/dashboard`, `/dashboard/top-cpu`, `/dashboard/top-memory` | viewer | Dashboard and resource rankings. |
| GET | `/license/status`, `/license/features` | viewer | License state and feature flags. |
| GET | `/alerts/settings`, `/events`, `/logs` | viewer | Alert settings and aggregated events/logs. |
| GET | `/system-logs`, `/device-logs`, `/config-change-logs` | viewer | Specialized log streams. |
| GET | `/log-center/options` | viewer | Log-center filter options. |
| GET | `/log-center/export` | viewer | Export log-center data. |
| GET | `/log-center/evidence-bundle` | viewer | Export audit evidence bundle. |
| PUT | `/audit-logs/:id/review` | viewer | Review an audit log. |
| PUT | `/system-logs/:id/review` | viewer | Review a system log. |
| PUT | `/config-change-logs/:id/review` | viewer | Review a configuration-change log. |
| PUT | `/device-logs/:id/ack` | viewer | Acknowledge a device log. |
| GET | `/notifications` | viewer | List notifications. |
| PUT | `/notifications/read-all`, `/notifications/:id/read` | viewer | Mark notifications read. |
| DELETE | `/notifications/all`, `/notifications/:id` | viewer | Delete notifications. |

### 7.3 Network devices, topology, and device actions

| Method | Path | Minimum role | Purpose |
|---|---|---|---|
| GET | `/devices`, `/devices/:id` | viewer | Paginated inventory and device details. |
| GET | `/devices/:id/metrics` | viewer | Device metrics. |
| GET | `/devices/:id/interfaces` | viewer | Interface inventory. |
| GET | `/devices/:id/traffic` | viewer | Interface traffic. |
| GET | `/devices/:id/events` | viewer | Device events. |
| GET | `/topology` | viewer | Topology nodes and links. |
| GET | `/topology/device/:id/interfaces` | viewer | Interfaces associated with topology links. |
| POST | `/devices` | editor | Create a device. |
| PUT | `/devices/:id` | editor | Update a device. |
| DELETE | `/devices/:id` | editor | Delete a device. |
| POST | `/devices/:id/image` | editor | Upload a device image. |
| POST | `/devices/:id/poll` | editor | Poll a device immediately. |
| POST | `/devices/bulk-scan` | editor | Scan devices in bulk. |
| POST | `/devices/bulk-delete` | editor | Delete devices in bulk. |
| POST | `/devices/bulk-update` | editor | Update devices in bulk. |
| POST | `/topology/links` | editor | Create a topology link. |
| PUT | `/topology/links/:id` | editor | Update a topology link. |
| DELETE | `/topology/links/:id` | editor | Delete a topology link. |
| PUT | `/topology/positions` | editor | Save topology positions. |
| POST | `/topology/discover` | editor | Start topology discovery. |
| POST | `/devices/:id/reboot` | editor | Reboot a device. |
| POST | `/devices/:id/backup` | editor | Create a configuration backup. |
| GET | `/devices/:id/backups` | editor | List configuration backups. |
| GET | `/devices/:id/backups/:backupId/download` | editor | Download a configuration backup. |
| GET | `/devices/:id/poe`, `/devices/:id/ap` | editor | Read PoE and AP status. |
| POST | `/devices/:id/poe/:portIndex/action` | editor | Run a PoE port action. |
| POST | `/devices/:id/interfaces/:ifIndex/status` | editor | Change interface administrative status. |

### 7.4 Report exports

All report routes require at least `editor` and return CSV, PDF, or JSON according to the
route. Query parameters are validated by the individual report handler.

`GET /reports/devices`, `/reports/devices/pdf`, `/reports/logs`, `/reports/logs/pdf`,
`/reports/traffic`, `/reports/health`, `/reports/availability`, `/reports/inventory`,
`/reports/interfaces`, `/reports/health-trend`, `/reports/sla`, `/reports/audit`,
`/reports/license-capacity`, `/reports/cameras`, `/reports/pdu`, `/reports/access-control`,
`/reports/topology`, `/reports/events`, `/reports/syslog`, `/reports/notifications`,
`/reports/iot-devices`, `/reports/iot-measurements`, `/reports/camera-recordings`,
`/reports/access-events`, `/reports/access-cards`, `/reports/access-schedules`, and
`/reports/config-backups`.

### 7.5 IoT, Modbus, and offline forwarding

| Method | Path | Minimum role | Purpose |
|---|---|---|---|
| GET | `/iot/status` | viewer | IoT module status. |
| GET | `/iot/capabilities` | viewer | Protocol and capability profiles. |
| GET | `/iot/devices` | viewer | IoT device inventory. |
| GET | `/iot/measurements` | viewer | Measurement query. |
| GET | `/iot/queue/status` | viewer | Store-and-forward queue status. |
| POST | `/iot/ingest` | editor | Receive normalized gateway, REST, or MQTT-bridge signals. |
| POST | `/iot/devices/:id/poll` | editor | Poll an IoT device immediately. |
| POST | `/iot/queue/flush` | editor | Retry queued forwarding. |
| POST | `/iot/devices` | admin | Create a device and its signal definitions. |
| PUT | `/iot/devices/:id` | admin | Update a device and its signals. |
| DELETE | `/iot/devices/:id` | admin | Delete an IoT device. |
| GET | `/iot/forwarder/settings` | admin | Read forwarding settings; bearer token is never returned. |
| PUT | `/iot/forwarder/settings` | admin | Set target URL, batch size, and `interval_ms`. |
| POST | `/iot/queue/cleanup` | admin | Remove successfully forwarded records. |

### 7.6 Integrations, users, licensing, and system administration

Integration routes are `GET /integrations/network-snapshot`, `GET /integrations/embed-snapshot`,
`GET/PUT /integrations/settings`, `GET/POST /integrations/embed-tokens`, and
`DELETE /integrations/embed-tokens/:tokenId`. Snapshot routes are read-only; settings and
embed-token management require `admin`.

User and license administration includes `GET/POST/PUT/DELETE /users` (admin),
`GET /audit-logs`, `GET /audit-logs/actions`, `GET /license/machine-id`,
`POST /license/activate`, `GET/POST /licenses`, `POST /license/reset`, and
`POST /license/reissue`.

System administration includes `GET /system/backup`, `GET /system/restore-readiness`,
`POST /system/restore`, encrypted backup/restore and encryption-password/status routes,
alert setting/test routes, ping/traceroute tools, branding update/logo routes,
`GET /system/host-status`, `GET/PUT /security/settings`, and `PUT /system/config/:key`.
All require `admin`.

### 7.7 Cameras, access control, PDU, and terminal access

Camera administration includes camera create/discovery/credential/update/delete/health routes
and `GET/PUT /nvr/config` (admin). Editors read camera inventory and manage recording and
`/recordings*`; authenticated viewers may read camera status, snapshots, and MJPEG streams.

Authenticated viewers may read `/access-control/status` and `/access-control/events`. Editors
read doors, cards, and door details. Administrators manage doors, cards, events, schedules,
schedule approval, and schedule access checks.

Authenticated viewers may read `/pdu/status`; editors manage `/pdu/devices`, device details,
and polling. `GET /devices/:id/terminal` provides the authenticated editor WebSocket terminal.

## 8. HTTP Status Codes

| Status | Meaning |
| --- | --- |
| `200` | Success. |
| `201` | Resource created. |
| `400` | Invalid request body or query parameter. |
| `401` | Missing or invalid token. |
| `403` | Role, license, or feature gate blocked the request. |
| `404` | Route or resource not found. |
| `409` | Conflict, such as duplicate device IP. |
| `423` | Integrity lockdown is active; application APIs are blocked. |
| `429` | Login or request rate limit exceeded. |
| `500` | Server-side error. |

## 9. Integration Recommendations

- Prefer HTTPS in customer deployments.
- Use one dedicated integration account per customer system.
- Store JWT only in server-side or secure storage.
- Use embed tokens for iframe pages instead of placing admin JWTs in iframe URLs.
- Refresh with `/api/v1/integrations/network-snapshot` for dashboard-style views.
- Use `/api/v1/iot/ingest` for IoT gateway push flows and Modbus TCP device polling for direct LAN devices.
- Use smaller existing APIs for drill-down pages after the user selects one device.
- Keep `schema_version` checks in the client so future payload changes can be handled safely.
- Do not rely on undocumented database columns. The API payload is the external contract.

## 10. Security Validation Gate

For customer deployments, treat these as minimum verification items before go-live:

- Device, PDU, camera, log, audit, export, and integration responses must not expose SNMP communities, passwords, RTSP credentials, license keys, API keys, or long-lived tokens.
- Use scoped iframe embed tokens for embedded pages. Do not use admin JWTs in iframe URLs.
- Keep iframe `frame_ancestors` restricted to explicit customer domains. Do not use broad `http:` or `https:` allowlists in production.
- Put OT and building devices on segmented VLANs or ACL-protected networks. Do not expose Modbus TCP, SNMP, RTSP, ONVIF, BACnet, or OPC-UA ports directly to the public internet.
- For MQTT, OPC-UA, and BACnet gateways, enforce source allowlists, scoped tokens, rate limits, replay protection, and audit logging before posting to `/api/v1/iot/ingest`.
- Keep Windows deployment on `start_nms.bat` or `start_nms.ps1`; do not launch `nms_server.exe` directly.

## 11. Smart Building 2016 / 2024 Alignment

This release supports smart-building integration evidence and operational data exchange. It does not claim that the product itself has obtained a smart-building label, ISO certification, or statutory approval.

Customer project teams should validate project compliance against the official Ministry of the Interior / Architecture and Building Research Institute materials:

- 2016 Smart Building Evaluation Manual: `https://www.abri.gov.tw/News_Content.aspx?n=20916&s=323060`
- 2024 Smart Building Evaluation Manual: `https://www.abri.gov.tw/News_Content.aspx?n=20916&s=322421`
- 2024 implementation notice: `https://www.moi.gov.tw/Common/EpaperClick.ashx?esq=%40EpaperSendQueueSN&p=D3C50BA14D2E597DBF5BA233B2670F24E65203E80CE503D13003C306C4ECC0A73685AEC600EFED6FBD32224A754AA75931B871A26B58CC04238D485186800BC2`

NMS alignment points for customer evidence:

- Network and device inventory can support communication and operations evidence.
- Audit logs, config-change logs, and export bundles can support traceability evidence.
- IoT direct Modbus TCP polling and REST ingest can support building equipment data collection.
- iframe embed and integration APIs can support third-party BMS, EMS, or smart-building dashboard integration.
