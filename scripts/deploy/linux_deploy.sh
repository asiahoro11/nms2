#!/bin/bash
# Made by YTSworks
# YTS工作室製作
# Complete NMS GitHub deployment script
# Run this on Linux server

set -e

cd ~/nms-deploy

# Create README.md
cat > README.md <<'EOF'
# NMS Lite - Network Management System

Production deployment package.

## Quick Deploy

```bash
git clone https://github.com/asiahoro11/Management Server.git /opt/nms
cd /opt/nms
chmod +x nms-server
./nms-server
```

Access: http://your-server:8080  
Default login: admin / admin123

## System Service

```bash
sudo tee /etc/systemd/system/nms.service > /dev/null <<'SERVICE'
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
SERVICE

sudo systemctl daemon-reload
sudo systemctl enable --now nms
```

## Features
- SNMP Device Monitoring
- Network Topology
- Real-time Statistics
- Alert Management

## Update
```bash
cd /opt/nms && git pull && sudo systemctl restart nms
```
EOF

# Create .gitignore
cat > .gitignore <<'EOF'
data/*.db
data/*.db-shm
data/*.db-wal
*.log
*.tmp
.DS_Store
EOF

# Git setup and push
git init
git branch -M main
git config user.name "asiahoro11"
git config user.email "asiahoro11@users.noreply.github.com"
git remote add origin https://asiahoro11:REDACTED-TOKEN-SCRUBBED-FROM-HISTORY@github.com/asiahoro11/Management Server.git
git add .
git commit -m "Deploy NMS Lite - Production Package"
git push -u origin main --force

echo "========================================="
echo "Deployment Complete!"
echo "Repository: https://github.com/asiahoro11/Management Server"
echo "========================================="
