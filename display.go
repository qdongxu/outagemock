package main

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// DisplayManager manages the console display for resource monitoring
type DisplayManager struct {
	config        *Config
	rampupStart   time.Time
	displayTicker *time.Ticker
	stopChan      chan bool
}

// ResourceStatus holds current status of all resources
type ResourceStatus struct {
	CPUPercent     float64
	MemoryTargetMB int64
	MemoryActualMB int64
	FileTargetMB   int64
	FileActualMB   int64
}

// NewDisplayManager creates a new display manager
func NewDisplayManager(config *Config, rampupStart time.Time) *DisplayManager {
	return &DisplayManager{
		config:      config,
		rampupStart: rampupStart,
		stopChan:    make(chan bool),
	}
}

// Start begins the display updates
func (dm *DisplayManager) Start() {
	dm.displayTicker = time.NewTicker(2 * time.Second)

	// Show startup parameters and header
	dm.showStartupParameters()
	dm.showHeader()

	go dm.updateLoop()
}

// Stop stops the display updates
func (dm *DisplayManager) Stop() {
	if dm.displayTicker != nil {
		dm.displayTicker.Stop()
	}
	close(dm.stopChan)
}

// UpdateStatus updates the resource status and triggers display refresh
func (dm *DisplayManager) UpdateStatus(status ResourceStatus) {
	dm.showStatus(status)
}

// clearScreen clears the terminal screen
func (dm *DisplayManager) clearScreen() {
	fmt.Print("\033[2J\033[H")
}

// showStartupParameters displays the startup configuration
func (dm *DisplayManager) showStartupParameters() {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                           OUTAGE MOCK - RESOURCE MONITOR                     ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════╣")

	// CPU Configuration
	if dm.config.CPUPercent > 0 {
		fmt.Printf("║ CPU Target: %.1f%% (across %d cores)                                         ║\n",
			dm.config.CPUPercent, runtime.NumCPU())
	} else {
		fmt.Println("║ CPU Target: Disabled                                                      ║")
	}

	// Memory Configuration
	if dm.config.MemoryMB > 0 {
		fmt.Printf("║ Memory Target: %d MB                                                        ║\n", dm.config.MemoryMB)
	} else {
		fmt.Println("║ Memory Target: Disabled                                                    ║")
	}

	// File Configuration
	if dm.config.FileSizeMB > 0 {
		fmt.Printf("║ File Target: %d MB (path: %s)                                    ║\n",
			dm.config.FileSizeMB, dm.config.FilePath)
	} else {
		fmt.Println("║ File Target: Disabled                                           ║")
	}

	// Duration and Rampup
	fmt.Printf("║ Duration: %s, Rampup: %s                                            ║\n",
		dm.config.Duration, dm.config.RampupTime)

	fmt.Println("╚════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

// showHeader displays the column headers
func (dm *DisplayManager) showHeader() {
	fmt.Println("┌─────────────────────────────────────────────────────────────┐")
	fmt.Println("│ Time    │ CPU % │ Memory (MB)    │ File (MB)     │ Progress │")
	fmt.Println("│         │       │ Target/Actual  │ Target/Actual │          │")
	fmt.Println("├─────────────────────────────────────────────────────────────┤")
}

// showStatus displays the current resource status
func (dm *DisplayManager) showStatus(status ResourceStatus) {
	elapsed := time.Since(dm.rampupStart)
	elapsedStr := fmt.Sprintf("%02d:%02d", int(elapsed.Minutes()), int(elapsed.Seconds())%60)

	// Calculate progress percentage
	var progress float64
	if dm.config.RampupTime > 0 {
		progress = float64(elapsed) / float64(dm.config.RampupTime)
		if progress > 1.0 {
			progress = 1.0
		}
	}
	progressStr := fmt.Sprintf("%.1f%%", progress*100)

	// Format CPU
	cpuStr := "N/A"
	if dm.config.CPUPercent > 0 {
		cpuStr = fmt.Sprintf("%.1f", status.CPUPercent)
	}

	// Format Memory
	memStr := "N/A"
	if dm.config.MemoryMB > 0 {
		memStr = fmt.Sprintf("%d/%d", status.MemoryTargetMB, status.MemoryActualMB)
	}

	// Format File
	fileStr := "N/A"
	if dm.config.FileSizeMB > 0 {
		fileStr = fmt.Sprintf("%d/%d", status.FileTargetMB, status.FileActualMB)
	}

	// Display status on a new line (like logs)
	fmt.Printf("│ %-7s │ %-5s │ %-13s │ %-13s │ %-7s │\n",
		elapsedStr, cpuStr, memStr, fileStr, progressStr)
}

// updateLoop handles periodic display updates
func (dm *DisplayManager) updateLoop() {
	for {
		select {
		case <-dm.displayTicker.C:
			// This will be called by the main goroutine to update status
			// The actual status update is handled by UpdateStatus method
		case <-dm.stopChan:
			return
		}
	}
}

// Helper function to truncate long strings
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Helper function to format file path for display
func formatFilePath(path string) string {
	// If path is too long, show only the filename
	if len(path) > 30 {
		parts := strings.Split(path, "/")
		if len(parts) > 0 {
			return "..." + parts[len(parts)-1]
		}
	}
	return path
}
