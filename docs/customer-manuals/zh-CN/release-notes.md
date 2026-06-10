# Management Server Release Notes

版本: `v1.2.4.8`  
日期: `2026-06-10`

## 版本定位

`v1.2.4.8` 是客户集成稳定版  重点是 API iframe embed IoT gateway 集成与安全收敛

## 主要变更

- API 手册更新为 `v1.2.4.8`
- 明确定义 `Ready` `Bridge Ready` `Planned`
- `/api/v1/integrations/network-snapshot` 作为客户 dashboard bootstrap API
- iframe embed token 支持创建 列表 撤销 过期检查
- IoT direct Modbus TCP 与 REST ingest 为 `Ready`
- MQTT OPC-UA BACnet 标示为 `Bridge Ready`

## 安全强化

- devices / PDU API 不返回 `snmp_community`
- 空白 SNMP community 更新不覆盖既有值
- camera RTSP URL 会遮罩 credential 与敏感 query
- audit/log detail 扩大遮罩 password token api key secret authorization SNMP community
- iframe 与 IoT UI 加入 token HTTPS allowlist OT network isolation 提示

## UI

- 修正 audit admin embed IoT 表格长字符串超出版型问题
- 手机与平板表格使用可控水平滚动
- monitor 名称确认为 `電視牆模式`

## 打包

- Windows amd64: `artifacts/windows/v1.2.4.8.zip`
- Linux amd64 + arm64: `artifacts/linux/v1.2.4.8.zip`
- Linux arm64: `artifacts/linux-arm64/v1.2.4.8.zip`

## 已验证

- backend `go test ./...` 通过
- frontend `node --check` 通过
- static mirror 已同步
- Windows package 已用 `start_nms.bat` 启动并返回 `v1.2.4.8`
- Linux arm64 binary 已验证为 AArch64 ELF

