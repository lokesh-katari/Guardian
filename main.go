package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/lokesh-katari/POLICE/internal"
	"github.com/lokesh-katari/POLICE/utils"
)

var (
	config     internal.Config
	logger     *log.Logger
	logFile    *os.File
	monitoring bool
	wg         sync.WaitGroup
)

func initConfig() internal.Config {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" {
		fmt.Errorf("Telegram bot token must be set via TELEGRAM_BOT_TOKEN environment variable or config file")

	}
	if chatID == "" {
		fmt.Errorf("Telegram chat ID must be set via TELEGRAM_CHAT_ID environment variable or config file")
	}
	return internal.Config{
		TelegramToken: token,
		ChatID:        chatID,
		AuthLogPath:   "/var/log/auth.log",
		CameraDevice:  0,
		CheckInterval: 2,
		SaveDir:       "/tmp/security_captures",
		StealthMode:   true,
		PatternStrings: []string{
			"Failed password for .* from .* port \\d+ ssh\\d*",
			"Failed password for invalid user .* from .* port \\d+",
			"authentication failure.*rhost=",
			"FAILED LOGIN .* FOR .*",
			"user unknown .* from",
		},
	}
}

func initLogger() {

	// Create log directory if it doesn't exist
	logDir := "logs"
	if _, err := os.Stat("/opt/security-bot"); err == nil {
		logDir = "/opt/security-bot/logs"
	}
	os.MkdirAll(logDir, 0755)

	var err error
	logPath := filepath.Join(logDir, "security_bot.log")
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logger = log.New(multiWriter, "[SECURITY-BOT] ", log.LstdFlags|log.Lshortfile)

	logger.Println("Logger initialized")
}

func cleanup() {
	if logFile != nil {
		logFile.Close()
	}
	logger.Println("Bot stopped")
}

func startBotServer() {

	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		message := "🔒 Linux Security Bot is running! 🔒\n" +
			"I'll notify you about failed login attempts on your system.\n\n" +
			"Available commands:\n" +
			"/help - Show this help message\n" +
			"/status - Check if monitoring is active\n" +
			"/test - Send a test alert"

		err := utils.SendTelegramMessage(utils.EscapeTelegramMarkdown(message), config)
		if err != nil {
			logger.Printf("Failed to send start message: %v", err)
		}

		fmt.Fprintf(w, "Start command processed")
	})

	http.HandleFunc("/help", func(w http.ResponseWriter, r *http.Request) {
		message := "🔒 *Linux Security Bot Help* 🔒\n\n" +
			"This bot monitors your Linux system for failed login attempts " +
			"and sends you notifications with webcam images when detected.\n\n" +
			"*Commands:*\n" +
			"/start - Start the bot\n" +
			"/help - Show this help message\n" +
			"/status - Check if monitoring is active\n" +
			"/test - Send a test alert with camera capture"

		err := utils.SendTelegramMessage(utils.EscapeTelegramMarkdown(message), config)
		if err != nil {
			logger.Printf("Failed to send help message: %v", err)
		}

		fmt.Fprintf(w, "Help command processed")
	})

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		active := "Active ✅"
		if !monitoring {
			active = "Inactive ❌"
		}

		stealth := "Enabled ✅"
		if !config.StealthMode {
			stealth = "Disabled ❌"
		}

		message := fmt.Sprintf("🔒 *Security Monitor Status* 🔒\n\n"+
			"Monitoring: %s\n"+
			"Stealth mode: %s\n"+
			"Log file: %s\n"+
			"Check interval: %d seconds\n"+
			"Image storage: %s",
			active, stealth, config.AuthLogPath, config.CheckInterval, config.SaveDir)

		err := utils.SendTelegramMessage(utils.EscapeTelegramMarkdown(message), config)
		if err != nil {
			logger.Printf("Failed to send status message: %v", err)
		}

		fmt.Fprintf(w, "Status command processed")
	})

	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		go utils.SendTestAlert(logger, config)
		fmt.Fprintf(w, "Test command processed")
	})

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is supported", http.StatusMethodNotAllowed)
			return
		}

		// update from Telegram
		var update struct {
			Message struct {
				Text string `json:"text"`
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
			} `json:"message"`
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}

		err = json.Unmarshal(body, &update)
		if err != nil {
			http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
			return
		}

		chatIDInt, err := strconv.ParseInt(config.ChatID, 10, 64)
		if err != nil || update.Message.Chat.ID != chatIDInt {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Process commands
		switch update.Message.Text {
		case "/start":
			http.Redirect(w, r, "/start", http.StatusSeeOther)
		case "/help":
			http.Redirect(w, r, "/help", http.StatusSeeOther)
		case "/status":
			http.Redirect(w, r, "/status", http.StatusSeeOther)
		case "/test":
			http.Redirect(w, r, "/test", http.StatusSeeOther)
		default:
			fmt.Fprintf(w, "Unknown command")
		}
	})

	go func() {
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			logger.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()
}

func main() {

	initLogger()
	defer cleanup()

	//loading the env file
	var err error
	err = godotenv.Load("/opt/security-bot/security-bot.env")
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	if os.Geteuid() != 0 {
		logger.Fatal("This script requires root privileges to access auth logs. Please run with sudo.")
	}

	config = initConfig()
	if !utils.CheckFfmpeg() {
		logger.Fatal("ffmpeg is required for camera capture. Please install it with: sudo apt-get install ffmpeg")
	}
	if config.StealthMode && !utils.CheckV4l2Ctl() {
		logger.Println("Warning: v4l2-ctl not found. Install v4l-utils for better camera LED control.")
		logger.Println("Continuing with basic stealth mode...")
	}

	err = os.MkdirAll(config.SaveDir, 0755)
	if err != nil {
		logger.Fatalf("Failed to create save directory: %v", err)
	}

	// Send startup notification
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	startupMessage := fmt.Sprintf("🔒 Security monitoring started on *%s* 🔒\n"+
		"I'll notify you about failed login attempts\\.\n"+
		"Stealth camera mode: *%s*\n"+
		"Service mode: *%s*",
		utils.EscapeTelegramMarkdown(hostname),
		utils.EscapeTelegramMarkdown(fmt.Sprintf("%t", config.StealthMode)),
		utils.EscapeTelegramMarkdown("SystemD"))

	err = utils.SendTelegramMessage(startupMessage, config)
	if err != nil {
		logger.Printf("Failed to send startup notification: %v", err)
	} else {
		logger.Println("Startup notification sent to Telegram")
	}

	//  HTTP server for bot commands
	startBotServer()

	// signal handling for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	monitoring = true
	wg.Add(1)
	go utils.MonitorLogs(&wg, config, logger, monitoring)

	logger.Println("Security bot is now running. Monitoring for failed login attempts...")

	// Wait for termination signal
	<-sigs
	logger.Println("Received termination signal. Shutting down gracefully...")

	// Send shutdown notification
	shutdownMessage := fmt.Sprintf("🔒 Security monitoring stopped on *%s* 🔒",
		utils.EscapeTelegramMarkdown(hostname))
	utils.SendTelegramMessage(shutdownMessage, config)

	// Stop monitoring
	monitoring = false
	wg.Wait()

	logger.Println("Security bot stopped")
}
