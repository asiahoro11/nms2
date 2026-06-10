# Management Server Release Notes

版本: `v1.2.4.8`  
日期: `2026-06-10`

## 版本定位

`v1.2.4.8` 是客戶整合穩定版  重點是 API iframe embed IoT gateway 整合與資安收斂

## 主要變更

- API 手冊更新為 `v1.2.4.8`
- 明確定義 `Ready` `Bridge Ready` `Planned`
- `/api/v1/integrations/network-snapshot` 作為客戶 dashboard bootstrap API
- iframe embed token 支援建立 列表 撤銷 過期檢查
- IoT direct Modbus TCP 與 REST ingest 為 `Ready`
- MQTT OPC-UA BACnet 標示為 `Bridge Ready`

## 資安強化

- devices / PDU API 不回傳 `snmp_community`
- 空白 SNMP community 更新不覆蓋既有值
- camera RTSP URL 會遮罩 credential 與敏感 query
- audit/log detail 擴大遮罩 password token api key secret authorization SNMP community
- iframe 與 IoT UI 加入 token HTTPS allowlist OT network isolation 提示

## UI

- 修正 audit admin embed IoT 表格長字串超出版型問題
- 手機與平板表格使用可控水平捲動
- monitor 名稱確認為 `電視牆模式`

## 打包

- Windows amd64: `artifacts/windows/v1.2.4.8.zip`
- Linux amd64 + arm64: `artifacts/linux/v1.2.4.8.zip`
- Linux arm64: `artifacts/linux-arm64/v1.2.4.8.zip`

## 已驗證

- backend `go test ./...` 通過
- frontend `node --check` 通過
- static mirror 已同步
- Windows package 已用 `start_nms.bat` 啟動並回傳 `v1.2.4.8`
- Linux arm64 binary 已驗證為 AArch64 ELF

