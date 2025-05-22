package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

// Configuration struct for app settings
type Config struct {
	TelegramToken  string
	ChatID         string
	AuthLogPath    string
	CameraDevice   int
	CheckInterval  int
	SaveDir        string
	StealthMode    bool
	PatternStrings []string
}

// Global variables
var (
	config     Config
	logger     *log.Logger
	logFile    *os.File
	monitoring bool
	wg         sync.WaitGroup
)

// Initialize configuration with default values
func initConfig() Config {
	return Config{
		TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		ChatID:        os.Getenv("CHAT_ID"),
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

// Initialize the logger
func initLogger() {
	// Create log directory if it doesn't exist
	os.MkdirAll("logs", 0755)

	// Open log file
	var err error
	logFile, err = os.OpenFile("logs/security_bot.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	// Create multi-writer to write logs to both stdout and file
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logger = log.New(multiWriter, "", log.LstdFlags)
}

// Cleanup resources before exit
func cleanup() {
	if logFile != nil {
		logFile.Close()
	}
	logger.Println("Bot stopped")
}

// Send a message to Telegram
func sendTelegramMessage(message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", config.TelegramToken)

	reqBody, err := json.Marshal(map[string]interface{}{
		"chat_id":    config.ChatID,
		"text":       message,
		"parse_mode": "MarkdownV2",
	})
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s, %s", resp.Status, string(body))
	}

	return nil
}

// Send an image to Telegram
func sendTelegramImage(imagePath, caption string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", config.TelegramToken)

	// Open the image file
	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create a multipart writer
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add chat_id field
	_ = writer.WriteField("chat_id", config.ChatID)

	// Add caption field
	_ = writer.WriteField("caption", caption)

	// Add the image
	part, err := writer.CreateFormFile("photo", filepath.Base(imagePath))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}

	// Close the writer
	err = writer.Close()
	if err != nil {
		return err
	}

	// Create and send request
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s, %s", resp.Status, string(respBody))
	}

	return nil
}

// Escape special characters for Telegram MarkdownV2
func escapeTelegramMarkdown(text string) string {
	// Characters that need to be escaped in MarkdownV2: _, *, [, ], (, ), ~, `, >, #, +, -, =, |, {, }, ., !
	escapeChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}

	for _, char := range escapeChars {
		text = strings.ReplaceAll(text, char, "\\"+char)
	}

	return text
}

// Try to disable camera LED using v4l2-ctl
func disableCameraLED(deviceNum int) {
	devicePath := fmt.Sprintf("/dev/video%d", deviceNum)

	// Check if device exists
	if _, err := os.Stat(devicePath); os.IsNotExist(err) {
		logger.Printf("Camera device %s does not exist", devicePath)
		return
	}

	// Try to disable LED
	cmd := exec.Command("v4l2-ctl", "--device", devicePath, "--set-ctrl=led1_mode=0")
	err := cmd.Run()
	if err != nil {
		logger.Printf("Failed to disable camera LED: %v", err)
	} else {
		logger.Printf("Attempted to disable camera LED for device %s", devicePath)
	}
}

// Capture an image from the webcam using ffmpeg
func captureImage() (string, error) {
	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	imagePath := filepath.Join(config.SaveDir, fmt.Sprintf("security_%s.jpg", timestamp))

	// Ensure save directory exists
	os.MkdirAll(config.SaveDir, 0755)

	// If in stealth mode, try to disable camera LED
	if config.StealthMode {
		disableCameraLED(config.CameraDevice)
	}

	// Prepare ffmpeg command with parameters to minimize camera time
	devicePath := fmt.Sprintf("/dev/video%d", config.CameraDevice)

	// Build the ffmpeg command with options for quick capture
	cmdArgs := []string{
		"-loglevel", "quiet", // Suppress ffmpeg output
		"-y",                 // Overwrite output files
		"-f", "video4linux2", // Input format
		"-input_format", "mjpeg", // Try to use MJPEG for faster grabbing
		"-i", devicePath, // Input device
		"-frames:v", "1", // Take just one frame
		"-q:v", "2", // High quality (lower number = higher quality)
		imagePath, // Output file
	}

	// Create the command
	cmd := exec.Command("ffmpeg", cmdArgs...)

	// Run the command with a timeout
	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("failed to start ffmpeg: %v", err)
	}

	// Kill the process after a timeout to ensure it doesn't hang
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Wait for command to finish or timeout
	select {
	case err := <-done:
		if err != nil {
			return "", fmt.Errorf("ffmpeg error: %v", err)
		}
	case <-time.After(3 * time.Second):
		// Kill the process if it takes too long
		cmd.Process.Kill()
		return "", fmt.Errorf("ffmpeg timed out")
	}

	// Check if file was created
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return "", fmt.Errorf("image file was not created")
	}

	logger.Printf("Image captured: %s", imagePath)
	return imagePath, nil
}

