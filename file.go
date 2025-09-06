package main

import (
	"fmt"
	"log"
	"os"
	"syscall"
	"time"
)

// getCurrentFileSizeUsage calculates current file size usage based on rampup progress
func (rm *ResourceMock) getCurrentFileSizeUsage() int64 {
	elapsed := time.Since(rm.rampupStart)

	// If rampup time is 0 or elapsed time exceeds rampup time, use target values
	if rm.config.RampupTime <= 0 || elapsed >= rm.config.RampupTime {
		return rm.config.FileSizeMB
	}

	// Calculate rampup progress (0.0 to 1.0)
	progress := float64(elapsed) / float64(rm.config.RampupTime)

	// Linear interpolation from 0 to target
	return int64(progress * float64(rm.config.FileSizeMB))
}

// consumeFile creates and grows a file to specified size during rampup
func (rm *ResourceMock) consumeFile() {
	defer rm.wg.Done()

	if rm.config.FileSizeMB <= 0 {
		return
	}

	// Create file
	file, err := os.Create(rm.filePath)
	if err != nil {
		log.Printf("Failed to create file: %v", err)
		return
	}
	rm.file = file

	// Unlink the file from disk while keeping the file descriptor open
	// This ensures the file content is held in memory and will be automatically
	// cleaned up when the process exits, without needing explicit cleanup
	err = syscall.Unlink(rm.filePath)
	if err != nil {
		log.Fatalf("Warning: failed to unlink file %s: %v", rm.filePath, err)
		// Continue anyway, the file will be cleaned up by the signal handler
	} else {
		fmt.Printf("Created and unlinked file: %s (rampup to %.1f MB) - will auto-cleanup on exit\n",
			rm.filePath, float64(rm.config.FileSizeMB))
	}

	buffer := make([]byte, 1024*1024) // 1MB buffer
	for i := range buffer {
		buffer[i] = byte(i % 256)
	}

	// Use ticker to control growth rate during rampup
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	lastFileSizeMB := int64(0)
	writtenBytes := int64(0) // Track total bytes written

	count := 0
	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			count++
			// Get current target file size based on rampup progress
			currentFileSizeMB := rm.getCurrentFileSizeUsage()

			// Calculate how much more to write
			currentFileSize := currentFileSizeMB * 1024 * 1024

			// Write more data if needed
			if writtenBytes < currentFileSize {
				bytesToWrite := currentFileSize - writtenBytes
				if bytesToWrite > int64(len(buffer)) {
					bytesToWrite = int64(len(buffer))
				}

				n, err := file.Write(buffer[:bytesToWrite])
				if err != nil {
					log.Fatalf("Failed to write to file: %v", err)
					return
				}

				// Update written bytes counter
				writtenBytes += int64(n)

				// Sync to ensure data is written to disk
				err = file.Sync()
				if err != nil {
					log.Fatalf("Failed to sync file: %v", err)
				}
			}

			// Update display if file size changed significantly
			if currentFileSizeMB != lastFileSizeMB {
				lastFileSizeMB = currentFileSizeMB
				if currentFileSizeMB > 0 && count%100 == 0 {
					fmt.Printf("File size: %.1f MB / %.1f MB\n",
						float64(currentFileSizeMB),
						float64(rm.config.FileSizeMB))

					count = 0
				}
			}
		}
	}
}
