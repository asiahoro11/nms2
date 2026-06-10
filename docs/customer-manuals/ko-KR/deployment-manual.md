# Management Server 배포 매뉴얼

버전: `v1.2.4.8`  
지원 플랫폼: Windows amd64  Linux amd64  Linux arm64

## 배포 패키지

| 플랫폼 | 파일 |
| --- | --- |
| Windows amd64 | `artifacts/windows/v1.2.4.8.zip` |
| Linux amd64 + arm64 | `artifacts/linux/v1.2.4.8.zip` |
| Linux arm64 전용 | `artifacts/linux-arm64/v1.2.4.8.zip` |

## Windows 배포

1. `v1.2.4.8.zip` 압축 해제
2. 압축 해제 폴더로 이동
3. `start_nms.bat` 로 시작
4. `http://127.0.0.1:8080` 접속

`nms_server.exe` 를 직접 실행하지 마십시오  
이 릴리스의 Windows 배포는 `start_nms.bat` 또는 `start_nms.ps1` 로 시작해야 합니다

## Linux amd64 배포

```bash
unzip v1.2.4.8.zip -d nms-v1.2.4.8
cd nms-v1.2.4.8
chmod +x nms_server_linux_amd64 start_nms.sh bin/go2rtc_linux_amd64
./start_nms.sh
```

## Linux arm64 배포

ARM 서버에는 arm64 전용 패키지를 권장합니다

```bash
unzip v1.2.4.8.zip -d nms-v1.2.4.8-arm64
cd nms-v1.2.4.8-arm64
chmod +x nms_server_linux_arm64 start_nms.sh bin/go2rtc_linux_arm64
./start_nms.sh
```

## systemd 예시

```ini
[Unit]
Description=Management Server
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/nms
ExecStart=/opt/nms/start_nms.sh
Restart=always
RestartSec=10
User=nms
Group=nms

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable nms
sudo systemctl start nms
sudo systemctl status nms
```

## Port

| Port | Protocol | 용도 |
| --- | --- | --- |
| `8080` | TCP | Web UI / API |
| `514` | UDP | Syslog |
| `162` | UDP | SNMP Trap |

## 검증

```bash
curl http://127.0.0.1:8080/api/v1/system/info
```

예상 응답에는 다음이 포함됩니다

```json
{
  "success": true,
  "data": {
    "version": "v1.2.4.8"
  }
}
```

## 업그레이드 주의사항

- 업그레이드 전 `data/` 백업
- 기존 `data/nms.db` 덮어쓰지 않기
- 고객 `config.yaml` 덮어쓰지 않기
- Windows 는 계속 `start_nms.bat` 사용
- Linux 는 CPU 아키텍처에 맞는 binary 사용

## 보안 권장사항

- 운영 환경에서는 HTTPS reverse proxy 사용
- Web UI / API 접근 IP 제한
- OT device 를 VLAN 또는 ACL 로 격리
- SNMP Modbus RTSP ONVIF BACnet OPC-UA 를 public internet 에 노출하지 않기
- iframe 연동은 명시적인 customer domain 만 허용
- admin password 와 integration token 정기 교체

