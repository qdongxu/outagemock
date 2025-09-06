package main

import (
	"fmt"
	"log"
	"os"
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

	fmt.Printf("Created file: %s (rampup to %.1f MB)\n", rm.filePath, float64(rm.config.FileSizeMB))

	buffer := make([]byte, 1024*1024) // 1MB buffer
	for i := range buffer {
		buffer[i] = byte(i % 256)
	}

	// Use ticker to control growth rate during rampup
	ticker := time.NewTicker(50 * time.Millisecond) // Faster ticker
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

			// Write more data if needed - write multiple MB per tick for faster growth
			if writtenBytes < currentFileSize {
				bytesToWrite := currentFileSize - writtenBytes
				// Write up to 10MB per tick for faster growth
				maxWritePerTick := int64(10 * 1024 * 1024) // 10MB
				if bytesToWrite > maxWritePerTick {
					bytesToWrite = maxWritePerTick
				}

				// Write data in chunks
				for bytesToWrite > 0 {
					chunkSize := bytesToWrite
					if chunkSize > int64(len(buffer)) {
						chunkSize = int64(len(buffer))
					}

					n, err := file.Write(buffer[:chunkSize])
					if err != nil {
						log.Fatalf("Failed to write to file: %v", err)
						return
					}

					// Update written bytes counter
					writtenBytes += int64(n)
					bytesToWrite -= int64(n)
				}

				// Sync to ensure data is written to disk
				err = file.Sync()
				if err != nil {
					log.Fatalf("Failed to sync file: %v", err)
				}
			}

			// Update display if file size changed significantly
			if currentFileSizeMB != lastFileSizeMB {
				lastFileSizeMB = currentFileSizeMB
				if currentFileSizeMB > 0 && count%20 == 0 { // More frequent updates
					fmt.Printf("File size: %.1f MB / %.1f MB\n",
						float64(currentFileSizeMB),
						float64(rm.config.FileSizeMB))

					count = 0
				}
			}
		}
	}
}
