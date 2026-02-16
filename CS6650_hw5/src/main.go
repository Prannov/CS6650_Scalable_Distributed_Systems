package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Product represents the product schema from OpenAPI spec
type Product struct {
	ProductID   int    `json:"product_id"`
	SKU         string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID  int    `json:"category_id"`
	Weight      int    `json:"weight"`
	SomeOtherID int    `json:"some_other_id"`
}

// Error represents the error schema
type Error struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ProductStore holds products in memory
type ProductStore struct {
	mu       sync.RWMutex
	products map[int]Product
}

var store = &ProductStore{
	products: make(map[int]Product),
}

func main() {
	http.HandleFunc("/v1/products/", productHandler)
	http.HandleFunc("/v1/health", healthHandler)
	
	port := ":8080"
	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func productHandler(w http.ResponseWriter, r *http.Request) {
	// Set content type
	w.Header().Set("Content-Type", "application/json")
	
	// Extract productId from path
	path := strings.TrimPrefix(r.URL.Path, "/v1/products/")
	parts := strings.Split(path, "/")
	
	if len(parts) == 0 || parts[0] == "" {
		sendError(w, http.StatusBadRequest, "INVALID_PATH", "Product ID is required", "")
		return
	}
	
	productID, err := strconv.Atoi(parts[0])
	if err != nil || productID < 1 {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "Product ID must be a positive integer", "")
		return
	}
	
	// Route based on path and method
	if len(parts) == 1 {
		// /products/{productId}
		switch r.Method {
		case http.MethodGet:
			getProduct(w, r, productID)
		default:
			sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", "")
		}
	} else if len(parts) == 2 && parts[1] == "details" {
		// /products/{productId}/details
		switch r.Method {
		case http.MethodPost:
			addProductDetails(w, r, productID)
		default:
			sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", "")
		}
	} else {
		sendError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", "")
	}
}

func getProduct(w http.ResponseWriter, r *http.Request, productID int) {
	store.mu.RLock()
	product, exists := store.products[productID]
	store.mu.RUnlock()
	
	if !exists {
		sendError(w, http.StatusNotFound, "NOT_FOUND", "Product not found", fmt.Sprintf("Product with ID %d does not exist", productID))
		return
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(product)
}

func addProductDetails(w http.ResponseWriter, r *http.Request, productID int) {
	var product Product
	
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid JSON format", err.Error())
		return
	}
	
	// Validate required fields
	if product.SKU == "" {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "SKU is required", "")
		return
	}
	if product.Manufacturer == "" {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "Manufacturer is required", "")
		return
	}
	if product.CategoryID < 1 {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "Category ID must be positive", "")
		return
	}
	if product.Weight < 0 {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "Weight cannot be negative", "")
		return
	}
	if product.SomeOtherID < 1 {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "Some other ID must be positive", "")
		return
	}
	
	// Set the product ID from path parameter
	product.ProductID = productID
	
	// Store the product
	store.mu.Lock()
	store.products[productID] = product
	store.mu.Unlock()
	
	log.Printf("Product %d added/updated successfully", productID)
	w.WriteHeader(http.StatusNoContent)
}

func sendError(w http.ResponseWriter, statusCode int, errorCode, message, details string) {
	w.WriteHeader(statusCode)
	errorResponse := Error{
		Error:   errorCode,
		Message: message,
		Details: details,
	}
	json.NewEncoder(w).Encode(errorResponse)
}