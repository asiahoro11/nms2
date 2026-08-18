# Management Server API 手冊

版本：`v1.2.4.9sp00022`
對象：客戶系統整合  
Base path：`/api/v1`

## 整合就緒狀態

`v1.2.4.9sp00022` 延續既有 API 合約，並加入 IoT 多訊號、離線續傳與完整報表匯出能力。

| 狀態 | 意義 | 客戶端使用建議 |
| --- | --- | --- |
| `Ready` | 已由 NMS API 或執行期能力直接實作。 | 可用於客戶系統整合。 |
| `Bridge Ready` | NMS 提供穩定的資料匯入或嵌入合約，通訊協定轉接器由 NMS 外部 gateway 執行。 | gateway 將資料正規化後，可依文件 API 整合。 |
| `Planned` | 保留的 roadmap 項目，目前尚未提供穩定 runtime contract。 | 不建議用於正式環境整合。 |

## 開始串接前，先看這幾點

這份文件是給實際要串接 NMS 的工程師看的，下面幾點先說清楚，可以少走一些冤枉路：

- API 的根路徑是 `/api/v1`。文件中的 `/devices` 代表完整路徑
  `/api/v1/devices`，不要再多加一層 `/api/v1`。
- 除了登入、公開系統資訊與完整性狀態外，其他 API 都要帶 Bearer JWT。JWT 會過期，
  請在服務端安全保存，不要寫進瀏覽器 log、URL 或匯出檔。
- `viewer` 可以讀資料，`editor` 可以執行一般設備與 IoT 操作，`admin` 才能改使用者、
  License、系統設定、IoT 設備設定與拋轉設定。即使角色正確，沒有對應 License 時仍可能收到 `403`。
- 清單 API 請實作分頁，不要假設一次會回傳全部資料。報表和 measurement 的資料量可能很大，
  建議用日期、裝置或狀態條件縮小範圍。
- 時間欄位可能是 ISO 8601 或 Unix epoch millisecond；IoT 拋轉的 `sendTime` 固定是毫秒整數，
  不要當成秒使用。
- Modbus 的 register address、function code、byte order、word order、scale 與 offset 要一起保存，
  只保存一個「數值」通常不足以重現現場設備的讀值方式。
- `/iot/queue/status` 顯示的是 NMS 本機的 store-and-forward queue。目標主機暫時離線時資料會先留在本機，
  這是預期行為，不代表資料已經送達客戶系統。
- 收到 `423 Locked` 時，請先查看 `/api/v1/system/integrity/status`，不要一直重試寫入 API；
  這表示 NMS 正在保護資料庫，必須依離線復原流程處理。

## 1. 驗證

所有整合 API 使用 JSON，並採用標準回應 envelope：

```json
{
  "success": true,
  "data": {}
}
```

登入：

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

成功回應：

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

若啟用 2FA，登入回應會包含：

```json
{
  "requires_two_factor": true,
  "challenge_token": "<challenge-token>",
  "two_factor_method": "totp"
}
```

驗證 2FA：

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

已驗證請求：

```http
Authorization: Bearer <jwt>
Accept: application/json
```

## 2. 最佳化整合 API

`v1.2.4.9sp00022` 提供整合讀取 API，適合客戶 dashboard 或外部系統一次取得 inventory、topology 與 dashboard summary。

```http
GET /api/v1/integrations/network-snapshot
Authorization: Bearer <jwt>
```

此 API 可取代常見的多次呼叫：

```text
GET /api/v1/dashboard
GET /api/v1/devices
GET /api/v1/topology
```

### Query Parameters

| 名稱 | 預設值 | 說明 |
| --- | --- | --- |
| `include` | `dashboard,devices,topology` | 逗號分隔的區段。允許值：`dashboard`、`devices`、`topology`。 |
| `page` | `1` | 裝置清單頁碼。 |
| `limit` | `500` | 裝置清單每頁筆數，最大 `2000`。 |
| `search` | 空值 | 依裝置名稱或 IP 搜尋，套用於內嵌裝置清單。 |
| `type` | 空值 | 裝置類型過濾，例如 `switch`、`router`、`server`。 |
| `status` | 空值 | 裝置狀態過濾，可用 `online` 或 `offline`。 |

