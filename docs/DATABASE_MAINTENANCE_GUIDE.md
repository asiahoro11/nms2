# Management Server (nms_sync) 資料庫維護指南

## 文件概覽

本目錄包含以下資料庫維護工具與文件：

| 檔案名稱 | 說明 | 類型 |
|---------|------|------|
| `nms_sync_database_analysis.md` | 完整的資料庫結構分析報告 | 文件 |
| `check_database_integrity.sql` | 資料庫完整性檢查 SQL 腳本 | SQL |
| `Check-DatabaseIntegrity.ps1` | 資料庫完整性檢查工具（PowerShell） | 工具 |
| `add_performance_indexes.sql` | 效能索引新增腳本 | SQL |
| `DATABASE_MAINTENANCE_GUIDE.md` | 維護指南 | 文件 |

---

## 快速開始

### 1. 檢查資料庫健康度

**使用 PowerShell 工具（推薦）**:

```powershell
cd c:\Users\HP\.gemini\antigravity\playground\azure-universe\nms_sync
.\Check-DatabaseIntegrity.ps1
```

**直接使用 SQLite**:

```powershell
sqlite3 data/nms.db < check_database_integrity.sql > report.txt
```

### 2. 新增效能索引

**首次部署或升級時執行**:

```powershell
sqlite3 data/nms.db < add_performance_indexes.sql
```

### 3. 定期維護

**每週執行一次**:

```powershell
# 優化資料庫
sqlite3 data/nms.db "ANALYZE; VACUUM;"
```

**每月執行一次**:

```powershell
# 清理歷史資料（依需求調整天數）
sqlite3 data/nms.db "DELETE FROM device_metrics WHERE collected_at < datetime('now', '-30 days');"
sqlite3 data/nms.db "DELETE FROM syslogs WHERE received_at < datetime('now', '-90 days');"
sqlite3 data/nms.db "DELETE FROM events WHERE created_at < datetime('now', '-90 days');"
```

---

## 資料庫結構說明

### 主要資料表

1. **devices** - 設備主表
   - 儲存網路設備基本資訊
   - 包含 SNMP 設定與位置座標等

2. **device_interfaces** - 設備介面表
   - 儲存介面資訊與狀態統計
   - 支援頻寬計算

3. **device_metrics** - 效能指標表
   - 儲存 CPU、記憶體、磁碟使用率
   - **時間序列資料，需定期清理**

4. **topology_links** - 拓撲連線表
   - 儲存設備間連線關係
   - 支援手動與自動發現

5. **syslogs** - 系統日誌表
   - 儲存 Syslog 訊息
   - **時間序列資料，需定期清理**

6. **events** - 事件表
   - 儲存系統事件（設備上下線、Port Up/Down）
   - **時間序列資料，需定期清理**

7. **users** - 使用者表
   - 儲存使用者帳號與權限
   - 支援三種角色：admin、editor、viewer

8. **licenses** - 授權表
   - 儲存授權資訊與設備數量限制
   - 支援設備數量限制

9. **system_config** - 系統設定表
   - 儲存全域設定參數

10. **alert_settings** - 告警設定表
    - 儲存各種告警規則設定

### 預設帳號

- **Username**: `admin`
- **Password**: `admin123`
- **首次登入**: 強制變更密碼

---

## 常見問題與解決方法

### 問題 1: 資料庫鎖定錯誤

**症狀**: `database is locked` 錯誤

**原因**:

- 多個程序同時寫入資料庫
- 長時間執行的查詢

**解決方法**:

1. 確認資料庫使用 WAL 模式：

   ```sql
   PRAGMA journal_mode=WAL;
   ```

2. 增加 busy_timeout：

   ```sql
   PRAGMA busy_timeout=30000;
   ```

3. 避免長時間交易。

### 問題 2: 資料庫檔案過大

**症狀**: 資料庫檔案持續增大

**原因**:

- 時間序列資料未定期清理
- 刪除資料後未執行 VACUUM

**解決方法**:

1. 清理歷史資料（參考「定期維護」章節）
2. 執行 VACUUM 回收空間：

   ```sql
   VACUUM;
   ```

### 問題 3: 查詢效能緩慢

**症狀**: 查詢回應時間過長

**原因**:

- 缺少索引
- 統計資料過時
- 資料量過大

**解決方法**:

1. 執行 `add_performance_indexes.sql` 新增索引
2. 更新統計資料：

   ```sql
   ANALYZE;
   ```

3. 清理歷史資料。

### 問題 4: 外鍵約束錯誤

**症狀**: 刪除或更新時出現外鍵錯誤

**原因**:

- 嘗試刪除被其他表參照的資料
- 外鍵約束未正確設定

**解決方法**:

1. 檢查外鍵約束：

   ```sql
   PRAGMA foreign_key_check;
   ```

