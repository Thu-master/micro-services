# E-commerce Microservices Project

A complete Go microservices architecture demonstrating gRPC, RabbitMQ messaging, and REST APIs.

## Architecture Overview

This project implements a distributed e-commerce system with the following components:

### Services

1. **Order Service** (REST API)
   - Handles order creation and management
   - Communicates with Inventory Service via gRPC
   - Publishes events to RabbitMQ
   - Listens for inventory updates

2. **Inventory Service** (gRPC)
   - Manages product stock
   - Provides stock check via gRPC unary call
   - Streams order status updates via gRPC server streaming
   - Consumes order events from RabbitMQ
   - Publishes inventory updates

3. **RabbitMQ**
   - Message broker for asynchronous communication
   - Event topics: `order.created`, `inventory.updated`
   - Management UI on port 15672

## Directory Structure

```
.
├── proto/
│   └── ecommerce.proto           # Protocol Buffer definitions
├── order-service/
│   ├── main.go                   # REST API server
│   ├── repository.go             # Mock database for orders
│   ├── rabbitmq.go               # RabbitMQ client helper
│   ├── pb/
│   │   ├── ecommerce.pb.go       # Generated protobuf code
│   │   └── ecommerce_grpc.pb.go  # Generated gRPC code
│   ├── go.mod
│   └── Dockerfile
├── inventory-service/
│   ├── main.go                   # gRPC server
│   ├── repository.go             # Mock database for inventory
│   ├── rabbitmq.go               # RabbitMQ client helper
│   ├── pb/
│   │   ├── ecommerce.pb.go       # Generated protobuf code
│   │   └── ecommerce_grpc.pb.go  # Generated gRPC code
│   ├── go.mod
│   └── Dockerfile
├── docker-compose.yml            # Docker Compose orchestration
├── test.http                     # REST Client test file
└── README.md
```

## Getting Started

### Prerequisites

- Docker & Docker Compose
- Go 1.22+ (optional, for local development)
- Protocol Buffer compiler (optional, for regenerating proto files)
- grpcurl (optional, for testing gRPC endpoints)

### Quick Start with Docker

1. **Clone the repository**
   ```bash
   cd g:\micro-service
   ```

2. **Build and start services**
   ```bash
   docker-compose up --build
   ```

3. **Verify services are running**
   - Order Service: `http://localhost:8080/health`
   - Inventory Service gRPC: `localhost:50051`
   - RabbitMQ Management: `http://localhost:15672` (guest/guest)

### Development Setup (Local)

1. **Install dependencies**
   ```bash
   cd order-service
   go mod download
   cd ../inventory-service
   go mod download
   ```

2. **Start RabbitMQ** (using Docker)
   ```bash
   docker run -d --name rabbitmq -p 5672:5672 -p 15672:15672 \
     -e RABBITMQ_DEFAULT_USER=guest \
     -e RABBITMQ_DEFAULT_PASS=guest \
     rabbitmq:3-management
   ```

3. **Run Inventory Service**
   ```bash
   cd inventory-service
   go run . &
   ```

4. **Run Order Service** (in another terminal)
   ```bash
   cd order-service
   go run .
   ```

## API Documentation

### REST Endpoints

#### Health Check
```http
GET /health
```

#### Create Order
```http
POST /orders
Content-Type: application/json

{
  "product_id": "PROD001",
  "quantity": 5,
  "customer_id": "CUST001"
}
```

**Response (201 Created):**
```json
{
  "id": "ORD-1234567890",
  "status": "PENDING",
  "message": "Order created successfully"
}
```

#### Get Order
```http
GET /orders/ORD-1234567890
```

### gRPC Endpoints

#### CheckStock (Unary)
```protobuf
service InventoryService {
  rpc CheckStock(CheckStockRequest) returns (CheckStockResponse);
}

message CheckStockRequest {
  string product_id = 1;
  int32 quantity = 2;
}

message CheckStockResponse {
  bool is_available = 1;
}
```

#### TrackOrder (Server Streaming)
```protobuf
rpc TrackOrder(TrackOrderRequest) returns (stream OrderStatusUpdate);

message TrackOrderRequest {
  string order_id = 1;
}

message OrderStatusUpdate {
  string status = 1;      // "PACKING", "SHIPPED", "DELIVERED"
  string timestamp = 2;
}
```