### 範例

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://nms.example.com/api/v1/integrations/network-snapshot?include=dashboard,devices,topology&limit=500"
```

整合 payload 會刻意排除敏感欄位，例如 `snmp_community`、密碼、license key 與 RTSP URL。

## 3. 既有讀取 API

以下 endpoint 仍可供需要較小範圍資料的客戶端使用。

| Method | Endpoint | 說明 |
| --- | --- | --- |
| `GET` | `/api/v1/system/info` | 執行版本與基本系統資訊，不需 token。 |
| `GET` | `/api/v1/auth/me` | 目前登入使用者。 |
| `GET` | `/api/v1/dashboard` | Dashboard summary。 |
| `GET` | `/api/v1/devices?page=1&limit=50` | 分頁裝置清單。 |
| `GET` | `/api/v1/devices/:id` | 裝置詳細資料。 |
| `GET` | `/api/v1/devices/:id/metrics` | 最新或歷史裝置 metrics。 |
| `GET` | `/api/v1/devices/:id/interfaces` | 裝置 interface 清單。 |
| `GET` | `/api/v1/devices/:id/events` | 裝置事件清單。 |
| `GET` | `/api/v1/topology` | Topology nodes 與 links。 |
| `GET` | `/api/v1/cameras/status` | 攝影機模組狀態。 |
| `GET` | `/api/v1/access-control/status` | 門禁模組狀態。 |
| `GET` | `/api/v1/pdu/status` | PDU/UPS 模組狀態。 |
| `GET` | `/api/v1/iot/status` | IoT/Modbus 整合狀態。 |
| `GET` | `/api/v1/iot/capabilities` | 支援的 IoT direct 與 gateway protocol profiles。 |
| `GET` | `/api/v1/iot/devices` | IoT/Modbus 裝置清單。 |
| `GET` | `/api/v1/iot/measurements?limit=100` | 最近 IoT measurement。 |
| `GET` | `/api/v1/iot/queue/status` | 本機 IoT store-and-forward queue 狀態。 |
| `GET` | `/api/v1/integrations/embed-snapshot?view=dashboard&token=<embed-token>` | iframe 頁面使用的 read-only payload。 |
| `GET` | `/api/v1/license/features` | 目前 license 允許的 feature flags。 |

## 4. iframe 嵌入整合

管理員建立 scoped embed token：

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

回應：

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

客戶系統 iframe 範例：

```html
<iframe
  src="https://nms.example.com/embed.html?view=dashboard&token=<embed-token>"
  style="width:100%;height:640px;border:0;"
  loading="lazy">
</iframe>
```

支援的 `view` 值：

| View | 說明 |
| --- | --- |
| `dashboard` | Dashboard widgets summary counters。 |
| `topology` | Topology node/link summary 與 node list。 |
| `devices` | 已清理敏感資訊的裝置 inventory。 |
| `alerts` | 最近系統事件。 |
| `iot` | IoT/Modbus 狀態與裝置最新值。 |

Embed token 為 read-only，且只允許指定 views。Server CSP 透過 `config.yaml` 的 `security.frame_ancestors` 允許 iframe 整合。新建立的 embed token 也會儲存在 server-side，管理員可列出與撤銷。

```yaml
security:
  frame_ancestors:
    - "'self'"
    - "https://customer.example.com"
```

管理 embed token：

```http
GET /api/v1/integrations/embed-tokens
Authorization: Bearer <admin-jwt>
```

```http
DELETE /api/v1/integrations/embed-tokens/<token_id>
Authorization: Bearer <admin-jwt>
```

Runtime iframe allowlist 管理：

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

允許的 `frame_ancestors` 值包含 `'self'`、`'none'`、`http:`、`https:`，以及精確的 `http://` 或 `https://` origin。

## 5. IoT / Modbus 整合 API

### Modbus TCP 裝置與背景輪詢

建立 Modbus TCP signal point。若是溫溼度感測器，建議每個 register 或 metric 建立一筆，即使兩筆資料指向同一個 Modbus TCP host。

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

NMS 會在背景自動輪詢已啟用的 `modbus_tcp` entry。也可手動 poll，用於 commissioning 與 troubleshooting：

