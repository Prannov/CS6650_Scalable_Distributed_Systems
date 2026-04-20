package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
)

var dbURL = os.Getenv("DB_URL")
var db *sql.DB

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
	// Tuned connection pool — simulates read replica configuration
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(25)
	return db.Ping()
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok", "service": "query-svc-nocache",
	})
}

func playerStatsHandler(w http.ResponseWriter, r *http.Request) {
	playerID := r.URL.Query().Get("player_id")
	if playerID == "" {
		http.Error(w, "missing player_id", http.StatusBadRequest)
		return
	}

	// Always hits Postgres — no cache
	row := db.QueryRowContext(r.Context(), `
		SELECT p.player_id, p.player_name, p.team_id,
		       COALESCE(s.points,0), COALESCE(s.assists,0),
		       COALESCE(s.rebounds,0), COALESCE(s.games_played,0),
		       COALESCE(s.updated_at::text,'')
		FROM players p
		LEFT JOIN player_stats s ON p.player_id = s.player_id
		WHERE p.player_id = $1`, playerID)

	var ps PlayerStats
	if err := row.Scan(&ps.PlayerID, &ps.PlayerName, &ps.TeamID,
		&ps.Points, &ps.Assists, &ps.Rebounds, &ps.GamesPlayed, &ps.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "player not found", http.StatusNotFound)
			return
		}
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Source", "db")
	json.NewEncoder(w).Encode(ps)
}

func leaderboardHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.QueryContext(r.Context(), `
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

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Source", "db")
	json.NewEncoder(w).Encode(results)
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
	mux.HandleFunc("/stats", playerStatsHandler)
	mux.HandleFunc("/leaderboard", leaderboardHandler)

	log.Printf("query-svc-nocache starting on :%s (no Redis)", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}