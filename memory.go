package main

import (
	"fmt"
	"time"
)

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
