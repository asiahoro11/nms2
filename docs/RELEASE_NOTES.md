# Management Server Release Notes

---
## v1.2.4-PoC | 2026-04-24

**本次版本定位:** 可直接交付客戶測試的 PoC 版，並保留同套程式轉正式 UUID 授權的出貨路徑。  
**主要目標:** 預設以 `0` 授權啟動；由 PoC License 解鎖測試範圍；未來切換成標準 UUID 授權時，不需要重新更換軟體。

### PoC Runtime
- 版本封裝調整為 `v1.2.4-PoC`，方便與現行 `v1.2.4` 主線對齊。
- PoC 版預設 `default_device_limit = 0`，裝置管理維持鎖定狀態。
- 客戶測試時可另外匯入 `PoC License`，由授權內容決定可用裝置數與模組能力。

### License Promotion
- `/api/v1/system/info` 與 Dashboard 版本字樣改為依實際授權狀態動態切換。
- 當系統存在有效的標準 UUID 授權時，版本字樣自動從 `v1.2.4-PoC` 轉為 `v1.2.4`。
- PoC License 與 Trial 不會移除 `PoC` 字樣，避免與正式出貨授權混淆。

### Frontend Alignment
- 登入頁與模式選擇頁會在載入後同步後端 runtime 版本字樣。
- 靜態資產版號維持 `v1.2.4`，後續正式授權切換時不需要更換整套軟體。

---
## v1.2.4 | Candidate

**本次版本定位:** 日誌與稽核強化版。  
**主要目標:** 將既有零散的系統日誌與稽核紀錄，重構為可搜尋、可匯出、可追溯、可支援後續維運與合規審查的正式模組。

### Logs & Audit
- 新增獨立主欄位：`日誌與稽核`
- 拆分四類紀錄：
  - `稽核日誌`
  - `系統日誌`
  - `設備日誌`
  - `設定變更`
- 各類日誌支援獨立查詢、篩選與個別匯出

### Audit Coverage
- 補齊高價值操作稽核：
  - 使用者登入 / 登出 / 登入失敗
  - 授權新增 / 啟用 / 試用授權
  - 設備新增 / 修改 / 刪除
  - 攝影機新增 / 修改 / 刪除
  - 拓樸變更
  - PoE 開 / 關 / 重啟
  - 設備重啟
  - 系統設定變更
- 稽核 detail 改為結構化 JSON，而非鬆散字串
- 補入 `recordset_id` 與 `correlation_id` 以串聯同一批操作

### Config Change
- 新增設定變更紀錄
- 針對設備、攝影機、授權、拓樸、系統設定保留 `before / after` 差異
- 區分變更來源：
  - `manual`
  - `batch`
  - `auto_discovery`
  - `system_sync`

### Presentation And Export
- 前端顯示統一中文化，再由 i18n 載入其他語系
- 不再直接顯示 raw action key、英文 reason 或未格式化 detail
- 匯出格式至少支援：
  - `CSV`
  - `JSON`
- 匯出內容包含篩選條件、時間範圍、匯出時間與筆數

### Compliance Direction
- 對齊智慧建築制度演進需求與管理維護可追溯性
- 強化對 `ISO/IEC 27001`、`ISO/IEC 27701`、`ISO 9001` 的證據支援能力
- 版本重點為「更接近可稽核」，不是直接宣稱完成認證

### Reference
- 詳細規格請參考：
  - `docs/AUDIT_AND_LOGGING_SPEC.md`
  - `docs/V1_2_4_SCOPE.md`

---
## v1.2.1-PoC | 2026-04-20

**Summary:** Introduces the dedicated PoC licensing line, locks device management by default, and adds a dual-mode generator for Formal and PoC licenses.

### License / PoC
- New `v1.2.1-PoC` line with `default_device_limit = 0` by default.
- Device management stays locked until a valid license enables `device_management` or grants licensed device capacity.
- PoC licenses are not bound to Machine ID and support custom device count and exact expiry time.
- Formal licenses remain Machine ID bound.

### Generator
- Added a dedicated generator with separate `Formal License` and `PoC License` interfaces.
- Formal licenses require Machine ID and generate machine-bound keys.
- PoC licenses can define custom expiry time and device count without UUID binding.

### Frontend / Gate
- Device and topology entry points now rely on `/api/v1/license/features` and `device_management`.
- Unauthorized device and topology read/write APIs are blocked by the backend.

---
## v1.2.3 | 2026-04-20

**本次更新摘要:** 完成監控模式與拓樸圖穩定化、攝影機預覽切換修復、錄影命名修正，以及版本與發布資訊收斂。

### Monitor / Topology
- 修正監控模式拓樸圖與 tooltip 殘留的 `??` / 亂碼顯示問題。
- 監控模式樹狀收合後，底下若有離線設備，父節點會正常顯示紅色呼吸燈提示。
- 管理模式後台拓樸圖同步補上相同的收合離線提示，不再只在 Monitor 生效。
- 拓樸統計欄位與速率文字顯示已統一收斂，避免再出現殘留的開發字串。

### Camera / Recording
- 修正從 `Monitor` 切回管理模式後，攝影機預覽可能卡住不再更新的 backend stale stream 問題。
- 攝影機管理頁的預覽流程已回到穩定模式，避免 fullscreen 或頁面切換後長時間停住。
- 錄影清單與匯出檔名改為優先使用 `camera_recordings.camera_name`，不再退回 `cam1/cam2/cam4` 類型命名。
- 新錄影會保留當下攝影機名稱快照，後續即使設備 ID 改動，錄影名稱仍可正確對應。

### UI / Release
- 清理前後台攝影機模組、錄影管理、刪除按鈕、監控提示中的可見亂碼與 raw key 顯示。
- 版本顯示與靜態資產 cache-busting 版號已統一收斂到 `v1.2.3`。
- 本版 release artifact 會同步帶入最新 `RELEASE_NOTES.md` 與 `RELEASE_NOTE.txt`。

### License
- 正式授權新增永久授權規則：當年限達到 `50` 年以上時，系統自動判定為 `永久授權`。
- NMS 授權頁會針對永久授權統一顯示 `永久授權`，不再顯示一般到期日期。

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
