# Management Server API 手冊

版本: `v1.2.4.8`  
對象: 客戶系統整合  
Base path: `/api/v1`

## 整合成熟度

| 狀態 | 意義 | 客戶使用建議 |
| --- | --- | --- |
| `Ready` | 已由 NMS 直接提供 API 或 runtime 能力 | 可用於正式整合 |
| `Bridge Ready` | NMS 提供穩定 ingest 或 embed 契約  協定 adapter 由外部 gateway 執行 | 可在 gateway 正規化資料後正式整合 |
| `Planned` | 路線圖項目  目前沒有穩定 runtime contract | 不建議做正式整合 |

## 認證

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

成功後使用

```http
Authorization: Bearer <jwt>
Accept: application/json
```

若啟用 2FA  需再呼叫

```http
POST /api/v1/auth/verify-2fa
Content-Type: application/json
```

## 最佳化整合 API

客戶 dashboard 建議先使用單一 snapshot API  減少多次 API 呼叫

```http
GET /api/v1/integrations/network-snapshot
Authorization: Bearer <jwt>
```

支援 query

| 參數 | 預設 | 說明 |
| --- | --- | --- |
| `include` | `dashboard,devices,topology` | 可選 `dashboard` `devices` `topology` |
| `page` | `1` | device page |
| `limit` | `500` | 最大 `2000` |
| `search` | 空白 | 依名稱或 IP 搜尋 |
| `type` | 空白 | device type |
| `status` | 空白 | `online` 或 `offline` |

範例

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://nms.example.com/api/v1/integrations/network-snapshot?include=dashboard,devices,topology&limit=500"
```

此 API 不會回傳 `snmp_community` password license key RTSP credential 等敏感欄位

## 常用讀取 API

| Method | Endpoint | 說明 |
| --- | --- | --- |
| `GET` | `/api/v1/system/info` | 版本與系統資訊 |
| `GET` | `/api/v1/auth/me` | 目前使用者 |
| `GET` | `/api/v1/dashboard` | dashboard summary |
| `GET` | `/api/v1/devices?page=1&limit=50` | device list |
| `GET` | `/api/v1/devices/:id` | device detail |
| `GET` | `/api/v1/topology` | topology nodes and links |
| `GET` | `/api/v1/cameras/status` | camera module status |
| `GET` | `/api/v1/pdu/status` | PDU/UPS module status |
| `GET` | `/api/v1/iot/status` | IoT/Modbus status |
| `GET` | `/api/v1/iot/capabilities` | IoT profile list |

## iframe Embed

Admin 建立 scoped embed token

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

客戶系統嵌入

```html
<iframe
  src="https://nms.example.com/embed.html?view=dashboard&token=<embed-token>"
  style="width:100%;height:640px;border:0;"
  loading="lazy"
  referrerpolicy="no-referrer">
</iframe>
```

生產環境必須設定明確 iframe allowlist

```yaml
security:
  frame_ancestors:
    - "'self'"
    - "https://customer.example.com"
```

## IoT / Modbus / Gateway

| Protocol | Mode | Status | 整合方式 |
| --- | --- | --- | --- |
| `modbus_tcp` | direct poll | `Ready` | 建立 `/api/v1/iot/devices` 後呼叫 `/api/v1/iot/devices/:id/poll` |
| `rest` | gateway ingest | `Ready` | gateway 將正規化資料 POST 到 `/api/v1/iot/ingest` |
| `mqtt` | gateway ingest | `Bridge Ready` | 外部 MQTT bridge 訂閱後 POST 到 `/api/v1/iot/ingest` |
| `opcua` | gateway ingest | `Bridge Ready` | 外部 OPC-UA gateway POST 到 `/api/v1/iot/ingest` |
| `bacnet` | gateway ingest | `Bridge Ready` | 外部 BACnet/IP collector POST 到 `/api/v1/iot/ingest` |
| `mqtt_native` | direct broker subscription | `Planned` | 本版未提供內建 broker client |
| `opcua_native` | direct node polling | `Planned` | 本版未提供內建 OPC-UA client |
| `bacnet_native` | direct BACnet/IP polling | `Planned` | 本版未提供內建 BACnet client |

REST ingest 範例

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

## 寫入 API 邊界

寫入 API 只給可信任系統使用  並受 RBAC license gate audit log 管控

| Method | Endpoint | Role | 說明 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/devices` | editor | 建立 device |
| `PUT` | `/api/v1/devices/:id` | editor | 更新 device |
| `DELETE` | `/api/v1/devices/:id` | editor | 刪除 device |
| `POST` | `/api/v1/iot/ingest` | editor | 寫入 IoT measurement |
| `POST` | `/api/v1/iot/devices/:id/poll` | editor | poll Modbus TCP device |
| `POST` | `/api/v1/iot/devices` | admin | 建立 IoT/Modbus device |
| `PUT` | `/api/v1/integrations/settings` | admin | 更新 iframe allowlist |
| `DELETE` | `/api/v1/integrations/embed-tokens/:token_id` | admin | revoke embed token |

## 狀態碼

| Status | 意義 |
| --- | --- |
| `200` | 成功 |
| `201` | 已建立 |
| `400` | request body 或 query 無效 |
| `401` | token 缺失或無效 |
| `403` | role license feature gate 阻擋 |
| `404` | route 或 resource 不存在 |
| `409` | conflict |
| `500` | server error |

## 資安驗證 Gate

- API response log audit export 不得出現 SNMP community password RTSP credential license key API key long-lived token
- iframe 一律使用 scoped embed token  不使用 admin JWT
- `frame_ancestors` 必須限制為客戶明確網域
- OT/building device 必須放在 VLAN 或 ACL 保護網段
- 不可從公網直接暴露 Modbus TCP SNMP RTSP ONVIF BACnet OPC-UA
- MQTT OPC-UA BACnet gateway 需有來源 allowlist scoped token rate limit replay 防護與 audit log
- Windows 客戶端必須使用 `start_nms.bat` 或 `start_nms.ps1` 啟動

## 智慧建築 2016 / 2024 對齊

本版支援智慧建築整合證據與營運資料交換  不代表產品已取得智慧建築標章 ISO 認證或法規核可

官方參考

- 2016 智慧建築評估手冊: `https://www.abri.gov.tw/News_Content.aspx?n=20916&s=323060`
- 2024 智慧建築評估手冊: `https://www.abri.gov.tw/News_Content.aspx?n=20916&s=322421`
- 2024 實施公告: `https://www.moi.gov.tw/Common/EpaperClick.ashx?esq=%40EpaperSendQueueSN&p=D3C50BA14D2E597DBF5BA233B2670F24E65203E80CE503D13003C306C4ECC0A73685AEC600EFED6FBD32224A754AA75931B871A26B58CC04238D485186800BC2`

