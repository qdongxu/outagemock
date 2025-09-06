package main

import (
	"fmt"
	"runtime"
	"time"
)

const BlockBytes = 1024 * 1024

// Page represents a 4KB memory page
type Page struct {
	data [4096]byte
}

// Get returns the byte at the specified position
func (p *Page) Get(pos int) byte {
	return p.data[pos]
}

// Set sets the byte at the specified position
func (p *Page) Set(pos int, value byte) {
	p.data[pos] = value
}

// Block represents a 1MB memory block containing 256 pages
type Block struct {
	pages [256]*Page
}

// NewBlock creates a new block with allocated pages
func NewBlock() *Block {
	block := &Block{}
	for i := 0; i < 256; i++ {
		block.pages[i] = &Page{}
		// Fill page with pattern to ensure physical allocation
		for j := 0; j < 4096; j += 1023 {
			block.pages[i].Set(j, byte(j))
		}
	}
	return block
}

func (b *Block) Iter() {
	for i := 0; i < 256; i++ {
		page := b.pages[i]
		for j := 0; j < 4096; j += 1023 {
			page.Set(j, page.Get(j+1))
		}
	}
}

// Area represents a memory area containing multiple blocks
type Area struct {
	blocks []*Block
	curPos int
}

// NewArea creates a new area with the specified capacity
func NewArea(capacity int) *Area {
	return &Area{
		blocks: make([]*Block, 0, capacity),
	}
}

// Increase adds a new block to the area
func (a *Area) Increase() {
	a.blocks = append(a.blocks, NewBlock())
}

// GetBlockCount returns the number of blocks in the area
func (a *Area) GetBlockCount() int {
	return len(a.blocks)
}

// GetTotalSizeMB returns the total size in MB
func (a *Area) GetTotalSizeMB() int64 {
	return int64(len(a.blocks)) // Each block is 1MB
}

// Access performs random access on the memory area
func (a *Area) Access() {
	blockCount := len(a.blocks)
	if blockCount == 0 {
		return
	}
	a.curPos++
	nextRange := blockCount/100 + 1
	// Access multiple random pages
	for i := 0; i < nextRange; i++ {
		a.curPos++
		if a.curPos >= blockCount {
			a.curPos = 0
		}
		block := a.blocks[a.curPos]
		block.Iter()
	}
}

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

	// Use CPU count goroutines for better distribution
	numGoroutines := runtime.NumCPU()

	// Channel to send target memory to each worker
	targetChans := make([]chan int64, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		targetChans[i] = make(chan int64, 1)
	}

	// Channel to collect 1MB increments from workers
	incrementChan := make(chan int, numGoroutines*100) // Buffer for increments

	// Start memory allocation goroutines
	for i := 0; i < numGoroutines; i++ {
		rm.wg.Add(1)
		go rm.memoryWorker(i, targetChans[i], incrementChan)
	}

	// Update memory allocation every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Track actual allocated memory
	totalActualMB := int64(0)

	for {
		select {
		case <-rm.ctx.Done():
			// Signal all workers to stop
			for i := 0; i < numGoroutines; i++ {
				close(targetChans[i])
			}
			close(incrementChan)
			return
		case <-ticker.C:
			// Get current target memory usage based on rampup progress
			currentMemoryMB := rm.getCurrentMemoryUsage()

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
				case targetChans[i] <- target:
				case <-rm.ctx.Done():
					return
				default:
					// Channel might be full, skip
				}
			}

			// Print current status
			if currentMemoryMB > 0 {
				fmt.Printf("Target: %d MB, Actual: %d MB allocated across %d goroutines\n",
					currentMemoryMB, totalActualMB, numGoroutines)
			}
		case <-incrementChan:
			// Worker allocated 1MB, increment counter
			totalActualMB++
		}
	}
}

// memoryWorker allocates memory blocks and maintains them using Area structure
func (rm *ResourceMock) memoryWorker(workerID int, targetChan <-chan int64, incrementChan chan<- int) {
	defer rm.wg.Done()

	// Create memory area with initial capacity
	area := NewArea(4096) // Pre-allocate capacity for 4096 blocks (4GB)
	var currentTargetMB int64

	// Ticker for allocation and access
	allocTicker := time.NewTicker(10 * time.Millisecond)
	defer allocTicker.Stop()

	for {
		select {
		case <-rm.ctx.Done():
			return
		case targetMB, ok := <-targetChan:
			if !ok {
				return // Channel closed
			}
			currentTargetMB = targetMB
		case <-allocTicker.C:
			// Access memory to keep it active
			area.Access()

			// Allocate 1MB if we haven't reached target yet
			if currentTargetMB > 0 {
				currentMB := area.GetTotalSizeMB()
				if currentMB < currentTargetMB {
					// Add one 1MB block
					area.Increase()

					// Send 1MB increment to controller
					select {
					case incrementChan <- 1:
					case <-rm.ctx.Done():
						return
					default:
						// Channel might be full, continue
					}
				}
			}
		}
	}
}
