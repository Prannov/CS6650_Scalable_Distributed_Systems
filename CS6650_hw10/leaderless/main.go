package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"cs6650-hw10/store"
)

type node struct {
	self  string   // this node's own base URL, e.g. "http://node1:8080"
	peers []string // all OTHER nodes' base URLs
	store *store.Store
}

func main() {
	n := &node{
		self:  getEnv("SELF", "http://localhost:8080"),
		store: store.New(),
	}

	if raw := os.Getenv("PEERS"); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			if p = strings.TrimSpace(p); p != "" {
				n.peers = append(n.peers, p)
			}
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/set", n.handleSet)
	mux.HandleFunc("/get", n.handleGet)
	mux.HandleFunc("/replicate", n.handleReplicate)
	mux.HandleFunc("/local_read", n.handleLocalRead)

	addr := getEnv("ADDR", ":8080")
	log.Printf("[leaderless] self=%s peers=%v listening on %s", n.self, n.peers, addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// handleSet — this node becomes the Write Coordinator for this request.
// W=N: must replicate to ALL peers before responding 201.
func (n *node) handleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	value := r.URL.Query().Get("value")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	// Write locally first to assign the version.
	version := n.store.Set(key, value)

	// Fan out to all peers and wait for all confirmations (W=N).
	if err := replicateAll(n.peers, key, value, version); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// handleGet — just return local value. R=1.
// This is intentionally simple and exposes the inconsistency window.
func (n *node) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	entry, ok := n.store.Get(key)
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

// handleReplicate — internal endpoint called by the Write Coordinator.
func (n *node) handleReplicate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	value := r.URL.Query().Get("value")
	versionStr := r.URL.Query().Get("version")

	var version int64
	if _, err := fmt.Sscan(versionStr, &version); err != nil {
		http.Error(w, "invalid version", http.StatusBadRequest)
		return
	}

	// Spec: follower sleeps 100ms before applying.
	time.Sleep(100 * time.Millisecond)

	n.store.SetWithVersion(key, value, version)
	w.WriteHeader(http.StatusOK)
}

// handleLocalRead — raw local read, no coordination. Used in unit tests.
func (n *node) handleLocalRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	entry, ok := n.store.Get(key)
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}