package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// ── CLI flags ────────────────────────────────────────────────────────────────

var (
	leaderAddr  = flag.String("leader", "http://localhost:8080", "leader/coordinator URL for writes")
	nodeAddrs   = flag.String("nodes", "http://localhost:8080,http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084", "comma-separated node URLs for reads")
	writePct    = flag.Int("writes", 10, "percentage of requests that are writes (1-99)")
	workers     = flag.Int("workers", 20, "number of concurrent workers")
	duration    = flag.Int("duration", 30, "test duration in seconds")
	keyPoolSize = flag.Int("keys", 20, "number of keys in the pool (small = more key collisions)")
	outFile     = flag.String("out", "results.json", "output file for raw results")
)

// ── Data structures ──────────────────────────────────────────────────────────

type result struct {
	Op          string  // "read" or "write"
	Key         string
	LatencyMs   float64
	Stale       bool    // true if read returned an older version than we last wrote
	Version     int64   // version returned by the node
	TimestampMs int64   // unix ms when this op completed
	IntervalMs  int64   // ms since last write to this key (reads only)
}

type entry struct {
	Value   string `json:"Value"`
	Version int64  `json:"Version"`
}

// knownVersions tracks the highest version we have confirmed written per key.
// lastWriteMs tracks the timestamp (unix ms) of the last confirmed write per key.
var (
	kvMu          sync.RWMutex
	knownVersions = make(map[string]int64)
	lastWriteMs   = make(map[string]int64)
)

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	flag.Parse()

	nodes := splitCSV(*nodeAddrs)
	keys := makeKeys(*keyPoolSize)

	results := make([]result, 0, 10000)
	var mu sync.Mutex

	var totalOps atomic.Int64
	deadline := time.Now().Add(time.Duration(*duration) * time.Second)

	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			cli := &http.Client{Timeout: 5 * time.Second}

			for time.Now().Before(deadline) {
				key := keys[rng.Intn(len(keys))]
				isWrite := rng.Intn(100) < *writePct

				var r result
				if isWrite {
					r = doWrite(cli, key, rng)
				} else {
					r = doRead(cli, key, nodes[rng.Intn(len(nodes))])
				}

				mu.Lock()
				results = append(results, r)
				mu.Unlock()
				totalOps.Add(1)
			}
		}()
	}

	wg.Wait()

	// ── Summary ──────────────────────────────────────────────────────────────
	var reads, writes, stale int
	var readLat, writeLat float64
	for _, r := range results {
		if r.Op == "write" {
			writes++
			writeLat += r.LatencyMs
		} else {
			reads++
			readLat += r.LatencyMs
			if r.Stale {
				stale++
			}
		}
	}

	fmt.Printf("\n=== Load Test Results ===\n")
	fmt.Printf("Duration:      %ds\n", *duration)
	fmt.Printf("Workers:       %d\n", *workers)
	fmt.Printf("Write%%:        %d%%\n", *writePct)
	fmt.Printf("Key pool:      %d keys\n", *keyPoolSize)
	fmt.Printf("Total ops:     %d\n", totalOps.Load())
	fmt.Printf("Writes:        %d  avg latency: %.2fms\n", writes, safeDivide(writeLat, float64(writes)))
	fmt.Printf("Reads:         %d  avg latency: %.2fms\n", reads, safeDivide(readLat, float64(reads)))
	fmt.Printf("Stale reads:   %d / %d  (%.1f%%)\n", stale, reads, safeDivide(float64(stale)*100, float64(reads)))

	// ── Write raw results for graphing ───────────────────────────────────────
	f, err := os.Create(*outFile)
	if err != nil {
		log.Fatalf("cannot create output file: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(results)
	fmt.Printf("\nRaw results written to %s\n", *outFile)
}

// ── Operations ───────────────────────────────────────────────────────────────

func doWrite(cli *http.Client, key string, rng *rand.Rand) result {
	value := fmt.Sprintf("val-%d", rng.Int63())
	url := fmt.Sprintf("%s/set?key=%s&value=%s", *leaderAddr, key, value)

	start := time.Now()
	resp, err := cli.Post(url, "", nil)
	lat := ms(time.Since(start))

	if err != nil || resp.StatusCode != http.StatusCreated {
		return result{Op: "write", Key: key, LatencyMs: lat}
	}
	defer resp.Body.Close()

	now := time.Now().UnixMilli()
	kvMu.Lock()
	knownVersions[key]++
	v := knownVersions[key]
	lastWriteMs[key] = now
	kvMu.Unlock()

	return result{Op: "write", Key: key, LatencyMs: lat, Version: v, TimestampMs: now}
}

func doRead(cli *http.Client, key, node string) result {
	url := fmt.Sprintf("%s/get?key=%s", node, key)

	start := time.Now()
	resp, err := cli.Get(url)
	lat := ms(time.Since(start))

	if err != nil || resp.StatusCode == http.StatusNotFound {
		return result{Op: "read", Key: key, LatencyMs: lat}
	}
	defer resp.Body.Close()

	var e entry
	json.NewDecoder(resp.Body).Decode(&e)

	now := time.Now().UnixMilli()

	kvMu.RLock()
	known := knownVersions[key]
	lastW := lastWriteMs[key]
	kvMu.RUnlock()

	stale := known > 0 && e.Version < known
	intervalMs := int64(0)
	if lastW > 0 {
		intervalMs = now - lastW
	}

	return result{Op: "read", Key: key, LatencyMs: lat, Version: e.Version, Stale: stale, TimestampMs: now, IntervalMs: intervalMs}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func makeKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("key-%03d", i)
	}
	return keys
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range splitRaw(s) {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitRaw(s string) []string {
	var parts []string
	cur := ""
	for _, c := range s {
		if c == ',' {
			parts = append(parts, cur)
			cur = ""
		} else {
			cur += string(c)
		}
	}
	return append(parts, cur)
}

func ms(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

func safeDivide(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}