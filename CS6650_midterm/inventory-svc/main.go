package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// 🔥 CHAOS SWITCH: set to true to simulate a slow downstream
const simulateSlow = true

func lookupHandler(w http.ResponseWriter, r *http.Request) {
	if simulateSlow {
		log.Println("💤 inventory: sleeping 5s (chaos mode)")
		time.Sleep(5 * time.Second)
	}

	q := r.URL.Query().Get("q")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"query":   q,
		"results": []string{"Nike Air Max", "Adidas Ultraboost", "New Balance 990"},
	})
}

func main() {
	http.HandleFunc("/lookup", lookupHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	log.Println("inventory-svc listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}