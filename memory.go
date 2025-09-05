package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"time"

)

// getCurrentMemoryUsage calculates current memory usage based on rampup progress
func (rm *ResourceMock) getCurrentMemoryUsage() int64 {
	elapsed := time.Since(rm.rampupStart)

	// If rampup time is 0 or elapsed time exceeds rampup time, use target values
	if rm.config.RampupTime <= 0 || elapsed >= rm.config.RampupTime {
		return rm.config.MemoryMB
	}

	// Calculate rampup progress (0.0 to 1.0)
	progress := float64(elapsed) / float64(rm.config.RampupTime)

	// Linear interpolation from 0 to target
	return int64(progress * float64(rm.config.MemoryMB))
}

// consumeMemory allocates and randomly accesses memory using multiple goroutines
func (rm *ResourceMock) consumeMemory() {
	defer rm.wg.Done()

	// Use CPU count * 10 goroutines for better distribution
	numGoroutines := runtime.NumCPU() * 10
	pageSize := 4 * 1024     // 4KB page size, same as kernel page size
	blockSize := 1024 * 1024 // 1MB block size for allocation

	// Channel to communicate target memory to goroutines
	memoryChan := make(chan int64, numGoroutines)

	// Start memory allocation goroutines
	for i := 0; i < numGoroutines; i++ {
		rm.wg.Add(1)
		go rm.memoryWorker(i, memoryChan, pageSize, blockSize)
	}

	// Update memory allocation every 100ms
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastAllocatedMB := int64(0)

	for {
		select {
		case <-rm.ctx.Done():
			// Signal all workers to stop
			close(memoryChan)
			return
		case <-ticker.C:
			// Get current target memory usage
			currentMemoryMB := rm.getCurrentMemoryUsage()

			// Update memory allocation if needed
			if currentMemoryMB != lastAllocatedMB {
				// Calculate memory per goroutine
				memoryPerGoroutine := currentMemoryMB / int64(numGoroutines)
				remainingMemory := currentMemoryMB % int64(numGoroutines)

				// Send target memory to each goroutine
				for i := 0; i < numGoroutines; i++ {
					target := memoryPerGoroutine
					if i < int(remainingMemory) {
						target++ // Distribute remaining memory to first few goroutines
					}
					select {
					case memoryChan <- target:
					case <-rm.ctx.Done():
						return
					}
				}

				lastAllocatedMB = currentMemoryMB
				if currentMemoryMB > 0 {
					fmt.Printf("Allocated %d MB of memory across %d goroutines\n", currentMemoryMB, numGoroutines)
				}
			}
		}
	}
}

// memoryWorker allocates 4KB pages and maintains them in a slice
func (rm *ResourceMock) memoryWorker(workerID int, memoryChan <-chan int64, pageSize int, blockSize int) {
	defer rm.wg.Done()

	// Use slice to store 4KB memory pages
	var memoryPages [][]byte
	var currentTargetMB int64

	// Ticker for adding 1MB blocks every 10ms
	blockTicker := time.NewTicker(10 * time.Millisecond)
	defer blockTicker.Stop()

	// Ticker for random access
	accessTicker := time.NewTicker(10 * time.Millisecond)
	defer accessTicker.Stop()

	for {
		select {
		case <-rm.ctx.Done():
			return
		case targetMB, ok := <-memoryChan:
			if !ok {
				return // Channel closed
			}
			currentTargetMB = targetMB
		case <-blockTicker.C:
			// Add 1MB block every 10ms if we haven't reached target
			if currentTargetMB > 0 {
				currentMB := int64(len(memoryPages)*pageSize) / (1024 * 1024)
				if currentMB < currentTargetMB {
					// Add 1MB worth of 4KB pages
					pagesToAdd := blockSize / pageSize // 256 pages = 1MB
					for i := 0; i < pagesToAdd; i++ {
						page := make([]byte, pageSize)
						// Fill page with pattern to ensure physical allocation
						for j := 0; j < pageSize; j += 128 {
							page[j] = byte(j)
						}
						memoryPages = append(memoryPages, page)
					}
				}
			}
		case <-accessTicker.C:
			length := len(memoryPages)
			// Random access to prevent swapping
			if length <= 0 {
				continue
			}
			for i := 0; i < length/100+1; i++ {
				// Access one random page
				pageIdx1 := rand.Int63n(int64(length))
				pageIdx2 := (pageIdx1 + 377 ) % int64(length)
				page1 := memoryPages[pageIdx1]
				page2 := memoryPages[pageIdx2]
				// Access one random position within the page
				pos1 := rand.Int31n(int32(pageSize))
				pos2 :=  (pos1 + 2739) % int32(pageSize)
				page1[pos1] = page2[pos2]
			}
		}
	}
}
