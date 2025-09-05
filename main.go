package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// Config holds the configuration for the resource mock
type Config struct {
	CPUPercent float64       // CPU usage percentage (0-100)
	MemoryMB   int64         // Memory size in MB
	FileSizeMB int64         // File size in MB
	FilePath   string        // File path
	Duration   time.Duration // Running duration
	RampupTime time.Duration // Time to ramp up CPU and memory linearly
}

// ResourceMock manages the resource consumption
type ResourceMock struct {
	config      Config
	memory      []byte
	file        *os.File
	filePath    string
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	cleanup     sync.Once
	rampupStart time.Time
}

func main() {
	var config Config

	flag.Float64Var(&config.CPUPercent, "cpu", 50.0, "CPU usage percentage (0-100)")
	flag.Int64Var(&config.MemoryMB, "memory", 100, "Memory size in MB")
	flag.Int64Var(&config.FileSizeMB, "fsize", 200, "File size in MB")
	flag.StringVar(&config.FilePath, "fpath", "outagemock_temp_file", "File path")
	flag.DurationVar(&config.Duration, "duration", 30*time.Second, "Running duration")
	flag.DurationVar(&config.RampupTime, "rampup", 10*time.Second, "Rampup time to reach target CPU and memory")
	flag.Parse()

	// Validate configuration
	if config.CPUPercent < 0 || config.CPUPercent > 100 {
		log.Fatal("CPU percentage must be between 0 and 100")
	}
	if config.MemoryMB < 0 {
		log.Fatal("Memory size must be non-negative")
	}
	if config.FileSizeMB < 0 {
		log.Fatal("File size must be non-negative")
	}
	if config.Duration <= 0 {
		log.Fatal("Duration must be positive")
	}

	fmt.Printf("Starting resource mock with:\n")
	fmt.Printf("  CPU: %.1f%% (rampup: %v)\n", config.CPUPercent, config.RampupTime)
	fmt.Printf("  Memory: %d MB (rampup: %v)\n", config.MemoryMB, config.RampupTime)
	fmt.Printf("  File: %d MB at %s (rampup: %v)\n", config.FileSizeMB, config.FilePath, config.RampupTime)
	fmt.Printf("  Duration: %v\n", config.Duration)

	// Create resource mock
	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	rm := &ResourceMock{
		config:   config,
		ctx:      ctx,
		cancel:   cancel,
		filePath: config.FilePath,
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start resource consumption
	rm.Start()

	// Wait for completion or signal
	select {
	case <-ctx.Done():
		fmt.Println("Duration completed, shutting down...")
	case sig := <-sigChan:
		fmt.Printf("Received signal %v, shutting down...\n", sig)
		rm.Stop()
	}

	// Cleanup and exit
	rm.Cleanup()
	fmt.Println("Resource mock completed")
}

// Start begins resource consumption
func (rm *ResourceMock) Start() {
	rm.rampupStart = time.Now()

	// Allocate memory if requested
	if rm.config.MemoryMB > 0 {
		rm.wg.Add(1)
		go rm.consumeMemory()
	}

	// Create and grow file if requested
	if rm.config.FileSizeMB > 0 {
		rm.wg.Add(1)
		go rm.consumeFile()
	}

	// Consume CPU if requested
	if rm.config.CPUPercent > 0 {
		rm.wg.Add(1)
		go rm.consumeCPU()
	}
}

// Stop stops all resource consumption
func (rm *ResourceMock) Stop() {
	rm.cancel()
}

// getCurrentResourceUsage calculates current CPU, memory, and file usage based on rampup progress
func (rm *ResourceMock) getCurrentResourceUsage() (float64, int64, int64) {
	elapsed := time.Since(rm.rampupStart)

	// If rampup time is 0 or elapsed time exceeds rampup time, use target values
	if rm.config.RampupTime <= 0 || elapsed >= rm.config.RampupTime {
		return rm.config.CPUPercent, rm.config.MemoryMB, rm.config.FileSizeMB
	}

	// Calculate rampup progress (0.0 to 1.0)
	progress := float64(elapsed) / float64(rm.config.RampupTime)

	// Linear interpolation from 0 to target
	currentCPU := progress * rm.config.CPUPercent
	currentMemory := int64(progress * float64(rm.config.MemoryMB))
	currentFileSize := int64(progress * float64(rm.config.FileSizeMB))

	return currentCPU, currentMemory, currentFileSize
}

// Cleanup performs cleanup operations
func (rm *ResourceMock) Cleanup() {
	rm.cleanup.Do(func() {
		rm.cancel()
		rm.wg.Wait()

		// Close and remove file
		if rm.file != nil {
			rm.file.Close()
		}
		if rm.filePath != "" {
			os.Remove(rm.filePath)
		}

		// Clear memory
		rm.memory = nil
		runtime.GC()
	})
}

// consumeMemory allocates and randomly accesses memory
func (rm *ResourceMock) consumeMemory() {
	defer rm.wg.Done()

	// Allocate initial memory (will be resized during rampup)
	rm.memory = make([]byte, 0)

	// Randomly access memory to prevent swapping
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	lastAllocatedMB := int64(0)

	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			// Get current target memory usage
			_, currentMemoryMB, _ := rm.getCurrentResourceUsage()

			// Resize memory if needed
			if currentMemoryMB != lastAllocatedMB {
				memorySize := currentMemoryMB * 1024 * 1024
				rm.memory = make([]byte, memorySize)

				// Fill memory with data to ensure it's actually allocated
				for i := range rm.memory {
					rm.memory[i] = byte(i % 256)
				}

				lastAllocatedMB = currentMemoryMB
				if currentMemoryMB > 0 {
					fmt.Printf("Allocated %d MB of memory\n", currentMemoryMB)
				}
			}

			// Random access to prevent swapping (only if we have memory allocated)
			if len(rm.memory) > 0 {
				for i := 0; i < 1000; i++ {
					idx := (i * 7919) % len(rm.memory) // Use prime number for better distribution
					rm.memory[idx] = byte(rm.memory[idx] + 1)
				}
			}
		}
	}
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

// consumeCPU simulates CPU usage
func (rm *ResourceMock) consumeCPU() {
	defer rm.wg.Done()

	if rm.config.CPUPercent <= 0 {
		return
	}

	fmt.Printf("Starting CPU consumption (rampup to %.1f%%)\n", rm.config.CPUPercent)

	lastCPUPercent := float64(-1)

	for {
		select {
		case <-rm.ctx.Done():
			return
		default:
			// Get current target CPU usage
			currentCPUPercent, _, _ := rm.getCurrentResourceUsage()

			// Update sleep time if CPU percentage changed
			if currentCPUPercent != lastCPUPercent {
				lastCPUPercent = currentCPUPercent
				if currentCPUPercent > 0 {
					fmt.Printf("CPU usage: %.1f%%\n", currentCPUPercent)
				}
			}

			// Calculate sleep time based on current CPU percentage
			// For 100% CPU, we don't sleep at all
			// For 50% CPU, we sleep 50% of the time
			sleepTime := time.Duration((100-currentCPUPercent)*10000) * time.Microsecond

			// Do some CPU-intensive work
			for i := 0; i < 10000; i++ {
				_ = i * i
			}

			// Sleep to control CPU usage
			if sleepTime > 0 {
				time.Sleep(sleepTime)
			}
		}
	}
}
