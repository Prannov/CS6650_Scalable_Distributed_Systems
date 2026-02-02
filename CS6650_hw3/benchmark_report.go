package main

import (
	"fmt"
	"sync"
	"time"
)

const (
	Goroutines = 50
	WritesPerG = 1000
	Runs       = 3
)

func main() {
	fmt.Printf("Benchmark: %d goroutines × %d writes each (%d total writes), %d runs\n",
		Goroutines, WritesPerG, Goroutines*WritesPerG, Runs)
	fmt.Println()

	mutexTimes := bench("mutex map", Runs, runMutexMapWrites)
	rwTimes := bench("rwmutex map", Runs, runRWMutexMapWrites)
	syncMapTimes := bench("sync.Map", Runs, runSyncMapWrites)

	fmt.Println("\n===== MEANS (ms) =====")
	fmt.Printf("mutex map:   %.3f ms\n", meanMillis(mutexTimes))
	fmt.Printf("rwmutex map: %.3f ms\n", meanMillis(rwTimes))
	fmt.Printf("sync.Map:    %.3f ms\n", meanMillis(syncMapTimes))

	fmt.Println("\n(Report tip) This workload is write-heavy, so RWMutex often does NOT help vs Mutex.")
}

func bench(name string, runs int, fn func() (time.Duration, int)) []time.Duration {
	fmt.Printf("=== %s ===\n", name)
	times := make([]time.Duration, 0, runs)
	for r := 1; r <= runs; r++ {
		d, n := fn()
		times = append(times, d)
		fmt.Printf("run %d: time=%v  len=%d\n", r, d, n)
	}
	return times
}

func meanMillis(ds []time.Duration) float64 {
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	return float64(sum.Microseconds()) / 1000.0 / float64(len(ds))
}

/* -------------------- MUTEX MAP -------------------- */

type mutexMap struct {
	mu sync.Mutex
	m  map[int]int
}

func runMutexMapWrites() (time.Duration, int) {
	sm := &mutexMap{m: make(map[int]int)}
	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(Goroutines)
	for g := 0; g < Goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			base := g * WritesPerG
			for i := 0; i < WritesPerG; i++ {
				sm.mu.Lock()
				sm.m[base+i] = i
				sm.mu.Unlock()
			}
		}()
	}
	wg.Wait()

	elapsed := time.Since(start)
	return elapsed, len(sm.m)
}

/* -------------------- RWMUTEX MAP -------------------- */

type rwmutexMap struct {
	mu sync.RWMutex
	m  map[int]int
}

func runRWMutexMapWrites() (time.Duration, int) {
	sm := &rwmutexMap{m: make(map[int]int)}
	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(Goroutines)
	for g := 0; g < Goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			base := g * WritesPerG
			for i := 0; i < WritesPerG; i++ {
				sm.mu.Lock()
				sm.m[base+i] = i
				sm.mu.Unlock()
			}
		}()
	}
	wg.Wait()

	elapsed := time.Since(start)
	return elapsed, len(sm.m)
}

/* -------------------- SYNC.MAP -------------------- */

func runSyncMapWrites() (time.Duration, int) {
	var m sync.Map
	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(Goroutines)
	for g := 0; g < Goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			base := g * WritesPerG
			for i := 0; i < WritesPerG; i++ {
				m.Store(base+i, i)
			}
		}()
	}
	wg.Wait()

	// Count entries using Range
	count := 0
	m.Range(func(_, _ any) bool {
		count++
		return true
	})

	elapsed := time.Since(start)
	return elapsed, count
}
