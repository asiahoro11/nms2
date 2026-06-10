# Management Server デプロイ手順書

バージョン: `v1.2.4.8`  
対象プラットフォーム: Windows amd64  Linux amd64  Linux arm64

## 配布パッケージ

| プラットフォーム | ファイル |
| --- | --- |
| Windows amd64 | `artifacts/windows/v1.2.4.8.zip` |
| Linux amd64 + arm64 | `artifacts/linux/v1.2.4.8.zip` |
| Linux arm64 専用 | `artifacts/linux-arm64/v1.2.4.8.zip` |

## Windows デプロイ

1. `v1.2.4.8.zip` を展開
2. 展開先フォルダへ移動
3. `start_nms.bat` で起動
4. `http://127.0.0.1:8080` を開く

`nms_server.exe` を直接実行しないでください  
本リリースの Windows デプロイは `start_nms.bat` または `start_nms.ps1` で起動します

## Linux amd64 デプロイ

```bash
unzip v1.2.4.8.zip -d nms-v1.2.4.8
cd nms-v1.2.4.8
chmod +x nms_server_linux_amd64 start_nms.sh bin/go2rtc_linux_amd64
./start_nms.sh
```

## Linux arm64 デプロイ

ARM サーバーでは arm64 専用パッケージを推奨します

```bash
unzip v1.2.4.8.zip -d nms-v1.2.4.8-arm64
cd nms-v1.2.4.8-arm64
chmod +x nms_server_linux_arm64 start_nms.sh bin/go2rtc_linux_arm64
./start_nms.sh
```

## systemd 例

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

## 検証

```bash
curl http://127.0.0.1:8080/api/v1/system/info
```

期待されるレスポンスには次が含まれます

```json
{
  "success": true,
  "data": {
    "version": "v1.2.4.8"
  }
}
```

## アップグレード注意事項

- アップグレード前に `data/` をバックアップする
- 既存の `data/nms.db` を上書きしない
- 顧客の `config.yaml` を上書きしない
- Windows は引き続き `start_nms.bat` を使用する
- Linux は CPU アーキテクチャに合う binary を使用する

## セキュリティ推奨

- 本番環境では HTTPS reverse proxy を使用する
- Web UI / API の接続元 IP を制限する
- OT device を VLAN または ACL で分離する
- SNMP Modbus RTSP ONVIF BACnet OPC-UA を public internet に公開しない
- iframe 連携は明示的な customer domain のみ許可する
- admin password と integration token を定期的にローテーションする

