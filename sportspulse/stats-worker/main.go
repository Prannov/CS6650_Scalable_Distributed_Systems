package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/twmb/franz-go/pkg/kgo"
)

var (
	kafkaBroker = os.Getenv("KAFKA_BROKER")
	kafkaTopic  = os.Getenv("KAFKA_TOPIC")
	kafkaGroup  = os.Getenv("KAFKA_GROUP")
	dbURL       = os.Getenv("DB_URL")
)

var db *sql.DB

// GameEvent mirrors the event-svc payload
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
	if err = db.Ping(); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}
	log.Println("postgres connected")
	return nil
}

// processEvent writes the raw event and upserts aggregated stats
func processEvent(ctx context.Context, evt GameEvent) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// 1. Insert raw event (idempotent — skip duplicates)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO game_events (event_id, player_id, team_id, event_type, value, occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (event_id) DO NOTHING`,
		evt.EventID, evt.PlayerID, evt.TeamID, evt.EventType, evt.Value, evt.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	// 2. Upsert aggregated stats based on event type
	switch evt.EventType {
	case "shot":
		_, err = tx.ExecContext(ctx, `
			INSERT INTO player_stats (player_id, points) VALUES ($1, $2)
			ON CONFLICT (player_id) DO UPDATE
			SET points = player_stats.points + $2, updated_at = NOW()`,
			evt.PlayerID, evt.Value)
	case "assist":
		_, err = tx.ExecContext(ctx, `
			INSERT INTO player_stats (player_id, assists) VALUES ($1, $2)
			ON CONFLICT (player_id) DO UPDATE
			SET assists = player_stats.assists + $2, updated_at = NOW()`,
			evt.PlayerID, evt.Value)
	case "rebound":
		_, err = tx.ExecContext(ctx, `
			INSERT INTO player_stats (player_id, rebounds) VALUES ($1, $2)
			ON CONFLICT (player_id) DO UPDATE
			SET rebounds = player_stats.rebounds + $2, updated_at = NOW()`,
			evt.PlayerID, evt.Value)
	}
	if err != nil {
		return fmt.Errorf("upsert stats: %w", err)
	}

	return tx.Commit()
}

func startConsumer(ctx context.Context) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaBroker),
		kgo.ConsumerGroup(kafkaGroup),
		kgo.ConsumeTopics(kafkaTopic),
	)
	if err != nil {
		log.Fatalf("kafka consumer init: %v", err)
	}
	defer cl.Close()
	log.Printf("kafka consumer started (broker=%s topic=%s group=%s)", kafkaBroker, kafkaTopic, kafkaGroup)

	for {
		fetches := cl.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return
		}
		fetches.EachError(func(t string, p int32, err error) {
			log.Printf("fetch error topic=%s partition=%d: %v", t, p, err)
		})
		fetches.EachRecord(func(r *kgo.Record) {
			var evt GameEvent
			if err := json.Unmarshal(r.Value, &evt); err != nil {
				log.Printf("unmarshal error: %v", err)
				return
			}
			if err := processEvent(ctx, evt); err != nil {
				log.Printf("process error event=%s: %v", evt.EventID, err)
				return
			}
			log.Printf("processed: player=%s type=%s value=%.1f", evt.PlayerID, evt.EventType, evt.Value)
		})
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "stats-worker"})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	if err := initDB(); err != nil {
		log.Fatalf("db init failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	go startConsumer(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	log.Printf("stats-worker starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}