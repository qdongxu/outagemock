package main

import (
	"runtime"
	"time"
)

// getCurrentCPUUsage calculates current CPU usage based on rampup progress
func (rm *ResourceMock) getCurrentCPUUsage() float64 {
	elapsed := time.Since(rm.rampupStart)

	// If rampup time is 0 or elapsed time exceeds rampup time, use target values
	if rm.config.RampupTime <= 0 || elapsed >= rm.config.RampupTime {
		return rm.config.CPUPercent
	}

	// Calculate rampup progress (0.0 to 1.0)
	progress := float64(elapsed) / float64(rm.config.RampupTime)

	// Linear interpolation from 0 to target
	return progress * rm.config.CPUPercent
}

// consumeCPU simulates CPU usage across multiple cores
func (rm *ResourceMock) consumeCPU() {
	defer rm.wg.Done()

	if rm.config.CPUPercent <= 0 {
		return
	}

	numCPU := runtime.NumCPU()
	//fmt.Printf("Starting CPU consumption (rampup to %.1f%% across %d cores)\n", rm.config.CPUPercent, numCPU)

	// Start one goroutine per CPU core
	for i := 0; i < numCPU; i++ {
		rm.wg.Add(1)
		go rm.cpuWorker(i)
	}
}

// cpuWorker simulates CPU usage on a single core
func (rm *ResourceMock) cpuWorker(coreID int) int {
	defer rm.wg.Done()

	workDuration := time.Duration(0)
	sleepDuration := time.Duration(0)
	count := 0
	currentCPUPercent := rm.getCurrentCPUUsage()

	for {
		select {
		case <-rm.ctx.Done():
			return count
		default:
			// Get current target CPU usage
			currentCPUPercent = rm.getCurrentCPUUsage()

			// Calculate work and sleep time based on current CPU percentage
			// For 30% CPU: work for 6ms, sleep for 14ms in a 20ms cycle
			workDuration = time.Duration(currentCPUPercent*0.2) * time.Millisecond
			sleepDuration = time.Duration((100-currentCPUPercent)*0.2) * time.Millisecond

			// Do CPU-intensive work for the calculated duration
			workStart := time.Now()
			for time.Since(workStart) <= workDuration {
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
