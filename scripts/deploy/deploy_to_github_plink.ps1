# UTF-8 ç·¨ç¢¼è¨­å?
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ConfirmPreference = "None"
# NMS GitHub Deployment - Using PuTTY plink
# This script uses plink for more reliable SSH connections

param(
    [string]$PlinkPath = "plink.exe"  # Assumes plink is in PATH, otherwise specify full path
)

$ErrorActionPreference = "Stop"

# Configuration
$LinuxHost = "10.100.100.11"
$LinuxUser = "ubuntu"
$LinuxPassword = "ji394Tiffany"
$NMSPath = "/opt/nms"
$DeployPath = '$HOME/nms-deploy'
$GitHubUser = "asiahoro11"
$GitHubToken = "REDACTED-TOKEN-SCRUBBED-FROM-HISTORY"
$GitHubRepo = "https://github.com/asiahoro11/Management Server.git"

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "NMS GitHub Deployment (via plink)" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

# Check if plink is available
try {
    $null = & $PlinkPath -V 2>&1
    Write-Host "??Found plink at: $PlinkPath" -ForegroundColor Green
}
catch {
    Write-Error "plink not found! Please install PuTTY or specify path with -PlinkPath parameter"
    Write-Host "Download from: https://www.chiark.greenend.org.uk/~sgtatham/putty/latest.html" -ForegroundColor Yellow
    exit 1
}

# Function to execute command via plink
function Invoke-PlinkCommand {
    param(
        [string]$Command,
        [string]$Description
    )
    
    Write-Host $Description -ForegroundColor Yellow
    
    $result = echo y | & $PlinkPath -batch -pw $LinuxPassword "${LinuxUser}@${LinuxHost}" $Command 2>&1
    
    if ($LASTEXITCODE -ne 0 -and $LASTEXITCODE -ne $null) {
        Write-Warning "Command may have issues, but continuing..."
    }
    
    return $result
}

Write-Host ""

# Step 1: Check NMS
Write-Host "[1/7] Checking NMS installation..." -ForegroundColor Cyan
$checkResult = Invoke-PlinkCommand "test -f $NMSPath/nms-server && echo 'NMS_FOUND' || echo 'NMS_NOT_FOUND'" "Verifying NMS..."
if ($checkResult -notmatch "NMS_FOUND") {
    Write-Error "NMS not found at $NMSPath on Linux server"
    exit 1
}
Write-Host "??NMS found" -ForegroundColor Green

# Step 2: Check NMS status
Write-Host ""
Write-Host "[2/7] Checking NMS service status..." -ForegroundColor Cyan
$statusResult = Invoke-PlinkCommand "systemctl is-active nms 2>/dev/null || pgrep -f nms-server > /dev/null && echo 'RUNNING' || echo 'NOT_RUNNING'" "Checking service..."
if ($statusResult -match "RUNNING") {
    Write-Host "??NMS is running" -ForegroundColor Green
}
else {
    Write-Warning "NMS service not detected, but continuing..."
}

# Step 3: Prepare deployment directory
Write-Host ""
Write-Host "[3/7] Preparing deployment directory..." -ForegroundColor Cyan
Invoke-PlinkCommand "rm -rf $DeployPath && mkdir -p $DeployPath && echo 'DIR_READY'" "Creating clean directory..."
Write-Host "??Deployment directory ready" -ForegroundColor Green

# Step 4: Copy NMS files
Write-Host ""
Write-Host "[4/7] Copying NMS files..." -ForegroundColor Cyan
$copyCommand = @"
cp $NMSPath/nms-server $DeployPath/ && \
chmod +x $DeployPath/nms-server && \
cp -r $NMSPath/frontend $DeployPath/ && \
mkdir -p $DeployPath/data && \
echo 'FILES_COPIED'
"@
$copyResult = Invoke-PlinkCommand $copyCommand "Copying binary and frontend..."
if ($copyResult -match "FILES_COPIED") {
    Write-Host "??Files copied successfully" -ForegroundColor Green
}
else {
    Write-Error "Failed to copy files"
    exit 1
}

# Step 5: Get file info
Write-Host ""
Write-Host "[5/7] Getting file information..." -ForegroundColor Cyan
$sizeResult = Invoke-PlinkCommand "du -h $DeployPath/nms-server 2>/dev/null | cut -f1" "Checking binary size..."
$frontendCount = Invoke-PlinkCommand "find $DeployPath/frontend -type f 2>/dev/null | wc -l" "Counting frontend files..."
Write-Host "  Binary size: $($sizeResult -replace "`n|`r",'')" -ForegroundColor Gray
Write-Host "  Frontend files: $($frontendCount -replace "`n|`r",'')" -ForegroundColor Gray

# Step 6: Create documentation
Write-Host ""
Write-Host "[6/7] Creating documentation..." -ForegroundColor Cyan

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
- Real-time Statistics
- Alert Management
- License Management

## Update
```bash
cd /opt/nms && git pull && sudo systemctl restart nms
```
'@

# Upload README
$readmeContent | Out-File -FilePath "README.tmp" -Encoding UTF8 -NoNewline
$readmeEscaped = $readmeContent -replace '"', '\"' -replace '`', '\\`' -replace '\$', '\$'
Remove-Item "README.tmp" -ErrorAction SilentlyContinue

Invoke-PlinkCommand "cat > $DeployPath/README.md <<'EOFREADME'`n$readmeContent`nEOFREADME" "Creating README..."

# Create .gitignore
$gitignoreContent = @'
data/*.db
data/*.db-shm
data/*.db-wal
*.log
*.tmp
.DS_Store
'@

Invoke-PlinkCommand "cat > $DeployPath/.gitignore <<'EOFGITIGNORE'`n$gitignoreContent`nEOFGITIGNORE" "Creating .gitignore..."
Write-Host "??Documentation created" -ForegroundColor Green

# Step 7: Git push
Write-Host ""
Write-Host "[7/7] Pushing to GitHub..." -ForegroundColor Cyan

$gitUrl = "https://${GitHubUser}:${GitHubToken}@github.com/asiahoro11/Management Server.git"
$gitCommands = @"
cd $DeployPath && \
git init && \
git branch -M main && \
git config user.name '$GitHubUser' && \
git config user.email '${GitHubUser}@users.noreply.github.com' && \
git remote add origin '$gitUrl' && \
git add . && \
git commit -m 'Deploy NMS Lite - $(date +"%Y-%m-%d %H:%M:%S")' && \
git push -u origin main --force && \
echo 'PUSH_SUCCESS'
"@

$pushResult = Invoke-PlinkCommand $gitCommands "Initializing git and pushing..."

Write-Host ""
if ($pushResult -match "PUSH_SUCCESS") {
    Write-Host "==========================================" -ForegroundColor Green
    Write-Host "??Deployment Complete!" -ForegroundColor Green
    Write-Host "==========================================" -ForegroundColor Green
}
else {
    Write-Host "==========================================" -ForegroundColor Yellow
    Write-Host "??Deployment may be complete (check GitHub)" -ForegroundColor Yellow
    Write-Host "==========================================" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "?“¦ Repository: $GitHubRepo" -ForegroundColor Cyan
Write-Host "?? View at: https://github.com/asiahoro11/Management Server" -ForegroundColor Cyan
Write-Host ""
Write-Host "To deploy on a new server:" -ForegroundColor Yellow
Write-Host "  git clone https://github.com/asiahoro11/Management Server.git /opt/nms" -ForegroundColor White
Write-Host "  cd /opt/nms && chmod +x nms-server && ./nms-server" -ForegroundColor White
Write-Host ""

