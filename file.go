package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

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

	fmt.Printf("Created file: %s (rampup to %.1f MB)\n", rm.filePath, float64(rm.config.FileSizeMB))

	buffer := make([]byte, 1024*1024) // 1MB buffer
	for i := range buffer {
		buffer[i] = byte(i % 256)
	}

	// Use ticker to control growth rate during rampup
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	lastFileSizeMB := int64(0)

	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			// Get current target file size based on rampup progress
			_, _, currentFileSizeMB := rm.getCurrentResourceUsage()

			// Calculate how much more to write
			currentFileSize := currentFileSizeMB * 1024 * 1024
			fileInfo, err := file.Stat()
			if err != nil {
				log.Printf("Failed to get file info: %v", err)
				return
			}

			currentSize := fileInfo.Size()

			// Write more data if needed
			if currentSize < currentFileSize {
				bytesToWrite := currentFileSize - currentSize
				if bytesToWrite > int64(len(buffer)) {
					bytesToWrite = int64(len(buffer))
				}

				_, err := file.Write(buffer[:bytesToWrite])
				if err != nil {
					log.Printf("Failed to write to file: %v", err)
					return
				}

				// Sync to ensure data is written to disk
				file.Sync()
			}

			// Update display if file size changed significantly
			if currentFileSizeMB != lastFileSizeMB {
				lastFileSizeMB = currentFileSizeMB
				if currentFileSizeMB > 0 {
					fmt.Printf("File size: %.1f MB / %.1f MB\n",
						float64(currentFileSizeMB),
						float64(rm.config.FileSizeMB))
				}
			}
		}
	}
}
