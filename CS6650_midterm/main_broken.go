package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
)

var inventoryURL = os.Getenv("INVENTORY_URL")

func searchHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	// ❌ No timeout — blocks forever if inventory is slow
	resp, err := http.Get(inventoryURL + "/lookup?q=" + q)
	if err != nil {
		http.Error(w, `{"error":"inventory unreachable"}`, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// ❌ Blocking read — goroutine stuck until inventory responds
	body, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	// Shows goroutine count so you can watch it explode
	fmt.Fprintf(w, `{"goroutines": %d}`, runtime.NumGoroutine())
}

func main() {
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/metrics", metricsHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	log.Println("search-svc (BROKEN) listening on :8080")
	// ❌ Default mux: no connection limits, no timeouts, unbounded goroutines
	log.Fatal(http.ListenAndServe(":8080", nil))
}