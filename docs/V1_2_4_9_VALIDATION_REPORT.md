# NMS v1.2.4.9 檢驗報告

檢驗日期：2026-06-30  
檢驗範圍：v1.2.4.9 主程式、Windows/Linux amd64/Linux arm64 發行包、License Generator、授權鎖定 UI、IoT 授權、i18n、系統管理頁穩定性。

## 1. 結論

本次已重新修正並編譯 v1.2.4.9。先前「進入系統管理就卡住」的主要風險點已處理：系統管理頁進入時不再自動觸發 `host-status`、`machine-id`、`licenses`、`users` 等重查 API，改為使用者點擊對應分頁後才載入資料。

追加修正：

- 告警設定卡在「載入中」：已修正 SQLite 單連線下 `GetAlertSettings()` 與 `FeatureFlags()` 互相等待的 deadlock。
- 告警通道消失：已修正無授權通道被直接隱藏的前端邏輯，改為顯示鎖頭並停用操作。
- 目前主機狀態 CPU/RAM/Storage 未顯示：已補回輕量系統統計，並保留 GPU 偵測跳過以維持穩定。

已重新產出：

| 交付物 | 狀態 |
| --- | --- |
| Windows v1.2.4.9 | 通過 |
| Linux amd64 v1.2.4.9 | 通過 |
| Linux arm64 v1.2.4.9 | 通過 |
| License Generator Windows amd64 | 通過 |
| License Generator Linux amd64 | 通過 |
| License Generator Linux arm64 | 通過 |

## 2. 本次修正項目

### 2.1 系統管理頁卡住修正

調整檔案：

- `apps/frontend/js/app.js`
- `apps/frontend/js/admin.js`

修正內容：

- `admin.js` 移除 DOMContentLoaded 時自動 `.click()` 預設分頁的行為。
- `app.js` 進入 `admin` 頁時不再自動載入 `loadHostStatus()`、`loadUsers()`、`loadLicenses()`。
- Admin 分頁事件綁定改為限定 `.admin-tabs .tab-btn`，避免影響其他模組內部 tab。
- 程式內切換 Admin 分頁時改用 `activateAdminTab()`，避免隱性觸發資料載入。
- `host-status` API 保留手動點擊才載入，前端數值處理已加上安全 fallback，避免 `.toFixed()` 類型錯誤。

### 2.2 IoT 授權與鎖頭圖示

修正內容：

- IoT 授權提示 key 修正為 `iot.license_notice`。
- `zh-TW` 顯示為繁體中文提示，不再出現 raw key。
- Camera、門禁、PDU/UPS、IoT / Modbus 的授權鎖定狀態統一顯示鎖頭圖示。
- `setModuleLock()` 會依授權功能動態更新桌面側欄與底部導覽鎖頭。

### 2.3 License Generator

License Generator 已整合為單一主要工具 `license-gen`，避免功能相近工具重複編譯成多份。

支援功能：

- `device`
- `camera`
- `camera_recording`
- `access_control`
- `pdu`
- `iot`
- `alert`
- `combined`
- `full`
- `custom`

`full` 授權包含：

- `device_management`
- `camera_viewer`
- `camera_recording`
- `access_control`
- `pdu`
- `iot`
- `line`
- `telegram`
- `whatsapp`
- `discord`
- `slack`

### 2.4 告警設定卡住修正

調整檔案：

- `apps/backend/modules/notifications/service.go`
- `apps/backend/modules/license/service.go`
- `apps/frontend/js/admin.js`

根因：

- SQLite 已設定 `SetMaxOpenConns(1)`。
- 舊版 `GetAlertSettings()` 在 `rows` 尚未關閉時呼叫 `FeatureFlags()`。
- `FeatureFlags()` 也會查詢資料庫，因此在單一連線下可能互相等待，導致 `/api/v1/alerts/settings` 不回應。

修正內容：

- `GetAlertSettings()` 先完整讀取 `alert_settings` 並關閉 `rows`，再查授權 feature。
- `FeatureFlags()` 在查詢 license feature 後先關閉 `rows`，再查 `device_management_enabled`。
- 前端 `loadAlertSettings()` 加入 5 秒 timeout。
- 前端載入失敗時顯示錯誤與重試按鈕，不再停留在「載入中」。
- 前端告警通道卡片不再因未授權而消失，會顯示鎖頭、授權提示，並停用開關、設定、測試按鈕。

### 2.5 目前主機狀態 CPU/RAM/Storage 修正

