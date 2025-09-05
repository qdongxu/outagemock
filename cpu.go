package main

import (
	"fmt"
	"runtime"
	"time"
)

// consumeCPU simulates CPU usage across multiple cores
func (rm *ResourceMock) consumeCPU() {
	defer rm.wg.Done()

	if rm.config.CPUPercent <= 0 {
		return
	}

	numCPU := runtime.NumCPU()
	fmt.Printf("Starting CPU consumption (rampup to %.1f%% across %d cores)\n", rm.config.CPUPercent, numCPU)

	// Start one goroutine per CPU core
	for i := 0; i < numCPU; i++ {
		rm.wg.Add(1)
		go rm.cpuWorker(i)
	}
}

// cpuWorker simulates CPU usage on a single core
func (rm *ResourceMock) cpuWorker(coreID int) int {
	defer rm.wg.Done()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastCPUPercent := float64(-1)
	count := 0
	currentCPUPercent, _, _ := rm.getCurrentResourceUsage()

	for {
		select {
		case <-rm.ctx.Done():
			return count
		case <-ticker.C:
			currentCPUPercent, _, _ = rm.getCurrentResourceUsage()
		default:
			// Get current target CPU usage

			// Update sleep time if CPU percentage changed (only log from first core)
			if currentCPUPercent != lastCPUPercent && coreID == 0 {
				lastCPUPercent = currentCPUPercent
				if currentCPUPercent > 0 {
					fmt.Printf("CPU usage: %.1f%% (across %d cores)\n", currentCPUPercent, runtime.NumCPU())
				}
			}

			// Calculate work and sleep time based on current CPU percentage
			// For 30% CPU: work for 6ms, sleep for 14ms in a 20ms cycle
			workDuration := time.Duration(currentCPUPercent*0.2) * time.Millisecond
			sleepDuration := time.Duration((100-currentCPUPercent)*0.2) * time.Millisecond

			// Do CPU-intensive work for the calculated duration
			workStart := time.Now()
			for time.Since(workStart) < workDuration {
				// CPU-intensive work
				for i := 0; i < 10000; i++ {
					count += (i*count + i + count) / 13
				}
			}

			// Sleep for the remaining time to achieve target CPU usage
			if sleepDuration > 0 {
				time.Sleep(sleepDuration)
			}
		}
	}
}
