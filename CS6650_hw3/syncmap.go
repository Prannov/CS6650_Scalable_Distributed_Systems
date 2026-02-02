package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	var m sync.Map

	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(50)

	for g := 0; g < 50; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				m.Store(g*1000+i, i)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	count := 0
	m.Range(func(_, _ any) bool {
		count++
		return true
	})

	fmt.Println("len(m):", count)
	fmt.Println("time:", elapsed)
}
