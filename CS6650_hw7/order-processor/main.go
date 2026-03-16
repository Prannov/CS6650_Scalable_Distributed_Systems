package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type Item struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type Order struct {
	OrderID    string    `json:"order_id"`
	CustomerID int       `json:"customer_id"`
	Status     string    `json:"status"`
	Items      []Item    `json:"items"`
	CreatedAt  time.Time `json:"created_at"`
}

// SNS wraps the actual message payload in a JSON envelope
type SNSEnvelope struct {
	Message string `json:"Message"`
}

var (
	sqsClient   *sqs.Client
	sqsQueueURL string
)

func processOrder(msg types.Message, wg *sync.WaitGroup) {
	defer wg.Done()

	// Unwrap SNS envelope
	var envelope SNSEnvelope
	if err := json.Unmarshal([]byte(*msg.Body), &envelope); err != nil {
		log.Printf("Failed to unmarshal SNS envelope: %v", err)
		return
	}
	var order Order
	if err := json.Unmarshal([]byte(envelope.Message), &order); err != nil {
		log.Printf("Failed to unmarshal order: %v", err)
		return
	}

	log.Printf("Processing order %s for customer %d", order.OrderID, order.CustomerID)

	// Simulate payment verification — same 3s delay
	time.Sleep(3 * time.Second)

	order.Status = "completed"
	log.Printf("Completed order %s", order.OrderID)

	// Delete message from SQS after successful processing
	_, err := sqsClient.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(sqsQueueURL),
		ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		log.Printf("Failed to delete message %s: %v", order.OrderID, err)
	}
}

func poll(numWorkers int) {
	sem := make(chan struct{}, numWorkers) // semaphore caps concurrent goroutines
	var wg sync.WaitGroup

	log.Printf("Starting polling with %d workers", numWorkers)
	for {
		result, err := sqsClient.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(sqsQueueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20, // long polling
		})
		if err != nil {
			log.Printf("SQS receive error: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		for _, msg := range result.Messages {
			sem <- struct{}{} // acquire worker slot
			wg.Add(1)
			go func(m types.Message) {
				defer func() { <-sem }() // release on completion
				processOrder(m, &wg)
			}(msg)
		}
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func main() {
	sqsQueueURL = os.Getenv("SQS_QUEUE_URL")
	if sqsQueueURL == "" {
		log.Fatal("SQS_QUEUE_URL environment variable required")
	}

	numWorkers := 1
	if w := os.Getenv("NUM_WORKERS"); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			numWorkers = n
		}
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("AWS config error: %v", err)
	}
	sqsClient = sqs.NewFromConfig(cfg)

	// Health check server (ECS requires this)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", healthHandler)
		log.Println("Health server on :8081")
		log.Fatal(http.ListenAndServe(":8081", mux))
	}()

	poll(numWorkers)
}
