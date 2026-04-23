# UTF-8 ç·¨ç¢¼è¨­å?
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ConfirmPreference = "None"
# NMS GitHub Deployment - Windows Controller Script
# This script orchestrates the deployment from Windows to Linux to GitHub

$ErrorActionPreference = "Stop"

$LinuxHost = "10.100.100.11"
$LinuxUser = "ubuntu"
$NMSPath = "/opt/nms"
$DeployPath = "`$HOME/nms-deploy"
$GitHubRepo = "https://github.com/asiahoro11/Management Server.git"
$GitHubUser = "asiahoro11"
$GitHubToken = "REDACTED-TOKEN-SCRUBBED-FROM-HISTORY"

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "NMS GitHub Deployment Automation" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

# Step 1: Check NMS on Linux
Write-Host "[1/6] Checking NMS installation on Linux server..." -ForegroundColor Yellow
$checkNMS = ssh "$LinuxUser@$LinuxHost" "test -f $NMSPath/nms-server && echo 'OK' || echo 'NOT_FOUND'"
if ($checkNMS -notmatch "OK") {
    Write-Error "NMS not found at $NMSPath"
    exit 1
}
Write-Host "??NMS found on Linux server" -ForegroundColor Green

# Step 2: Prepare deployment directory
Write-Host ""
Write-Host "[2/6] Preparing deployment directory..." -ForegroundColor Yellow
ssh "$LinuxUser@$LinuxHost" "rm -rf $DeployPath && mkdir -p $DeployPath"
Write-Host "??Deployment directory ready" -ForegroundColor Green

# Step 3: Copy files
Write-Host ""
Write-Host "[3/6] Copying NMS files..." -ForegroundColor Yellow
ssh "$LinuxUser@$LinuxHost" @"
cp $NMSPath/nms-server $DeployPath/ && \
chmod +x $DeployPath/nms-server && \
cp -r $NMSPath/frontend $DeployPath/ && \
mkdir -p $DeployPath/data && \
echo 'Files copied'
"@
Write-Host "??NMS files copied" -ForegroundColor Green

# Step 4: Create README
Write-Host ""
Write-Host "[4/6] Creating documentation..." -ForegroundColor Yellow
$readmeContent = @'
# NMS Lite - Network Management System

Production deployment package for NMS Lite.

## Quick Deploy

```bash
git clone https://github.com/asiahoro11/Management Server.git /opt/nms
cd /opt/nms
chmod +x nms-server
./nms-server
```

Access: http://your-server:8080

**Default Login:** admin / admin123

## System Service

```bash
sudo tee /etc/systemd/system/nms.service > /dev/null <<EOF
[Unit]
Description=NMS Lite
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/nms
ExecStart=/opt/nms/nms-server
Restart=always

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now nms
```

## Features

- SNMP Device Monitoring
- Network Topology Visualization
- Real-time Interface Statistics
- Alert Management
- License Management

## Update

```bash
cd /opt/nms && git pull && sudo systemctl restart nms
```
'@

# Upload README
$readmeContent | ssh "$LinuxUser@$LinuxHost" "cat > $DeployPath/README.md"
Write-Host "??Documentation created" -ForegroundColor Green

# Step 5: Create .gitignore
Write-Host ""
Write-Host "[5/6] Creating .gitignore..." -ForegroundColor Yellow
@"
data/*.db
data/*.db-shm
data/*.db-wal
*.log
*.tmp
.DS_Store
"@ | ssh "$LinuxUser@$LinuxHost" "cat > $DeployPath/.gitignore"
Write-Host "??.gitignore created" -ForegroundColor Green

# Step 6: Git push
Write-Host ""
Write-Host "[6/6] Pushing to GitHub..." -ForegroundColor Yellow

$gitUrl = "https://${GitHubUser}:${GitHubToken}@github.com/asiahoro11/Management Server.git"
$gitCommands = "cd $DeployPath && git init && git branch -M main && git config user.name '$GitHubUser' && git config user.email '${GitHubUser}@users.noreply.github.com' && git remote add origin '$gitUrl' && git add . && git commit -m 'Deploy NMS Lite - Production Build' && git push -u origin main --force && echo 'PUSH_SUCCESS'"

ssh "$LinuxUser@$LinuxHost" $gitCommands

Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "??Deployment Complete!" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green
Write-Host ""
Write-Host "?“¦ Repository: https://github.com/asiahoro11/Management Server" -ForegroundColor Cyan
Write-Host ""
Write-Host "To deploy on a new server:" -ForegroundColor Yellow
Write-Host "  git clone https://github.com/asiahoro11/Management Server.git /opt/nms" -ForegroundColor White
Write-Host "  cd /opt/nms" -ForegroundColor White
Write-Host "  chmod +x nms-server" -ForegroundColor White
Write-Host "  ./nms-server" -ForegroundColor White
Write-Host ""

