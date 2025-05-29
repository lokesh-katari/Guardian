#!/bin/bash

# SystemD Service Setup Script for Go Gaurdian Bot
# This script will set up the security bot to run automatically at system startup

BOT_BINARY_PATH="/opt/security-bot/security_bot"
BOT_CONFIG_DIR="/opt/security-bot"
SERVICE_NAME="security-bot"
SERVICE_USER="root"  


RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' 

echo -e "${GREEN}Setting up Security Bot as a SystemD service...${NC}"

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}This script must be run as root (use sudo)${NC}"
   exit 1
fi

# Create the service directory
echo -e "${YELLOW}Creating service directory...${NC}"
mkdir -p /opt/security-bot
mkdir -p /opt/security-bot/logs
mkdir -p /tmp/security_captures

# Copy the binary if it exists in current directory
if [ -f "./security_bot" ]; then
    echo -e "${YELLOW}Copying binary to service directory...${NC}"
    cp ./security_bot /opt/security-bot/
    chmod +x /opt/security-bot/security_bot
else
    echo -e "${YELLOW}Binary not found in current directory. Please build and place it at ${BOT_BINARY_PATH}${NC}"
fi

# Create the systemd service file
echo -e "${YELLOW}Creating systemd service file...${NC}"
cat > /etc/systemd/system/${SERVICE_NAME}.service << EOF
[Unit]
Description=Linux Security Telegram Bot
Documentation=Security monitoring bot that detects failed login attempts
After=network.target network-online.target
Wants=network-online.target
StartLimitIntervalSec=0

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
WorkingDirectory=${BOT_CONFIG_DIR}
ExecStart=${BOT_BINARY_PATH}
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=security-bot

# Environment variables
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
Environment=HOME=/root

# Security settings
NoNewPrivileges=false
ProtectSystem=false
ProtectHome=false
ReadWritePaths=/tmp /var/log /opt/security-bot

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF

# Create a configuration file template
echo -e "${YELLOW}Creating configuration template...${NC}"
cat > /opt/security-bot/config.example << 'EOF'
# Security Bot Configuration
# Copy this file to config.json and edit the values

{
    "telegram_token": "YOUR_TELEGRAM_BOT_TOKEN",
    "chat_id": "YOUR_CHAT_ID",
    "auth_log_path": "/var/log/auth.log",
    "camera_device": 0,
    "check_interval": 2,
    "save_dir": "/tmp/security_captures",
    "stealth_mode": true
}
EOF

# Create a startup script for easier management
echo -e "${YELLOW}Creating management script...${NC}"
cat > /opt/security-bot/manage.sh << 'EOF'
#!/bin/bash

SERVICE_NAME="security-bot"

case "$1" in
    start)
        echo "Starting Security Bot..."
        systemctl start $SERVICE_NAME
        ;;
    stop)
        echo "Stopping Security Bot..."
        systemctl stop $SERVICE_NAME
        ;;
    restart)
        echo "Restarting Security Bot..."
        systemctl restart $SERVICE_NAME
        ;;
    status)
        systemctl status $SERVICE_NAME
        ;;
    logs)
        journalctl -u $SERVICE_NAME -f
        ;;
    enable)
        echo "Enabling Security Bot to start at boot..."
        systemctl enable $SERVICE_NAME
        ;;
    disable)
        echo "Disabling Security Bot from starting at boot..."
        systemctl disable $SERVICE_NAME
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|logs|enable|disable}"
        exit 1
        ;;
esac
EOF

chmod +x /opt/security-bot/manage.sh

# Create log rotation configuration
echo -e "${YELLOW}Setting up log rotation...${NC}"
cat > /etc/logrotate.d/security-bot << EOF
/opt/security-bot/logs/*.log {
    daily
    missingok
    rotate 7
    compress
    delaycompress
    copytruncate
    notifempty
    create 644 root root
}
EOF

# Reload systemd and enable the service
echo -e "${YELLOW}Reloading systemd daemon...${NC}"
systemctl daemon-reload

echo -e "${YELLOW}Enabling service to start at boot...${NC}"
systemctl enable ${SERVICE_NAME}

# Set proper permissions
chown -R root:root /opt/security-bot
chmod -R 755 /opt/security-bot
chmod 644 /etc/systemd/system/${SERVICE_NAME}.service

echo -e "${GREEN}✅ SystemD service setup complete!${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Edit your bot configuration in the Go source code"
echo "2. Build the Go binary: go build -o security_bot"
echo "3. Copy the binary to /opt/security-bot/"
echo "4. Start the service: sudo systemctl start ${SERVICE_NAME}"
echo "5. Check status: sudo systemctl status ${SERVICE_NAME}"
echo ""
echo -e "${YELLOW}Management commands:${NC}"
echo "• Start service: sudo /opt/security-bot/manage.sh start"
echo "• Stop service: sudo /opt/security-bot/manage.sh stop"
echo "• View logs: sudo /opt/security-bot/manage.sh logs"
echo "• Check status: sudo /opt/security-bot/manage.sh status"
echo ""
echo -e "${GREEN}The service will now automatically start when your system boots!${NC}"