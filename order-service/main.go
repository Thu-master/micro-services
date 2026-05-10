package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"order-service/pb"
)

const orderServicePort = ":8080"

type OrderRequest struct {
	ProductID  string `json:"product_id"`
	Quantity   int32  `json:"quantity"`
	CustomerID string `json:"customer_id"`
}

type OrderResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

var (
	orderRepo    *OrderRepository
	rabbitClient *RabbitMQClient
	grpcClient   pb.InventoryServiceClient
)

func main() {
	// Initialize repositories
	orderRepo = NewOrderRepository()

	// Get environment variables with defaults
	rabbitmqURI := os.Getenv("RABBITMQ_URI")
	if rabbitmqURI == "" {
		rabbitmqURI = "amqp://guest:guest@rabbitmq:5672/"
	}

	inventoryHost := os.Getenv("INVENTORY_SERVICE_HOST")
	if inventoryHost == "" {
		inventoryHost = "inventory-service"
	}
	inventoryPort := os.Getenv("INVENTORY_SERVICE_PORT")
	if inventoryPort == "" {
		inventoryPort = "50051"
	}

	// Connect to gRPC Inventory Service
	grpcAddr := fmt.Sprintf("%s:%s", inventoryHost, inventoryPort)
	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to inventory service: %v", err)
	}
	defer conn.Close()
	grpcClient = pb.NewInventoryServiceClient(conn)

	// Connect to RabbitMQ
	var rabbitErr error
	rabbitClient, rabbitErr = NewRabbitMQClient(rabbitmqURI, "ecommerce", "order-queue")
	if rabbitErr != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", rabbitErr)
	}
	defer rabbitClient.Close()

	// Start async consumer for inventory.updated events
	err = rabbitClient.BindQueue("inventory.updated")
	if err != nil {
		log.Fatalf("Failed to bind queue: %v", err)
	}

	go startInventoryUpdateConsumer()

	// Setup HTTP routes
	http.HandleFunc("/orders", createOrder)
	http.HandleFunc("/orders/", getOrder)
	http.HandleFunc("/health", healthCheck)

	log.Printf("Order Service listening on %s\n", orderServicePort)
	if err := http.ListenAndServe(orderServicePort, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func createOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var orderReq OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&orderReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Call gRPC to check stock
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	checkReq := &pb.CheckStockRequest{
		ProductId: orderReq.ProductID,
		Quantity:  orderReq.Quantity,
	}

	checkResp, err := grpcClient.CheckStock(ctx, checkReq)
	if err != nil {
		log.Printf("gRPC error: %v", err)
		http.Error(w, "Stock check failed", http.StatusInternalServerError)
		return
	}

	if !checkResp.IsAvailable {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(OrderResponse{
			Status:  "FAILED",
			Message: "Product not available",
		})
		return
	}

	// Create order
	orderID := fmt.Sprintf("ORD-%d", time.Now().UnixNano())
	order := &Order{
		ID:         orderID,
		ProductID:  orderReq.ProductID,
		Quantity:   orderReq.Quantity,
		CustomerID: orderReq.CustomerID,
		Status:     "PENDING",
	}

	if err := orderRepo.CreateOrder(order); err != nil {
		http.Error(w, "Failed to create order", http.StatusInternalServerError)
		return
	}

	// Publish order.created event to RabbitMQ
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eventData := map[string]interface{}{
		"order_id":   orderID,
		"product_id": orderReq.ProductID,
		"quantity":   orderReq.Quantity,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	if err := rabbitClient.Publish(ctx, "order.created", eventData); err != nil {
		log.Printf("Failed to publish event: %v", err)
		http.Error(w, "Failed to process order", http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(OrderResponse{
		ID:      orderID,
		Status:  "PENDING",
		Message: "Order created successfully",
	})
}

func getOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orderID := r.URL.Path[len("/orders/"):]
	if orderID == "" {
		http.Error(w, "Order ID required", http.StatusBadRequest)
		return
	}

	order, err := orderRepo.GetOrder(orderID)
	if err != nil || order == nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func startInventoryUpdateConsumer() {
	ctx := context.Background()
	err := rabbitClient.Consume(ctx, func(body []byte) error {
		var event map[string]interface{}
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}

		if orderID, ok := event["order_id"].(string); ok {
			status := "PROCESSING"
			if val, hasStatus := event["status"]; hasStatus {
				status = val.(string)
			}
			orderRepo.UpdateOrderStatus(orderID, status)
			log.Printf("Updated order %s to status %s", orderID, status)
		}

		return nil
	})

	if err != nil {
		log.Printf("Consumer error: %v", err)
	}
}
