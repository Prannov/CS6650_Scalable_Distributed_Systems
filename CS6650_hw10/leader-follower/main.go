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

// node holds all runtime state for this instance.
type node struct {
	role  string   // "leader" or "follower"
	peers []string // follower URLs (only meaningful on leader)
	mode  string   // replication mode: "W5R1", "W1R5", "W3R3"
	store *store.Store
}

func main() {
	n := &node{
		role:  getEnv("ROLE", "follower"),
		mode:  getEnv("MODE", "W5R1"),
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
	log.Printf("[%s] mode=%s peers=%v listening on %s", n.role, n.mode, n.peers, addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

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
	if n.role == "follower" {
		http.Error(w, "writes must go to the leader", http.StatusForbidden)
		return
	}

	version := n.store.Set(key, value)

	if err := replicate(n, key, value, version); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

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

	var entry store.Entry
	var ok bool

	switch n.mode {
	case "W1R5":
		entry, ok = readAll(n, key)
	case "W3R3":
		entry, ok = readQuorum(n, key, 3)
	default:
		entry, ok = n.store.Get(key)
	}

	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

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

	// Spec: follower sleeps 100ms before applying the write.
	time.Sleep(100 * time.Millisecond)

	n.store.SetWithVersion(key, value, version)
	w.WriteHeader(http.StatusOK)
}

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