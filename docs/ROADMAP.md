# Management Server Product Roadmap

**最後更新**: 2026-04-02
**當前版本**: v1.1.0

---

## v1.1.0 - 已發布 (2026-04-02)

- EdgeCore 系列 SSH 整合（ECS2100 / ECS4150 / ECS2220）
- PoE 控制：ON / OFF / Recycle（SSH CLI 優先，SNMP fallback）
- 設定備份：`show running-config` 存入 DB 歸檔
- Frontend：總覽 / 介面 / PoE tab 的 EdgeCore 設備顯示
- 攝影機模組 GPU 加速：CUDA / QSV / VAAPI / D3D11VA
- 攝影機併發：每台 16ch，GPU 加速總量 64ch
- PoE 每 port 功耗：EdgeCore private OID `1.3.6.1.4.1.259.10.1.43.1.28.6.1.14.1.(port)`
- PoE 電源預算顯示：總功率 / 已使用功率 / 溫度
- 系統資訊：GPU 規格顯示（lspci / wmic，含 VM 偵測）

---

## v1.1.1 - 待規劃

### NVR 整合支援

**狀態**: 待確認設備型號 / 介面 / RTSP URL 格式 / SNMP community

#### 規劃功能

- [ ] NVR 設備類型支援（新增 device_type = `nvr`）
- [ ] NVR SNMP 監控
  - 硬碟狀態、容量、RAID 狀態
  - 錄影 channel 狀態（錄影中 / 離線 / 錯誤）
  - CPU / 記憶體使用率
  - 系統溫度（若支援）
- [ ] NVR RTSP 串流整合
  - 將 NVR channel 接入 RTSP 串流
  - 顯示即時影像到監控模組（利用現有併發 / GPU 加速架構）
  - 支援主流廠牌：`rtsp://ip:554/ch{N}/main`、`rtsp://ip/Streaming/Channels/{N}01` 等
- [ ] NVR 設備 Detail 頁
  - 硬碟狀態 tab
  - Channel 清單 tab（總覽 / 狀態 / 錄影中）
- [ ] Frontend：NVR 專屬 icon / badge

#### 待確認（需廠商資訊）

- 目標 NVR 廠牌 / 型號
- SNMP community 版本（v1 / v2c）與可用 OID
- RTSP URL 格式
- 是否需要 ONVIF 支援

---

## Backlog（未排期）

- [ ] RWD 攝影機監控畫面顯示問題（小螢幕 / 平板）
- [ ] Topology 圖加入 EdgeCore PoE 狀態 overlay
- [ ] ProNMS-Cloud 整合：單站點資料同步至雲端
- [ ] 多站點管理（Multi-site dashboard）
- [ ] 設備自動搜索（SNMP broadcast / CDP / LLDP）
- [ ] 告警規則：PoE 功耗超標通知

---

## 版本編號規則

| 版本格式 | 定義 |
|---------|------|
| `v1.1.0` | 重大功能版本 |
| `v1.1.1` | 次要功能版本 |
| `v1.1.xspN` | Hotfix / 小修正 |
