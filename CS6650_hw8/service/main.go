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

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	_ "github.com/go-sql-driver/mysql"
)

// ── Models ───────────────────────────────────────────────────────────────────

type CartItem struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type Cart struct {
	CartID     string     `json:"cart_id"`
	CustomerID string     `json:"customer_id"`
	Items      []CartItem `json:"items"`
	CreatedAt  string     `json:"created_at"`
}

// ── Backend interface ─────────────────────────────────────────────────────────

type Store interface {
	CreateCart(customerID string) (*Cart, error)
	GetCart(cartID string) (*Cart, error)
	AddItems(cartID string, items []CartItem) (*Cart, error)
}

// ── MySQL backend ─────────────────────────────────────────────────────────────

type MySQLStore struct{ db *sql.DB }

func newMySQL() *MySQLStore {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		os.Getenv("MYSQL_USER"), os.Getenv("MYSQL_PASS"),
		os.Getenv("MYSQL_HOST"), os.Getenv("MYSQL_PORT"),
		os.Getenv("MYSQL_DB"),
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("mysql open: %v", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	// retry until RDS is ready
	for i := 0; i < 10; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		log.Printf("waiting for mysql (%d/10): %v", i+1, err)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		log.Fatalf("mysql ping failed: %v", err)
	}
	migrate(db)
	return &MySQLStore{db: db}
}

func migrate(db *sql.DB) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS carts (
		cart_id     VARCHAR(36)  PRIMARY KEY,
		customer_id VARCHAR(36)  NOT NULL,
		created_at  DATETIME     NOT NULL,
		INDEX idx_customer (customer_id)
	)`)
	if err != nil {
		log.Fatalf("migrate carts: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS cart_items (
		id          BIGINT AUTO_INCREMENT PRIMARY KEY,
		cart_id     VARCHAR(36)  NOT NULL,
		product_id  VARCHAR(36)  NOT NULL,
		name        VARCHAR(255) NOT NULL,
		quantity    INT          NOT NULL DEFAULT 1,
		price       DECIMAL(10,2) NOT NULL,
		FOREIGN KEY (cart_id) REFERENCES carts(cart_id) ON DELETE CASCADE,
		INDEX idx_cart (cart_id)
	)`)
	if err != nil {
		log.Fatalf("migrate cart_items: %v", err)
	}
}

func (s *MySQLStore) CreateCart(customerID string) (*Cart, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO carts (cart_id, customer_id, created_at) VALUES (?,?,?)`,
		id, customerID, now)
	if err != nil {
		return nil, err
	}
	return &Cart{CartID: id, CustomerID: customerID, Items: []CartItem{}, CreatedAt: now.Format(time.RFC3339)}, nil
}

func (s *MySQLStore) GetCart(cartID string) (*Cart, error) {
	row := s.db.QueryRow(`SELECT cart_id, customer_id, created_at FROM carts WHERE cart_id=?`, cartID)
	c := &Cart{}
	var createdAt time.Time
	if err := row.Scan(&c.CartID, &c.CustomerID, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)

	rows, err := s.db.Query(`SELECT product_id, name, quantity, price FROM cart_items WHERE cart_id=?`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	c.Items = []CartItem{}
	for rows.Next() {
		var it CartItem
		if err := rows.Scan(&it.ProductID, &it.Name, &it.Quantity, &it.Price); err != nil {
			return nil, err
		}
		c.Items = append(c.Items, it)
	}
	return c, nil
}

func (s *MySQLStore) AddItems(cartID string, items []CartItem) (*Cart, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// verify cart exists
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM carts WHERE cart_id=?`, cartID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, nil
	}

	for _, it := range items {
		_, err := tx.Exec(`
			INSERT INTO cart_items (cart_id, product_id, name, quantity, price)
			VALUES (?,?,?,?,?)
			ON DUPLICATE KEY UPDATE quantity=quantity+VALUES(quantity)`,
			cartID, it.ProductID, it.Name, it.Quantity, it.Price)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetCart(cartID)
}

// ── DynamoDB backend ──────────────────────────────────────────────────────────

type DynamoStore struct {
	client *dynamodb.Client
	table  string
}

func newDynamo() *DynamoStore {
	cfg, err := awscfg.LoadDefaultConfig(context.Background(), awscfg.WithRegion(os.Getenv("AWS_REGION")))
	if err != nil {
		log.Fatalf("dynamo config: %v", err)
	}
	return &DynamoStore{client: dynamodb.NewFromConfig(cfg), table: os.Getenv("DYNAMODB_TABLE")}
}