// Check logs for failed password attempts
func monitorLogs() {
	defer wg.Done()

	// Compile regex patterns
	var patterns []*regexp.Regexp
	for _, pattern := range config.PatternStrings {
		re, err := regexp.Compile(pattern)
		if err != nil {
			logger.Printf("Invalid regex pattern: %s - %v", pattern, err)
			continue
		}
		patterns = append(patterns, re)
	}

	// Get initial file size
	file, err := os.Open(config.AuthLogPath)
	if err != nil {
		logger.Fatalf("Failed to open auth log: %v", err)
		return
	}

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		logger.Fatalf("Failed to get file info: %v", err)
		file.Close()
		return
	}

	// Start from end of file
	lastPosition := fileInfo.Size()
	file.Close()

	// Hash set to track seen events and prevent duplicates
	seenEntries := make(map[string]bool)

	// Monitor loop
	logger.Printf("Starting log monitoring at %s", config.AuthLogPath)
	for monitoring {
		// Open file for reading
		file, err := os.Open(config.AuthLogPath)
		if err != nil {
			logger.Printf("Failed to open auth log: %v", err)
			time.Sleep(time.Duration(config.CheckInterval) * time.Second)
			continue
		}

		// Get current file size
		fileInfo, err := file.Stat()
		if err != nil {
			logger.Printf("Failed to get file info: %v", err)
			file.Close()
			time.Sleep(time.Duration(config.CheckInterval) * time.Second)
			continue
		}

		currentSize := fileInfo.Size()

		// Check if file was rotated or decreased in size
		if currentSize < lastPosition {
			lastPosition = 0
		}

		// Check for new content
		if currentSize > lastPosition {
			// Seek to last position
			_, err := file.Seek(lastPosition, 0)
			if err != nil {
				logger.Printf("Failed to seek: %v", err)
				file.Close()
				time.Sleep(time.Duration(config.CheckInterval) * time.Second)
				continue
			}

			// Read new content
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()

				// Check if line matches any pattern
				for _, pattern := range patterns {
					if pattern.MatchString(line) {
						// Create hash of the line to avoid duplicates
						lineHash := fmt.Sprintf("%x", line)

						// Check if we've seen this entry before
						if !seenEntries[lineHash] {
							seenEntries[lineHash] = true

							logger.Printf("Failed login detected: %s", line)

							// Handle the failed login attempt
							handleFailedLogin(line)
						}
						break
					}
				}
			}

			if err := scanner.Err(); err != nil {
				logger.Printf("Scanner error: %v", err)
			}

			// Update last position
			lastPosition = currentSize
		}

		file.Close()
		time.Sleep(time.Duration(config.CheckInterval) * time.Second)
	}
}

