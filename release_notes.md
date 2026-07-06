# Management Server 發行說明

## v1.2.4.8sp0006 | 2026-06-22

### 修正目的

修正進入「系統管理」後，後端或前端請求卡住，連帶造成設備清單、授權 UUID 等其他頁面長時間顯示載入中的問題。

### 主要修正

- 進入「系統管理」頁面時不再自動呼叫任何管理 API，避免只看見頁面就觸發主機狀態、授權、稽核或其他管理資料載入。
- 系統管理分頁改為使用者點選分頁後才載入該分頁資料，並保留單一請求逾時保護。
- `/api/v1/admin/system/host-status` 改為輕量回應，不再使用可能阻塞的主機 CPU、記憶體、磁碟與 GPU 探測。
- 系統 UUID 查詢加入逾時與快取，避免 Windows `wmic`、`reg` 或 Linux system command 異常時拖住授權頁面。
- 前端 API helper 加入逾時控制，單一異常請求不會讓其他頁面永久停在載入中。
- 全站頁面與表格加入 frame containment 與內部捲動範圍，避免稽核表格或大型資料表撐開整個系統畫面。

### 原因說明

修改前正常，是因為原本進入系統管理時不會主動載入多個高風險管理 API。前一版修改後，系統管理頁面加入了自動載入管理資料的行為；即使後來縮小成只載入特定分頁，主機狀態仍會呼叫底層系統探測。這些探測在部分 Windows 或 Linux 環境可能卡住，導致後端 handler 佔住請求，前端其他頁面也跟著等不到回應。

本版修正方向是回到較安全的行為：進入系統管理不自動打管理 API；需要資料時才按分頁載入；主機狀態 API 本身也不再執行可能阻塞的系統探測。

### 發行檔

- Windows amd64：`artifacts/windows/v1.2.4.8sp0006.zip`
- Linux amd64：`artifacts/linux/v1.2.4.8sp0006.zip`
- Linux arm64：`artifacts/linux-arm64/v1.2.4.8sp0006.zip`

### 建置設定

- Release profile：`protected-internal`
- Go binary obfuscation：已啟用
- JavaScript minification：已啟用
- HTML minification：已啟用

---