調整檔案：

- `apps/backend/api/handlers/system.go`

修正內容：

- 使用既有 `gopsutil` 取回 CPU、RAM、Storage 實際數值。
- Host Status 統計加入 2 秒 timeout，避免系統 API 卡住時拖死頁面。
- GPU 偵測仍跳過，避免 Windows WMI / ffmpeg 探測造成系統管理頁卡住。

## 3. 實測結果

### 3.1 JavaScript 語法檢查

| 檔案 | 結果 |
| --- | --- |
| `apps/frontend/js/app.js` | 通過 |
| `apps/frontend/js/admin.js` | 通過 |

### 3.2 Static sync

已執行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\sync-static.ps1
```

結果：通過，`apps/frontend` 已同步到 Go embedded static 目錄。

### 3.3 主程式 build

已執行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build\build_both_releases.ps1
```

產出：

- `artifacts/windows/v1.2.4.9.zip`
- `artifacts/linux/v1.2.4.9.zip`
- `artifacts/linux-arm64/v1.2.4.9.zip`

### 3.4 License Generator build

已執行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build\build_license_generators.ps1
```

產出：

- `artifacts/license-generators/v1.2.4.9/windows-amd64/license-gen.exe`
- `artifacts/license-generators/v1.2.4.9/linux-amd64/license-gen`
- `artifacts/license-generators/v1.2.4.9/linux-arm64/license-gen`

Windows CLI smoke test：

```powershell
.\artifacts\license-generators\v1.2.4.9\windows-amd64\license-gen.exe -id TEST-MACHINE-001 -type iot -duration-days 14
```

結果：通過，成功輸出 `Type: iot`、`Features: [iot]` 與 license key。

### 3.5 二進位嵌入內容檢查

已確認三平台主程式二進位都包含本次 Admin 自動載入修正字串：

- Windows `nms_server.exe`：通過
- Linux amd64 `nms_server_linux_amd64`：通過
- Linux arm64 `nms_server_linux_arm64`：通過

### 3.6 Windows artifact 啟動驗證

使用正式啟動腳本驗證：

```powershell
artifacts\windows\v1.2.4.9\start_nms.ps1
```

結果：

- 服務正常 listen `0.0.0.0:8080`
- `/api/v1/system/info` 回傳版本 `v1.2.4.9`
- `/static/js/admin.js` 已包含「不自動載入 Admin 資料」修正
- `/static/js/app.js` 已限定 Admin tab 綁定範圍

### 3.7 系統管理相關 API 手動驗證

使用預設 admin token 手動呼叫：

| API | 結果 | 耗時 |
| --- | --- | --- |
| `/api/v1/system/host-status` | 通過 | 202 ms |
| `/api/v1/license/machine-id` | 通過 | 57 ms |
| `/api/v1/alerts/settings` | 通過 | 126 ms |

`/api/v1/system/host-status` 實測回傳：

- CPU：30.1%
- RAM：31.50 GB total / 20.71 GB used
- Storage：953.85 GB total / 169.60 GB used
- GPU：維持跳過偵測以保護穩定性

server log 檢查結果：

- 進入頁面時未自動出現 `/api/v1/system/host-status`
- 進入頁面時未自動出現 `/api/v1/license/machine-id`
- 只有手動測試時才出現上述 API 呼叫

## 4. i18n 檢查

已確認：

- `iot.license_notice` 已存在於 `zh-TW`
- 不再使用錯誤的 top-level `iot.license_notice`
- i18n JSON key parity 先前檢查通過：5 個語系，1184 keys

## 5. 已知限制

- Windows 環境執行 `go run ./apps/backend` 時，曾出現暫存 `.exe` `Access is denied`，因此本次以實際 artifact 啟動與 API smoke test 作為主要驗證。
- Host Status 目前為穩定性優先，GPU 偵測在 Admin 頁已跳過，避免 Windows WMI / ffmpeg 探測造成頁面卡住。

## 6. 最終判定

v1.2.4.9 主程式與 License Generator 已重新編譯完成。  
本次針對「看到系統管理就卡住」的問題，已改為進入頁面不自動載入重查 API，並完成 Windows artifact 實際啟動與 API 驗證。

建議交付版本：

- `artifacts/windows/v1.2.4.9.zip`
- `artifacts/linux/v1.2.4.9.zip`
- `artifacts/linux-arm64/v1.2.4.9.zip`
- `artifacts/license-generators/v1.2.4.9/`
