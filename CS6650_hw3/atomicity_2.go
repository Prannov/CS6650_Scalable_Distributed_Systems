package main

import (
	"fmt"
	"sync"
)

func main() {
	var ops uint64
	var wg sync.WaitGroup

	wg.Add(50)
	for i := 0; i < 50; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				ops++ // data race
			}
		}()
	}

	wg.Wait()
	fmt.Println("regular ops:", ops)
}
