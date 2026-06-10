# Management Server Deployment Manual

Version: `v1.2.4.8`  
Platforms: Windows amd64  Linux amd64  Linux arm64

## Packages

| Platform | File |
| --- | --- |
| Windows amd64 | `artifacts/windows/v1.2.4.8.zip` |
| Linux amd64 + arm64 | `artifacts/linux/v1.2.4.8.zip` |
| Linux arm64 dedicated | `artifacts/linux-arm64/v1.2.4.8.zip` |

## Windows Deployment

1. Extract `v1.2.4.8.zip`
2. Enter the extracted folder
3. Start with `start_nms.bat`
4. Open `http://127.0.0.1:8080`

Do not run `nms_server.exe` directly.  
This release expects Windows deployments to start through `start_nms.bat` or `start_nms.ps1`.

## Linux amd64 Deployment

```bash
unzip v1.2.4.8.zip -d nms-v1.2.4.8
cd nms-v1.2.4.8
chmod +x nms_server_linux_amd64 start_nms.sh bin/go2rtc_linux_amd64
./start_nms.sh
```

## Linux arm64 Deployment

Use the dedicated arm64 package when deploying to ARM servers.

```bash
unzip v1.2.4.8.zip -d nms-v1.2.4.8-arm64
cd nms-v1.2.4.8-arm64
chmod +x nms_server_linux_arm64 start_nms.sh bin/go2rtc_linux_arm64
./start_nms.sh
```

## systemd Example

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

## Ports

| Port | Protocol | Purpose |
| --- | --- | --- |
| `8080` | TCP | Web UI / API |
| `514` | UDP | Syslog |
| `162` | UDP | SNMP Trap |

## Verification

```bash
curl http://127.0.0.1:8080/api/v1/system/info
```

Expected response includes:

```json
{
  "success": true,
  "data": {
    "version": "v1.2.4.8"
  }
}
```

## Upgrade Notes

- Back up `data/` before upgrading.
- Do not overwrite existing `data/nms.db`.
- Do not overwrite customer `config.yaml`.
- Windows still starts through `start_nms.bat`.
- Linux must use the binary that matches CPU architecture.

## Security Recommendations

- Use HTTPS reverse proxy in production.
- Restrict Web UI / API source IPs.
- Isolate OT devices with VLANs or ACLs.
- Do not expose SNMP, Modbus, RTSP, ONVIF, BACnet, or OPC-UA to the public internet.
- Allow iframe integration only from explicit customer domains.
- Rotate admin passwords and integration tokens regularly.

