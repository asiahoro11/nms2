# Management Server API 手册

版本: `v1.2.4.8`  
对象: 客户系统集成  
Base path: `/api/v1`

## 集成成熟度

| 状态 | 含义 | 客户使用建议 |
| --- | --- | --- |
| `Ready` | 已由 NMS 直接提供 API 或 runtime 能力 | 可用于正式集成 |
| `Bridge Ready` | NMS 提供稳定 ingest 或 embed 契约  协议 adapter 由外部 gateway 执行 | gateway 正规化数据后可正式集成 |
| `Planned` | 路线图项目  当前没有稳定 runtime contract | 不建议做正式集成 |

## 认证

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

成功后使用

```http
Authorization: Bearer <jwt>
Accept: application/json
```

如果启用 2FA  需要再调用

```http
POST /api/v1/auth/verify-2fa
Content-Type: application/json
```

## 优化集成 API

客户 dashboard 建议优先使用单一 snapshot API  减少多次 API 调用

```http
GET /api/v1/integrations/network-snapshot
Authorization: Bearer <jwt>
```

支持 query

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `include` | `dashboard,devices,topology` | 可选 `dashboard` `devices` `topology` |
| `page` | `1` | device page |
| `limit` | `500` | 最大 `2000` |
| `search` | 空 | 按名称或 IP 搜索 |
| `type` | 空 | device type |
| `status` | 空 | `online` 或 `offline` |

示例

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://nms.example.com/api/v1/integrations/network-snapshot?include=dashboard,devices,topology&limit=500"
```

此 API 不会返回 `snmp_community` password license key RTSP credential 等敏感字段

## 常用读取 API

| Method | Endpoint | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/system/info` | 版本与系统信息 |
| `GET` | `/api/v1/auth/me` | 当前用户 |
| `GET` | `/api/v1/dashboard` | dashboard summary |
| `GET` | `/api/v1/devices?page=1&limit=50` | device list |
| `GET` | `/api/v1/devices/:id` | device detail |
| `GET` | `/api/v1/topology` | topology nodes and links |
| `GET` | `/api/v1/cameras/status` | camera module status |
| `GET` | `/api/v1/pdu/status` | PDU/UPS module status |
| `GET` | `/api/v1/iot/status` | IoT/Modbus status |
| `GET` | `/api/v1/iot/capabilities` | IoT profile list |

## iframe Embed

Admin 创建 scoped embed token

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

客户系统嵌入

```html
<iframe
  src="https://nms.example.com/embed.html?view=dashboard&token=<embed-token>"
  style="width:100%;height:640px;border:0;"
  loading="lazy"
  referrerpolicy="no-referrer">
</iframe>
```

生产环境必须设置明确 iframe allowlist

```yaml
security:
  frame_ancestors:
    - "'self'"
    - "https://customer.example.com"
```

## IoT / Modbus / Gateway

| Protocol | Mode | Status | 集成方式 |
| --- | --- | --- | --- |
| `modbus_tcp` | direct poll | `Ready` | 创建 `/api/v1/iot/devices` 后调用 `/api/v1/iot/devices/:id/poll` |
| `rest` | gateway ingest | `Ready` | gateway 将正规化数据 POST 到 `/api/v1/iot/ingest` |
| `mqtt` | gateway ingest | `Bridge Ready` | 外部 MQTT bridge 订阅后 POST 到 `/api/v1/iot/ingest` |
| `opcua` | gateway ingest | `Bridge Ready` | 外部 OPC-UA gateway POST 到 `/api/v1/iot/ingest` |
| `bacnet` | gateway ingest | `Bridge Ready` | 外部 BACnet/IP collector POST 到 `/api/v1/iot/ingest` |
| `mqtt_native` | direct broker subscription | `Planned` | 本版未提供内置 broker client |
| `opcua_native` | direct node polling | `Planned` | 本版未提供内置 OPC-UA client |
| `bacnet_native` | direct BACnet/IP polling | `Planned` | 本版未提供内置 BACnet client |

REST ingest 示例

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

## 写入 API 边界

写入 API 仅供可信系统使用  并受 RBAC license gate audit log 管控

| Method | Endpoint | Role | 说明 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/devices` | editor | 创建设备 |
| `PUT` | `/api/v1/devices/:id` | editor | 更新设备 |
| `DELETE` | `/api/v1/devices/:id` | editor | 删除设备 |
| `POST` | `/api/v1/iot/ingest` | editor | 写入 IoT measurement |
| `POST` | `/api/v1/iot/devices/:id/poll` | editor | poll Modbus TCP device |
| `POST` | `/api/v1/iot/devices` | admin | 创建 IoT/Modbus device |
| `PUT` | `/api/v1/integrations/settings` | admin | 更新 iframe allowlist |
| `DELETE` | `/api/v1/integrations/embed-tokens/:token_id` | admin | revoke embed token |

## 状态码

| Status | 含义 |
| --- | --- |
| `200` | 成功 |
| `201` | 已创建 |
| `400` | request body 或 query 无效 |
| `401` | token 缺失或无效 |
| `403` | role license feature gate 阻挡 |
| `404` | route 或 resource 不存在 |
| `409` | conflict |
| `500` | server error |

## 安全验证 Gate

- API response log audit export 不得出现 SNMP community password RTSP credential license key API key long-lived token
- iframe 一律使用 scoped embed token  不使用 admin JWT
- `frame_ancestors` 必须限制为客户明确域名
- OT/building device 必须放在 VLAN 或 ACL 保护网段
- 不可从公网直接暴露 Modbus TCP SNMP RTSP ONVIF BACnet OPC-UA
- MQTT OPC-UA BACnet gateway 需要来源 allowlist scoped token rate limit replay 防护与 audit log
- Windows 客户端必须使用 `start_nms.bat` 或 `start_nms.ps1` 启动

## 智慧建筑 2016 / 2024 对齐

本版支持智慧建筑集成证据与运营数据交换  不代表产品已取得智慧建筑标章 ISO 认证或法规核可

官方参考

- 2016 智慧建筑评估手册: `https://www.abri.gov.tw/News_Content.aspx?n=20916&s=323060`
- 2024 智慧建筑评估手册: `https://www.abri.gov.tw/News_Content.aspx?n=20916&s=322421`
- 2024 实施公告: `https://www.moi.gov.tw/Common/EpaperClick.ashx?esq=%40EpaperSendQueueSN&p=D3C50BA14D2E597DBF5BA233B2670F24E65203E80CE503D13003C306C4ECC0A73685AEC600EFED6FBD32224A754AA75931B871A26B58CC04238D485186800BC2`

