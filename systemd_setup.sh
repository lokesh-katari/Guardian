#!/bin/bash

# SystemD Service Setup Script for Go Security Bot
# This script sets up the security bot to run automatically at system startup
# Uses TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID environment variables 

BOT_BINARY_PATH="/opt/security-bot/security_bot"
BOT_CONFIG_DIR="/opt/security-bot"
SERVICE_NAME="security-bot"
SERVICE_USER="root"
SERVICE_ENV_FILE="/opt/security-bot/security-bot.env"

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

# Check for required dependencies
echo -e "${YELLOW}Checking dependencies...${NC}"
if ! command -v ffmpeg >/dev/null 2>&1; then
    echo -e "${RED}ffmpeg is required but not installed. Install it with: sudo apt-get install ffmpeg${NC}"
    exit 1
fi
if ! command -v v4l2-ctl >/dev/null 2>&1; then
    echo -e "${YELLOW}Warning: v4l2-ctl not found. Install v4l-utils for camera LED control: sudo apt-get install v4l-utils${NC}"
    echo -e "${YELLOW}Continuing without v4l2-ctl...${NC}"
fi

# Creating the service directories
echo -e "${YELLOW}Creating service directories...${NC}"
mkdir -p "${BOT_CONFIG_DIR}/logs"
mkdir -p "/tmp/security_captures"
chmod 700 "${BOT_CONFIG_DIR}"
chmod 700 "${BOT_CONFIG_DIR}/logs"
chmod 700 "/tmp/security_captures"


if [ -f "./security_bot" ]; then
    echo -e "${YELLOW}Copying binary to ${BOT_BINARY_PATH}...${NC}"
    cp ./security_bot "${BOT_BINARY_PATH}"
    chmod 755 "${BOT_BINARY_PATH}"
else
    echo -e "${RED}Error: Binary not found in current directory. Please build it with 'go build -o security_bot' and place it here.${NC}"
    exit 1
fi


echo -e "${YELLOW}Configuring Telegram settings...${NC}"
read -p "Enter your Telegram Bot Token: " TELEGRAM_BOT_TOKEN
if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo -e "${RED}Error: Telegram Bot Token is required.${NC}"
    exit 1
fi
read -p "Enter your Telegram Chat ID: " TELEGRAM_CHAT_ID
if [ -z "$TELEGRAM_CHAT_ID" ]; then
    echo -e "${RED}Error: Telegram Chat ID is required.${NC}"
    exit 1
fi

# Create environment file
echo -e "${YELLOW}Creating environment file at ${SERVICE_ENV_FILE}...${NC}"
cat > "${SERVICE_ENV_FILE}" << EOF
TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}
TELEGRAM_CHAT_ID=${TELEGRAM_CHAT_ID}
EOF
chmod 600 "${SERVICE_ENV_FILE}"

# Create the systemd service file
echo -e "${YELLOW}Creating systemd service file at /etc/systemd/system/${SERVICE_NAME}.service...${NC}"
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
EnvironmentFile=${SERVICE_ENV_FILE}
ExecStart=${BOT_BINARY_PATH}
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=security-bot

# Security settings
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/var/log/auth.log ${BOT_CONFIG_DIR}/logs

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF
chmod 644 /etc/systemd/system/${SERVICE_NAME}.service

# Create management script
echo -e "${YELLOW}Creating management script at ${BOT_CONFIG_DIR}/manage.sh...${NC}"
cat > "${BOT_CONFIG_DIR}/manage.sh" << 'EOF'
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
chmod 755 "${BOT_CONFIG_DIR}/manage.sh"

# Creating the log rotation configuration for ebery 7 days
echo -e "${YELLOW}Setting up log rotation...${NC}"
cat > /etc/logrotate.d/security-bot << EOF
${BOT_CONFIG_DIR}/logs/*.log {
    daily
    missingok
    rotate 7
    compress
    delaycompress
    copytruncate
    notifempty
    create 600 root root
}
EOF
chmod 644 /etc/logrotate.d/security-bot

# Reloading  systemd and enabling the service
echo -e "${YELLOW}Reloading systemd daemon...${NC}"
systemctl daemon-reload

echo -e "${YELLOW}Enabling service to start at boot...${NC}"
systemctl enable ${SERVICE_NAME}

# Setting proper permissions
chown -R root:root "${BOT_CONFIG_DIR}"
chmod -R 750 "${BOT_CONFIG_DIR}"
chmod 600 "${SERVICE_ENV_FILE}"

echo -e "${YELLOW}Starting the service...${NC}"
systemctl start ${SERVICE_NAME}

echo -e "${GREEN}✅ SystemD service setup complete!${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Verify the service is running: sudo /opt/security-bot/manage.sh status"
echo "2. View logs: sudo /opt/security-bot/manage.sh logs"
echo "3. To update Telegram settings, edit ${SERVICE_ENV_FILE} and restart with:"
echo "   sudo /opt/security-bot/manage.sh restart"
echo ""
echo -e "${YELLOW}Management commands:${NC}"
echo "• Start: sudo /opt/security-bot/manage.sh start"
echo "• Stop: sudo /opt/security-bot/manage.sh stop"
echo "• Restart: sudo /opt/security-bot/manage.sh restart"
echo "• Status: sudo /opt/security-bot/manage.sh status"
echo "• Logs: sudo /opt/security-bot/manage.sh logs"
echo "• Enable at boot: sudo /opt/security-bot/manage.sh enable"
echo "• Disable at boot: sudo /opt/security-bot/manage.sh disable"
echo ""
echo -e "${GREEN}The service is running and will start automatically on system boot!${NC}"