// Handle a failed login attempt by capturing image and sending alert
func handleFailedLogin(logEntry string) {
	// Capture image
	imagePath, err := captureImage()
	if err != nil {
		logger.Printf("Failed to capture image: %v", err)
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Format message with proper escaping for Telegram
	timestamp := escapeTelegramMarkdown(time.Now().Format("2006-01-02 15:04:05"))
	hostnameEscaped := escapeTelegramMarkdown(hostname)
	logEntryEscaped := escapeTelegramMarkdown(logEntry)

	message := fmt.Sprintf("⚠️ *SECURITY ALERT* ⚠️\n\n*Time:* %s\n*Host:* %s\n*Event:* Failed login attempt detected\n\n*Log entry:*\n`%s`",
		timestamp, hostnameEscaped, logEntryEscaped)

	// Send text alert
	err = sendTelegramMessage(message)
	if err != nil {
		logger.Printf("Failed to send Telegram message: %v", err)
	} else {
		logger.Println("Alert message sent to Telegram")
	}

	// Send image if available
	if imagePath != "" {
		caption := "Image captured during failed login attempt"
		err = sendTelegramImage(imagePath, caption)
		if err != nil {
			logger.Printf("Failed to send image to Telegram: %v", err)
		} else {
			logger.Println("Image sent to Telegram")
		}
	}
}

// Send a test alert to Telegram
func sendTestAlert() {
	logger.Println("Sending test alert")

	// Capture image
	imagePath, err := captureImage()
	if err != nil {
		logger.Printf("Failed to capture test image: %v", err)
	}

	// Send test message
	testMessage := "🔒 *TEST ALERT* 🔒\n\nThis is a test security alert\\. The security monitoring system is working correctly\\."
	err = sendTelegramMessage(testMessage)
	if err != nil {
		logger.Printf("Failed to send test message: %v", err)
	} else {
		logger.Println("Test alert sent to Telegram")
	}

	// Send test image if available
	if imagePath != "" {
		caption := "Test image capture"
		err = sendTelegramImage(imagePath, caption)
		if err != nil {
			logger.Printf("Failed to send test image: %v", err)
		} else {
			logger.Println("Test image sent to Telegram")
		}
	}
}

// Set up and start the HTTP server for bot commands
func startBotServer() {
	// Handle /start command
	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		message := "🔒 Linux Security Bot is running! 🔒\n" +
			"I'll notify you about failed login attempts on your system.\n\n" +
			"Available commands:\n" +
			"/help - Show this help message\n" +
			"/status - Check if monitoring is active\n" +
			"/test - Send a test alert"

		err := sendTelegramMessage(escapeTelegramMarkdown(message))
		if err != nil {
			logger.Printf("Failed to send start message: %v", err)
		}

		fmt.Fprintf(w, "Start command processed")
	})

	// Handle /help command
	http.HandleFunc("/help", func(w http.ResponseWriter, r *http.Request) {
		message := "🔒 *Linux Security Bot Help* 🔒\n\n" +
			"This bot monitors your Linux system for failed login attempts " +
			"and sends you notifications with webcam images when detected.\n\n" +
			"*Commands:*\n" +
			"/start - Start the bot\n" +
			"/help - Show this help message\n" +
			"/status - Check if monitoring is active\n" +
			"/test - Send a test alert with camera capture"

		err := sendTelegramMessage(escapeTelegramMarkdown(message))
		if err != nil {
			logger.Printf("Failed to send help message: %v", err)
		}

		fmt.Fprintf(w, "Help command processed")
	})

	// Handle /status command
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

		err := sendTelegramMessage(escapeTelegramMarkdown(message))
		if err != nil {
			logger.Printf("Failed to send status message: %v", err)
		}

		fmt.Fprintf(w, "Status command processed")
	})

	// Handle /test command
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		go sendTestAlert()
		fmt.Fprintf(w, "Test command processed")
	})

	// Handle webhook from Telegram
	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is supported", http.StatusMethodNotAllowed)
			return
		}

		// Parse update from Telegram
		var update struct {
			Message struct {
				Text string `json:"text"`
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
			} `json:"message"`
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}

		err = json.Unmarshal(body, &update)
		if err != nil {
			http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
			return
		}

		// Validate that the message is from authorized chat
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

	// Start HTTP server
	go func() {
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			logger.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()
}

// Setup webhook for Telegram bot
func setupWebhook(webhookURL string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", config.TelegramToken)

	// Create request body
	reqBody, err := json.Marshal(map[string]string{
		"url": webhookURL,
	})
	if err != nil {
		return err
	}

	// Send request
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to set webhook: %s, %s", resp.Status, string(body))
	}
	//
	logger.Println("Webhook set successfully")
	return nil
}

// Check if ffmpeg is installed
func checkFfmpeg() bool {
	cmd := exec.Command("which", "ffmpeg")
	err := cmd.Run()
	return err == nil
}

// Check if v4l2-ctl is installed (for camera LED control)
func checkV4l2Ctl() bool {
	cmd := exec.Command("which", "v4l2-ctl")
	err := cmd.Run()
	return err == nil
}

// Main function
func main() {
	// Initialize logger

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	initLogger()
	defer cleanup()

	// Check if running as root
	if os.Geteuid() != 0 {
		logger.Fatal("This script requires root privileges to access auth logs. Please run with sudo.")
	}

	// Initialize configuration
	config = initConfig()

	// Check required dependencies
	if !checkFfmpeg() {
		logger.Fatal("ffmpeg is required for camera capture. Please install it with: sudo apt-get install ffmpeg")
	}

	// Check optional dependencies
	if config.StealthMode && !checkV4l2Ctl() {
		logger.Println("Warning: v4l2-ctl not found. Install v4l-utils for better camera LED control.")
		logger.Println("Continuing with basic stealth mode...")
	}

	// Create save directory
	err = os.MkdirAll(config.SaveDir, 0755)
	if err != nil {
		logger.Fatalf("Failed to create save directory: %v", err)
	}

	// Send startup notification
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	var stealthModeStatus string
	if config.StealthMode {
		stealthModeStatus = "Enabled"
	} else {
		stealthModeStatus = "Disabled"
	}
	startupMessage := fmt.Sprintf("🔒 Security monitoring started on *%s* 🔒\n"+
		"I'll notify you about failed login attempts\\.\n"+
		"Stealth camera mode: *%s*",
		escapeTelegramMarkdown(hostname),
		escapeTelegramMarkdown(stealthModeStatus))

	err = sendTelegramMessage(startupMessage)
	if err != nil {
		logger.Printf("Failed to send startup notification: %v", err)
	}

	// Set up HTTP server for bot commands
	startBotServer()

	// Setup signal handling for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Start monitoring
	monitoring = true
	wg.Add(1)
	go monitorLogs()

	// Wait for termination signal
	<-sigs
	logger.Println("Received termination signal. Shutting down...")

	// Stop monitoring
	monitoring = false
	wg.Wait()
}
