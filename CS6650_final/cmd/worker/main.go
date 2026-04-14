package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"album-store/internal/db"
	"album-store/internal/queue"
	"album-store/internal/storage"
)

type photoMsg struct {
	PhotoID string `json:"photo_id"`
	AlbumID string `json:"album_id"`
	S3Key   string `json:"s3_key"`
	URL     string `json:"url"`
}

func main() {
	dbDSN     := mustEnv("DATABASE_URL")
	sqsURL    := mustEnv("SQS_QUEUE_URL")
	s3Bucket  := mustEnv("S3_BUCKET")
	awsRegion := getEnv("AWS_REGION", "us-east-1")

	database, err := db.New(dbDSN)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	sqsQueue, err := queue.NewSQSQueue(sqsURL, awsRegion)
	if err != nil {
		log.Fatalf("sqs: %v", err)
	}

	// S3 store needed only to verify bucket config; actual URL came from API
	_, err = storage.NewS3Store(s3Bucket, awsRegion)
	if err != nil {
		log.Fatalf("s3: %v", err)
	}

	log.Println("Worker started, polling SQS...")
	ctx := context.Background()

	// Run multiple concurrent pollers for throughput
	for i := 0; i < 5; i++ {
		go pollLoop(ctx, sqsQueue, database)
	}

	// Block forever
	select {}
}

func pollLoop(ctx context.Context, q *queue.SQSQueue, database *db.DB) {
	for {
		msgs, err := q.Receive(ctx)
		if err != nil {
			log.Printf("receive error: %v", err)
			continue
		}
		for _, m := range msgs {
			processMessage(ctx, m, q, database)
		}
	}
}

func processMessage(ctx context.Context, m queue.Message, q *queue.SQSQueue, database *db.DB) {
	var msg photoMsg
	if err := json.Unmarshal([]byte(m.Body), &msg); err != nil {
		log.Printf("unmarshal: %v", err)
		// Bad message — delete it so it doesn't loop forever
		_ = q.Delete(ctx, m.ReceiptHandle)
		return
	}

	// The API already uploaded to S3 and passed the URL — just mark completed
	if err := database.MarkPhotoCompleted(msg.PhotoID, msg.URL); err != nil {
		log.Printf("mark completed photo=%s err=%v", msg.PhotoID, err)
		database.MarkPhotoFailed(msg.PhotoID)
		// Don't delete from SQS — let it retry via visibility timeout
		return
	}

	log.Printf("completed photo=%s album=%s", msg.PhotoID, msg.AlbumID)
	_ = q.Delete(ctx, m.ReceiptHandle)
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
