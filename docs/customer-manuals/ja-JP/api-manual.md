# Management Server API マニュアル

バージョン: `v1.2.4.8`  
対象: 顧客システム連携  
Base path: `/api/v1`

## 連携成熟度

| 状態 | 意味 | 顧客利用 |
| --- | --- | --- |
| `Ready` | NMS が直接 API または runtime 機能を提供 | 本番連携に利用可能 |
| `Bridge Ready` | NMS は安定した ingest または embed 契約を提供し  protocol adapter は外部 gateway で実行 | gateway がデータを正規化する場合に利用可能 |
| `Planned` | ロードマップ項目  本リリースでは安定した runtime contract なし | 本番連携には使用しない |

## 認証

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

成功後は次の header を使用します

```http
Authorization: Bearer <jwt>
Accept: application/json
```

2FA が有効な場合は次も呼び出します

```http
POST /api/v1/auth/verify-2fa
Content-Type: application/json
```

## 最適化連携 API

顧客 dashboard は まず snapshot API を使用し API 呼び出し回数を減らすことを推奨します

```http
GET /api/v1/integrations/network-snapshot
Authorization: Bearer <jwt>
```

Query parameters

| Name | Default | 説明 |
| --- | --- | --- |
| `include` | `dashboard,devices,topology` | `dashboard` `devices` `topology` を指定可能 |
| `page` | `1` | device page |
| `limit` | `500` | 最大 `2000` |
| `search` | empty | 名前または IP で検索 |
| `type` | empty | device type |
| `status` | empty | `online` または `offline` |

例

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://nms.example.com/api/v1/integrations/network-snapshot?include=dashboard,devices,topology&limit=500"
```

この API は `snmp_community` password license key RTSP credential などの機密フィールドを返しません

## 主な読み取り API

| Method | Endpoint | 説明 |
| --- | --- | --- |
| `GET` | `/api/v1/system/info` | バージョンとシステム情報 |
| `GET` | `/api/v1/auth/me` | 現在のユーザー |
| `GET` | `/api/v1/dashboard` | dashboard summary |
| `GET` | `/api/v1/devices?page=1&limit=50` | device list |
| `GET` | `/api/v1/devices/:id` | device detail |
| `GET` | `/api/v1/topology` | topology nodes and links |
| `GET` | `/api/v1/cameras/status` | camera module status |
| `GET` | `/api/v1/pdu/status` | PDU/UPS module status |
| `GET` | `/api/v1/iot/status` | IoT/Modbus status |
| `GET` | `/api/v1/iot/capabilities` | IoT profile list |

## iframe Embed

Admin が scoped embed token を作成します

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

顧客システムでの iframe 例

```html
<iframe
  src="https://nms.example.com/embed.html?view=dashboard&token=<embed-token>"
  style="width:100%;height:640px;border:0;"
  loading="lazy"
  referrerpolicy="no-referrer">
</iframe>
```

本番環境では明示的な iframe allowlist が必要です

```yaml
security:
  frame_ancestors:
    - "'self'"
    - "https://customer.example.com"
```

## IoT / Modbus / Gateway

| Protocol | Mode | Status | 連携方法 |
| --- | --- | --- | --- |
| `modbus_tcp` | direct poll | `Ready` | `/api/v1/iot/devices` を作成後 `/api/v1/iot/devices/:id/poll` を呼び出す |
| `rest` | gateway ingest | `Ready` | gateway が正規化データを `/api/v1/iot/ingest` に POST |
| `mqtt` | gateway ingest | `Bridge Ready` | 外部 MQTT bridge が購読し `/api/v1/iot/ingest` に POST |
| `opcua` | gateway ingest | `Bridge Ready` | 外部 OPC-UA gateway が `/api/v1/iot/ingest` に POST |
| `bacnet` | gateway ingest | `Bridge Ready` | 外部 BACnet/IP collector が `/api/v1/iot/ingest` に POST |
| `mqtt_native` | direct broker subscription | `Planned` | 本リリースに内蔵 broker client は含まれません |
| `opcua_native` | direct node polling | `Planned` | 本リリースに内蔵 OPC-UA client は含まれません |
| `bacnet_native` | direct BACnet/IP polling | `Planned` | 本リリースに内蔵 BACnet client は含まれません |

REST ingest 例

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

## 書き込み API の境界

書き込み API は信頼済みシステム専用であり RBAC license gate audit log の対象です

| Method | Endpoint | Role | 説明 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/devices` | editor | device 作成 |
| `PUT` | `/api/v1/devices/:id` | editor | device 更新 |
| `DELETE` | `/api/v1/devices/:id` | editor | device 削除 |
| `POST` | `/api/v1/iot/ingest` | editor | IoT measurement 書き込み |
| `POST` | `/api/v1/iot/devices/:id/poll` | editor | Modbus TCP device poll |
| `POST` | `/api/v1/iot/devices` | admin | IoT/Modbus device 作成 |
| `PUT` | `/api/v1/integrations/settings` | admin | iframe allowlist 更新 |
| `DELETE` | `/api/v1/integrations/embed-tokens/:token_id` | admin | embed token revoke |

## ステータスコード

| Status | 意味 |
| --- | --- |
| `200` | 成功 |
| `201` | 作成済み |
| `400` | request body または query が不正 |
| `401` | token なし または無効 |
| `403` | role license feature gate により拒否 |
| `404` | route または resource が存在しない |
| `409` | conflict |
| `500` | server error |

## セキュリティ検証 Gate

- API response log audit export に SNMP community password RTSP credential license key API key long-lived token を含めない
- iframe は scoped embed token を使用し admin JWT を使用しない
- `frame_ancestors` は顧客の明示ドメインに限定する
- OT/building device は VLAN または ACL 保護ネットワークに配置する
- Modbus TCP SNMP RTSP ONVIF BACnet OPC-UA を public internet に直接公開しない
- MQTT OPC-UA BACnet gateway は source allowlist scoped token rate limit replay protection audit log が必要
- Windows は `start_nms.bat` または `start_nms.ps1` で起動する

## スマートビルディング 2016 / 2024 整合

本リリースはスマートビルディング連携証跡と運用データ交換を支援します  製品自体がスマートビルディング標章 ISO 認証または法的承認を取得したことを意味しません

公式参考

- 2016 Smart Building Evaluation Manual: `https://www.abri.gov.tw/News_Content.aspx?n=20916&s=323060`
- 2024 Smart Building Evaluation Manual: `https://www.abri.gov.tw/News_Content.aspx?n=20916&s=322421`
- 2024 implementation notice: `https://www.moi.gov.tw/Common/EpaperClick.ashx?esq=%40EpaperSendQueueSN&p=D3C50BA14D2E597DBF5BA233B2670F24E65203E80CE503D13003C306C4ECC0A73685AEC600EFED6FBD32224A754AA75931B871A26B58CC04238D485186800BC2`

