package main

import (
	"fmt"
	"testing"
	"time"
)

// 基准测试：模拟消耗 10MB 内存、10MB 文件、50% CPU，持续 2 秒
func BenchmarkResourceMock(b *testing.B) {
	count := 0
	workDuration := 6 * time.Millisecond
	for i := 0; i < b.N; i++ {
		workStart := time.Now()
		for time.Since(workStart) < workDuration {
			for i := 0; i < 10000; i++ {
				count += i * count
			}
		}
	}
	fmt.Println(count)
}
