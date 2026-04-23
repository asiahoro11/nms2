# Management Server Release Notes

---

## v1.2.2alpha4 — 2026-04-17

**核心更新:** 頻寬超限告警通知、攝影機密碼解密容錯、拓樸樹狀圖層次修正

### 🔔 告警通知 (Bandwidth Alerts)
- **攝影機頻寬超限告警：** 攝影機 MJPEG 串流超過頻寬限制時，寫入鈴鐺通知（改自 events 表 → notifications 表）
- **網路設備頻寬超限告警：** SNMP 輪詢時若介面流量超過 link speed 的 80%，自動寫入 notifications，5 分鐘冷卻
- 告警通知可在右上角鈴鐺面板查看、標記已讀、清除

### 🌐 拓樸圖修正 (Topology Fix)
- **樹狀圖層次修正：** 移除「同類型 switch 直連視為 HA pair」的錯誤推斷，只有明確 `link_type=ha/vrf/spine_leaf` 才合併同層，IPCAM 等下游設備現在正確掛在其上游 switch 下

### 📷 攝影機模組 (Camera Module)
- **密碼解密容錯：** 舊版加密密碼無法以當前 AES-GCM key 解密時，自動 fallback 解析 RTSP URL 中的明文密碼（`rtsp://user:pass@host/`），修復 401 Unauthorized 問題

---

## v1.2.2alpha3 — 2026-04-16

**核心更新:** 拓樸樹狀圖節點收折、IPCAM Bitrate 設定、監控模式拓樸修正

### 🌐 拓樸圖優化 (Topology Enhancements)
- **樹狀拓樸節點收折/展開：** 點選有子設備的節點可收折/展開；收折節點右下角顯示數字視覺提示
- **收折狀態持久化：** 節點收折狀態儲存在瀏覽器 `localStorage`，重新整理頁面可自動恢復
- **統一渲染引擎：** 主頁面 (Monitor) 與管理模式共享同一套拓樸渲染邏輯，確保視覺與設定一致性

### 📷 攝影機模組改善 (Camera Module)
- **攝影 Bitrate 設定：** 攝影機設定中新增「錄影 Bitrate (kbps)」欄位
- **轉碼錄影：** 若設定 Bitrate > 0，錄影程序啟用 `libx264` 轉碼以符合頻寬需求；設為 0 則維持 `Stream Copy` 高效模式
- **ONVIF 相容性：** 錄影來源支援 ONVIF 協定連接，確保錄影流能正確更新

### 🔧 系統改善
- **編譯版本更新：** `build_both_releases.ps1` 支援 v1.2.2alpha3 版本，Windows 與 Linux 同步編譯
- **快取清除：** 版本更新後自動 append `?v=v1.2.2alpha3` 至資源檔案，防止舊版 JS 緩衝衝突

---

## v1.2.0 — 2026-04-08

**核心更新:** 監控模式正式化、雙模式登入架構、角色導向主控台、多語系 i18n 補完

### 📺 監控模式 (Demo → Monitor 正式化)
- `demo.html` 正式更名為 `monitor.html`，路由從 `/demo` 改為 `/monitor`
- 頁面標題、Badge、退出按鈕文字全面更新為「監控模式」
- Badge 與標題後綴支援 5 語系即時切換（繁中 / 簡中 / EN / 日本語 / 한국어）
- 系統名稱顯示修正：優先顯示 sysName，其次自定義名稱，fallback IP
- 拓樸連線消失問題修復：自動刷新時比對 node/link ID，結構有變化則觸發完整 rebuild

### 🎛️ 雙模式主控台 (新架構)
- 新增 `mode-selection.html`：登入後統一跳至此頁選擇模式
- **角色導向顯示規則：**
  - `admin` / `editor`：顯示管理模式 + 監控模式兩張卡片
  - `viewer`：僅顯示監控模式，無法進入管理介面
- 主控台支援語系切換、深色/淺色主題、登出
- 管理模式右上角「監控模式」按鈕改為「🏠 主控台」，跳回模式選擇頁
- 監控模式「離開監控」跳回主控台（保留 token，不需重新登入）

### 🔐 登入流程重構
- 登入成功後統一跳至 `mode-selection.html`，不再在登入頁選模式
- 登入與變更密碼後強制 cache-bust（等同 Ctrl+F5），確保載入最新 JS/CSS
- 已登入狀態開啟登入頁，自動跳至主控台選擇頁（而非管理模式）

### 🌐 i18n 補完
- 修復登入頁「登入模式」下拉選單未隨語系切換的問題
- 各語言檔補充 `login.mode_label` / `login.mode_admin` / `login.mode_monitor`
- 修復管理模式「資源使用率」標題硬碼問題，改用 `data-i18n="admin.resource_usage"`
- 修復攝影機編輯 Modal 所有標籤、placeholder、按鈕改用 `t()` i18n 呼叫
- 刪除確認對話框、操作成功/失敗 Toast 全面 i18n 化

### 🔧 Build 流程改善
- `build_both_releases.ps1` 同時 Windows + Linux 雙平台版本一鍵編譯
- 新增 sp999 防呆機制：偵測到 sp999 版本時要求輸入 YES 才繼續
- 版本號統一讀取 `config.go`，腳本無需手動修改版本號

---

## v1.1.0sp2 — 2026-04-07

**核心更新:** Demo 展示模式完善、攝影機 RWD 改善、Viewer 權限強化、UI Bug 修復

### 📺 Demo 展示模式（更新）
- 完整實裝 Demo 頁面（`/demo`）：無需登出，無需重新登入
- 左側：互動式網路拓樸圖（D3.js，可縮放/拖曳）
- 右側：設備統計卡片 + 即時事件 Log
- 支援 5 種語系：繁中 / 簡中 / EN / 日本語 / 한국어
- 支援深色 / 淺色主題切換（持久化至 localStorage）
- 5 秒自動刷新，更新設備 on/off 狀態 + 線路流量（不重置 zoom/pan）
- 自動更新時樣保持 zoom/pan 位置（修復首次載入 bug）
- 流量格式統一使用 SI bps（修復與管理模式數值不一致問題）

---

## v1.1.0 — 2026-04-03

**核心更新:** EdgeCore SSH 整合、PoE 控制、攝影機 GPU 加速、ffmpeg 自動下載

### 🔌 EdgeCore SSH 整合
- 支援 ECS2100-10P、ECS4150、ECS2220 系列
- SSH 指令序列：備份、重啟、PoE ON/OFF/Recycle
- isPrompt 修正：`!<stackingDB>` 不再誤判為 prompt

### ⚡ PoE 控制
- PoE ON/OFF/Recycle（SSH 優先，SNMP fallback）
- 每 port 即時功耗（EdgeCore private OID）
- PoE 功率摘要卡片（總功率 / 已使用 / 進度條 >85% 紅 >60% 橙）

### 🎥 攝影機強化
- GPU 加速串流（NVIDIA CUDA / Intel QSV / AMD VAAPI / D3D11VA）
- 攝影機分頁（16ch/頁，最多 64ch）
- 攝影機 status 修正（第一幀成功後才寫 online）
- ffmpeg 自動下載（找不到時從 GitHub BtbN builds 下載）

### 🔧 其他
- 設備離線時介面速率 tab 灰化 + 警告 banner
- 系統狀態頁 GPU 規格顯示
- PoE 0W 顯示「非 PoE 設備 / 0W」
