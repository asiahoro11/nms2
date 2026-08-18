# 通用 IoT 介面與 API 開發排程

## 目標

建立 transport、device、point 與 measurement 分離的通用模型，使同一套
IoT UI 與 API 可支援：

- Modbus TCP
- Modbus RTU over TCP
- Modbus RTU／RS232／RS485
- REST／Webhook gateway
- MQTT gateway
- 後續 OPC-UA、BACnet 與廠商 driver adapter

「相容所有設備」定義為：未來設備只需增加 transport／driver adapter，
不再修改核心設備、訊號、歷史資料與拋轉 API。v1 API 在遷移期間保持可用。

## 開發排程

| 階段 | 日期 | 交付內容 | 完成條件 |
|---|---|---|---|
| 0. 契約定版 | 2026-07-24～2026-07-29 | Canonical device／transport／point／sample schema、錯誤碼、版本策略 | API contract 與 migration 文件審核完成 |
| 1. 通用資料模型 | 2026-07-30～2026-08-07 | `iot_transports`、`iot_devices_v2`、`iot_points`、driver capability registry | v1 資料可冪等遷移且可回滾 |
| 2. Transport adapters | 2026-08-08～2026-08-21 | Modbus TCP、RTU over TCP、serial RTU／RS485 adapter；共用連線池、timeout、retry | 同一 point model 通過四種 transport 測試 |
| 3. 通用 API v2 | 2026-08-22～2026-09-04 | Device、point、capability、ingest、measurement、batch sample API | OpenAPI contract test 與 idempotency 測試通過 |
| 4. 通用 IoT UI | 2026-09-05～2026-09-18 | 依 capability 動態顯示欄位；設備與多訊號編輯器；v1 相容模式 | 不再以協定硬編碼欄位，既有設備可編輯 |
| 5. 相容與認證 | 2026-09-19～2026-09-30 | 壓力、斷線續傳、endianness、bit/register、廠商樣機驗證 | v1 regression、200 devices soak test、交付文件完成 |

## Canonical API 草案

```text
GET    /api/v2/iot/capabilities
GET    /api/v2/iot/transports
POST   /api/v2/iot/transports
GET    /api/v2/iot/devices
POST   /api/v2/iot/devices
GET    /api/v2/iot/devices/{id}/points
PUT    /api/v2/iot/devices/{id}/points
POST   /api/v2/iot/samples:ingest
GET    /api/v2/iot/measurements
GET    /api/v2/iot/queue/status
POST   /api/v2/iot/queue:flush
```

通用 sample：

```json
{
  "schemaVersion": "nms.iot.sample.v2",
  "deviceId": "DEV001",
  "sampleId": "DEV001-1781231400000",
  "timestampMs": 1781231400000,
  "points": [
    {
      "key": "chwSupplyTempC",
      "value": 7.0,
      "valueType": "number",
      "unit": "C",
      "quality": "good"
    }
  ]
}
```

`valueType` 預計支援 `number`、`integer`、`boolean`、`string`、`enum`、
`bytes`；每個 point 保存 unit、quality、scale、offset 與原始值。

## 相容原則

- `/api/v1/iot/*` 保持不變，由 compatibility adapter 對接 v2 service。
- 現行 `{deviceId, sendTime, tagData}` 拋轉契約保持不變，另提供可選 v2
  canonical sample。
- UI 階段前不改現有 v1 版面，降低本版變更範圍。
- Transport credentials 與 serial／TCP 設定分開保存，API 不回傳秘密。
- Driver 不得直接寫資料表，只能透過 point/sample service。
