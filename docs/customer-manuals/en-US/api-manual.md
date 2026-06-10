# Management Server API Manual

Version: `v1.2.4.8`  
Audience: customer system integration  
Base path: `/api/v1`

## Integration Readiness

| Status | Meaning | Customer use |
| --- | --- | --- |
| `Ready` | Implemented as a direct NMS API or runtime capability | Safe for production integration |
| `Bridge Ready` | NMS provides a stable ingest or embed contract while the protocol adapter runs outside NMS | Safe when a gateway normalizes data into the documented API |
| `Planned` | Roadmap item without a stable runtime contract in this release | Do not build production integrations on this item |

## Authentication

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

Use the returned JWT with:

```http
Authorization: Bearer <jwt>
Accept: application/json
```

If 2FA is enabled, call:

```http
POST /api/v1/auth/verify-2fa
Content-Type: application/json
```

## Optimized Integration API

Customer dashboards should use the snapshot API first to reduce API round trips.

```http
GET /api/v1/integrations/network-snapshot
Authorization: Bearer <jwt>
```

Query parameters:

| Name | Default | Description |
| --- | --- | --- |
| `include` | `dashboard,devices,topology` | Allowed values are `dashboard`, `devices`, `topology` |
| `page` | `1` | Device page |
| `limit` | `500` | Maximum `2000` |
| `search` | empty | Search by name or IP |
| `type` | empty | Device type |
| `status` | empty | `online` or `offline` |

Example:

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://nms.example.com/api/v1/integrations/network-snapshot?include=dashboard,devices,topology&limit=500"
```

This API omits sensitive fields such as `snmp_community`, passwords, license keys, and RTSP credentials.

## Common Read APIs

| Method | Endpoint | Description |
| --- | --- | --- |
| `GET` | `/api/v1/system/info` | Version and system metadata |
| `GET` | `/api/v1/auth/me` | Current user |
| `GET` | `/api/v1/dashboard` | Dashboard summary |
| `GET` | `/api/v1/devices?page=1&limit=50` | Device list |
| `GET` | `/api/v1/devices/:id` | Device detail |
| `GET` | `/api/v1/topology` | Topology nodes and links |
| `GET` | `/api/v1/cameras/status` | Camera module status |
| `GET` | `/api/v1/pdu/status` | PDU/UPS module status |
| `GET` | `/api/v1/iot/status` | IoT/Modbus status |
| `GET` | `/api/v1/iot/capabilities` | IoT profile list |

## iframe Embed

Create a scoped embed token as admin:

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

Customer iframe example:

```html
<iframe
  src="https://nms.example.com/embed.html?view=dashboard&token=<embed-token>"
  style="width:100%;height:640px;border:0;"
  loading="lazy"
  referrerpolicy="no-referrer">
</iframe>
```

Production deployments must use explicit iframe allowlists:

```yaml
security:
  frame_ancestors:
    - "'self'"
    - "https://customer.example.com"
```

## IoT / Modbus / Gateway

| Protocol | Mode | Status | Integration path |
| --- | --- | --- | --- |
| `modbus_tcp` | direct poll | `Ready` | Create `/api/v1/iot/devices`, then call `/api/v1/iot/devices/:id/poll` |
| `rest` | gateway ingest | `Ready` | Gateway posts normalized data to `/api/v1/iot/ingest` |
| `mqtt` | gateway ingest | `Bridge Ready` | External MQTT bridge subscribes and posts to `/api/v1/iot/ingest` |
| `opcua` | gateway ingest | `Bridge Ready` | External OPC-UA gateway posts to `/api/v1/iot/ingest` |
| `bacnet` | gateway ingest | `Bridge Ready` | External BACnet/IP collector posts to `/api/v1/iot/ingest` |
| `mqtt_native` | direct broker subscription | `Planned` | Built-in broker client is not included |
| `opcua_native` | direct node polling | `Planned` | Built-in OPC-UA client is not included |
| `bacnet_native` | direct BACnet/IP polling | `Planned` | Built-in BACnet client is not included |

REST ingest example:

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

## Write API Boundary

Write APIs are for trusted systems only and are governed by RBAC, license gates, and audit logs.

| Method | Endpoint | Role | Description |
| --- | --- | --- | --- |
| `POST` | `/api/v1/devices` | editor | Create device |
| `PUT` | `/api/v1/devices/:id` | editor | Update device |
| `DELETE` | `/api/v1/devices/:id` | editor | Delete device |
| `POST` | `/api/v1/iot/ingest` | editor | Write IoT measurement |
| `POST` | `/api/v1/iot/devices/:id/poll` | editor | Poll Modbus TCP device |
| `POST` | `/api/v1/iot/devices` | admin | Create IoT/Modbus device |
| `PUT` | `/api/v1/integrations/settings` | admin | Update iframe allowlist |
| `DELETE` | `/api/v1/integrations/embed-tokens/:token_id` | admin | Revoke embed token |

## Status Codes

| Status | Meaning |
| --- | --- |
| `200` | Success |
| `201` | Created |
| `400` | Invalid body or query |
| `401` | Missing or invalid token |
| `403` | Blocked by role, license, or feature gate |
| `404` | Route or resource not found |
| `409` | Conflict |
| `500` | Server error |

## Security Validation Gate

- API responses, logs, audit entries, and exports must not expose SNMP communities, passwords, RTSP credentials, license keys, API keys, or long-lived tokens.
- iframe pages must use scoped embed tokens, not admin JWTs.
- `frame_ancestors` must be restricted to explicit customer domains.
- OT/building devices must be placed on VLANs or ACL-protected networks.
- Do not expose Modbus TCP, SNMP, RTSP, ONVIF, BACnet, or OPC-UA directly to the public internet.
- MQTT, OPC-UA, and BACnet gateways need source allowlists, scoped tokens, rate limits, replay protection, and audit logs.
- Windows deployments must start through `start_nms.bat` or `start_nms.ps1`.

## Smart Building 2016 / 2024 Alignment

This release supports smart-building integration evidence and operational data exchange. It does not claim that the product has obtained a smart-building label, ISO certification, or statutory approval.

Official references:

- 2016 Smart Building Evaluation Manual: `https://www.abri.gov.tw/News_Content.aspx?n=20916&s=323060`
- 2024 Smart Building Evaluation Manual: `https://www.abri.gov.tw/News_Content.aspx?n=20916&s=322421`
- 2024 implementation notice: `https://www.moi.gov.tw/Common/EpaperClick.ashx?esq=%40EpaperSendQueueSN&p=D3C50BA14D2E597DBF5BA233B2670F24E65203E80CE503D13003C306C4ECC0A73685AEC600EFED6FBD32224A754AA75931B871A26B58CC04238D485186800BC2`

