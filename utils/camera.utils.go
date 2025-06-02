package utils

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/lokesh-katari/POLICE/internal"
)

func DisableCameraLED(deviceNum int, logger *log.Logger) {
	devicePath := fmt.Sprintf("/dev/video%d", deviceNum)

	// Check if device exists
	if _, err := os.Stat(devicePath); os.IsNotExist(err) {
		logger.Printf("Camera device %s does not exist", devicePath)
		return
	}

	cmd := exec.Command("v4l2-ctl", "--device", devicePath, "--set-ctrl=led1_mode=0")
	err := cmd.Run()
	if err != nil {
		logger.Printf("Failed to disable camera LED: %v", err)
	} else {
		logger.Printf("Attempted to disable camera LED for device %s", devicePath)
	}
}

func CaptureImage(config internal.Config, logger *log.Logger) (string, error) {

	timestamp := time.Now().Format("20060102_150405")
	imagePath := filepath.Join(config.SaveDir, fmt.Sprintf("security_%s.jpg", timestamp))
	os.MkdirAll(config.SaveDir, 0755)

	if config.StealthMode {
		DisableCameraLED(config.CameraDevice, logger)
	}

	devicePath := fmt.Sprintf("/dev/video%d", config.CameraDevice)

	cmdArgs := []string{
		"-loglevel", "quiet",
		"-y",
		"-f", "video4linux2",
		"-input_format", "mjpeg",
		"-i", devicePath,
		"-frames:v", "1",
		"-q:v", "2",
		imagePath,
	}

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

	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return "", fmt.Errorf("image file was not created")
	}

	logger.Printf("Image captured: %s", imagePath)
	return imagePath, nil
}
