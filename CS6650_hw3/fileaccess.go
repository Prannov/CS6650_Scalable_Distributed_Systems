package main

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

const (
	filename = "out.txt"
	nLines   = 100000
)

func main() {
	unbufDur, err := writeUnbuffered(filename, nLines)
	if err != nil {
		panic(err)
	}

	bufDur, err := writeBuffered(filename, nLines)
	if err != nil {
		panic(err)
	}

	fmt.Println("unbuffered:", unbufDur)
	fmt.Println("buffered:  ", bufDur)
}

func writeUnbuffered(path string, n int) (time.Duration, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	start := time.Now()
	for i := 0; i < n; i++ {
		line := fmt.Sprintf("%d\n", i)
		if _, err := f.Write([]byte(line)); err != nil {
			return 0, err
		}
	}
	return time.Since(start), nil
}

func writeBuffered(path string, n int) (time.Duration, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	start := time.Now()
	for i := 0; i < n; i++ {
		if _, err := w.WriteString(fmt.Sprintf("%d\n", i)); err != nil {
			return 0, err
		}
	}

	if err := w.Flush(); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}
