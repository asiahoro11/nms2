# Management Server Release Notes

버전: `v1.2.4.8`  
날짜: `2026-06-10`

## 릴리스 포지션

`v1.2.4.8` 은 API iframe embed IoT gateway 연동과 보안 강화를 위한 고객 연동 안정화 릴리스입니다

## 주요 변경

- API manual 을 `v1.2.4.8` 로 갱신
- `Ready` `Bridge Ready` `Planned` 명확화
- `/api/v1/integrations/network-snapshot` 을 고객 dashboard bootstrap API 로 정의
- iframe embed token 생성 목록 revoke 만료 확인 지원
- IoT direct Modbus TCP 와 REST ingest 는 `Ready`
- MQTT OPC-UA BACnet 은 `Bridge Ready`

## 보안 강화

- devices / PDU API 는 `snmp_community` 반환하지 않음
- 빈 SNMP community 업데이트는 기존 값 유지
- camera RTSP URL 은 credential 과 민감 query 를 redaction
- audit/log detail 은 password token api key secret authorization SNMP community redaction
- iframe 과 IoT UI 에 token HTTPS allowlist OT network isolation 안내 추가

## UI

- audit admin embed IoT table 긴 문자열 overflow 수정
- mobile tablet table 은 제어된 가로 스크롤 사용
- monitor 이름은 `電視牆模式`

## 패키지

- Windows amd64: `artifacts/windows/v1.2.4.8.zip`
- Linux amd64 + arm64: `artifacts/linux/v1.2.4.8.zip`
- Linux arm64: `artifacts/linux-arm64/v1.2.4.8.zip`

## 검증 완료

- backend `go test ./...` 통과
- frontend `node --check` 통과
- static mirror synced
- Windows package 는 `start_nms.bat` 로 시작하고 `v1.2.4.8` 반환
- Linux arm64 binary 는 AArch64 ELF 로 확인됨

