# NMS Linux 部署手冊 (Deployment Manual)

本文件提供從零開始在 Linux 伺服器部署 NMS 管理系統的完整流程。部署方式使用 GitHub 上預先編譯好的執行檔，提供快速部署體驗。

## 1. 系統需求 (System Requirements)

在開始之前，請確保您的伺服器符合以下規格：

- **作業系統**: Linux (建議 Ubuntu 22.04 LTS / Debian 11 / CentOS 8) — 架構須為 amd64 (x86_64)
- **硬體資源**:
  - CPU: 2 Core 以上
  - RAM: 建議 1GB 以上（最低 512MB）
  - 硬碟: 20GB 以上（主要用於儲存 Log）
- **網路埠 (Ports)**:
  - TCP 8080 (Web 管理介面)
  - UDP 514 (Syslog 接收)
  - UDP 162 (SNMP Trap 接收)

## 2. 環境準備 (Environment Setup)

請以擁有 `sudo` 權限的使用者登入伺服器，並安裝必要的相依套件：

### Ubuntu / Debian 系統

```bash
# 1. 更新系統套件清單
sudo apt update && sudo apt upgrade -y

# 2. 安裝必要工具 (Git, Curl, SNMP MIBs 下載器)
# snmp 與 snmp-mibs-downloader 用於讓系統能解析標準 MIB 資料
sudo apt install -y git curl snmp snmp-mibs-downloader

# 3. 啟用 Non-Free MIBs (Ubuntu 需要此步驟才能下載完整 MIB)
sudo sed -i 's/mibs :/# mibs :/g' /etc/snmp/snmp.conf
sudo download-mibs
```

### CentOS / RHEL 系統

```bash
# 安裝 Git 與相關工具
sudo dnf install -y git curl net-snmp net-snmp-utils
```

## 3. 部署步驟 (Deployment Steps)

我們將 NMS 部署至 `/opt/nms` 目錄下。

### 步驟 1: 下載程式 (Clone Repository)

**注意**: 由於 `Management Server` 為私有倉庫 (Private Repo)，系統會要求輸入帳號密碼：

- **Username**: 您的 GitHub 帳號
- **Password**: 請輸入 **Personal Access Token (PAT)**，而非登入密碼。

為方便自動部署，亦可以將 Token 包含在 URL 中（請注意 Token 安全）：

```bash
# 1. 切換到 /opt 目錄
cd /opt

# 2. 使用 Token 下載 (格式: https://<Token>@github.com/...)
# 請將 <YOUR_PAT> 替換為您的 Personal Access Token
sudo git clone https://<YOUR_PAT>@github.com/asiahoro11/management-server.git nms

# 或直接使用 Token 連線 clone，執行時會提示密碼再貼上 Token
# sudo git clone https://github.com/asiahoro11/management-server.git nms

# 3. 進入目錄
cd nms

# 4. 賦予主程式執行權限
sudo chmod +x nms-server
```

### 步驟 2: 權限設定 (Permissions)

為了安全考量，建議建立專用使用者執行 NMS，並確認權限正確。這裡示範使用一般使用者或當前使用者：

```bash
# 確認資料庫目錄存在
sudo mkdir -p data

# 確保當前使用者有權限寫入 (以 ubuntu 使用者執行為例)
sudo chown -R $USER:$USER /opt/nms
```

> **注意**: NMS 需要監聽 UDP 514 Port (Syslog)，在 Linux 上，非 root 使用者預設無法監聽 1024 以下的 Port。
> 可以使用以下指令賦予程式綁定低位 Port 的能力，就不需要用 root 執行：

```bash
sudo setcap 'cap_net_bind_service=+ep' /opt/nms/nms-server
```

## 4. 系統服務設定 (Service Setup)

設定 Systemd 服務，讓 NMS 在開機時自動啟動，並在崩潰時自動重啟。

### 建立服務檔案

請執行以下指令建立 `/etc/systemd/system/nms.service`：

```bash
sudo tee /etc/systemd/system/nms.service > /dev/null <<'EOF'
[Unit]
Description=Management Server
After=network.target

[Service]
# 服務類型
Type=simple

# 執行路徑與指令
WorkingDirectory=/opt/nms
ExecStart=/opt/nms/nms-server

# 執行身份 (請修改為您實際使用的使用者，例如 ubuntu 或 root)
# 若您已設定 setcap，這裡可以使用一般使用者 (如 ubuntu)
# 若未設定 setcap 且需要聽 514 port，請使用 User=root
User=ubuntu
Group=ubuntu

# 重啟策略設定
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
```

### 啟動服務

```bash
# 1. 重新載入 Systemd 設定
sudo systemctl daemon-reload

# 2. 設定服務開機自動啟動
sudo systemctl enable nms

# 3. 立即啟動服務
sudo systemctl start nms

# 4. 檢查服務狀態 (應顯示 Active: active (running))
sudo systemctl status nms
```

## 5. 防火牆設定 (Firewall)

若您的伺服器有啟用 `ufw` 防火牆，請開放必要的埠：

```bash
sudo ufw allow 8080/tcp  # Web UI
sudo ufw allow 514/udp   # Syslog
sudo ufw allow 162/udp   # SNMP Trap
sudo ufw reload
```

## 6. 驗證與登入 (Verification)

1. 打開瀏覽器，輸入 `http://<您的伺服器IP>:8080` (例如 `http://192.168.1.100:8080`)。
2. 您應該能看到 NMS 的登入畫面。
3. **預設帳號**: `admin`
4. **預設密碼**: `admin123`

> ⚠️ **警告**: 首次登入後，請務必至「系統管理」修改管理員密碼。

## 7. 維護與更新 (Maintenance)

當有新版本發布時，您可以透過以下步驟更新：

```bash
# 1. 進入目錄
cd /opt/nms

# 2. 下載最新程式
git pull

# 3. 重新賦予執行權限 (以防萬一)
chmod +x nms-server

# 4. 重啟服務
sudo systemctl restart nms
```

---

**檔案結構說明**:

- `/opt/nms/nms-server`: 主程式執行檔
- `/opt/nms/data/nms.db`: 資料庫檔案（請定期備份此檔案）
- `/opt/nms/frontend/`: 網頁前端介面檔案
- `/opt/nms/config.yaml`: 設定檔（選用，預設使用內建設定）
