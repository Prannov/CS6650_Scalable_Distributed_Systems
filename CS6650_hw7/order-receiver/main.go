package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
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

var (
	snsClient *sns.Client
	snsTopicARN string
	// Buffered channel acts as a semaphore to simulate 1 order/3s throughput
	// This limits concurrent payment verifications to 1 slot
	paymentSlots = make(chan struct{}, 1)
)

func init() {
	// Pre-fill one slot so the first request can take it
	paymentSlots <- struct{}{}
}

func simulatePaymentVerification() {
	// Block until a slot is available (true throughput bottleneck)
	slot := <-paymentSlots
	time.Sleep(3 * time.Second)
	paymentSlots <- slot // return slot
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func syncOrderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	order.OrderID = generateID()
	order.CreatedAt = time.Now()
	order.Status = "processing"

	// Blocks the HTTP handler goroutine — customers wait 3s
	simulatePaymentVerification()

	order.Status = "completed"
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(order)
}

func asyncOrderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	order.OrderID = generateID()
	order.CreatedAt = time.Now()
	order.Status = "pending"

	payload, err := json.Marshal(order)
	if err != nil {
		http.Error(w, "Failed to marshal order", http.StatusInternalServerError)
		return
	}

	_, err = snsClient.Publish(context.Background(), &sns.PublishInput{
		TopicArn: aws.String(snsTopicARN),
		Message:  aws.String(string(payload)),
	})
	if err != nil {
		log.Printf("SNS publish error: %v", err)
		http.Error(w, "Failed to queue order", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202
	json.NewEncoder(w).Encode(map[string]string{
		"order_id": order.OrderID,
		"status":   "pending",
		"message":  "Order accepted for processing",
	})
}

func generateID() string {
	return time.Now().Format("20060102150405.000000000")
}

func main() {
	snsTopicARN = os.Getenv("SNS_TOPIC_ARN")
	if snsTopicARN == "" {
		log.Fatal("SNS_TOPIC_ARN environment variable required")
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("AWS config error: %v", err)
	}
	snsClient = sns.NewFromConfig(cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/orders/sync", syncOrderHandler)
	mux.HandleFunc("/orders/async", asyncOrderHandler)

	log.Println("Order Receiver listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}