func (s *DynamoStore) CreateCart(customerID string) (*Cart, error) {
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	itemsJSON, _ := json.Marshal([]CartItem{})

	_, err := s.client.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item: map[string]types.AttributeValue{
			"cart_id":     &types.AttributeValueMemberS{Value: id},
			"customer_id": &types.AttributeValueMemberS{Value: customerID},
			"items":       &types.AttributeValueMemberS{Value: string(itemsJSON)},
			"created_at":  &types.AttributeValueMemberS{Value: now},
		},
	})
	if err != nil {
		return nil, err
	}
	return &Cart{CartID: id, CustomerID: customerID, Items: []CartItem{}, CreatedAt: now}, nil
}

func (s *DynamoStore) GetCart(cartID string) (*Cart, error) {
	out, err := s.client.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key:       map[string]types.AttributeValue{"cart_id": &types.AttributeValueMemberS{Value: cartID}},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}
	c := &Cart{
		CartID:     out.Item["cart_id"].(*types.AttributeValueMemberS).Value,
		CustomerID: out.Item["customer_id"].(*types.AttributeValueMemberS).Value,
		CreatedAt:  out.Item["created_at"].(*types.AttributeValueMemberS).Value,
	}
	_ = json.Unmarshal([]byte(out.Item["items"].(*types.AttributeValueMemberS).Value), &c.Items)
	return c, nil
}

func (s *DynamoStore) AddItems(cartID string, newItems []CartItem) (*Cart, error) {
	cart, err := s.GetCart(cartID)
	if err != nil || cart == nil {
		return nil, err
	}
	// merge by product_id
	idx := map[string]int{}
	for i, it := range cart.Items {
		idx[it.ProductID] = i
	}
	for _, ni := range newItems {
		if i, ok := idx[ni.ProductID]; ok {
			cart.Items[i].Quantity += ni.Quantity
		} else {
			cart.Items = append(cart.Items, ni)
		}
	}
	itemsJSON, _ := json.Marshal(cart.Items)
	_, err = s.client.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
		TableName: aws.String(s.table),
		Key:       map[string]types.AttributeValue{"cart_id": &types.AttributeValueMemberS{Value: cartID}},
		UpdateExpression: aws.String("SET #it = :items"),
		ExpressionAttributeNames:  map[string]string{"#it": "items"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":items": &types.AttributeValueMemberS{Value: string(itemsJSON)}},
	})
	return cart, err
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

type Server struct{ store Store }

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/shopping-carts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.createCart(w, r)
		} else {
			http.Error(w, "method not allowed", 405)
		}
	})
	mux.HandleFunc("/shopping-carts/", func(w http.ResponseWriter, r *http.Request) {
		// /shopping-carts/{id} or /shopping-carts/{id}/items
		path := r.URL.Path[len("/shopping-carts/"):]
		if len(path) == 0 {
			http.NotFound(w, r)
			return
		}
		if len(path) > 6 && path[len(path)-6:] == "/items" {
			cartID := path[:len(path)-6]
			if r.Method == http.MethodPost {
				s.addItems(w, r, cartID)
			} else {
				http.Error(w, "method not allowed", 405)
			}
		} else {
			if r.Method == http.MethodGet {
				s.getCart(w, r, path)
			} else {
				http.Error(w, "method not allowed", 405)
			}
		}
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *Server) createCart(w http.ResponseWriter, r *http.Request) {
	var body struct{ CustomerID string `json:"customer_id"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CustomerID == "" {
		http.Error(w, "customer_id required", 400)
		return
	}
	cart, err := s.store.CreateCart(body.CustomerID)
	if err != nil {
		log.Printf("createCart: %v", err)
		http.Error(w, "internal error", 500)
		return
	}
	writeJSON(w, 201, cart)
}

func (s *Server) getCart(w http.ResponseWriter, r *http.Request, cartID string) {
	cart, err := s.store.GetCart(cartID)
	if err != nil {
		log.Printf("getCart: %v", err)
		http.Error(w, "internal error", 500)
		return
	}
	if cart == nil {
		http.Error(w, "not found", 404)
		return
	}
	writeJSON(w, 200, cart)
}

func (s *Server) addItems(w http.ResponseWriter, r *http.Request, cartID string) {
	var body struct{ Items []CartItem `json:"items"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Items) == 0 {
		http.Error(w, "items required", 400)
		return
	}
	cart, err := s.store.AddItems(cartID, body.Items)
	if err != nil {
		log.Printf("addItems: %v", err)
		http.Error(w, "internal error", 500)
		return
	}
	if cart == nil {
		http.Error(w, "cart not found", 404)
		return
	}
	writeJSON(w, 200, cart)
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	var store Store
	backend := os.Getenv("DB_BACKEND")
	switch backend {
	case "dynamodb":
		log.Println("backend: dynamodb")
		store = newDynamo()
	default:
		log.Println("backend: mysql")
		store = newMySQL()
	}

	srv := &Server{store: store}
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", srv.routes()))
}