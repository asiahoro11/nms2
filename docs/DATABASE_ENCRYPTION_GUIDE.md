# Management Server 資料庫加密功能說明

## 概述

Management Server v1.0.9sp2 提供資料庫加密功能，包含兩層安全保護：

1. **備份檔案加密** (AES-256-GCM) - 使用者自定義密碼加密備份檔
2. **資料庫本身加密** (SQLCipher) - 防止直接存取 nms.db

## 已實作功能

### 備份檔案加密 (AES-256-GCM)

**功能說明**:

- 使用者可設定 8 字元以上的加密密碼
- 備份檔案使用 AES-256-GCM 加密
- 密碼透過 PBKDF2 (100,000 iterations) 衍生加密金鑰
- 支援加密備份匯出與還原

**API 端點**:

```
POST /api/v1/system/backup/encrypted
POST /api/v1/system/restore/encrypted
POST /api/v1/system/encryption/password
GET  /api/v1/system/encryption/status
```

**使用方式**:

1. 管理員登入系統
2. 進入「系統管理」→「備份與還原」
3. 點擊「加密備份」
4. 輸入加密密碼（至少 8 字元）
5. 下載 `.enc` 加密檔案

**還原流程**:

1. 進入「備份與還原」
2. 選擇「還原加密備份」
3. 上傳 `.enc` 檔案
4. 輸入解密密碼
5. 確認還原（系統將自動重啟）

### SQLite 資料庫加密 (待完整整合)

**注意**: SQLCipher 整合需要：

1. 使用 `github.com/mutecomm/go-sqlcipher/v4` 替代標準 `sqlite3`
2. 修改所有資料庫連線使用 SQLCipher driver
3. 重新編譯並啟用 CGO 支援

**替代方案**:

- 使用作業系統層級加密 (BitLocker, LUKS)
- 使用備份檔案加密（已實作）

## 安全性說明

### 加密強度

1. **AES-256-GCM**:
   - 軍事等級加密演算法
   - Galois/Counter Mode 提供認證加密
   - 防止篡改與重放攻擊

2. **PBKDF2 金鑰衍生**:
   - 100,000 次迭代
   - SHA-256 雜湊
   - 防止暴力破解

3. **隨機 Nonce**:
   - 每次加密使用不同的 nonce
   - 確保相同內容產生不同密文

### 密碼建議

- **最少長度**: 8 字元（系統強制）
- **建議長度**: 16+ 字元
- **建議組合**:
  - 大小寫字母
  - 數字
  - 特殊符號
  - 避免字典詞彙

**範例強密碼**: `Nms@2026!Secure#Backup`

### 密碼管理

⚠️ **重要警告**:

- 系統不會儲存或顯示加密密碼
- 密碼遺失即**無法**還原加密備份
- 請務必將密碼記錄於安全位置

**建議做法**:

1. 使用密碼管理工具（KeePass, 1Password）
2. 將密碼寫入文件並實體保存
3. 定期測試備份還原流程

## 使用情境

### 情境 1: 定期加密備份

```bash
# 每日自動加密備份腳本
curl -X POST http://localhost:8080/api/v1/system/backup/encrypted \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"password": "YOUR_SECURE_PASSWORD"}' \
  -o "backup_$(date +%Y%m%d).enc"
```

### 情境 2: 遷移至新伺服器

1. 原伺服器匯出加密備份
2. 傳輸 `.enc` 檔案到新伺服器
3. 新伺服器完成安裝 Management Server
4. 使用「還原加密備份」功能

### 情境 3: 異地備份

1. 定期匯出加密備份
2. 上傳至雲端儲存服務（Google Drive, Dropbox）
3. 即使雲端帳號被盜，仍需密碼才能解密

## 資料庫 Schema 更新

```sql
-- 加密狀態追蹤
INSERT INTO system_config (config_key, config_value, description)
VALUES ('db_encryption_enabled', '0', 'Database encryption status');
```

## 前端整合 (TODO)

需要在管理介面加入：

### 備份管理頁面增強