2. 確認外鍵已啟用：

   ```sql
   PRAGMA foreign_keys = ON;
   ```

3. 使用 CASCADE 刪除（已在 Schema 中設定）。

---

## 日常維護作業

### 備份資料庫

**方法 1: 檔案複製**

```powershell
# 停止服務
Stop-Service Management Server  # 或手動停止服務

# 複製資料庫檔案
Copy-Item data/nms.db data/nms.db.backup

# 啟動服務
Start-Service Management Server
```

**方法 2: SQLite 備份指令**

```powershell
sqlite3 data/nms.db ".backup data/nms.db.backup"
```

### 還原資料庫

```powershell
# 停止服務
Stop-Service Management Server

# 還原備份
Copy-Item data/nms.db.backup data/nms.db -Force

# 啟動服務
Start-Service Management Server
```

### 匯出資料

**匯出為 SQL**:

```powershell
sqlite3 data/nms.db ".dump" > nms_dump.sql
```

**匯出特定資料表為 CSV**:

```powershell
sqlite3 data/nms.db -header -csv "SELECT * FROM devices;" > devices.csv
```

### 修復損毀的資料庫

```powershell
# 1. 備份損毀的資料庫
Copy-Item data/nms.db data/nms.db.corrupted

# 2. 匯出資料
sqlite3 data/nms.db ".dump" > nms_dump.sql

# 3. 建立新資料庫
Remove-Item data/nms.db
sqlite3 data/nms.db < nms_dump.sql

# 4. 驗證完整性
sqlite3 data/nms.db "PRAGMA integrity_check;"
```

---

## 效能監控

### 檢查資料庫大小

```sql
SELECT
    (page_count * page_size) / 1024 / 1024 AS 'Size (MB)'
FROM pragma_page_count(), pragma_page_size();
```

### 檢查各資料表大小

```sql
SELECT
    name AS 'Table',
    (SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND tbl_name=m.name) AS 'Indexes',
    (SELECT COUNT(*) FROM sqlite_sequence WHERE name=m.name) AS 'Rows (approx)'
FROM sqlite_master m
WHERE type='table' AND name NOT LIKE 'sqlite_%'
ORDER BY name;
```

### 檢查索引使用情況

```sql
-- 需要透過 SQLite 查詢計畫分析
EXPLAIN QUERY PLAN SELECT * FROM devices WHERE is_online = 1;
```

---

## 安全性建議

### 1. 定期變更預設密碼

```sql
-- 強制所有使用者重新設定密碼
UPDATE users SET force_change_password = 1;
```

### 2. 停用不活躍的帳號

```sql
-- 停用特定使用者
UPDATE users SET is_active = 0 WHERE username = 'old_user';
```

### 3. 定期檢查授權狀態

```sql
SELECT
    license_key,
    license_type,
    valid_until,
    CASE
        WHEN valid_until IS NULL THEN 'Permanent'
        WHEN datetime(valid_until) > datetime('now') THEN 'Valid'
        ELSE 'Expired'
    END AS 'Status'
FROM licenses
WHERE is_active = 1;
```

### 4. 限制資料庫檔案存取

```powershell
# Windows: 設定 ACL 權限
icacls data\nms.db /grant "Administrators:(F)" /inheritance:r
icacls data\nms.db /grant "SYSTEM:(F)"
```

---

## 維護排程建議

### 每日

- 自動備份資料庫檔案

### 每週

- 執行 `ANALYZE` 更新統計資料
- 檢查資料庫檔案大小
- 檢查錯誤日誌

### 每月

- 執行 `VACUUM` 回收空間
- 清理過期時間序列資料
- 檢查授權有效狀態
- 審查使用者帳號

### 每季

- 執行完整性檢查
- 檢視效能報告
- 評估是否需要新增索引
- 測試備份還原流程

---

## 緊急處理步驟

### 資料庫無法啟動

1. 檢查資料庫檔案是否存在
2. 檢查檔案權限
3. 執行完整性檢查
4. 嘗試從備份還原

### 效能明顯下降

1. 檢查資料庫大小
2. 執行 `ANALYZE` 與 `VACUUM`
3. 清理歷史資料
4. 檢查是否有長時間執行的查詢

### 資料遺失

1. 立即停止服務
2. 不要寫入任何資料
3. 從最近的備份還原
4. 檢查 WAL 檔案是否可恢復

---

## 技術支援

如遇到無法解決的問題時，請提供以下資料：

1. 資料庫完整性檢查報告
2. 錯誤訊息與日誌
3. 資料庫檔案大小
4. Management Server 版本
5. 作業系統版本

---

## 變更記錄

| 日期 | 版本 | 變更內容 |
|------|------|---------|
| 2026-01-30 | 1.0 | 初版發布 |

---

**維護人員**: 請定期更新此文件，確保內容與實際維護流程一致。
