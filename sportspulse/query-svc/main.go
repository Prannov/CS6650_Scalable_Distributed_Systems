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
	"github.com/redis/go-redis/v9"
)

var (
	dbURL    = os.Getenv("DB_URL")
	redisURL = os.Getenv("REDIS_URL")
)

var (
	db  *sql.DB
	rdb *redis.Client
)

const cacheTTL = 10 * time.Second

// PlayerStats is the read model returned to clients
type PlayerStats struct {
	PlayerID    string  `json:"player_id"`
	PlayerName  string  `json:"player_name"`
	TeamID      string  `json:"team_id"`
	Points      float64 `json:"points"`
	Assists     float64 `json:"assists"`
	Rebounds    float64 `json:"rebounds"`
	GamesPlayed int     `json:"games_played"`
	UpdatedAt   string  `json:"updated_at"`
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

func initRedis() error {
	rdb = redis.NewClient(&redis.Options{Addr: redisURL})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	log.Println("redis connected")
	return nil
}

func getPlayerFromDB(ctx context.Context, playerID string) (*PlayerStats, error) {
	row := db.QueryRowContext(ctx, `
		SELECT p.player_id, p.player_name, p.team_id,
		       COALESCE(s.points,0), COALESCE(s.assists,0),
		       COALESCE(s.rebounds,0), COALESCE(s.games_played,0),
		       COALESCE(s.updated_at::text, '')
		FROM players p
		LEFT JOIN player_stats s ON p.player_id = s.player_id
		WHERE p.player_id = $1`, playerID)

	var ps PlayerStats
	if err := row.Scan(&ps.PlayerID, &ps.PlayerName, &ps.TeamID,
		&ps.Points, &ps.Assists, &ps.Rebounds, &ps.GamesPlayed, &ps.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &ps, nil
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

	ctx := r.Context()
	cacheKey := "player:" + playerID

	// 1. Try Redis cache first
	cached, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Source", "cache")
		w.Write([]byte(cached))
		log.Printf("cache HIT: player_id=%s", playerID)
		return
	}

	// 2. Cache miss — fetch from Postgres
	log.Printf("cache MISS: player_id=%s", playerID)
	ps, err := getPlayerFromDB(ctx, playerID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if ps == nil {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}

	// 3. Store in Redis for next request
	payload, _ := json.Marshal(ps)
	rdb.Set(ctx, cacheKey, payload, cacheTTL)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Source", "db")
	w.Write(payload)
}

func leaderboardHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cacheKey := "leaderboard:points"

	// Try cache first
	cached, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Source", "cache")
		w.Write([]byte(cached))
		return
	}

	// Fetch top 10 by points from Postgres
	rows, err := db.QueryContext(ctx, `
		SELECT p.player_id, p.player_name, p.team_id,
		       COALESCE(s.points,0), COALESCE(s.assists,0),
		       COALESCE(s.rebounds,0), COALESCE(s.games_played,0),
		       COALESCE(s.updated_at::text,'')
		FROM players p
		LEFT JOIN player_stats s ON p.player_id = s.player_id
		ORDER BY s.points DESC NULLS LAST
		LIMIT 10`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []PlayerStats
	for rows.Next() {
		var ps PlayerStats
		rows.Scan(&ps.PlayerID, &ps.PlayerName, &ps.TeamID,
			&ps.Points, &ps.Assists, &ps.Rebounds, &ps.GamesPlayed, &ps.UpdatedAt)
		results = append(results, ps)
	}

	payload, _ := json.Marshal(results)
	rdb.Set(ctx, cacheKey, payload, cacheTTL)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Source", "db")
	w.Write(payload)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	if err := initDB(); err != nil {
		log.Fatalf("db init failed: %v", err)
	}
	defer db.Close()

	if err := initRedis(); err != nil {
		log.Fatalf("redis init failed: %v", err)
	}
	defer rdb.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/stats", playerStatsHandler)
	mux.HandleFunc("/leaderboard", leaderboardHandler)
	mux.Handle("/", http.FileServer(http.Dir("./static")))

	log.Printf("query-svc starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}