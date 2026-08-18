# 資料庫與程式完整性封鎖

## 目的

偵測到高可信度的資料庫或執行檔完整性異常時，NMS 進入可逆的
fail-closed 模式。此機制不刪除、不覆寫資料，也不使用邏輯炸彈。

封鎖後：

- 所有 `/api/` 請求回傳 HTTP `423 Locked`。
- SQLite connection 啟用 `PRAGMA query_only = ON`，拒絕後續資料寫入。
- 若在啟動前已封鎖，直接用 SQLite 唯讀模式開庫，不執行 migration，且不啟動
  DB worker、Pinger、Scheduler、Syslog、Cloud、Time Sync、Camera health、
  License health、IoT background loop 與 NVR auto-resume。
- 靜態前端檔案仍可讀取，避免修改或刪除所有版面。
- `/api/v1/system/integrity/status` 保留為唯讀狀態端點，不查詢資料庫。
- `<database path>.integrity-lock.json` 保存封鎖狀態，重新啟動後仍會封鎖。

## 自動檢查

每五分鐘及伺服器啟動時檢查；封鎖標記與執行檔雜湊會在資料庫 migration
之前先檢查：

1. SQLite file header。
2. `PRAGMA quick_check`。
3. 是否存在先前的 integrity lock marker。
4. 若設定 `NMS_EXPECTED_BINARY_SHA256`，驗證目前執行檔 SHA-256。

只有 SQLite 明確回報損毀／非資料庫，或可信任執行檔雜湊不符時才會全域
封鎖。暫時性的 `busy`／`locked` 不會觸發全域封鎖。

大量登入失敗沿用帳號與來源 IP rate limit，不會觸發全域資料庫封鎖，
避免攻擊者利用登入請求造成拒絕服務。

## 啟用執行檔完整性驗證

Windows：

```powershell
$env:NMS_EXPECTED_BINARY_SHA256 = (Get-FileHash -Algorithm SHA256 .\nms_server.exe).Hash
.\nms_server.exe
```

Linux：

```bash
export NMS_EXPECTED_BINARY_SHA256="$(sha256sum ./nms_server | awk '{print $1}')"
./nms_server
```

正式環境應由 Windows Service、systemd、容器平台或秘密管理服務注入可信任
雜湊，不應把可信任值放在可與程式一起被修改的同一個目錄。

## 離線復原

系統不提供遠端解除封鎖 API。復原流程：

1. 停止 NMS。
2. 保全並複製資料庫、WAL、SHM、log 與 integrity marker 作為鑑識資料。
3. 修復或由可信任加密備份還原資料庫；若是程式異常，重新部署可信任 binary。
4. 重新計算並更新部署環境中的 `NMS_EXPECTED_BINARY_SHA256`。
5. 確認原因排除後，離線刪除 `<database path>.integrity-lock.json`。
6. 啟動 NMS；完整性檢查仍失敗時會立即重新封鎖。

## 邊界

`quick_check` 能偵測 SQLite 損毀，但無法區分「合法應用程式更新」與
「攻擊者透過合法 SQLite 寫入所做的資料修改」。若要驗證有效但未授權的
資料異動，下一階段需導入不可變 audit chain、外部簽章／HSM，以及 live
database encryption；不應用會破壞資料的邏輯炸彈替代。
