# IoT Edge Buffer 同版更新

版本: `v1.2.4.8`  
對象: 客戶系統整合與 IoT / Modbus TCP 應用

## 功能摘要

`v1.2.4.8` 維持原版本號並加入 Modbus TCP 背景取值 本機暫存 斷線續傳與成功轉拋後 10 分鐘清除。

- NMS 可背景輪詢已啟用的 `modbus_tcp` 點位
- 支援 FC03 Holding Register 與 FC04 Input Register
- 支援 `uint16` `int16` `uint32` `int32` `float32`
- 支援 `metric` `poll_interval_seconds` `scale` `offset` `byte_order` `word_order`
- 每筆取值會先寫入 NMS 本機 SQLite
- 客戶 endpoint 斷線或非 2xx 回應時資料保留在 NMS 主機
- 線路恢復後可自動續傳或呼叫 `/api/v1/iot/queue/flush`
- 成功轉拋後資料標記為 `sent` 保留 10 分鐘後清除
- 客戶系統只需接收 NMS HTTP forward payload 不需再個別連 Modbus TCP 設備

## API

| Method | Endpoint | 說明 |
| --- | --- | --- |
| `GET` | `/api/v1/iot/queue/status` | 查詢本機暫存與轉拋狀態 |
| `PUT` | `/api/v1/iot/forwarder/settings` | 設定客戶 HTTP webhook 轉拋端點 |
| `POST` | `/api/v1/iot/queue/flush` | 手動觸發續傳 |
| `POST` | `/api/v1/iot/queue/cleanup` | 清除已成功轉拋且超過 10 分鐘保留時間的資料 |

