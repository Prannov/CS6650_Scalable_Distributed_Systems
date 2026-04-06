package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"cs6650-hw10/store"
)

// replicate fans out a write to followers according to the node's mode.
func replicate(n *node, key, value string, version int64) error {
	switch n.mode {
	case "W5R1":
		return replicateSync(n, key, value, version, len(n.peers)) // wait for all
	case "W1R5":
		go replicateSync(n, key, value, version, len(n.peers)) // fire and forget
		return nil
	case "W3R3":
		// W=3 means leader + 2 followers must confirm before responding.
		// Remaining followers get replicated asynchronously.
		return replicateQuorum(n, key, value, version, 2)
	default:
		return fmt.Errorf("unknown mode: %s", n.mode)
	}
}

// replicateSync sends the update to all followers sequentially.
// The spec requires the leader to sleep 200ms after each message to a follower.
// It waits for `required` confirmations before returning.
func replicateSync(n *node, key, value string, version int64, required int) error {
	confirmed := 0
	for _, peer := range n.peers {
		err := sendReplicate(peer, key, value, version)

		// Spec: leader sleeps 200ms after each message to a follower.
		time.Sleep(200 * time.Millisecond)

		if err != nil {
			return fmt.Errorf("replication to %s failed: %w", peer, err)
		}
		confirmed++
		if confirmed >= required {
			break
		}
	}
	return nil
}

// replicateQuorum sends to all followers concurrently, waits for `need`
// confirmations, then lets the rest finish in the background.
func replicateQuorum(n *node, key, value string, version int64, need int) error {
	type result struct {
		peer string
		err  error
	}

	ch := make(chan result, len(n.peers))

	for _, peer := range n.peers {
		peer := peer
		go func() {
			err := sendReplicate(peer, key, value, version)
			time.Sleep(200 * time.Millisecond) // spec: 200ms after each message
			ch <- result{peer, err}
		}()
	}

	confirmed := 0
	for i := 0; i < len(n.peers); i++ {
		res := <-ch
		if res.err != nil {
			return fmt.Errorf("quorum replication to %s failed: %w", res.peer, res.err)
		}
		confirmed++
		if confirmed >= need {
			// Drain remaining results in background so goroutines don't leak.
			go func() {
				for j := i + 1; j < len(n.peers); j++ {
					<-ch
				}
			}()
			return nil
		}
	}
	return nil
}

// sendReplicate makes the HTTP call to a single follower's /replicate endpoint.
func sendReplicate(peer, key, value string, version int64) error {
	url := fmt.Sprintf("%s/replicate?key=%s&value=%s&version=%d", peer, key, value, version)
	resp, err := http.Post(url, "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("follower returned %d", resp.StatusCode)
	}
	return nil
}

// readAll fetches the entry from the leader's local store AND all followers,
// then returns the entry with the highest version. Used for W1R5.
// Followers sleep 50ms before responding (spec).
func readAll(n *node, key string) (store.Entry, bool) {
	entries := collectEntries(n, key, len(n.peers))
	return newest(entries)
}

// readQuorum fetches from local store + followers concurrently, waits until
// `quorum` responses are collected, then returns the newest. Used for W3R3.
func readQuorum(n *node, key string, quorum int) (store.Entry, bool) {
	// Leader counts as one response.
	entries := collectEntries(n, key, quorum-1)
	return newest(entries)
}

// collectEntries reads from the local store and fans out GET requests to
// `needed` followers concurrently, returning all entries received.
func collectEntries(n *node, key string, needed int) []store.Entry {
	var mu sync.Mutex
	var entries []store.Entry

	// Include local value immediately.
	if e, ok := n.store.Get(key); ok {
		entries = append(entries, e)
	}

	type result struct {
		entry store.Entry
		ok    bool
	}
	ch := make(chan result, len(n.peers))

	for _, peer := range n.peers {
		peer := peer
		go func() {
			e, ok := fetchRemote(peer, key)
			ch <- result{e, ok}
		}()
	}

	collected := 0
	for i := 0; i < len(n.peers) && collected < needed; i++ {
		res := <-ch
		if res.ok {
			mu.Lock()
			entries = append(entries, res.entry)
			mu.Unlock()
			collected++
		}
	}

	// Drain remaining goroutines in background.
	go func() {
		for i := collected; i < len(n.peers); i++ {
			<-ch
		}
	}()

	return entries
}

// fetchRemote calls /local_read on a peer and returns the Entry.
// The peer (follower) will sleep 50ms before responding per spec.
func fetchRemote(peer, key string) (store.Entry, bool) {
	url := fmt.Sprintf("%s/local_read?key=%s", peer, key)
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode == http.StatusNotFound {
		return store.Entry{}, false
	}
	defer resp.Body.Close()

	var e store.Entry
	if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
		return store.Entry{}, false
	}
	return e, true
}

// newest returns the entry with the highest version from a slice.
func newest(entries []store.Entry) (store.Entry, bool) {
	if len(entries) == 0 {
		return store.Entry{}, false
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Version > entries[j].Version
	})
	return entries[0], true
}