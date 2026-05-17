package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"time"

	"inventory-service/pb"

	"google.golang.org/grpc"
)

const (
	grpcPort = ":50051"
)

var (
	inventoryRepo *InventoryRepository
	rabbitClient  *RabbitMQClient
)

// connectRabbitMQWithRetry attempts to connect to RabbitMQ with retry logic
func connectRabbitMQWithRetry(uri, exchange, queue string, maxRetries int) (*RabbitMQClient, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		client, err := NewRabbitMQClient(uri, exchange, queue)
		if err == nil {
			log.Printf("Successfully connected to RabbitMQ on attempt %d", i+1)
			return client, nil
		}
		lastErr = err
		log.Printf("RabbitMQ connection attempt %d/%d failed: %v. Retrying in 2 seconds...", i+1, maxRetries, err)
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

// InventoryServiceImpl implements the InventoryService
type InventoryServiceImpl struct {
	pb.UnimplementedInventoryServiceServer
}

func (s *InventoryServiceImpl) CheckStock(ctx context.Context, req *pb.CheckStockRequest) (*pb.CheckStockResponse, error) {
	log.Printf("CheckStock called: ProductID=%s, Quantity=%d", req.ProductId, req.Quantity)

	isAvailable := inventoryRepo.CheckStock(req.ProductId, req.Quantity)
	return &pb.CheckStockResponse{IsAvailable: isAvailable}, nil
}

func (s *InventoryServiceImpl) TrackOrder(req *pb.TrackOrderRequest, stream pb.InventoryService_TrackOrderServer) error {
	log.Printf("TrackOrder called: OrderID=%s", req.OrderId)

	statuses := []string{"PACKING", "SHIPPED", "DELIVERED"}
	for i, status := range statuses {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		default:
		}

		update := &pb.OrderStatusUpdate{
			Status:    status,
			Timestamp: time.Now().Format(time.RFC3339),
		}

		if err := stream.Send(update); err != nil {
			log.Printf("Failed to send update: %v", err)
			return err
		}

		// Store tracking status
		inventoryRepo.AddTrackingStatus(req.OrderId, status)

		// Wait between updates (except after the last one)
		if i < len(statuses)-1 {
			time.Sleep(3 * time.Second)
		}
	}

	return nil
}

func main() {
	// Initialize repositories
	inventoryRepo = NewInventoryRepository()
	inventoryRepo.Initialize()

	// Get environment variables with defaults
	rabbitmqURI := os.Getenv("RABBITMQ_URI")
	if rabbitmqURI == "" {
		rabbitmqURI = "amqp://guest:guest@rabbitmq:5672/"
	}

	// Connect to RabbitMQ with retry logic
	var err error
	rabbitClient, err = connectRabbitMQWithRetry(rabbitmqURI, "ecommerce", "inventory-queue", 10)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ after retries: %v", err)
	}
	defer rabbitClient.Close()

	// Bind to order.created events
	err = rabbitClient.BindQueue("order.created")
	if err != nil {
		log.Fatalf("Failed to bind queue: %v", err)
	}

	// Start RabbitMQ consumer in a goroutine
	go startOrderCreatedConsumer()

	// Start gRPC server
	listener, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", grpcPort, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterInventoryServiceServer(grpcServer, &InventoryServiceImpl{})

	log.Printf("Inventory Service gRPC server listening on %s\n", grpcPort)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve gRPC: %v", err)
	}
}

func startOrderCreatedConsumer() {
	ctx := context.Background()
	err := rabbitClient.Consume(ctx, func(body []byte) error {
		var event map[string]interface{}
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}

		log.Printf("Received order.created event: %v", event)

		productID := ""
		quantity := int32(0)
		orderID := ""

		if pid, ok := event["product_id"].(string); ok {
			productID = pid
		}
		if qty, ok := event["quantity"].(float64); ok {
			quantity = int32(qty)
		}
		if oid, ok := event["order_id"].(string); ok {
			orderID = oid
		}

		// Simulate processing time
		time.Sleep(5 * time.Second)

		// Deduct stock
		if err := inventoryRepo.DeductStock(productID, quantity); err != nil {
			log.Printf("Failed to deduct stock: %v", err)
			return err
		}

		log.Printf("Stock deducted for product %s: %d", productID, quantity)

		// Publish inventory.updated event back to RabbitMQ
		ctxPub, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		updateEvent := map[string]interface{}{
			"order_id":   orderID,
			"product_id": productID,
			"status":     "PROCESSING",
			"timestamp":  time.Now().Format(time.RFC3339),
		}

		if err := rabbitClient.Publish(ctxPub, "inventory.updated", updateEvent); err != nil {
			log.Printf("Failed to publish inventory.updated event: %v", err)
			return err
		}

		return nil
	})

	if err != nil {
		log.Printf("Consumer error: %v", err)
	}
}
