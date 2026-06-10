# Management Server 資料庫與備份加密手冊

版本: `v1.2.4.8`

## 範圍

本手冊說明客戶如何處理備份加密 密碼保護與安全保存  
實際是否啟用完整資料庫加密需依客戶授權與部署政策確認

## 基本原則

- 備份檔必須加密
- 加密密碼不得與系統登入密碼相同
- 密碼需由客戶密碼庫保存
- 不要把密碼寫在 script repo email ticket 或文件內

## 建議密碼規則

- 至少 12 字元
- 包含大小寫 英數與符號
- 不使用公司名稱 專案名稱 設備名稱
- 每次交付或維護後可輪替

## 加密備份流程

若系統已啟用加密備份 API  使用 admin token 呼叫

```http
POST /api/v1/system/backup/encrypted
Authorization: Bearer <admin-jwt>
Content-Type: application/json
```

```json
{
  "password": "<backup-password>"
}
```

## 還原流程

```http
POST /api/v1/system/restore/encrypted
Authorization: Bearer <admin-jwt>
Content-Type: multipart/form-data
```

欄位

| 欄位 | 說明 |
| --- | --- |
| `file` | 加密備份檔 |
| `password` | 備份密碼 |

## 驗收

- 備份檔沒有明文資料
- 錯誤密碼無法還原
- 正確密碼可在測試環境還原
- 還原後 `/api/v1/system/info` 正常
- 還原後使用者 權限 device topology audit 資料存在

## 保存政策

- 至少保留每日 7 份 每週 4 份 每月 3 份
- 至少一份離線或異地保存
- 定期抽測還原
- 過期備份需安全刪除

