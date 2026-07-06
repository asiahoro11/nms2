#!/bin/bash
# Made by YTSworks
# YTSå·¥ä½œå®¤è£½ä½œ
# Automated NMS GitHub Deployment Script
# This script prepares NMS files and pushes to GitHub automatically

set -e

# Configuration
NMS_SOURCE_PATH="/opt/nms"
DEPLOY_PATH="$HOME/nms-deploy"
GITHUB_REPO="https://github.com/asiahoro11/Management Server.git"
GITHUB_USER="asiahoro11"
GITHUB_TOKEN="REDACTED-TOKEN-SCRUBBED-FROM-HISTORY"

echo "========================================="
echo "NMS Automated GitHub Deployment"
echo "========================================="
echo ""

# 1. Check NMS installation
echo "[1/8] Checking NMS installation..."
if [ ! -f "$NMS_SOURCE_PATH/nms-server" ]; then
    echo "ERROR: NMS not found at $NMS_SOURCE_PATH"
    exit 1
fi
echo "??NMS found at $NMS_SOURCE_PATH"

# 2. Check NMS status
echo ""
echo "[2/8] Checking NMS status..."
if systemctl is-active --quiet nms 2>/dev/null || pgrep -f nms-server > /dev/null; then
    echo "??NMS is running"
else
    echo "??WARNING: NMS not running (continuing anyway)"
fi

# 3. Prepare deployment directory
echo ""
echo "[3/8] Preparing deployment directory..."
if [ -d "$DEPLOY_PATH/.git" ]; then
    echo "Backing up git configuration..."
    cp -r "$DEPLOY_PATH/.git" "/tmp/nms-git-backup"
    GIT_BACKUP=1
fi

rm -rf "$DEPLOY_PATH"
mkdir -p "$DEPLOY_PATH"

if [ "$GIT_BACKUP" = "1" ]; then
    mv "/tmp/nms-git-backup" "$DEPLOY_PATH/.git"
    echo "??Git configuration restored"
else
    echo "??New deployment directory created"
fi

# 4. Copy NMS files
echo ""
echo "[4/8] Copying NMS files..."
cp "$NMS_SOURCE_PATH/nms-server" "$DEPLOY_PATH/"
chmod +x "$DEPLOY_PATH/nms-server"
echo "??Binary copied"

cp -r "$NMS_SOURCE_PATH/frontend" "$DEPLOY_PATH/"
echo "??Frontend copied"

mkdir -p "$DEPLOY_PATH/data"
echo "??Data directory created"

# 5. Create README
echo ""
echo "[5/8] Creating documentation..."
cat > "$DEPLOY_PATH/README.md" <<'EOF'
# NMS Lite - Network Management System

Production-ready deployment package for NMS Lite.

## ?? Quick Deploy

```bash
# Clone repository
git clone https://github.com/asiahoro11/Management Server.git /opt/nms
cd /opt/nms

# Make executable
chmod +x nms-server

# Run
./nms-server
```

Access: `http://your-server:8080`

**Default Login:**
- Username: `admin`
- Password: `admin123`

## ?? System Service Setup

```bash
sudo tee /etc/systemd/system/nms.service > /dev/null <<'SERVICE'
[Unit]
Description=NMS Lite - Network Management System
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=/opt/nms
ExecStart=/opt/nms/nms-server
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
SERVICE

sudo systemctl daemon-reload
sudo systemctl enable nms
sudo systemctl start nms
```

## ?? Update

```bash
cd /opt/nms
git pull
sudo systemctl restart nms
```

## ?? Structure

```
/opt/nms/
?œâ??€ nms-server       # Executable (Linux amd64)
?œâ??€ frontend/        # Web UI (minified)
?œâ??€ data/           # Database (auto-created)
?”â??€ README.md
```

## ??Features

- ?? SNMP Device Monitoring
- ?—ºï¸?Network Topology (LLDP/MAC)
- ?? Real-time Interface Statistics
- ?? Syslog/Event Collection
- ?? Multi-channel Alerts
- ?? License Management
- ?‘¥ Role-based Access Control

## ?’» Requirements

- Linux (amd64)
- 512MB+ RAM
- Port 8080 available

## ?? Troubleshooting

```bash
# Check status
sudo systemctl status nms

# View logs
sudo journalctl -u nms -f

# Test connectivity
curl http://localhost:8080
```

---
?“¦ Pre-compiled deployment package
EOF

echo "??README.md created"

# 6. Create .gitignore
cat > "$DEPLOY_PATH/.gitignore" <<'EOF'
# Database files
data/*.db
data/*.db-shm
data/*.db-wal

# Logs
*.log

# Temp files
*.tmp
.DS_Store
EOF

echo "??.gitignore created"

# 7. Git setup and push
echo ""
echo "[6/8] Setting up Git..."
cd "$DEPLOY_PATH"

# Configure git credential
git config --global credential.helper store

if [ ! -d ".git" ]; then
    git init
    git branch -M main
    echo "??Git initialized"
else
    echo "??Using existing git repository"
fi

# Set remote with token
git remote remove origin 2>/dev/null || true
git remote add origin "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/asiahoro11/Management Server.git"
echo "??Remote configured"

# 8. Commit and push
echo ""
echo "[7/8] Committing files..."
git add .
git commit -m "Deploy NMS Lite - $(date '+%Y-%m-%d %H:%M:%S')" || echo "No changes to commit"

echo ""
echo "[8/8] Pushing to GitHub..."
git push -u origin main --force

# Summary
echo ""
echo "========================================="
echo "??Deployment Complete!"
echo "========================================="
echo ""
echo "?“¦ Repository: https://github.com/asiahoro11/Management Server"
echo "?? Local path: $DEPLOY_PATH"
echo ""
echo "Binary size: $(du -h $DEPLOY_PATH/nms-server | cut -f1)"
echo "Frontend files: $(find $DEPLOY_PATH/frontend -type f | wc -l)"
echo ""
echo "?? Next steps:"
echo "   - Clone on target server: git clone https://github.com/asiahoro11/Management Server.git"
echo "   - Run: cd Management Server && chmod +x nms-server && ./nms-server"
echo ""
echo "========================================="
