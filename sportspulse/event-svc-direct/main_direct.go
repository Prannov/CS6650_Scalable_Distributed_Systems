package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var dbURL = os.Getenv("DB_URL")
var db *sql.DB

type GameEvent struct {
	EventID    string    `json:"event_id"`
	PlayerID   string    `json:"player_id"`
	TeamID     string    `json:"team_id"`
	EventType  string    `json:"event_type"`
	Value      float64   `json:"value"`
	OccurredAt time.Time `json:"occurred_at"`
}

func initDB() error {
	var err error
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		return fmt.Errorf("db open: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	return db.Ping()
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok", "service": "event-svc-direct",
	})
}

func ingestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var evt GameEvent
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if evt.EventID == "" || evt.PlayerID == "" || evt.EventType == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}
	if evt.OccurredAt.IsZero() {
		evt.OccurredAt = time.Now().UTC()
	}

	// Write directly to Postgres — no Kafka buffer
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO game_events (event_id, player_id, team_id, event_type, value, occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (event_id) DO NOTHING`,
		evt.EventID, evt.PlayerID, evt.TeamID, evt.EventType, evt.Value, evt.OccurredAt,
	)
	if err != nil {
		http.Error(w, "db insert error", http.StatusInternalServerError)
		return
	}

	switch evt.EventType {
	case "shot":
		_, err = tx.Exec(`
			INSERT INTO player_stats (player_id, points) VALUES ($1,$2)
			ON CONFLICT (player_id) DO UPDATE
			SET points = player_stats.points + $2, updated_at = NOW()`,
			evt.PlayerID, evt.Value)
	case "assist":
		_, err = tx.Exec(`
			INSERT INTO player_stats (player_id, assists) VALUES ($1,$2)
			ON CONFLICT (player_id) DO UPDATE
			SET assists = player_stats.assists + $2, updated_at = NOW()`,
			evt.PlayerID, evt.Value)
	case "rebound":
		_, err = tx.Exec(`
			INSERT INTO player_stats (player_id, rebounds) VALUES ($1,$2)
			ON CONFLICT (player_id) DO UPDATE
			SET rebounds = player_stats.rebounds + $2, updated_at = NOW()`,
			evt.PlayerID, evt.Value)
	}
	if err != nil {
		http.Error(w, "stats update error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "commit error", http.StatusInternalServerError)
		return
	}

	log.Printf("direct write: player=%s type=%s value=%.1f", evt.PlayerID, evt.EventType, evt.Value)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "accepted", "event_id": evt.EventID,
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := initDB(); err != nil {
		log.Fatalf("db init failed: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/events", ingestHandler)

	log.Printf("event-svc-direct starting on :%s (no Kafka)", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}