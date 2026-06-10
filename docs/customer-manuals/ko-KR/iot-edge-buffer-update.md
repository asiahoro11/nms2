# IoT Edge Buffer 동일 버전 업데이트

버전: `v1.2.4.8`  
대상: customer system integration 과 IoT / Modbus TCP applications

## 요약

`v1.2.4.8` 버전 번호는 유지하고 Modbus TCP background collection local buffering offline resume 성공 전송 후 10 분 cleanup 을 추가했습니다.

- NMS 는 활성화된 `modbus_tcp` point 를 background polling 할 수 있습니다
- FC03 Holding Register 와 FC04 Input Register 를 지원합니다
- `uint16` `int16` `uint32` `int32` `float32` 를 지원합니다
- `metric` `poll_interval_seconds` `scale` `offset` `byte_order` `word_order` 설정을 지원합니다
- 각 reading 은 먼저 NMS local SQLite 에 저장됩니다
- customer endpoint 가 offline 이거나 non-2xx 를 반환하면 record 는 NMS host 에 유지됩니다
- 회선 복구 후 자동 retry 또는 `/api/v1/iot/queue/flush` 로 재전송할 수 있습니다
- 성공 전송 후 record 는 `sent` 로 표시되고 10 분 보관 후 삭제됩니다
- customer system 은 각 Modbus TCP device 에 직접 접속하지 않고 NMS HTTP forward payload 를 받을 수 있습니다

## API

| Method | Endpoint | 설명 |
| --- | --- | --- |
| `GET` | `/api/v1/iot/queue/status` | local queue 와 forwarding status |
| `PUT` | `/api/v1/iot/forwarder/settings` | customer HTTP webhook forwarder settings |
| `POST` | `/api/v1/iot/queue/flush` | queue delivery retry |
| `POST` | `/api/v1/iot/queue/cleanup` | 10 분 hold 이후 sent record cleanup |

