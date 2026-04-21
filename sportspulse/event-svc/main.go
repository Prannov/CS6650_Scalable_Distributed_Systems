package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

var (
	kafkaBroker = os.Getenv("KAFKA_BROKER")
	kafkaTopic  = os.Getenv("KAFKA_TOPIC")
)

var producer *kgo.Client

// GameEvent represents a single in-game action
type GameEvent struct {
	EventID    string    `json:"event_id"`
	PlayerID   string    `json:"player_id"`
	TeamID     string    `json:"team_id"`
	EventType  string    `json:"event_type"` // "shot", "assist", "rebound"
	Value      float64   `json:"value"`
	OccurredAt time.Time `json:"occurred_at"`
}

func initKafka() error {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaBroker),
		kgo.DefaultProduceTopic(kafkaTopic),
	)
	if err != nil {
		return fmt.Errorf("kafka init: %w", err)
	}
	producer = cl
	log.Printf("kafka producer connected to %s topic=%s", kafkaBroker, kafkaTopic)
	return nil
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "event-svc",
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
		http.Error(w, "missing required fields: event_id, player_id, event_type", http.StatusBadRequest)
		return
	}
	if evt.OccurredAt.IsZero() {
		evt.OccurredAt = time.Now().UTC()
	}

	// Serialize event to JSON for Kafka
	payload, err := json.Marshal(evt)
	if err != nil {
		http.Error(w, "serialization error", http.StatusInternalServerError)
		return
	}

	// Publish to Kafka — use player_id as key so same player's events
	// go to the same partition (preserves ordering per player)
	record := &kgo.Record{
		Key:   []byte(evt.PlayerID),
		Value: payload,
	}
	if err := producer.ProduceSync(context.Background(), record).FirstErr(); err != nil {
		log.Printf("kafka produce error: %v", err)
		http.Error(w, "failed to publish event", http.StatusInternalServerError)
		return
	}

	log.Printf("published event: player=%s type=%s value=%.1f", evt.PlayerID, evt.EventType, evt.Value)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "accepted",
		"event_id": evt.EventID,
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if err := initKafka(); err != nil {
		log.Fatalf("failed to connect to kafka: %v", err)
	}
	defer producer.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/events", ingestHandler)

	log.Printf("event-svc starting on :%s", port)
	if err := http.ListenAndServe(":"+port, corsMiddleware(mux)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}