package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/sony/gobreaker"
)

var inventoryURL = os.Getenv("INVENTORY_URL")

// ✅ BULKHEAD: max 20 concurrent calls to inventory
var inventorySem = make(chan struct{}, 20)

// ✅ CIRCUIT BREAKER: opens after 5 consecutive failures
var cb = gobreaker.NewCircuitBreaker(gobreaker.Settings{
	Name:        "inventory-svc",
	MaxRequests: 3,               // allow 3 requests in half-open state
	Interval:    10 * time.Second,
	Timeout:     30 * time.Second, // stay open for 30s before retrying
	ReadyToTrip: func(counts gobreaker.Counts) bool {
		return counts.ConsecutiveFailures >= 5
	},
	OnStateChange: func(name string, from, to gobreaker.State) {
		log.Printf("🔌 circuit breaker [%s]: %s → %s", name, from, to)
	},
})

func searchHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	// ✅ FAIL FAST: total request budget is 2s
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	// ✅ BULKHEAD: reject immediately if pool is full
	select {
	case inventorySem <- struct{}{}:
		defer func() { <-inventorySem }()
	default:
		log.Println("🧱 bulkhead: rejected request, pool full")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Source", "bulkhead-reject")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{
			"results": []string{},
			"note":    "server too busy, try again shortly",
		})
		return
	}

	// ✅ CIRCUIT BREAKER: skip inventory if it's known-bad
	result, err := cb.Execute(func() (any, error) {
		req, err := http.NewRequestWithContext(ctx, "GET",
			inventoryURL+"/lookup?q="+q, nil)
		if err != nil {
			return nil, err
		}
		client := &http.Client{Timeout: 1500 * time.Millisecond}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	})

	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		// ✅ GRACEFUL DEGRADATION: return empty result instead of error
		log.Printf("⚠️  inventory unavailable (%v), serving fallback", err)
		w.Header().Set("X-Source", "fallback")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []string{},
			"note":    "inventory temporarily unavailable",
		})
		return
	}

	w.Header().Set("X-Source", "live")
	w.Write(result.([]byte))
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	state := cb.State().String()
	fmt.Fprintf(w, `{"goroutines":%d,"circuit_breaker":"%s","bulkhead_used":%d,"bulkhead_cap":20}`,
		runtime.NumGoroutine(), state, len(inventorySem))
}

func main() {
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/metrics", metricsHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	log.Println("search-svc (FIXED) listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}