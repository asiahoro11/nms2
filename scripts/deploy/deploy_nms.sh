#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}==========================================${NC}"
echo -e "${GREEN}   Management Server One-Click Deployment Script   ${NC}"
echo -e "${GREEN}==========================================${NC}"

# Check Root
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}Error: Please run as root (sudo ./deploy_nms.sh)${NC}"
  exit 1
fi

# Detect OS
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$NAME
else
    echo -e "${YELLOW}Warning: Unknown OS. Assuming Debian/Ubuntu based.${NC}"
fi

echo -e "${GREEN}[1/5] Installing Dependencies...${NC}"
if [[ "$OS" == *"Ubuntu"* ]] || [[ "$OS" == *"Debian"* ]]; then
    apt update -qq
    apt install -y git curl snmp snmp-mibs-downloader golang make
    
    # Enable non-free MIBs for Ubuntu
    sed -i 's/mibs :/# mibs :/g' /etc/snmp/snmp.conf 2>/dev/null || true
    download-mibs >/dev/null 2>&1 || true
elif [[ "$OS" == *"CentOS"* ]] || [[ "$OS" == *"Red Hat"* ]] || [[ "$OS" == *"Fedora"* ]]; then
    dnf install -y git curl net-snmp net-snmp-utils golang
else
    echo -e "${YELLOW}Skipping dependency install for unsupported OS. Please ensure git and go are installed.${NC}"
fi

echo -e "${GREEN}[2/5] Updating Source Code...${NC}"
git pull

echo -e "${GREEN}[3/5] Building Project...${NC}"
if [ -d "backend" ]; then
    cd backend
    echo "Building Backend..."
    go mod tidy
    # Build binary to root directory
    go build -o ../nms-server main.go
    cd ..
else
    echo -e "${RED}Error: 'backend' directory not found! Are you in the project root?${NC}"
    exit 1
fi

if [ ! -f "nms-server" ]; then
    echo -e "${RED}Error: Build failed! nms-server binary not created.${NC}"
    exit 1
fi

chmod +x nms-server

echo -e "${GREEN}[4/5] Configuring System...${NC}"
# Set capabilities for binding low ports (if applicable)
if command -v setcap &> /dev/null; then
    setcap 'cap_net_bind_service=+ep' $(pwd)/nms-server || echo -e "${YELLOW}Warning: setcap failed. If you use port < 1024, service might fail as non-root.${NC}"
fi

# Determine service user (SUDO_USER if available, else current user which is root, or fallback to 'ubuntu' if exists)
REAL_USER=${SUDO_USER:-$USER}
# Try to avoid running as root if possible, unless the user explicitly wants to
if [ "$REAL_USER" == "root" ] && id "ubuntu" &>/dev/null; then
    REAL_USER="ubuntu"
    echo -e "${YELLOW}Running as root, but 'ubuntu' user detected. Will run service as 'ubuntu'.${NC}"
fi

INSTALL_DIR=$(pwd)
echo "Install Directory: $INSTALL_DIR"
echo "Service User: $REAL_USER"

# Create Systemd Service
cat > /etc/systemd/system/nms.service <<EOF
[Unit]
Description=Management Server Network Management System
After=network.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/nms-server
User=$REAL_USER
Group=$REAL_USER
Restart=always
RestartSec=10
Environment=GIN_MODE=release

[Install]
WantedBy=multi-user.target
EOF

# Fix permissions for the user
chown -R $REAL_USER:$REAL_USER $INSTALL_DIR

echo -e "${GREEN}[5/5] Restarting Service...${NC}"
systemctl daemon-reload
systemctl enable nms
systemctl restart nms

echo -e "${GREEN}==========================================${NC}"
echo -e "${GREEN}   Deployment Success!   ${NC}"
echo -e "${GREEN}==========================================${NC}"
echo "Service Status:"
systemctl status nms --no-pager | head -n 10
echo ""
echo -e "Web UI: http://$(hostname -I | cut -d' ' -f1):8080"
