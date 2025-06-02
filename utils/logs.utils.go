package utils

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/lokesh-katari/POLICE/internal"
)

func MonitorLogs(wg *sync.WaitGroup, config internal.Config, logger *log.Logger, monitoring bool) {
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
	file, err := os.Open(config.AuthLogPath)
	if err != nil {
		logger.Fatalf("Failed to open auth log: %v", err)
		return
	}

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
		file, err := os.Open(config.AuthLogPath)
		if err != nil {
			logger.Printf("Failed to open auth log: %v", err)
			time.Sleep(time.Duration(config.CheckInterval) * time.Second)
			continue
		}
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
							HandleFailedLogin(line, logger, config)
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
