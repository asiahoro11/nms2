# Management Server 部署手冊

版本: `v1.2.4.8`  
適用平台: Windows amd64  Linux amd64  Linux arm64

## 交付包

| 平台 | 檔案 |
| --- | --- |
| Windows amd64 | `artifacts/windows/v1.2.4.8.zip` |
| Linux amd64 + arm64 | `artifacts/linux/v1.2.4.8.zip` |
| Linux arm64 單獨包 | `artifacts/linux-arm64/v1.2.4.8.zip` |

## Windows 部署

1. 解壓縮 `v1.2.4.8.zip`
2. 進入解壓縮目錄
3. 使用 `start_nms.bat` 啟動
4. 瀏覽 `http://127.0.0.1:8080`

不要直接執行 `nms_server.exe`  
本版已要求 Windows 使用 `start_nms.bat` 或 `start_nms.ps1`

## Linux amd64 部署

```bash
unzip v1.2.4.8.zip -d nms-v1.2.4.8
cd nms-v1.2.4.8
chmod +x nms_server_linux_amd64 start_nms.sh bin/go2rtc_linux_amd64
./start_nms.sh
```

## Linux arm64 部署

建議使用 arm64 單獨包

```bash
unzip v1.2.4.8.zip -d nms-v1.2.4.8-arm64
cd nms-v1.2.4.8-arm64
chmod +x nms_server_linux_arm64 start_nms.sh bin/go2rtc_linux_arm64
./start_nms.sh
```

## systemd 服務範例

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

| Port | Protocol | 用途 |
| --- | --- | --- |
| `8080` | TCP | Web UI / API |
| `514` | UDP | Syslog |
| `162` | UDP | SNMP Trap |

## 驗證

```bash
curl http://127.0.0.1:8080/api/v1/system/info
```

預期回傳包含

```json
{
  "success": true,
  "data": {
    "version": "v1.2.4.8"
  }
}
```

## 升級注意事項

- 升級前先備份 `data/`
- 不要覆蓋既有 `data/nms.db`
- 不要覆蓋客戶自訂 `config.yaml`
- Windows 仍使用 `start_nms.bat`
- Linux 依 CPU 架構選擇 `nms_server_linux_amd64` 或 `nms_server_linux_arm64`

## 資安建議

- 生產環境使用 HTTPS reverse proxy
- 限制 Web UI / API 來源 IP
- OT 設備使用 VLAN/ACL 隔離
- 不要將 SNMP Modbus RTSP ONVIF BACnet OPC-UA 暴露到公網
- iframe 整合只允許明確 customer domain
- 定期輪替 admin password 與 integration token

