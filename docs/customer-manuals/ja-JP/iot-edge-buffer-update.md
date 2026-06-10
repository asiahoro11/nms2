# IoT Edge Buffer 同一バージョン更新

バージョン: `v1.2.4.8`  
対象: customer system integration と IoT / Modbus TCP applications

## 概要

`v1.2.4.8` のバージョン番号は維持し Modbus TCP のバックグラウンド収集 local buffering offline resume 成功送信後 10 分 cleanup を追加しました。

- NMS は有効な `modbus_tcp` point をバックグラウンドで polling できます
- FC03 Holding Register と FC04 Input Register をサポートします
- `uint16` `int16` `uint32` `int32` `float32` をサポートします
- `metric` `poll_interval_seconds` `scale` `offset` `byte_order` `word_order` を設定できます
- 各 reading は先に NMS ローカル SQLite に保存されます
- customer endpoint が offline または non-2xx の場合 record は NMS host に保持されます
- 回線復旧後 自動 retry または `/api/v1/iot/queue/flush` で再送できます
- 成功送信後 record は `sent` になり 10 分保持後に削除されます
- customer system は各 Modbus TCP device に直接接続せず NMS HTTP forward payload を受け取れます

## API

| Method | Endpoint | 説明 |
| --- | --- | --- |
| `GET` | `/api/v1/iot/queue/status` | local queue と forwarding status |
| `PUT` | `/api/v1/iot/forwarder/settings` | customer HTTP webhook forwarder settings |
| `POST` | `/api/v1/iot/queue/flush` | queue delivery retry |
| `POST` | `/api/v1/iot/queue/cleanup` | 10 分保持後の sent record cleanup |