```http
POST /api/v1/iot/devices/1/poll
Authorization: Bearer <editor-or-admin-jwt>
```

支援的 `data_type`：

| Type | Registers | 說明 |
| --- | --- | --- |
| `uint16` | 1 | Unsigned 16-bit integer。 |
| `int16` | 1 | Signed 16-bit integer。 |
| `uint32` | 2 | Unsigned 32-bit integer。 |
| `int32` | 2 | Signed 32-bit integer。 |
| `float32` | 2 | IEEE 754 float。 |

支援的 function codes：

| Function | 說明 |
| --- | --- |
| `3` | Holding Register FC03 |
| `4` | Input Register FC04 |

最終值計算方式：

```text
decoded_register_value * scale + offset
```

### 本機 Store-And-Forward Queue

每次背景 Modbus TCP 讀值與每筆 REST ingest payload 都會先寫入本機 SQLite。客戶系統可整合 NMS，而不需直接與每台 Modbus TCP 裝置通訊。

當 forward endpoint 離線或回傳非 2xx 狀態時，NMS 會將 measurements 保留在本機並稍後重試。Forward endpoint 恢復後，NMS 會從本機 queue 繼續送出。成功送出的 records 會標記為 `sent`，本機保留 10 分鐘後由背景 cleanup 移除。

讀取 queue 狀態：

```http
GET /api/v1/iot/queue/status
Authorization: Bearer <editor-or-admin-jwt>
```

設定 HTTP forwarder：

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

Token 儲存在 server-side，不會由 settings API 回傳。Forward payload 包含 `deviceId`、Unix epoch millisecond `sendTime` 與多訊號 `tagData`；穩定的樣本識別碼會放在 `X-NMS-Idempotency-Key` header。

強制 retry flush：

```http
POST /api/v1/iot/queue/flush
Authorization: Bearer <editor-or-admin-jwt>
```

強制清除已送出且已過期的 records：

```http
POST /api/v1/iot/queue/cleanup
Authorization: Bearer <admin-jwt>
```

### REST / Webhook Ingest

當 IoT gateway、MQTT bridge 或客戶系統要將數值推送到 NMS 時使用：

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

Ingest API 會依 `external_id` upsert 一筆 lightweight IoT device，並將 measurement 儲存在 `iot_measurements`。

支援的 IoT profiles：

```http
GET /api/v1/iot/capabilities
Authorization: Bearer <jwt>
```

| Protocol | Mode | Status | Integration path |
| --- | --- | --- | --- |
| `modbus_tcp` | direct poll | `Ready` | 設定 `/api/v1/iot/devices`；NMS 背景輪詢已啟用 entries 並寫入本機 queue records。 |
| `rest` | gateway ingest | `Ready` | 將正規化後的 values 推送到 `/api/v1/iot/ingest`；NMS 會寫入同一套本機 queue records。 |
| `http_forward` | store and forward | `Ready` | NMS 將 queued IoT records 轉送到客戶 webhook，支援 offline retry 與成功後 10 分鐘 cleanup。 |
| `mqtt` | gateway ingest | `Bridge Ready` | MQTT bridge 在外部訂閱後，將正規化 values POST 到 `/api/v1/iot/ingest`。 |
| `opcua` | gateway ingest | `Bridge Ready` | OPC-UA gateway 將 node values POST 到 `/api/v1/iot/ingest`。 |
| `bacnet` | gateway ingest | `Bridge Ready` | BACnet/IP collector 將 point values POST 到 `/api/v1/iot/ingest`。 |
| `mqtt_native` | direct broker subscription | `Planned` | 本版尚無內建 broker client contract，請使用 bridge ingest。 |
| `opcua_native` | direct node polling | `Planned` | 本版尚無內建 OPC-UA client contract，請使用 bridge ingest。 |
| `bacnet_native` | direct BACnet/IP polling | `Planned` | 本版尚無內建 BACnet client contract，請使用 bridge ingest。 |

## 6. Managed Integrations 寫入 API

寫入 API 僅建議提供給可信任整合，角色需為 editor 或 admin。所有寫入都會受到 license、RBAC 與 audit logging 管控。

