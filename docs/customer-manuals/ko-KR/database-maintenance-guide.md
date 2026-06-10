# Management Server 데이터베이스 유지보수 매뉴얼

버전: `v1.2.4.8`

## 범위

이 문서는 고객 운영 담당자가 SQLite 데이터베이스 백업 복원 상태 점검 보안 유지보수를 수행하기 위한 기본 절차를 제공합니다

## 중요 경로

| 경로 | 용도 |
| --- | --- |
| `data/nms.db` | 메인 데이터베이스 |
| `data/` | runtime data |
| `config.yaml` | 고객 환경 설정 |

## 백업

```bash
sudo systemctl stop nms
cp -a data "data-backup-$(date +%Y%m%d-%H%M%S)"
sudo systemctl start nms
```

온라인 백업은 내장 백업 기능 또는 일관성을 보장하는 고객 snapshot 도구 사용을 권장합니다

## 복원

```bash
sudo systemctl stop nms
cp -a data data-before-restore
rm -rf data
cp -a data-backup-YYYYMMDD-HHMMSS data
sudo systemctl start nms
```

## 상태 점검

```bash
sqlite3 data/nms.db "PRAGMA integrity_check;"
sqlite3 data/nms.db "PRAGMA quick_check;"
```

예상 결과

```text
ok
```

## 정리 및 성능

```bash
sqlite3 data/nms.db "VACUUM;"
sqlite3 data/nms.db "ANALYZE;"
```

유지보수 시간대에 실행하십시오

## 보안 유지보수

- `data/` 는 service account 만 읽기/쓰기 가능하도록 제한
- 백업 파일은 암호화하여 보관
- 백업을 Web root 에 두지 않음
- `nms.db` 를 ticket email chat 으로 전송하지 않음
- 인원 또는 권한 변경 후 admin password 와 integration token 교체

## 업그레이드 전 점검

- `data/` 백업 완료
- `config.yaml` 백업 완료
- 이전 artifact 로 rollback 가능
- 현재 버전과 배포 경로 기록 완료

