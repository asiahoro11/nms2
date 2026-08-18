# IoT 拋轉 JSON 格式

## 拋轉設定表單

將 [`iot-forwarder-settings.json`](./iot-forwarder-settings.json) 的內容送至：

```http
PUT /api/v1/iot/forwarder/settings
Content-Type: application/json
Authorization: Bearer <NMS JWT>
```

欄位說明：

| 欄位 | 類型 | 說明 |
|---|---|---|
| `enabled` | boolean | 是否啟用離線續傳與定期拋轉 |
| `url` | string | 接收主機的完整 HTTP/HTTPS URL，包含 IP、Port 與 Path |
| `token` | string | 選填；NMS 拋轉時使用的 Bearer Token。設定後不會由查詢 API 回傳 |
| `clear_token` | boolean | 設為 `true` 時清除已儲存的拋轉 Token |
| `batch_size` | integer | 每次從本機佇列取出的最大資料筆數，範圍 1–500 |
| `interval_ms` | integer | 定期拋轉間隔，單位毫秒，範圍 100–86400000 |

## 實際訊號封包

NMS 會針對同一台設備、同一個取樣批次，把多個暫存器訊號合併成
[`iot-forward-payload.json`](./iot-forward-payload.json) 的格式，並逐筆 HTTP POST 至設定的 `url`。

```http
POST <configured url>
Content-Type: application/json
Authorization: Bearer <configured token>
X-NMS-Idempotency-Key: <stable sample id>
```

| 欄位 | 類型 | 說明 |
|---|---|---|
| `deviceId` | string | 設備外部 ID／UUID，例如 `DEV001` |
| `sendTime` | integer | 訊號取樣時間，Unix epoch milliseconds |
| `tagData` | object | 訊號代碼對應數值；一台設備可在同一封包包含多個暫存器訊號 |

接收端回傳 HTTP 2xx 才視為成功。斷線或非 2xx 時，資料會保留在 NMS
本機 SQLite 佇列，待連線恢復後續傳。
