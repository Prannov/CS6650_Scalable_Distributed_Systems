package main

import (
	"fmt"
	"runtime"
	"time"
)

const N = 1_000_000

func main() {
	runtime.GOMAXPROCS(1)
	avg1, dur1 := pingPong(N)
	fmt.Printf("GOMAXPROCS(1): total=%v avg=%v\n", dur1, avg1)

	runtime.GOMAXPROCS(runtime.NumCPU())
	avg2, dur2 := pingPong(N)
	fmt.Printf("GOMAXPROCS(%d): total=%v avg=%v\n", runtime.GOMAXPROCS(0), dur2, avg2)
}

func pingPong(n int) (avg time.Duration, total time.Duration) {
	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	done := make(chan struct{})

	go func() { // A
		for i := 0; i < n; i++ {
			<-ch1
			ch2 <- struct{}{}
		}
	}()

	go func() { // B (avoid final extra send)
		for i := 0; i < n; i++ {
			<-ch2
			if i < n-1 {
				ch1 <- struct{}{}
			}
		}
		close(done)
	}()

	start := time.Now()
	ch1 <- struct{}{}
	<-done

	total = time.Since(start)
	avg = total / time.Duration(2*n)
	return avg, total
}
