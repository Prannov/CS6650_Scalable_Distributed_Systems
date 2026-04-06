package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// replicateAll fans out a write to every peer concurrently and waits for
// all of them to confirm (W=N). Returns an error if any peer fails.
func replicateAll(peers []string, key, value string, version int64) error {
	type result struct {
		peer string
		err  error
	}

	ch := make(chan result, len(peers))

	for _, peer := range peers {
		peer := peer
		go func() {
			err := sendReplicate(peer, key, value, version)
			// Spec: coordinator sleeps 200ms after each message.
			time.Sleep(200 * time.Millisecond)
			ch <- result{peer, err}
		}()
	}

	var mu sync.Mutex
	var errs []string

	for i := 0; i < len(peers); i++ {
		res := <-ch
		if res.err != nil {
			mu.Lock()
			errs = append(errs, fmt.Sprintf("%s: %v", res.peer, res.err))
			mu.Unlock()
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("replication failures: %v", errs)
	}
	return nil
}

// sendReplicate POSTs a replication message to a single peer.
func sendReplicate(peer, key, value string, version int64) error {
	url := fmt.Sprintf("%s/replicate?key=%s&value=%s&version=%d", peer, key, value, version)
	resp, err := http.Post(url, "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("peer returned %d", resp.StatusCode)
	}
	return nil
}