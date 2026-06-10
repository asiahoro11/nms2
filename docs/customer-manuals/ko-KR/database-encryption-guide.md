# Management Server 데이터베이스 및 백업 암호화 매뉴얼

버전: `v1.2.4.8`

## 범위

이 문서는 암호화 백업 비밀번호 보호 안전한 보관 절차를 설명합니다  
전체 데이터베이스 암호화 사용 여부는 고객 라이선스와 배포 정책에 따라 결정됩니다

## 기본 원칙

- 백업 파일은 반드시 암호화
- 백업 비밀번호는 로그인 비밀번호와 다르게 설정
- 비밀번호는 고객 password vault 에 보관
- script repo email ticket document 에 비밀번호를 기록하지 않음

## 권장 비밀번호 규칙

- 최소 12자
- 대문자 소문자 숫자 기호 포함
- 회사명 프로젝트명 device 명 사용 금지
- 납품 또는 유지보수 후 필요 시 교체

## 암호화 백업

```http
POST /api/v1/system/backup/encrypted
Authorization: Bearer <admin-jwt>
Content-Type: application/json
```

```json
{
  "password": "<backup-password>"
}
```

## 복원

```http
POST /api/v1/system/restore/encrypted
Authorization: Bearer <admin-jwt>
Content-Type: multipart/form-data
```

| Field | 설명 |
| --- | --- |
| `file` | 암호화 백업 파일 |
| `password` | 백업 비밀번호 |

## 검수

- 백업 파일에 평문 데이터가 없음
- 잘못된 비밀번호로 복원 불가
- 올바른 비밀번호로 테스트 환경 복원 가능
- 복원 후 `/api/v1/system/info` 정상
- users permissions devices topology audit data 유지

## 보관 정책

- daily 7  weekly 4  monthly 3 이상 보관
- 최소 1개는 offline 또는 offsite 보관
- 정기 복원 테스트
- 만료 백업은 안전하게 삭제

