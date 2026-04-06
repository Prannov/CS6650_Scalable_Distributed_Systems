package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Addresses — adjust if you change port mappings in docker-compose.
const (
	lfLeader    = "http://localhost:8080"
	lfFollower1 = "http://localhost:8081"
	lfFollower2 = "http://localhost:8082"
	lfFollower3 = "http://localhost:8083"
	lfFollower4 = "http://localhost:8084"

	llNode1 = "http://localhost:8080"
	llNode2 = "http://localhost:8081"
	llNode3 = "http://localhost:8082"
	llNode4 = "http://localhost:8083"
	llNode5 = "http://localhost:8084"
)

var lfFollowers = []string{lfFollower1, lfFollower2, lfFollower3, lfFollower4}
var llNodes = []string{llNode1, llNode2, llNode3, llNode4, llNode5}

type entry struct {
	Value   string `json:"Value"`
	Version int64  `json:"Version"`
}

// ── Leader-Follower Tests ────────────────────────────────────────────────────

// TestLF_LeaderReadConsistent: after a confirmed write, reading from the leader
// must always return the correct value.
func TestLF_LeaderReadConsistent(t *testing.T) {
	key, value := "lf-test-1", "hello"
	mustSet(t, lfLeader, key, value)

	e := mustGet(t, lfLeader, key)
	if e.Value != value {
		t.Errorf("leader read: got %q, want %q", e.Value, value)
	}
}

// TestLF_FollowerReadConsistent: after the leader acknowledges a write, reading
// from every follower must eventually return the correct value.
// (After the write is confirmed under W=5, all followers are already updated.)
func TestLF_FollowerReadConsistent(t *testing.T) {
	key, value := "lf-test-2", "world"
	mustSet(t, lfLeader, key, value)

	for _, f := range lfFollowers {
		e := mustGet(t, f, key)
		if e.Value != value {
			t.Errorf("follower %s read: got %q, want %q", f, e.Value, value)
		}
	}
}

// TestLF_InconsistencyWindow: sends a write then immediately fires local_read
// requests to followers BEFORE the replication window closes.
// Under W=1/R=5 or W=3/R=3 this should occasionally show stale data.
// The test does NOT fail on staleness — it just reports it so you can see it.
func TestLF_InconsistencyWindow(t *testing.T) {
	key := "lf-incon"
	stale := 0

	for i := 0; i < 20; i++ {
		value := fmt.Sprintf("v%d", i)

		// Fire write and immediately probe followers without waiting.
		go mustSet(t, lfLeader, key, value)

		// Give just enough time for the write to reach the leader but not all followers.
		time.Sleep(50 * time.Millisecond)

		for _, f := range lfFollowers {
			e, ok := localRead(f, key)
			if ok && e.Value != value {
				stale++
			}
		}
	}

	t.Logf("Observed %d stale local_reads out of %d probes", stale, 20*len(lfFollowers))
}

// ── Leaderless Tests ─────────────────────────────────────────────────────────

// TestLL_CoordinatorReadConsistent: after the coordinator confirms the write,
// reading from the coordinator must return the correct value.
func TestLL_CoordinatorReadConsistent(t *testing.T) {
	key, value := "ll-test-1", "coordinator-ok"
	coordinator := llNode1
	mustSet(t, coordinator, key, value)

	e := mustGet(t, coordinator, key)
	if e.Value != value {
		t.Errorf("coordinator read: got %q, want %q", e.Value, value)
	}
}

// TestLL_OtherNodeConsistent: after the coordinator confirms the write (W=N),
// reading from any other node must also return the correct value.
func TestLL_OtherNodeConsistent(t *testing.T) {
	key, value := "ll-test-2", "all-nodes-ok"
	mustSet(t, llNode1, key, value)

	for _, node := range []string{llNode2, llNode3, llNode4, llNode5} {
		e := mustGet(t, node, key)
		if e.Value != value {
			t.Errorf("node %s read: got %q, want %q", node, e.Value, value)
		}
	}
}

// TestLL_InconsistencyWindow: fires a write to one node then immediately reads
// from other nodes before replication completes.
// Reports (but does not fail on) stale reads to demonstrate the window.
func TestLL_InconsistencyWindow(t *testing.T) {
	key := "ll-incon"
	stale := 0
	total := 0

	for i := 0; i < 20; i++ {
		value := fmt.Sprintf("v%d", i)

		// Write to node1 without waiting for it to finish.
		go mustSet(t, llNode1, key, value)

		// Probe other nodes almost immediately.
		time.Sleep(30 * time.Millisecond)

		for _, node := range []string{llNode2, llNode3, llNode4, llNode5} {
			e, ok := localRead(node, key)
			total++
			if ok && e.Value != value {
				stale++
			}
		}
	}

	t.Logf("Observed %d stale reads out of %d total probes", stale, total)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func mustSet(t *testing.T, base, key, value string) {
	t.Helper()
	url := fmt.Sprintf("%s/set?key=%s&value=%s", base, key, value)
	resp, err := http.Post(url, "", nil)
	if err != nil {
		t.Fatalf("set %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("set %s: status %d", url, resp.StatusCode)
	}
}

func mustGet(t *testing.T, base, key string) entry {
	t.Helper()
	url := fmt.Sprintf("%s/get?key=%s", base, key)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get %s: status %d", url, resp.StatusCode)
	}
	var e entry
	json.NewDecoder(resp.Body).Decode(&e)
	return e
}

func localRead(base, key string) (entry, bool) {
	url := fmt.Sprintf("%s/local_read?key=%s", base, key)
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode == http.StatusNotFound {
		return entry{}, false
	}
	defer resp.Body.Close()
	var e entry
	json.NewDecoder(resp.Body).Decode(&e)
	return e, true
}