```javascript
// 加密備份匯出
async function exportEncryptedBackup() {
    const password = prompt('請輸入加密密碼 (至少 8 字元):');
    if (!password || password.length < 8) {
        showToast('密碼長度必須至少 8 字元', 'error');
        return;
    }

    const confirmPassword = prompt('請再次輸入密碼確認');
    if (password !== confirmPassword) {
        showToast('兩次密碼不一致', 'error');
        return;
    }

    try {
        const response = await fetch('/api/v1/system/backup/encrypted', {
            method: 'POST',
            headers: {
                'Authorization': 'Bearer ' + getToken(),
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ password })
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '加密備份失敗');
        }

        // Download file
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `nms_backup_encrypted_${new Date().toISOString().slice(0,10)}.enc`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(url);

        showToast('加密備份已下載', 'success');
    } catch (error) {
        showToast(error.message, 'error');
    }
}

// 加密備份還原
async function restoreEncryptedBackup() {
    const fileInput = document.getElementById('encrypted-backup-file');
    const password = document.getElementById('decrypt-password').value;

    if (!fileInput.files[0]) {
        showToast('請選擇備份檔案', 'error');
        return;
    }

    if (!password) {
        showToast('請輸入解密密碼', 'error');
        return;
    }

    if (!confirm('還原備份將替換當前資料庫並重啟系統，確認繼續？')) {
        return;
    }

    const formData = new FormData();
    formData.append('backup_file', fileInput.files[0]);
    formData.append('password', password);

    try {
        const response = await fetch('/api/v1/system/restore/encrypted', {
            method: 'POST',
            headers: {
                'Authorization': 'Bearer ' + getToken()
            },
            body: formData
        });

        const result = await response.json();

        if (result.success) {
            showToast('還原成功，系統將於 5 秒後重啟', 'success');
            setTimeout(() => {
                window.location.href = '/login?restored=true';
            }, 5000);
        } else {
            showToast(result.error || '還原失敗', 'error');
        }
    } catch (error) {
        showToast('還原失敗: ' + error.message, 'error');
    }
}
```

### HTML 介面範例

```html
<div class="backup-section">
    <h3>加密備份</h3>

    <!-- 匯出加密備份 -->
    <div class="card">
        <h4>匯出加密備份</h4>
        <p>使用自定義密碼加密備份檔案，提供額外安全保護。</p>
        <button onclick="exportEncryptedBackup()" class="btn btn-primary">
            匯出加密備份
        </button>
    </div>

    <!-- 還原加密備份 -->
    <div class="card">
        <h4>還原加密備份</h4>
        <div class="form-group">
            <label>選擇加密備份檔案 (.enc)</label>
            <input type="file" id="encrypted-backup-file" accept=".enc">
        </div>
        <div class="form-group">
            <label>解密密碼</label>
            <input type="password" id="decrypt-password" placeholder="請輸入解密密碼">
        </div>
        <button onclick="restoreEncryptedBackup()" class="btn btn-warning">
            還原加密備份
        </button>
    </div>
</div>
```

## 測試驗證

### 功能測試清單

- [ ] 加密備份匯出成功
- [ ] 密碼長度驗證（少於 8 字元應拒絕）
- [ ] 正確密碼可成功解密
- [ ] 錯誤密碼應拒絕還原
- [ ] 還原後資料完整性
- [ ] 系統自動重啟
- [ ] Audit Log 記錄

### 測試腳本

```bash
#!/bin/bash
# 測試加密備份功能

TOKEN="your-admin-token"
API_URL="http://localhost:8080/api/v1"

# 1. 匯出加密備份
echo "測試 1: 匯出加密備份"
curl -X POST "$API_URL/system/backup/encrypted" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"password": "TestPass123!"}' \
  -o test_backup.enc

# 2. 檢查檔案大小
echo "測試 2: 檢查備份檔案"
ls -lh test_backup.enc

# 3. 檢查加密狀態
echo "測試 3: 檢查加密狀態"
curl -X GET "$API_URL/system/encryption/status" \
  -H "Authorization: Bearer $TOKEN"

echo "測試完成"
```

## 疑難排解

### 問題: 密碼正確但無法解密

**可能原因**:

- 檔案在傳輸中損毀
- 使用不同版本的加密演算法

**解決方法**:

- 重新下載備份檔案
- 檢查檔案 MD5/SHA256 checksum

### 問題: 還原後系統無法啟動

**可能原因**:

- 資料庫版本不相容
- 資料庫結構變更

**解決方法**:

- 檢查 `data/nms.db.before_restore_*` 備份檔
- 考慮使用舊版資料庫
- 聯繫技術支援

## 未來改進

1. **SQLCipher 完整整合**
   - 啟用 CGO 編譯支援
   - 整合 SQLCipher library

2. **密鑰管理增強**
   - 支援 HSM (Hardware Security Module)
   - 支援 Key Rotation

3. **自動備份加密**
   - 定期自動加密備份
   - 自動上傳至雲端

4. **多使用者備份密碼**
   - 支援多個授權使用者
   - 每個使用者獨立的密碼

## 參考資料

- AES-GCM: https://en.wikipedia.org/wiki/Galois/Counter_Mode
- PBKDF2: https://en.wikipedia.org/wiki/PBKDF2
- SQLCipher: https://www.zetetic.net/sqlcipher/

---

**Management Server Development Team**
2026-02-14