## Testing

### Using REST Client Extension (VS Code)
1. Install "REST Client" extension
2. Open `test.http`
3. Click "Send Request" on any request

### Using cURL
```bash
# Create order
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"product_id":"PROD001","quantity":5,"customer_id":"CUST001"}'

# Get order
curl http://localhost:8080/orders/ORD-1234567890

# Health check
curl http://localhost:8080/health
```

### Using grpcurl
```bash
# Check stock
grpcurl -plaintext -d '{"product_id":"PROD001","quantity":5}' \
  localhost:50051 ecommerce.InventoryService/CheckStock

# Track order (streaming)
grpcurl -plaintext -d '{"order_id":"ORD-1234567890"}' \
  localhost:50051 ecommerce.InventoryService/TrackOrder
```

## Message Flow

### Order Creation Flow
1. Client sends POST request to Order Service with order details
2. Order Service calls Inventory Service (gRPC) to check stock
3. If stock available:
   - Order Service saves order with status "PENDING"
   - Order Service publishes `order.created` event to RabbitMQ
   - Order Service returns 201 with Order ID
4. Inventory Service consumes `order.created` event:
   - Deducts stock from inventory
   - Publishes `inventory.updated` event
5. Order Service consumes `inventory.updated` event:
   - Updates order status to "PROCESSING"

### Order Tracking Flow
1. Client sends gRPC TrackOrder request with Order ID
2. Inventory Service streams 3-4 status updates (PACKING, SHIPPED, DELIVERED)
3. Updates are sent every 3 seconds for realistic simulation

## Environment Variables

### Order Service
- `RABBITMQ_URI` - RabbitMQ connection string (default: `amqp://guest:guest@rabbitmq:5672/`)
- `INVENTORY_SERVICE_HOST` - Inventory Service hostname (default: `inventory-service`)
- `INVENTORY_SERVICE_PORT` - Inventory Service gRPC port (default: `50051`)

### Inventory Service
- `RABBITMQ_URI` - RabbitMQ connection string (default: `amqp://guest:guest@rabbitmq:5672/`)

## Regenerating Protocol Buffers

If you modify `proto/ecommerce.proto`:

```bash
# Install protoc compiler and plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate code
protoc --go_out=. --go-grpc_out=. proto/ecommerce.proto

# Copy generated files to both services
cp proto/*.pb.go order-service/pb/
cp proto/*.pb.go inventory-service/pb/
```

## Docker Commands

```bash
# Build and start all services
docker-compose up --build

# Start services in background
docker-compose up -d --build

# Stop services
docker-compose down

# View logs
docker-compose logs -f order-service
docker-compose logs -f inventory-service
docker-compose logs -f rabbitmq

# Stop and remove all containers, volumes
docker-compose down -v
```

## Troubleshooting

### Services can't connect to RabbitMQ
- Ensure RabbitMQ service is running: `docker-compose logs rabbitmq`
- Check RABBITMQ_URI environment variables
- Wait for RabbitMQ to be fully started (check health check)

### gRPC connection refused
- Ensure Inventory Service is running
- Check INVENTORY_SERVICE_HOST and INVENTORY_SERVICE_PORT
- Verify service is listening on port 50051

### No stock available error
- Initial stock: PROD001 (100), PROD002 (50), PROD003 (200)
- Create orders with lower quantities
- Check inventory in RabbitMQ Management UI

## Project Features

✅ Multi-stage Docker builds with Alpine images  
✅ gRPC unary and streaming RPC patterns  
✅ RabbitMQ asynchronous event messaging  
✅ REST API with JSON  
✅ Mock in-memory databases  
✅ Goroutine-based concurrent processing  
✅ Error handling and logging  
✅ Docker Compose orchestration  
✅ Health checks  
✅ Environment variable configuration  

## License

MIT License - Feel free to use and modify

## Resources

- [Protocol Buffers Documentation](https://developers.google.com/protocol-buffers)
- [gRPC Go Documentation](https://grpc.io/docs/languages/go/)
- [RabbitMQ Go Client](https://github.com/rabbitmq/amqp091-go)
- [Docker Documentation](https://docs.docker.com/)
