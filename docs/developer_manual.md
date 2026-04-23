# NMS 開發者與部署手冊 (Secure Build Guide)

本手冊說明如何使用自動化腳本生成「加密版」部署檔案，並將其部署至 Linux 伺服器。

## 1. 準備工作 (Prerequisites)

在您的 Windows 開發機上，請確保已安裝：

1.  **Go 語言 (Golang)**: 用於編譯後端。
2.  **Python**: 用於執行前端混淆 (Minification) 腳本。
3.  **PowerShell**: 用於執行自動化建置腳本。

## 2. 執行建置 (Build Process)

在專案根目錄下，開啟 PowerShell 並執行：

```powershell
# 若遇到權限錯誤，請使用以下指令繞過限制：
powershell -ExecutionPolicy Bypass -File .\build_release.ps1
```

### 腳本執行動作：
1.  **清理環境**: 清空 `dist` 目錄 (但會保留 `.git` 設定，方便版控)。
2.  **後端編譯**:
    *   設定目標系統為 Linux (`GOOS=linux`)。
    *   停用 CGO (`CGO_ENABLED=0`)。
    *   編譯出單一執行檔 `dist/nms-server`。
3.  **前端處理**:
    *   複製 `frontend` 資料夾到 `dist/frontend`。
    *   呼叫 Python 腳本將所有 `js` 與 `css` 檔案進行混淆 (移除空白與註解)。

### 產出結果：
執行成功後，您會看到一個 `dist` 資料夾，內容如下：

```text
dist/
├── nms-server      (Linux 執行檔，已加密)
├── frontend/       (前端檔案，已混淆)
│   ├── index.html
│   ├── js/ ...
│   └── css/ ...
└── data/           (空的資料目錄)
```

## 3. 部署方法 A: 直接上傳 (Direct Upload)

將 `dist` 資料夾內的**所有內容**上傳至 Linux 伺服器 (如 `/opt/nms`)，然後賦予權限並啟動。

## 4. 部署方法 B: GitHub 自動部署 (GitHub Deployment) ✅ 推薦

您可以將 `dist` 資料夾當作一個獨立的 Git 倉庫，專門用來發布版本。

### 首次設定 (Windows 端)
1.  執行打包腳本 `.\build_release.ps1`。
2.  進入 `dist` 資料夾：`cd dist`
3.  初始化 Git 並連結到您的「部署用」遠端倉庫 (請先在 GitHub 建立一個**新的空倉庫**，例如 `nms-deploy`)：
    ```bash
    git init
    git branch -M main
    git remote add origin https://github.com/YourName/nms-deploy.git
    ```

### 發布新版 (Windows 端)
每次開發完成後：
1.  執行 `.\build_release.ps1` (這會更新 `dist` 內的檔案，但保留 `.git`)。
2.  進入 `dist` 並推送：
    ```bash
    cd dist
    git add .
    git commit -m "Update deployment version"
    git push -u origin main
    ```

### 伺服器端部署 (Linux 端)
1.  首次下載：
    ```bash
    git clone https://github.com/YourName/nms-deploy.git /opt/nms
    ```
2.  日後更新：
    ```bash
    cd /opt/nms
    git pull
    chmod +x nms-server
    sudo systemctl restart nms
    ```

## 5. 維護與更新 (Maintenance)

### 程式碼更新
1.  在 Windows 開發機上修改程式碼 (Go 或 HTML/JS)。
2.  再次執行 `.\build_release.ps1` 產生新的加密檔案。
3.  使用上述「方法 B」推送至 GitHub。
4.  在伺服器上 `git pull` 並重啟服務。

### 資料庫備份
資料庫檔案位於 `/opt/nms/data/nms.db`。此檔案與程式碼分開，更新程式**不會**影響現有資料。
建議定期備份該檔案。
