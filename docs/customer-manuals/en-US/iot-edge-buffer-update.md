# IoT Edge Buffer Same-Version Update

Version: `v1.2.4.8`  
Audience: customer system integration and IoT / Modbus TCP applications

## Summary

`v1.2.4.8` keeps the same version number and adds Modbus TCP background collection, local buffering, offline resume, and cleanup 10 minutes after successful forwarding.

- NMS polls enabled `modbus_tcp` points in the background.
- FC03 Holding Register and FC04 Input Register are supported.
- Supported data types are `uint16`, `int16`, `uint32`, `int32`, and `float32`.
- Supported point fields include `metric`, `poll_interval_seconds`, `scale`, `offset`, `byte_order`, and `word_order`.
- Each reading is written to local SQLite first.
- If the customer endpoint is offline or returns a non-2xx response, records remain on the NMS host.
- After recovery, records resume automatically or through `/api/v1/iot/queue/flush`.
- Successfully forwarded records are marked `sent`, kept for 10 minutes, then cleaned up.
- Customer systems can receive normalized NMS HTTP forward payloads without connecting to each Modbus TCP device.

## APIs

| Method | Endpoint | Description |
| --- | --- | --- |
| `GET` | `/api/v1/iot/queue/status` | Local queue and forwarding status |
| `PUT` | `/api/v1/iot/forwarder/settings` | Customer HTTP webhook forwarder settings |
| `POST` | `/api/v1/iot/queue/flush` | Manually retry queued delivery |
| `POST` | `/api/v1/iot/queue/cleanup` | Drop sent records after the 10-minute hold window |

