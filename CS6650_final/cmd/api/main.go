package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"album-store/internal/db"
	"album-store/internal/handlers"
	"album-store/internal/queue"
	"album-store/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
)

func main() {
	// ── Config from env ───────────────────────────────────────────────────
	dbDSN       := mustEnv("DATABASE_URL")
	redisAddr   := mustEnv("REDIS_ADDR")
	sqsURL      := mustEnv("SQS_QUEUE_URL")
	s3Bucket    := mustEnv("S3_BUCKET")
	awsRegion   := getEnv("AWS_REGION", "us-east-1")
	port        := getEnv("PORT", "8080")

	// ── Dependencies ──────────────────────────────────────────────────────
	database, err := db.New(dbDSN)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	if err := runMigrations(database); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		PoolSize: 50,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis: %v", err)
	}

	s3Store, err := storage.NewS3Store(s3Bucket, awsRegion)
	if err != nil {
		log.Fatalf("s3: %v", err)
	}

	sqsQueue, err := queue.NewSQSQueue(sqsURL, awsRegion)
	if err != nil {
		log.Fatalf("sqs: %v", err)
	}

	h := &handlers.Handler{
		DB:       database,
		Redis:    rdb,
		S3:       s3Store,
		SQS:      sqsQueue,
		S3Bucket: s3Bucket,
	}

	// ── Router ────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.Health)
	r.Get("/albums", h.ListAlbums)
	r.Put("/albums/{album_id}", h.PutAlbum)
	r.Get("/albums/{album_id}", h.GetAlbum)
	r.Post("/albums/{album_id}/photos", h.UploadPhoto)
	r.Get("/albums/{album_id}/photos/{photo_id}", h.GetPhoto)
	r.Delete("/albums/{album_id}/photos/{photo_id}", h.DeletePhoto)

	log.Printf("API listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func runMigrations(database *db.DB) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS albums (
			album_id    TEXT PRIMARY KEY,
			title       TEXT NOT NULL,
			description TEXT NOT NULL,
			owner       TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS photos (
			photo_id TEXT PRIMARY KEY,
			album_id TEXT NOT NULL REFERENCES albums(album_id),
			seq      BIGINT NOT NULL,
			status   TEXT NOT NULL DEFAULT 'processing',
			s3_key   TEXT NOT NULL,
			url      TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_photos_album_id ON photos(album_id);
	`)
	return err
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("env %s is required", key)
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
