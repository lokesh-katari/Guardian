# Gaurdian Telegram Bot 🔐

A lightweight security monitoring daemon that detects failed login attempts on your Linux system and instantly notifies you via Telegram with photo evidence from your camera.

## 🌟 Features

- **Real-time Monitoring**: Continuously monitors `/var/log/auth.log` for failed login attempts
- **Instant Alerts**: Sends immediate Telegram notifications when intrusion attempts are detected
- **Photo Evidence**: Captures photos from system camera during security events
- **SystemD Integration**: Runs as a background service with automatic startup
- **Robust Logging**: Comprehensive logging with automatic rotation
- **Zero Configuration**: Simple one-time setup with interactive script

## 🏗️ Architecture
![image](https://github.com/user-attachments/assets/42480828-7fa0-411e-9c64-96ac856530c8)

## 🛠️ Prerequisites

Before installation, ensure you have:

- **Linux System** with SystemD support
- **Root Access** for installation
- **Camera** (optional, for photo capture)
- **Internet Connection** for Telegram notifications

### Required Dependencies

- `ffmpeg` - For camera capture
- `v4l-utils` - For camera LED control (optional)

## 📱 Telegram Setup

### Step 1: Create a Telegram Bot

1. Open Telegram and search for `@BotFather`
2. Start a conversation and send `/newbot`
3. Follow the prompts to create your bot
4. Save the **Bot Token** (looks like: `123456789:ABCdefGHIjklMNOpqrsTUVwxyz`)

### Step 2: Get Your Chat ID

1. Send a message to your bot
2. Visit: `https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates`
3. Find your **Chat ID** in the response (usually a number like `123456789`)

## 🚀 Installation

### 1. Build the Binary

```bash
# Clone the repository
git clone https://github.com/lokesh-katari/Guardian.git
cd Guardian

# Build the Go binary
go build -o security_bot
```

### 2. Run the Setup Script

```bash
# Make the setup script executable
sudo chmod +x systemd_setup.sh

# Run the setup (requires root privileges)
sudo ./systemd_setup.sh
```

### 3. Enter Configuration

When prompted, enter:

- **Telegram Bot Token**: Your bot token from BotFather
- **Telegram Chat ID**: Your personal chat ID

The script will automatically:

- ✅ Install the binary to `/opt/security-bot/`
- ✅ Create SystemD service configuration
- ✅ Set up environment variables
- ✅ Configure log rotation
- ✅ Start the service
- ✅ Enable automatic startup

## 🔧 Management

Use the provided management script to control the service:

```bash
# Check service status
sudo /opt/security-bot/manage.sh status

# View real-time logs
sudo /opt/security-bot/manage.sh logs

# Start the service
sudo /opt/security-bot/manage.sh start

# Stop the service
sudo /opt/security-bot/manage.sh stop

# Restart the service
sudo /opt/security-bot/manage.sh restart

# Enable auto-start at boot
sudo /opt/security-bot/manage.sh enable

# Disable auto-start at boot
sudo /opt/security-bot/manage.sh disable
```
## ⚙️ Configuration

### Environment Variables

Edit `/opt/security-bot/security-bot.env`:

```bash
TELEGRAM_BOT_TOKEN=your_bot_token_here
TELEGRAM_CHAT_ID=your_chat_id_here
```

After editing, restart the service:

```bash
sudo /opt/security-bot/manage.sh restart
```

## 📋 Logs

The service logs to multiple locations:

- **SystemD Journal**: `journalctl -u security-bot -f`
- **Application Logs**: `/opt/security-bot/logs/`
- **System Auth Logs**: `/var/log/auth.log` (monitored)

Log rotation is automatically configured for 7-day retention.

## 🔒 Security Features

- **Minimal Privileges**: Runs with restricted systemd security settings
- **Protected Directories**: Uses SystemD's `ProtectSystem` and `ProtectHome`
- **Private Temp**: Isolated temporary directory access
- **Secure Permissions**: Environment file protected with 600 permissions
- **No New Privileges**: Prevents privilege escalation

## 🚨 What Gets Monitored

The bot monitors for:

- Failed SSH login attempts
- Failed sudo attempts
- Invalid user login attempts
- Brute force attack patterns
- Any authentication failures logged to `/var/log/auth.log`

## 📧 Notification Format

When an intrusion is detected, you'll receive:

- 🚨 **Alert Message** with details
- 📸 **Photo** from system camera (if available)
- 🕒 **Timestamp** of the event
- 🖥️ **System Information**

## 🛠️ Troubleshooting

### Service Won't Start

```bash
# Check service status
sudo systemctl status security-bot

# View detailed logs
sudo journalctl -u security-bot -n 50
```

### No Camera Photos

- Ensure camera is connected and working
- Install v4l-utils: `sudo apt-get install v4l-utils`
- Check camera permissions

### No Telegram Messages

- Verify bot token and chat ID
- Check internet connectivity
- Ensure bot is not blocked

### Permission Issues

```bash
# Reset permissions
sudo chown -R root:root /opt/security-bot/
sudo chmod -R 750 /opt/security-bot/
sudo chmod 600 /opt/security-bot/security-bot.env
```

## 🔄 Updates

To update the bot:

1. Build new binary: `go build -o security_bot`
2. Stop service: `sudo /opt/security-bot/manage.sh stop`
3. Replace binary: `sudo cp security_bot /opt/security-bot/`
4. Start service: `sudo /opt/security-bot/manage.sh start`

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## ⚠️ Disclaimer

This tool is designed for legitimate security monitoring of your own systems. Ensure compliance with local laws and regulations when deploying security monitoring solutions.

---

**Made with ❤️ **
