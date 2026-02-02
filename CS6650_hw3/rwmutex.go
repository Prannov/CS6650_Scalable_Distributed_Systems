package main

import (
	"fmt"
	"sync"
	"time"
)

type SafeMap struct {
	mu sync.RWMutex
	m  map[int]int
}

func (s *SafeMap) Set(k, v int) {
	s.mu.Lock()
	s.m[k] = v
	s.mu.Unlock()
}

func (s *SafeMap) Len() int {
	s.mu.RLock()
	n := len(s.m)
	s.mu.RUnlock()
	return n
}

func main() {
	sm := SafeMap{m: make(map[int]int)}

	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(50)
	for g := 0; g < 50; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				sm.Set(g*1000+i, i)
			}
		}()
	}
	wg.Wait()

	fmt.Println("len(m):", sm.Len())
	fmt.Println("time:", time.Since(start))
}
