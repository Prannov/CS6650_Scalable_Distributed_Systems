package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const (
	Goroutines     = 50
	OpsPerG        = 100_000
	PreloadKeys    = 50_000
	ReadPercent    = 95 // 95% reads, 5% writes
	Runs           = 3
	RandomSeedBase = 1337
)

func main() {
	fmt.Printf("Read-heavy benchmark: %d goroutines × %d ops each = %d ops\n",
		Goroutines, OpsPerG, Goroutines*OpsPerG)
	fmt.Printf("PreloadKeys=%d, Read=%d%% Write=%d%%, Runs=%d\n\n",
		PreloadKeys, ReadPercent, 100-ReadPercent, Runs)

	mutexTimes := bench("mutex map", Runs, runReadHeavyMutex)
	rwTimes := bench("rwmutex map", Runs, runReadHeavyRWMutex)
	syncMapTimes := bench("sync.Map", Runs, runReadHeavySyncMap)

	fmt.Println("\n===== MEANS (ms) =====")
	fmt.Printf("mutex map:   %.3f ms\n", meanMillis(mutexTimes))
	fmt.Printf("rwmutex map: %.3f ms\n", meanMillis(rwTimes))
	fmt.Printf("sync.Map:    %.3f ms\n", meanMillis(syncMapTimes))

	fmt.Println("\nExpected trend (usually): RWMutex <= Mutex in read-heavy workloads.")
	fmt.Println("sync.Map may win or lose depending on access pattern; it's not guaranteed.")
}

func bench(name string, runs int, fn func(seed int) time.Duration) []time.Duration {
	fmt.Printf("=== %s ===\n", name)
	times := make([]time.Duration, 0, runs)
	for r := 1; r <= runs; r++ {
		seed := RandomSeedBase + r
		d := fn(seed)
		times = append(times, d)
		fmt.Printf("run %d: time=%v\n", r, d)
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

func runReadHeavyMutex(seed int) time.Duration {
	sm := &mutexMap{m: make(map[int]int, PreloadKeys)}
	for k := 0; k < PreloadKeys; k++ {
		sm.m[k] = k
	}

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(Goroutines)

	for g := 0; g < Goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(seed + g*10_000)))

			for i := 0; i < OpsPerG; i++ {
				k := r.Intn(PreloadKeys)
				if r.Intn(100) < ReadPercent {
					sm.mu.Lock()
					_ = sm.m[k]
					sm.mu.Unlock()
				} else {
					sm.mu.Lock()
					sm.m[k] = sm.m[k] + 1
					sm.mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	return time.Since(start)
}

/* -------------------- RWMUTEX MAP -------------------- */

type rwmutexMap struct {
	mu sync.RWMutex
	m  map[int]int
}

func runReadHeavyRWMutex(seed int) time.Duration {
	sm := &rwmutexMap{m: make(map[int]int, PreloadKeys)}
	for k := 0; k < PreloadKeys; k++ {
		sm.m[k] = k
	}

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(Goroutines)

	for g := 0; g < Goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(seed + g*10_000)))

			for i := 0; i < OpsPerG; i++ {
				k := r.Intn(PreloadKeys)
				if r.Intn(100) < ReadPercent {
					sm.mu.RLock()
					_ = sm.m[k]
					sm.mu.RUnlock()
				} else {
					sm.mu.Lock()
					sm.m[k] = sm.m[k] + 1
					sm.mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	return time.Since(start)
}

/* -------------------- sync.Map -------------------- */

func runReadHeavySyncMap(seed int) time.Duration {
	var m sync.Map
	for k := 0; k < PreloadKeys; k++ {
		m.Store(k, k)
	}

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(Goroutines)

	for g := 0; g < Goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(seed + g*10_000)))

			for i := 0; i < OpsPerG; i++ {
				k := r.Intn(PreloadKeys)
				if r.Intn(100) < ReadPercent {
					_, _ = m.Load(k)
				} else {
					// RMW isn't atomic here; this is just to create write pressure.
					// If you need atomic increment per key, you’d store *atomic.Int64.
					v, _ := m.Load(k)
					m.Store(k, v.(int)+1)
				}
			}
		}()
	}

	wg.Wait()
	return time.Since(start)
}
