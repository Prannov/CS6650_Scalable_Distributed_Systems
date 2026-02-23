package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Product represents a searchable product
type Product struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Brand       string `json:"brand"`
}

// SearchResponse is the API response format
type SearchResponse struct {
	Products    []Product `json:"products"`
	TotalFound  int       `json:"total_found"`
	SearchTime  string    `json:"search_time"`
	Checked     int       `json:"products_checked"` // for verification
}

var (
	store    sync.Map
	products []Product // slice for indexed access (bounded search)
	totalProducts = 100_000
	searchLimit   = 100  // exactly 100 products checked per request
	maxResults    = 20
)

var brands     = []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta", "Eta", "Theta"}
var categories = []string{"Electronics", "Books", "Home", "Sports", "Clothing", "Toys", "Food", "Tools"}
var descs      = []string{
	"High quality product for everyday use.",
	"Premium grade, built to last.",
	"Budget-friendly and reliable.",
	"Top-rated by customers worldwide.",
}

func generateProducts() {
	log.Println("Generating 100,000 products...")
	products = make([]Product, totalProducts)
	for i := 0; i < totalProducts; i++ {
		brand := brands[i%len(brands)]
		p := Product{
			ID:          i + 1,
			Name:        fmt.Sprintf("Product %s %d", brand, i+1),
			Category:    categories[i%len(categories)],
			Description: descs[i%len(descs)],
			Brand:       brand,
		}
		products[i] = p
		store.Store(p.ID, p)
	}
	log.Printf("Generated %d products\n", totalProducts)
}

// simulateWork burns CPU to mimic fixed-cost computation (e.g. AI inference)
func simulateWork() {
	x := 0.0
	for i := 0; i < 100_000; i++ {
		x += float64(i) * 0.001
	}
	_ = x
}

// search checks EXACTLY searchLimit products, regardless of matches found
func search(query string) ([]Product, int, int) {
	q := strings.ToLower(query)
	var results []Product
	checked := 0

	simulateWork() // fixed CPU cost per request

	for i := 0; i < searchLimit; i++ { // always exactly 100 iterations
		p := products[i]
		checked++
		if strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Category), q) {
			if len(results) < maxResults {
				results = append(results, p)
			}
		}
	}

	return results, len(results), checked
}

// requestCounter for basic stats
var reqCount int64

func searchHandler(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&reqCount, 1)
	start := time.Now()

	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, `{"error":"missing query parameter q"}`, http.StatusBadRequest)
		return
	}

	results, total, checked := search(q)
	elapsed := time.Since(start)

	resp := SearchResponse{
		Products:   results,
		TotalFound: total,
		SearchTime: elapsed.String(),
		Checked:    checked,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy","products":%d}`, totalProducts)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"total_requests":%d,"products_loaded":%d}`,
		atomic.LoadInt64(&reqCount), totalProducts)
}

func main() {
	generateProducts()

	http.HandleFunc("/products/search", searchHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/stats", statsHandler)

	port := "8080"
	log.Printf("Server starting on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}