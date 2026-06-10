# Management Server Release Notes

バージョン: `v1.2.4.8`  
日付: `2026-06-10`

## リリース位置付け

`v1.2.4.8` は API iframe embed IoT gateway 連携とセキュリティ強化を目的とした顧客連携安定版です

## 主な変更

- API manual を `v1.2.4.8` に更新
- `Ready` `Bridge Ready` `Planned` を明確化
- `/api/v1/integrations/network-snapshot` を顧客 dashboard bootstrap API として定義
- iframe embed token の作成 一覧 revoke 有効期限確認をサポート
- IoT direct Modbus TCP と REST ingest は `Ready`
- MQTT OPC-UA BACnet は `Bridge Ready`

## セキュリティ強化

- devices / PDU API は `snmp_community` を返さない
- 空白 SNMP community 更新では既存値を保持
- camera RTSP URL は credential と機密 query を redaction
- audit/log detail は password token api key secret authorization SNMP community を redaction
- iframe と IoT UI に token HTTPS allowlist OT network isolation の注意を追加

## UI

- audit admin embed IoT table の長い文字列 overflow を修正
- mobile tablet table は制御された横スクロールを使用
- monitor 名称は `電視牆模式`

## パッケージ

- Windows amd64: `artifacts/windows/v1.2.4.8.zip`
- Linux amd64 + arm64: `artifacts/linux/v1.2.4.8.zip`
- Linux arm64: `artifacts/linux-arm64/v1.2.4.8.zip`

## 検証済み

- backend `go test ./...` pass
- frontend `node --check` pass
- static mirror synced
- Windows package は `start_nms.bat` で起動し `v1.2.4.8` を返す
- Linux arm64 binary は AArch64 ELF として確認済み

