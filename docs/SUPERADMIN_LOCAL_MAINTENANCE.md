# SuperAdmin 本機初始化與復原

SuperAdmin 沒有預設密碼，也不接受一般登入頁、Admin JWT 或遠端忘記密碼流程。初始化與無 TOTP 的密碼重設只能在 NMS 主機本機執行。

## 執行前

1. 停止 NMS 服務，避免工具與服務同時寫入 SQLite。
2. 備份 `data/nms.db`、`data/nms.db-wal` 與 `data/nms.db-shm`（若存在）。
3. 以 NMS 服務帳號或具備資料庫檔案權限的本機帳號執行。
4. 不要把密碼放在命令列、批次檔或操作紀錄中；工具會以遮罩方式互動輸入。

## 第一次初始化

Windows：

```powershell
.\manage_superadmin.bat
```

Windows 使用者可直接雙擊 `manage_superadmin.bat`。工具會自動尋找 `data\nms.db`，並依現況判斷要初始化帳號或變更密碼，不需要輸入參數。

Linux：

```bash
./tools/superadmin-local_linux_amd64 -mode init -db ./data/nms.db
```

ARM64 主機請使用 `superadmin-local_linux_arm64`。

## 忘記密碼時重設

若 SuperAdmin 已啟用 TOTP，可先在隱藏登入視窗使用 TOTP 或復原碼重設。無法使用時，停止 NMS 並在主機本機執行：

```powershell
.\manage_superadmin.bat
```

本機重設會撤銷所有 SuperAdmin 短效工作階段，保留既有 TOTP 設定，並寫入稽核紀錄。完成後重新啟動 NMS，再由已登入的 Admin 連點「系統管理」8 次開啟隱藏登入視窗。

## 權限邊界

- 一般 Admin 只能看到「尚未初始化」狀態，不能設定或修改 SuperAdmin 密碼。
- Admin 建立的其他管理員不能修改內建 Admin 或 SuperAdmin 密碼。
- SuperAdmin 權杖有效時間短、只存在記憶體，且綁定原始 Admin 工作階段。
- 登出、重新整理、Admin 密碼變更或逾時後，SuperAdmin 權限立即失效。
- Branding、Logo、診斷及授權重設等隱藏 API 只接受有效 SuperAdmin 權杖。
