package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lokesh-katari/POLICE/internal"
)

// Escape special characters for Telegram MarkdownV2
func EscapeTelegramMarkdown(text string) string {

	escapeChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}

	for _, char := range escapeChars {
		text = strings.ReplaceAll(text, char, "\\"+char)
	}

	return text
}

func SendTelegramMessage(message string, config internal.Config) error {
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
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s, %s", resp.Status, string(body))
	}

	return nil
}

func SendTelegramImage(imagePath, caption string, config internal.Config) error {
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

	_ = writer.WriteField("chat_id", config.ChatID)

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
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s, %s", resp.Status, string(respBody))
	}

	return nil
}

func SendTestAlert(logger *log.Logger, config internal.Config) {
	logger.Println("Sending test alert")

	imagePath, err := CaptureImage(config, logger)
	if err != nil {
		logger.Printf("Failed to capture test image: %v", err)
	}

	testMessage := "🔒 *TEST ALERT* 🔒\n\nThis is a test security alert\\. The security monitoring system is working correctly\\."
	err = SendTelegramMessage(testMessage, config)
	if err != nil {
		logger.Printf("Failed to send test message: %v", err)
	} else {
		logger.Println("Test alert sent to Telegram")
	}

	// Send test image if available
	if imagePath != "" {
		caption := "Test image capture"
		err = SendTelegramImage(imagePath, caption, config)
		if err != nil {
			logger.Printf("Failed to send test image: %v", err)
		} else {
			logger.Println("Test image sent to Telegram")
		}
	}
}

func HandleFailedLogin(logEntry string, logger *log.Logger, config internal.Config) {
	// Capture image
	imagePath, err := CaptureImage(config, logger)
	if err != nil {
		logger.Printf("Failed to capture image: %v", err)
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	timestamp := EscapeTelegramMarkdown(time.Now().Format("2006-01-02 15:04:05"))
	hostnameEscaped := EscapeTelegramMarkdown(hostname)
	logEntryEscaped := EscapeTelegramMarkdown(logEntry)

	message := fmt.Sprintf("⚠️ *SECURITY ALERT* ⚠️\n\n*Time:* %s\n*Host:* %s\n*Event:* Failed login attempt detected\n\n*Log entry:*\n`%s`",
		timestamp, hostnameEscaped, logEntryEscaped)

	// Send text alert
	err = SendTelegramMessage(message, config)
	if err != nil {
		logger.Printf("Failed to send Telegram message: %v", err)
	} else {
		logger.Println("Alert message sent to Telegram")
	}

	if imagePath != "" {
		caption := "Image captured during failed login attempt"
		err = SendTelegramImage(imagePath, caption, config)
		if err != nil {
			logger.Printf("Failed to send image to Telegram: %v", err)
		} else {
			logger.Println("Image sent to Telegram")
		}
	}
}
