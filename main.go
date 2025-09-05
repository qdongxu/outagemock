package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
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

// parseFileSize parses a file size string with units (B, K, M, G, T)
// Examples: "100M", "1.5G", "500K", "2T"
func parseFileSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, nil
	}

	// Regular expression to match number and unit
	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([BKMGTP]?)$`)
	matches := re.FindStringSubmatch(strings.ToUpper(sizeStr))

	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid file size format: %s (expected format: number + unit, e.g., 100M, 1.5G)", sizeStr)
	}

	// Parse the numeric part
	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number in file size: %s", matches[1])
	}

	// Get the unit (default to B if not specified)
	unit := matches[2]
	if unit == "" {
		unit = "B"
	}

	// Convert to bytes based on unit
	var multiplier float64
	switch unit {
	case "B":
		multiplier = 1
	case "K":
		multiplier = 1024
	case "M":
		multiplier = 1024 * 1024
	case "G":
		multiplier = 1024 * 1024 * 1024
	case "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unsupported unit: %s (supported: B, K, M, G, T)", unit)
	}

	// Calculate total bytes
	totalBytes := int64(value * multiplier)

	// Convert to MB for internal use
	return totalBytes / (1024 * 1024), nil
}

// runDaemonCleanup runs the daemon cleanup process
func runDaemonCleanup(filePath string, delay time.Duration) {
	// Sleep for the specified delay
	time.Sleep(delay + 5*time.Second)

	// Try to remove the file, ignore if it doesn't exist
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		// Only log if it's not a "file not found" error
		log.Printf("Daemon cleanup: failed to remove file %s: %v", filePath, err)
	}

	// Exit the daemon process
	os.Exit(0)
}

func main() {
	var config Config
	var fileSizeStr string
	var daemonCleanupFile string
	var daemonDelay time.Duration

	flag.Float64Var(&config.CPUPercent, "cpu", 0, "CPU usage percentage (0-100)")
	flag.Int64Var(&config.MemoryMB, "memory", 0, "Memory size in MB")
	flag.StringVar(&fileSizeStr, "fsize", "0", "File size with unit (e.g., 100M, 1.5G, 500K, 2T)")
	flag.StringVar(&config.FilePath, "fpath", "outagemock_temp_file", "File path")
	flag.DurationVar(&config.Duration, "duration", 30*time.Second, "Running duration")
	flag.DurationVar(&config.RampupTime, "rampup", 10*time.Second, "Rampup time to reach target CPU and memory")

	// Check if we're running in daemon cleanup mode first
	for i, arg := range os.Args[1:] {
		switch arg {
		case "-daemon-cleanup":
			if i+1 < len(os.Args)-1 {
				daemonCleanupFile = os.Args[i+2]
			}
		case "-daemon-delay":
			if i+1 < len(os.Args)-1 {
				if d, err := time.ParseDuration(os.Args[i+2]); err == nil {
					daemonDelay = d
				}
			}
		}
	}

	// If daemon mode, run cleanup and exit
	if daemonCleanupFile != "" && daemonDelay > 0 {
		runDaemonCleanup(daemonCleanupFile, daemonDelay)
		return
	}

	// Parse regular flags only if not in daemon mode
	flag.Parse()

	// Parse file size with units
	var err error
	config.FileSizeMB, err = parseFileSize(fileSizeStr)
	if err != nil {
		log.Fatalf("Error parsing file size: %v", err)
	}

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

	// Start background daemon to clean up file after duration
	if config.FileSizeMB > 0 && config.FilePath != "" {
		err = forkFileCleanupDaemon(config.FilePath, config.Duration)
		if err != nil {
			log.Printf("Warning: failed to start cleanup daemon: %v", err)
			os.Exit(1)
		}
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
