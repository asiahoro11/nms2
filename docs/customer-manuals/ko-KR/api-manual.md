# Management Server API 매뉴얼

버전: `v1.2.4.8`  
대상: 고객 시스템 연동  
Base path: `/api/v1`

## 연동 준비 상태

| 상태 | 의미 | 고객 사용 |
| --- | --- | --- |
| `Ready` | NMS 가 직접 API 또는 runtime 기능을 제공 | 운영 연동에 사용 가능 |
| `Bridge Ready` | NMS 는 안정적인 ingest 또는 embed 계약을 제공하고 protocol adapter 는 외부 gateway 에서 실행 | gateway 가 데이터를 정규화하면 운영 연동 가능 |
| `Planned` | 로드맵 항목  현재 릴리스에는 안정적인 runtime contract 없음 | 운영 연동에 사용하지 않음 |

## 인증

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

성공 후 다음 header 를 사용합니다

```http
Authorization: Bearer <jwt>
Accept: application/json
```

2FA 가 활성화된 경우 다음 API 를 추가로 호출합니다

```http
POST /api/v1/auth/verify-2fa
Content-Type: application/json
```

## 최적화 연동 API

고객 dashboard 는 snapshot API 를 먼저 사용하여 API 호출 수를 줄이는 것을 권장합니다

```http
GET /api/v1/integrations/network-snapshot
Authorization: Bearer <jwt>
```

Query parameters

| Name | Default | 설명 |
| --- | --- | --- |
| `include` | `dashboard,devices,topology` | `dashboard` `devices` `topology` 선택 가능 |
| `page` | `1` | device page |
| `limit` | `500` | 최대 `2000` |
| `search` | empty | 이름 또는 IP 검색 |
| `type` | empty | device type |
| `status` | empty | `online` 또는 `offline` |

예시

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://nms.example.com/api/v1/integrations/network-snapshot?include=dashboard,devices,topology&limit=500"
```

이 API 는 `snmp_community` password license key RTSP credential 등 민감 필드를 반환하지 않습니다

## 주요 읽기 API

| Method | Endpoint | 설명 |
| --- | --- | --- |
| `GET` | `/api/v1/system/info` | 버전 및 시스템 정보 |
| `GET` | `/api/v1/auth/me` | 현재 사용자 |
| `GET` | `/api/v1/dashboard` | dashboard summary |
| `GET` | `/api/v1/devices?page=1&limit=50` | device list |
| `GET` | `/api/v1/devices/:id` | device detail |
| `GET` | `/api/v1/topology` | topology nodes and links |
| `GET` | `/api/v1/cameras/status` | camera module status |
| `GET` | `/api/v1/pdu/status` | PDU/UPS module status |
| `GET` | `/api/v1/iot/status` | IoT/Modbus status |
| `GET` | `/api/v1/iot/capabilities` | IoT profile list |

## iframe Embed

Admin 이 scoped embed token 을 생성합니다

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

고객 시스템 iframe 예시

```html
<iframe
  src="https://nms.example.com/embed.html?view=dashboard&token=<embed-token>"
  style="width:100%;height:640px;border:0;"
  loading="lazy"
  referrerpolicy="no-referrer">
</iframe>
```

운영 환경에서는 명확한 iframe allowlist 를 설정해야 합니다

```yaml
security:
  frame_ancestors:
    - "'self'"
    - "https://customer.example.com"
```

## IoT / Modbus / Gateway

| Protocol | Mode | Status | 연동 방식 |
| --- | --- | --- | --- |
| `modbus_tcp` | direct poll | `Ready` | `/api/v1/iot/devices` 생성 후 `/api/v1/iot/devices/:id/poll` 호출 |
| `rest` | gateway ingest | `Ready` | gateway 가 정규화 데이터를 `/api/v1/iot/ingest` 로 POST |
| `mqtt` | gateway ingest | `Bridge Ready` | 외부 MQTT bridge 가 구독 후 `/api/v1/iot/ingest` 로 POST |
| `opcua` | gateway ingest | `Bridge Ready` | 외부 OPC-UA gateway 가 `/api/v1/iot/ingest` 로 POST |
| `bacnet` | gateway ingest | `Bridge Ready` | 외부 BACnet/IP collector 가 `/api/v1/iot/ingest` 로 POST |
| `mqtt_native` | direct broker subscription | `Planned` | 내장 broker client 는 포함되지 않음 |
| `opcua_native` | direct node polling | `Planned` | 내장 OPC-UA client 는 포함되지 않음 |
| `bacnet_native` | direct BACnet/IP polling | `Planned` | 내장 BACnet client 는 포함되지 않음 |

REST ingest 예시

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

## 쓰기 API 범위

쓰기 API 는 신뢰된 시스템에서만 사용하며 RBAC license gate audit log 의 통제를 받습니다

| Method | Endpoint | Role | 설명 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/devices` | editor | device 생성 |
| `PUT` | `/api/v1/devices/:id` | editor | device 갱신 |
| `DELETE` | `/api/v1/devices/:id` | editor | device 삭제 |
| `POST` | `/api/v1/iot/ingest` | editor | IoT measurement 기록 |
| `POST` | `/api/v1/iot/devices/:id/poll` | editor | Modbus TCP device poll |
| `POST` | `/api/v1/iot/devices` | admin | IoT/Modbus device 생성 |
| `PUT` | `/api/v1/integrations/settings` | admin | iframe allowlist 갱신 |
| `DELETE` | `/api/v1/integrations/embed-tokens/:token_id` | admin | embed token revoke |

## 상태 코드

| Status | 의미 |
| --- | --- |
| `200` | 성공 |
| `201` | 생성됨 |
| `400` | request body 또는 query 오류 |
| `401` | token 누락 또는 무효 |
| `403` | role license feature gate 에 의해 차단 |
| `404` | route 또는 resource 없음 |
| `409` | conflict |
| `500` | server error |

## 보안 검증 Gate

- API response log audit export 에 SNMP community password RTSP credential license key API key long-lived token 이 노출되면 안 됨
- iframe 은 scoped embed token 을 사용하고 admin JWT 를 사용하지 않음
- `frame_ancestors` 는 고객 명시 도메인으로 제한
- OT/building device 는 VLAN 또는 ACL 보호 네트워크에 배치
- Modbus TCP SNMP RTSP ONVIF BACnet OPC-UA 를 public internet 에 직접 노출하지 않음
- MQTT OPC-UA BACnet gateway 는 source allowlist scoped token rate limit replay protection audit log 필요
- Windows 는 `start_nms.bat` 또는 `start_nms.ps1` 로 시작

## 스마트 빌딩 2016 / 2024 정렬

이 릴리스는 스마트 빌딩 연동 증적과 운영 데이터 교환을 지원합니다  제품 자체가 스마트 빌딩 라벨 ISO 인증 또는 법적 승인을 획득했다는 의미는 아닙니다

공식 참고

- 2016 Smart Building Evaluation Manual: `https://www.abri.gov.tw/News_Content.aspx?n=20916&s=323060`
- 2024 Smart Building Evaluation Manual: `https://www.abri.gov.tw/News_Content.aspx?n=20916&s=322421`
- 2024 implementation notice: `https://www.moi.gov.tw/Common/EpaperClick.ashx?esq=%40EpaperSendQueueSN&p=D3C50BA14D2E597DBF5BA233B2670F24E65203E80CE503D13003C306C4ECC0A73685AEC600EFED6FBD32224A754AA75931B871A26B58CC04238D485186800BC2`

