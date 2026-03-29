#!/bin/bash
set -e

echo "🏗️  Setting up SportsPulse repo..."

# ── Directories ────────────────────────────────────────────
mkdir -p event-svc stats-worker query-svc load-tests infra

# ── Clean any stray empty files ────────────────────────────
rm -f event-svc/handler.go event-svc/kafka.go
rm -f stats-worker/consumer.go stats-worker/aggregator.go stats-worker/db.go
rm -f query-svc/handler.go query-svc/cache.go query-svc/db.go

# ── event-svc/go.mod ───────────────────────────────────────
cat > event-svc/go.mod << 'EOF'
module event-svc

go 1.22
EOF

# ── event-svc/main.go ──────────────────────────────────────
cat > event-svc/main.go << 'EOF'
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

var kafkaBroker = os.Getenv("KAFKA_BROKER")
var kafkaTopic  = os.Getenv("KAFKA_TOPIC")

type GameEvent struct {
	EventID    string    `json:"event_id"`
	PlayerID   string    `json:"player_id"`
	TeamID     string    `json:"team_id"`
	EventType  string    `json:"event_type"`
	Value      float64   `json:"value"`
	OccurredAt time.Time `json:"occurred_at"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "event-svc"})
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
	log.Printf("received event: player=%s type=%s value=%.1f", evt.PlayerID, evt.EventType, evt.Value)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted", "event_id": evt.EventID})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" { port = "8080" }
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/events", ingestHandler)
	log.Printf("event-svc starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
EOF

# ── event-svc/Dockerfile ───────────────────────────────────
cat > event-svc/Dockerfile << 'EOF'
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o event-svc .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/event-svc .
EXPOSE 8080
CMD ["./event-svc"]
EOF

# ── stats-worker/go.mod ────────────────────────────────────
cat > stats-worker/go.mod << 'EOF'
module stats-worker

go 1.22
EOF

# ── stats-worker/main.go ───────────────────────────────────
cat > stats-worker/main.go << 'EOF'
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	kafkaBroker = os.Getenv("KAFKA_BROKER")
	kafkaTopic  = os.Getenv("KAFKA_TOPIC")
	kafkaGroup  = os.Getenv("KAFKA_GROUP")
	dbURL       = os.Getenv("DB_URL")
)

type GameEvent struct {
	EventID    string    `json:"event_id"`
	PlayerID   string    `json:"player_id"`
	TeamID     string    `json:"team_id"`
	EventType  string    `json:"event_type"`
	Value      float64   `json:"value"`
	OccurredAt time.Time `json:"occurred_at"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "stats-worker"})
}

func processEvent(evt GameEvent) error {
	log.Printf("processing event: player=%s type=%s value=%.1f", evt.PlayerID, evt.EventType, evt.Value)
	return nil
}

func main() {
	port := os.Getenv("PORT")
	if port == "" { port = "8082" }
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	log.Printf("stats-worker starting on :%s (kafka=%s topic=%s group=%s)", port, kafkaBroker, kafkaTopic, kafkaGroup)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
EOF

# ── stats-worker/Dockerfile ────────────────────────────────
cat > stats-worker/Dockerfile << 'EOF'
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o stats-worker .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/stats-worker .
EXPOSE 8082
CMD ["./stats-worker"]
EOF

# ── query-svc/go.mod ───────────────────────────────────────
cat > query-svc/go.mod << 'EOF'
module query-svc

go 1.22
EOF

# ── query-svc/main.go ──────────────────────────────────────
cat > query-svc/main.go << 'EOF'
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

var (
	dbURL    = os.Getenv("DB_URL")
	redisURL = os.Getenv("REDIS_URL")
)

type PlayerStats struct {
	PlayerID    string  `json:"player_id"`
	PlayerName  string  `json:"player_name"`
	TeamID      string  `json:"team_id"`
	Points      float64 `json:"points"`
	Assists     float64 `json:"assists"`
	Rebounds    float64 `json:"rebounds"`
	GamesPlayed int     `json:"games_played"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "query-svc"})
}

func playerStatsHandler(w http.ResponseWriter, r *http.Request) {
	playerID := r.URL.Query().Get("player_id")
	if playerID == "" {
		http.Error(w, "missing required query param: player_id", http.StatusBadRequest)
		return
	}
	stub := PlayerStats{PlayerID: playerID, PlayerName: "Stub Player", TeamID: "team-1"}
	log.Printf("query: player_id=%s (stub)", playerID)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Source", "stub")
	json.NewEncoder(w).Encode(stub)
}

func leaderboardHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Source", "stub")
	json.NewEncoder(w).Encode([]PlayerStats{})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" { port = "8081" }
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/stats", playerStatsHandler)
	mux.HandleFunc("/leaderboard", leaderboardHandler)
	log.Printf("query-svc starting on :%s (db=%s redis=%s)", port, dbURL, redisURL)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
EOF

# ── query-svc/Dockerfile ───────────────────────────────────
cat > query-svc/Dockerfile << 'EOF'
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o query-svc .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/query-svc .
EXPOSE 8081
CMD ["./query-svc"]
EOF

echo ""
echo "✅ Done! Now verifying builds..."
cd event-svc && go build . && echo "✅ event-svc builds" && cd ..
cd stats-worker && go build . && echo "✅ stats-worker builds" && cd ..
cd query-svc && go build . && echo "✅ query-svc builds" && cd ..
echo ""
echo "🚀 Run 'docker compose up --build' to start the full stack"