| Method | Endpoint | Role | 說明 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/devices` | editor | 建立裝置。 |
| `PUT` | `/api/v1/devices/:id` | editor | 更新裝置。 |
| `DELETE` | `/api/v1/devices/:id` | editor | 刪除裝置。 |
| `POST` | `/api/v1/devices/bulk-scan` | editor | 掃描並新增裝置。 |
| `POST` | `/api/v1/topology/discover` | editor | 啟動 LLDP topology discovery。 |
| `PUT` | `/api/v1/topology/positions` | editor | 儲存 topology positions。 |
| `POST` | `/api/v1/iot/ingest` | editor | 推送 IoT measurement。 |
| `POST` | `/api/v1/iot/devices/:id/poll` | editor | Poll Modbus TCP 裝置。 |
| `POST` | `/api/v1/iot/queue/flush` | editor | 重試 queued IoT forwarding。 |
| `POST` | `/api/v1/iot/devices` | admin | 建立 IoT/Modbus 裝置。 |
| `PUT` | `/api/v1/iot/devices/:id` | admin | 更新 IoT/Modbus 裝置。 |
| `DELETE` | `/api/v1/iot/devices/:id` | admin | 刪除 IoT/Modbus 裝置。 |
| `GET` | `/api/v1/iot/forwarder/settings` | admin | 讀取 IoT forwarder settings，不回傳 bearer token。 |
| `PUT` | `/api/v1/iot/forwarder/settings` | admin | 更新 IoT forwarder settings。 |
| `POST` | `/api/v1/iot/queue/cleanup` | admin | 清除已送出且超過 10 分鐘保留時間的 IoT queue records。 |
| `GET` | `/api/v1/integrations/settings` | admin | 讀取 iframe 與 integration settings。 |
| `PUT` | `/api/v1/integrations/settings` | admin | 更新 iframe allowlist。 |
| `GET` | `/api/v1/integrations/embed-tokens` | admin | 列出已簽發 embed tokens。 |
| `DELETE` | `/api/v1/integrations/embed-tokens/:token_id` | admin | 撤銷 embed token。 |

## 7. 完整 API 分類目錄

本節列出目前 `router.go` 實際註冊的 API。除公開端點與完整性狀態端點外，均需
`Authorization: Bearer <jwt>`。`:id`、`:tokenId`、`:ifIndex` 與 `:backupId` 為
path parameter。角色欄位表示最低角色：`viewer`、`editor`、`admin`。

### 7.1 公開、驗證與系統

| Method | Path | 最低權限 | 用途 |
|---|---|---|---|
| POST | `/auth/login` | public | 登入並取得 JWT 或 2FA challenge。 |
| POST | `/auth/verify-2fa` | public | 完成 TOTP 2FA 登入。 |
| POST | `/auth/forgot-password` | public | 申請密碼重設。 |
| POST | `/auth/reset-password` | public | 使用 reset token 設定新密碼。 |
| GET | `/system/info` | public | 版本與系統基本資訊。 |
| GET | `/branding` | public | 讀取品牌設定。 |
| GET | `/system/config` | public | 讀取公開系統設定。 |
| POST | `/license/debug` | public/debug | License debug 查詢；正式環境應停用。 |
| GET | `/system/integrity/status` | public | 完整性封鎖狀態；封鎖時仍可讀取。 |

### 7.2 使用者、Dashboard、授權與日誌

| Method | Path | 最低權限 | 用途 |
|---|---|---|---|
| POST | `/auth/logout` | viewer | 登出。 |
| GET | `/auth/me` | viewer | 目前登入者。 |
| POST | `/auth/change-password` | viewer | 變更自己的密碼。 |
| GET | `/auth/2fa/status` | viewer | 讀取 2FA 狀態。 |
| POST | `/auth/2fa/enroll` | viewer | 建立 TOTP enrollment。 |
| POST | `/auth/2fa/confirm` | viewer | 確認 TOTP enrollment。 |
| POST | `/auth/2fa/disable` | viewer | 停用自己的 2FA。 |
| POST | `/auth/2fa/recovery-codes/regenerate` | viewer | 重新產生 recovery codes。 |
| GET | `/dashboard` | viewer | Dashboard summary。 |
| GET | `/dashboard/top-cpu` | viewer | CPU 使用率排行。 |
| GET | `/dashboard/top-memory` | viewer | Memory 使用率排行。 |
| GET | `/license/status` | viewer | License 狀態。 |
| GET | `/license/features` | viewer | Feature flags。 |
| GET | `/alerts/settings` | viewer | 告警設定。 |
| GET | `/events` | viewer | 系統事件。 |
| GET | `/logs` | viewer | 聚合日誌。 |
| GET | `/system-logs` | viewer | 系統日誌。 |
| GET | `/device-logs` | viewer | 裝置日誌。 |
| GET | `/config-change-logs` | viewer | 設定異動日誌。 |
| GET | `/log-center/options` | viewer | 日誌中心篩選選項。 |
| GET | `/log-center/export` | viewer | 匯出日誌中心資料。 |
| GET | `/log-center/evidence-bundle` | viewer | 匯出稽核證據包。 |
| PUT | `/audit-logs/:id/review` | viewer | 審閱 audit log。 |
| PUT | `/system-logs/:id/review` | viewer | 審閱 system log。 |
| PUT | `/config-change-logs/:id/review` | viewer | 審閱 config-change log。 |
| PUT | `/device-logs/:id/ack` | viewer | 確認裝置日誌。 |
| GET | `/notifications` | viewer | 通知清單。 |
| PUT | `/notifications/read-all` | viewer | 全部標記已讀。 |
| PUT | `/notifications/:id/read` | viewer | 單筆標記已讀。 |
| DELETE | `/notifications/all` | viewer | 清除全部通知。 |
| DELETE | `/notifications/:id` | viewer | 刪除單筆通知。 |

### 7.3 網路裝置、拓撲與操作

| Method | Path | 最低權限 | 用途 |
|---|---|---|---|
| GET | `/devices` | viewer | 分頁裝置清單。 |
| GET | `/devices/:id` | viewer | 裝置詳細資料。 |
| GET | `/devices/:id/metrics` | viewer | 裝置 metrics。 |
| GET | `/devices/:id/interfaces` | viewer | 介面清單。 |
| GET | `/devices/:id/traffic` | viewer | 介面流量。 |
| GET | `/devices/:id/events` | viewer | 裝置事件。 |
| GET | `/topology` | viewer | 拓撲 nodes 與 links。 |
| GET | `/topology/device/:id/interfaces` | viewer | 拓撲連線介面。 |
| POST | `/devices` | editor | 建立裝置。 |
| PUT | `/devices/:id` | editor | 更新裝置。 |
| DELETE | `/devices/:id` | editor | 刪除裝置。 |
| POST | `/devices/:id/image` | editor | 上傳裝置圖片。 |
| POST | `/devices/:id/poll` | editor | 立即輪詢裝置。 |
| POST | `/devices/bulk-scan` | editor | 批次掃描裝置。 |
| POST | `/devices/bulk-delete` | editor | 批次刪除裝置。 |
| POST | `/devices/bulk-update` | editor | 批次更新裝置。 |
| POST | `/topology/links` | editor | 建立拓撲連線。 |
| PUT | `/topology/links/:id` | editor | 更新拓撲連線。 |
| DELETE | `/topology/links/:id` | editor | 刪除拓撲連線。 |
| PUT | `/topology/positions` | editor | 更新拓撲位置。 |
| POST | `/topology/discover` | editor | 啟動拓撲探索。 |
| POST | `/devices/:id/reboot` | editor | 重啟裝置。 |
| POST | `/devices/:id/backup` | editor | 建立設定備份。 |
| GET | `/devices/:id/backups` | editor | 列出設定備份。 |
| GET | `/devices/:id/backups/:backupId/download` | editor | 下載設定備份。 |
| GET | `/devices/:id/poe` | editor | PoE 狀態。 |
| GET | `/devices/:id/ap` | editor | AP 狀態。 |
| POST | `/devices/:id/poe/:portIndex/action` | editor | 執行 PoE port 動作。 |
| POST | `/devices/:id/interfaces/:ifIndex/status` | editor | 設定介面啟用狀態。 |

### 7.4 報表匯出

所有報表端點最低為 `editor`，支援依 endpoint 回傳 CSV、PDF 或 JSON；查詢參數由各報表 handler 驗證。

`GET /reports/devices`、`/reports/devices/pdf`、`/reports/logs`、`/reports/logs/pdf`、
`/reports/traffic`、`/reports/health`、`/reports/availability`、`/reports/inventory`、
`/reports/interfaces`、`/reports/health-trend`、`/reports/sla`、`/reports/audit`、
`/reports/license-capacity`、`/reports/cameras`、`/reports/pdu`、`/reports/access-control`、
`/reports/topology`、`/reports/events`、`/reports/syslog`、`/reports/notifications`、
`/reports/iot-devices`、`/reports/iot-measurements`、`/reports/camera-recordings`、
`/reports/access-events`、`/reports/access-cards`、`/reports/access-schedules`、
`/reports/config-backups`。

### 7.5 IoT、Modbus 與離線續傳

| Method | Path | 最低權限 | 用途 |
|---|---|---|---|
| GET | `/iot/status` | viewer | IoT 模組狀態。 |
| GET | `/iot/capabilities` | viewer | 通訊協定與能力。 |
| GET | `/iot/devices` | viewer | IoT 裝置清單。 |
| GET | `/iot/measurements` | viewer | measurement 查詢。 |
| GET | `/iot/queue/status` | viewer | store-and-forward queue 狀態。 |
| POST | `/iot/ingest` | editor | 接收 gateway／REST／MQTT bridge 正規化訊號。 |
| POST | `/iot/devices/:id/poll` | editor | 立即輪詢 IoT 裝置。 |
| POST | `/iot/queue/flush` | editor | 立即重送 queue。 |
| POST | `/iot/devices` | admin | 建立 IoT 裝置與多筆 signal。 |
| PUT | `/iot/devices/:id` | admin | 更新 IoT 裝置與 signals。 |
| DELETE | `/iot/devices/:id` | admin | 刪除 IoT 裝置。 |
| GET | `/iot/forwarder/settings` | admin | 讀取拋轉設定；不回傳 token。 |
| PUT | `/iot/forwarder/settings` | admin | 設定目標 URL、batch 與 `interval_ms`。 |
| POST | `/iot/queue/cleanup` | admin | 清理已成功拋轉資料。 |

### 7.6 整合、使用者、License 與系統管理

整合：`GET /integrations/network-snapshot`、`GET /integrations/embed-snapshot`、
`GET /integrations/settings`、`PUT /integrations/settings`、
`GET /integrations/embed-tokens`、`POST /integrations/embed-tokens`、
`DELETE /integrations/embed-tokens/:tokenId`。前兩個為讀取端點，設定與 token 管理由 `admin` 控制。

使用者與 License：`GET/POST/PUT/DELETE /users`（admin）、`GET /audit-logs`、
`GET /audit-logs/actions`、`GET /license/machine-id`、`POST /license/activate`、
`GET/POST /licenses`、`POST /license/reset`、`POST /license/reissue`。

系統管理：`GET /system/backup`、`GET /system/restore-readiness`、
`POST /system/restore`、`POST /system/backup/encrypted`、`POST /system/restore/encrypted`、
`POST /system/encryption/password`、`GET /system/encryption/status`、
`POST /alerts/settings`、`PUT /alerts/settings/:type`、`POST /alerts/test/:type`、
`POST /tools/ping`、`POST /tools/traceroute`、`PUT /branding`、
`POST /branding/logo`、`DELETE /branding/logo`、`GET /system/host-status`、
`GET/PUT /security/settings`、`PUT /system/config/:key`；以上均為 admin。

### 7.7 攝影機、門禁、PDU 與終端機

攝影機：admin 可 `POST /cameras`、`POST /cameras/onvif/probe`、`POST /cameras/discover`、
`POST /cameras/bulk-scan`、`PUT /cameras/batch/credentials`、`PUT/DELETE /cameras/:id`、
`POST /cameras/:id/health`、`GET/PUT /nvr/config`；editor 可讀取 `/cameras`、
`/cameras/monitor`、`/cameras/:id`、`/cameras/recording/status`，管理錄影與 `/recordings*`。
已登入 viewer 可讀取 `/cameras/status`、`/cameras/:id/snapshot` 與 MJPEG stream。

門禁：已登入 viewer 可讀取 `/access-control/status`、`/access-control/events`；editor 可讀取
`/access-control/doors`、`/access-control/doors/:id`、`/access-control/cards`；admin 管理 doors、cards、events、
`/access-control/schedules` 及 `/access-control/schedules/:id/approve`、`/check`。

PDU：已登入 viewer 可讀取 `/pdu/status`；editor 管理 `/pdu/devices`、`/pdu/devices/:id` 與 poll。
WebSSH：`GET /devices/:id/terminal` 使用已登入使用者的 editor 權限與 WebSocket upgrade。

## 8. HTTP Status Codes

| Status | 意義 |
| --- | --- |
| `200` | 成功。 |
| `201` | Resource 已建立。 |
| `400` | Request body 或 query parameter 無效。 |
| `401` | Token 缺失或無效。 |
| `403` | Role、license 或 feature gate 阻擋請求。 |
| `404` | Route 或 resource 不存在。 |
| `409` | Conflict，例如重複的 device IP。 |
| `423` | 完整性封鎖啟用，所有應用程式 API 暫停。 |
| `429` | 登入或請求頻率超過限制。 |
| `500` | Server-side error。 |

## 9. 整合建議

- 客戶部署環境建議使用 HTTPS。
- 每套客戶系統使用一組專用 integration account。
- JWT 僅存放於 server-side 或安全儲存區。
- iframe 頁面請使用 embed token，不要把 admin JWT 放在 iframe URL。
- Dashboard 類型頁面建議使用 `/api/v1/integrations/network-snapshot` refresh。
- IoT gateway push flow 使用 `/api/v1/iot/ingest`；LAN 內直連裝置使用 Modbus TCP device polling。
- 使用者選取單一裝置後，再使用較小的既有 API 做 drill-down。
- Client 建議檢查 `schema_version`，以便未來 payload 調整時能安全處理。
- 不要依賴未文件化的 database columns。API payload 才是對外合約。

## 10. 資安驗證 Gate

客戶部署 go-live 前，至少應完成以下驗證：

- Device、PDU、camera、log、audit、export 與 integration responses 不得暴露 SNMP communities、密碼、RTSP credentials、license keys、API keys 或 long-lived tokens。
- 嵌入頁面使用 scoped iframe embed tokens，不要使用 admin JWT。
- iframe `frame_ancestors` 應限制為明確的客戶 domain，正式環境不要使用廣泛的 `http:` 或 `https:` allowlists。
- OT 與 building devices 應放在分段 VLAN 或 ACL 保護網路中，不要將 Modbus TCP、SNMP、RTSP、ONVIF、BACnet 或 OPC-UA ports 直接暴露到 public internet。
- MQTT、OPC-UA 與 BACnet gateways 在 POST 到 `/api/v1/iot/ingest` 前，應落實 source allowlists、scoped tokens、rate limits、replay protection 與 audit logging。
- Windows 部署請使用 `start_nms.bat` 或 `start_nms.ps1`，不要直接啟動 `nms_server.exe`。

## 11. 智慧建築 2016 / 2024 對齊

本版本支援智慧建築整合佐證與營運資料交換；但不代表產品本身已取得智慧建築標章、ISO 認證或法定核可。

客戶專案團隊應依內政部 / 建築研究所官方資料確認專案合規：

- 2016 智慧建築評估手冊：`https://www.abri.gov.tw/News_Content.aspx?n=20916&s=323060`
- 2024 智慧建築評估手冊：`https://www.abri.gov.tw/News_Content.aspx?n=20916&s=322421`
- 2024 實施公告：`https://www.moi.gov.tw/Common/EpaperClick.ashx?esq=%40EpaperSendQueueSN&p=D3C50BA14D2E597DBF5BA233B2670F24E65203E80CE503D13003C306C4ECC0A73685AEC600EFED6FBD32224A754AA75931B871A26B58CC04238D485186800BC2`

NMS 可作為客戶佐證資料的對齊項目：

- Network 與 device inventory 可支援通訊與營運佐證。
- Audit logs、config-change logs 與 export bundles 可支援可追溯性佐證。
- IoT direct Modbus TCP polling 與 REST ingest 可支援建築設備資料蒐集。
- iframe embed 與 integration APIs 可支援第三方 BMS、EMS 或智慧建築 dashboard 整合。
