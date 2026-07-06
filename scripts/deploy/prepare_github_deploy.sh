#!/bin/bash
# Made by YTSworks
# YTS工作室製作
# NMS GitHub Deployment Preparation Script
# Run this on your Linux NMS server to prepare files for GitHub deployment

set -e

# Configuration
NMS_SOURCE_PATH="/opt/nms"
DEPLOY_PATH="$HOME/nms-deploy"
GITHUB_REPO=""  # User will set this

echo "========================================="
echo "NMS GitHub Deployment Preparation"
echo "========================================="
echo ""

# 1. Check if NMS exists
echo "1. Checking NMS installation..."
if [ ! -f "$NMS_SOURCE_PATH/nms-server" ]; then
    echo "ERROR: NMS server binary not found at $NMS_SOURCE_PATH/nms-server"
    echo "Please check your NMS installation path."
    exit 1
fi
echo "   ✓ Found NMS at $NMS_SOURCE_PATH"

# 2. Check NMS is running
echo ""
echo "2. Checking NMS status..."
if systemctl is-active --quiet nms 2>/dev/null; then
    echo "   ✓ NMS service is running"
elif pgrep -f nms-server > /dev/null; then
    echo "   ✓ NMS process is running"
else
    echo "   ⚠ WARNING: NMS doesn't appear to be running"
    read -p "   Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# 3. Create deployment directory
echo ""
echo "3. Preparing deployment directory..."
if [ -d "$DEPLOY_PATH" ]; then
    echo "   Deployment directory already exists: $DEPLOY_PATH"
    read -p "   Remove and recreate? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # Backup .git if exists
        if [ -d "$DEPLOY_PATH/.git" ]; then
            echo "   Backing up existing git configuration..."
            cp -r "$DEPLOY_PATH/.git" "/tmp/nms-deploy-git-backup"
            GIT_BACKUP=1
        fi
        rm -rf "$DEPLOY_PATH"
        mkdir -p "$DEPLOY_PATH"
        if [ "$GIT_BACKUP" = "1" ]; then
            echo "   Restoring git configuration..."
            mv "/tmp/nms-deploy-git-backup" "$DEPLOY_PATH/.git"
        fi
    fi
else
    mkdir -p "$DEPLOY_PATH"
    echo "   ✓ Created: $DEPLOY_PATH"
fi

# 4. Copy NMS files
echo ""
echo "4. Copying NMS files..."

# Copy binary
echo "   Copying nms-server binary..."
cp "$NMS_SOURCE_PATH/nms-server" "$DEPLOY_PATH/"
chmod +x "$DEPLOY_PATH/nms-server"

# Copy frontend
echo "   Copying frontend directory..."
if [ -d "$NMS_SOURCE_PATH/frontend" ]; then
    cp -r "$NMS_SOURCE_PATH/frontend" "$DEPLOY_PATH/"
else
    echo "   ERROR: Frontend directory not found"
    exit 1
fi

# Create data directory (empty)
echo "   Creating data directory..."
mkdir -p "$DEPLOY_PATH/data"

# Copy additional files if they exist
[ -f "$NMS_SOURCE_PATH/README.md" ] && cp "$NMS_SOURCE_PATH/README.md" "$DEPLOY_PATH/"
[ -f "$NMS_SOURCE_PATH/restart.sh" ] && cp "$NMS_SOURCE_PATH/restart.sh" "$DEPLOY_PATH/"

echo "   ✓ Files copied successfully"

# 5. Create deployment README
echo ""
echo "5. Creating deployment documentation..."
cat > "$DEPLOY_PATH/README.md" <<'EOF'
# NMS Deployment Package

Pre-compiled Network Management System for easy deployment.

## Quick Start

### 1. Clone this repository

```bash
git clone <your-repo-url> /opt/nms
cd /opt/nms
chmod +x nms-server
```

### 2. Run NMS

```bash
./nms-server
```

Server runs on port 8080 by default.

### 3. Access Web Interface

```
http://your-server-ip:8080
```

**Default Login:**
- Username: `admin`
- Password: `admin123`

⚠️ **Important:** Change the default password after first login!

## System Service Setup

Create systemd service for auto-start:

```bash
sudo tee /etc/systemd/system/nms.service > /dev/null <<'SERVICE'
[Unit]
Description=NMS Network Management System
After=network.target

[Service]
Type=simple
User=ubuntu
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
sudo systemctl status nms
```

## Update Deployment

```bash
cd /opt/nms
git pull
sudo systemctl restart nms
```

## File Structure

```
/opt/nms/
├── nms-server          # Main executable (Linux amd64)
├── frontend/           # Web interface (minified)
│   ├── index.html
│   ├── css/
│   └── js/
├── data/               # Database directory
│   └── nms.db          # (created automatically)
└── README.md
```

## System Requirements

- **OS:** Linux (amd64)
- **RAM:** 512MB minimum, 1GB+ recommended
- **Disk:** 500MB for application, 1GB+ for data
- **Network:** Port 8080 (configurable)

## Features

- SNMP-based device monitoring
- Network topology visualization (LLDP/MAC)
- Real-time interface statistics
- Syslog/Event log collection
- Alert management (Email, LINE, Telegram, WhatsApp)
- License management
- Multi-user support with role-based access

## Troubleshooting

### Check service status
```bash
sudo systemctl status nms
```

### View logs
```bash
sudo journalctl -u nms -f
```

### Reset admin password
```bash
cd /opt/nms
sqlite3 data/nms.db "UPDATE users SET password_hash='<new_hash>' WHERE username='admin';"
```

## License

This deployment package includes compiled/minified code for production use.

---
*Generated from production NMS server*
EOF

echo "   ✓ README.md created"

# 6. Create .gitignore
echo ""
echo "6. Creating .gitignore..."
cat > "$DEPLOY_PATH/.gitignore" <<'EOF'
# Database files (don't commit actual data)
data/nms.db
data/nms.db-shm
data/nms.db-wal
data/*.db

# Logs
*.log

# Temporary files
*.tmp
.DS_Store
EOF

echo "   ✓ .gitignore created"

# 7. Show file summary
echo ""
echo "7. Deployment package summary:"
echo "   Location: $DEPLOY_PATH"
echo ""
du -sh "$DEPLOY_PATH/nms-server" 2>/dev/null || echo "   Binary: nms-server"
echo "   Frontend files: $(find $DEPLOY_PATH/frontend -type f | wc -l) files"
echo ""

# 8. Git initialization
echo "8. Git repository setup..."
cd "$DEPLOY_PATH"

if [ -d ".git" ]; then
    echo "   Git repository already initialized"
    git status
else
    echo "   Initializing git repository..."
    git init
    git branch -M main
    echo "   ✓ Git initialized"
fi

# 9. Instructions for GitHub
echo ""
echo "========================================="
echo "✓ Preparation Complete!"
echo "========================================="
echo ""
echo "📦 Deployment package ready at: $DEPLOY_PATH"
echo ""
echo "Next steps to upload to GitHub:"
echo ""
echo "1. Create a new GitHub repository (e.g., 'nms-deploy')"
echo ""
echo "2. Run these commands:"
echo "   cd $DEPLOY_PATH"
echo "   git add ."
echo "   git commit -m 'Initial NMS deployment package'"
echo "   git remote add origin https://github.com/YOUR_USERNAME/nms-deploy.git"
echo "   git push -u origin main"
echo ""
echo "3. For updates in the future:"
echo "   cd $DEPLOY_PATH"
echo "   # ... update files ..."
echo "   git add ."
echo "   git commit -m 'Update version'"
echo "   git push"
echo ""
echo "========================================="
