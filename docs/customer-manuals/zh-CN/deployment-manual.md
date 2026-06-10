# Management Server 部署手册

版本: `v1.2.4.8`  
适用平台: Windows amd64  Linux amd64  Linux arm64

## 交付包

| 平台 | 文件 |
| --- | --- |
| Windows amd64 | `artifacts/windows/v1.2.4.8.zip` |
| Linux amd64 + arm64 | `artifacts/linux/v1.2.4.8.zip` |
| Linux arm64 单独包 | `artifacts/linux-arm64/v1.2.4.8.zip` |

## Windows 部署

1. 解压 `v1.2.4.8.zip`
2. 进入解压目录
3. 使用 `start_nms.bat` 启动
4. 打开 `http://127.0.0.1:8080`

不要直接运行 `nms_server.exe`  
本版要求 Windows 使用 `start_nms.bat` 或 `start_nms.ps1`

## Linux amd64 部署

```bash
unzip v1.2.4.8.zip -d nms-v1.2.4.8
cd nms-v1.2.4.8
chmod +x nms_server_linux_amd64 start_nms.sh bin/go2rtc_linux_amd64
./start_nms.sh
```

## Linux arm64 部署

建议使用 arm64 单独包

```bash
unzip v1.2.4.8.zip -d nms-v1.2.4.8-arm64
cd nms-v1.2.4.8-arm64
chmod +x nms_server_linux_arm64 start_nms.sh bin/go2rtc_linux_arm64
./start_nms.sh
```

## systemd 服务示例

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

## 验证

```bash
curl http://127.0.0.1:8080/api/v1/system/info
```

预期返回包含

```json
{
  "success": true,
  "data": {
    "version": "v1.2.4.8"
  }
}
```

## 升级注意事项

- 升级前先备份 `data/`
- 不要覆盖既有 `data/nms.db`
- 不要覆盖客户自定义 `config.yaml`
- Windows 仍使用 `start_nms.bat`
- Linux 依 CPU 架构选择 `nms_server_linux_amd64` 或 `nms_server_linux_arm64`

## 安全建议

- 生产环境使用 HTTPS reverse proxy
- 限制 Web UI / API 来源 IP
- OT 设备使用 VLAN/ACL 隔离
- 不要将 SNMP Modbus RTSP ONVIF BACnet OPC-UA 暴露到公网
- iframe 集成只允许明确 customer domain
- 定期轮替 admin password 与 integration token

