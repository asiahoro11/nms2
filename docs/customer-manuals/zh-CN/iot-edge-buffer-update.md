# IoT Edge Buffer 同版本更新

版本: `v1.2.4.8`  
对象: 客户系统集成与 IoT / Modbus TCP 应用

## 功能摘要

`v1.2.4.8` 维持原版本号并加入 Modbus TCP 后台取值 本机暂存 断线续传与成功转抛后 10 分钟清除。

- NMS 可后台轮询已启用的 `modbus_tcp` 点位
- 支持 FC03 Holding Register 与 FC04 Input Register
- 支持 `uint16` `int16` `uint32` `int32` `float32`
- 支持 `metric` `poll_interval_seconds` `scale` `offset` `byte_order` `word_order`
- 每笔取值会先写入 NMS 本机 SQLite
- 客户 endpoint 断线或非 2xx 响应时资料保留在 NMS 主机
- 线路恢复后可自动续传或调用 `/api/v1/iot/queue/flush`
- 成功转抛后资料标记为 `sent` 保留 10 分钟后清除
- 客户系统只需接收 NMS HTTP forward payload 不需再个别连接 Modbus TCP 设备

## API

| Method | Endpoint | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/iot/queue/status` | 查询本机暂存与转抛状态 |
| `PUT` | `/api/v1/iot/forwarder/settings` | 设置客户 HTTP webhook 转抛端点 |
| `POST` | `/api/v1/iot/queue/flush` | 手动触发续传 |
| `POST` | `/api/v1/iot/queue/cleanup` | 清除已成功转抛且超过 10 分钟保留时间的资料 |

