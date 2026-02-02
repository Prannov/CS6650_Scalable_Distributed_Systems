package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	var ops atomic.Uint64
	var wg sync.WaitGroup

	wg.Add(50)
	for i := 0; i < 50; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				ops.Add(1)
			}
		}()
	}

	wg.Wait()
	fmt.Println("atomic ops:", ops.Load())
}
