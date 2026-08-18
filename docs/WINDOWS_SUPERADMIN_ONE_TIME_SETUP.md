# Windows SuperAdmin 一次性設定手冊

本手冊用於 Windows 版 NMS 第一次建立 SuperAdmin，或經授權後在主機本機重設密碼。

SuperAdmin 沒有預設密碼，密碼也不會寫在程式、設定檔或資料庫初始化腳本內。設定工具僅供本機一次性使用；完成設定並確認登入成功後，請將工具刪除。

## 一、執行前準備

1. 將 NMS 發布包解壓縮到正式安裝目錄。
2. 先啟動一次 NMS，確認已建立 `data\nms.db`。
3. 完整停止 NMS 程式或 Windows 服務。
4. 備份 `data\nms.db`、`data\nms.db-wal` 與 `data\nms.db-shm`（存在時才需備份）。
5. 確認目前登入 Windows 的帳號有權讀寫 NMS 安裝目錄與資料庫。

> 注意：設定期間不要啟動 NMS，避免兩個程序同時寫入 SQLite。

## 二、設定 SuperAdmin

1. 在 NMS 安裝目錄雙擊 `manage_superadmin.bat`。
2. 工具會自動尋找 `data\nms.db`，並判斷要建立 SuperAdmin 或變更既有密碼。
3. 輸入新的 SuperAdmin 密碼，再輸入一次確認。
4. 密碼至少 12 個字元，並同時包含英文大寫、英文小寫、數字及符號；不要包含 `SuperAdmin` 字樣。
5. 畫面出現「SuperAdmin 密碼已安全更新」後關閉工具。

密碼輸入時不會顯示在畫面，也不會出現在命令列參數或設定檔中。

## 三、驗證設定結果

1. 重新啟動 NMS，使用一般 Admin 登入。
2. 連續點選「系統管理」8 次，開啟 SuperAdmin 隱藏登入視窗。
3. 使用 `SuperAdmin` 與剛設定的密碼登入。
4. 若帳號已啟用 TOTP，需一併輸入 TOTP 驗證碼；未啟用時只需帳號與密碼。
5. 確認可以進入 SuperAdmin 專用設定後登出。

## 四、驗證成功後刪除一次性工具

確認 SuperAdmin 可以登入後，請刪除：

```text
manage_superadmin.bat
tools\superadmin-local.exe
```

刪除工具不會刪除 SuperAdmin 帳號，也不會影響已設定的密碼或 TOTP。不要把工具複製到共用資料夾、桌面、郵件或雲端硬碟長期保存。

## 五、忘記密碼時

- 若仍可使用 TOTP 或復原碼，依隱藏登入視窗提供的流程處理。
- 若完全無法登入，需由授權維護人員在 NMS 主機本機重設。
- 從原始發布包重新取得兩個一次性工具檔案前，先依發布的 SHA-256 清單驗證壓縮檔完整性。
- 重設完成並確認登入後，再次刪除兩個工具檔案。

本機重設會撤銷既有 SuperAdmin 短效工作階段、清除未完成的登入挑戰、保留既有 TOTP 設定，並在資料庫寫入稽核紀錄。

## 六、常見問題

### 顯示找不到資料庫

先啟動一次 NMS 建立 `data\nms.db`，停止 NMS 後，再從發布包根目錄執行 `manage_superadmin.bat`。

### 顯示資料庫無法開啟或被鎖定

確認 NMS 主程式與 Windows 服務都已停止，再重新執行。不要直接刪除資料庫鎖定檔。

### Windows Defender 或 SmartScreen 顯示警告

先核對發布包 SHA-256，不要直接略過警告。正式對外版本應使用公司 Authenticode 憑證簽章；未簽章版本無法保證不出現 SmartScreen 提示。

### 可以把密碼寫進批次檔嗎？

不可以。密碼只能在本機工具的遮罩輸入畫面中輸入，禁止放入批次檔、命令列、工單、聊天記錄或系統設定檔。
