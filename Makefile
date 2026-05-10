.PHONY: help build up down logs test clean

help:
	@echo "E-commerce Microservices - Available Commands"
	@echo ""
	@echo "  make build          Build Docker images"
	@echo "  make up             Start all services"
	@echo "  make down           Stop all services"
	@echo "  make restart        Restart all services"
	@echo "  make logs           View logs from all services"
	@echo "  make logs-order     View order service logs"
	@echo "  make logs-inventory View inventory service logs"
	@echo "  make clean          Remove containers and volumes"
	@echo "  make dev            Start services for local development"
	@echo "  make test           Run tests"
	@echo "  make health         Check service health"

build:
	docker-compose build

up:
	docker-compose up -d

down:
	docker-compose down

restart: down up

logs:
	docker-compose logs -f

logs-order:
	docker-compose logs -f order-service

logs-inventory:
	docker-compose logs -f inventory-service

logs-rabbitmq:
	docker-compose logs -f rabbitmq

clean:
	docker-compose down -v
	find . -name "*.o" -delete
	find . -name "*.a" -delete

dev: build up logs

test:
	@echo "Testing Order Service..."
	@curl -s http://localhost:8080/health | jq .
	@echo ""
	@echo "Creating sample order..."
	@curl -s -X POST http://localhost:8080/orders \
		-H "Content-Type: application/json" \
		-d '{"product_id":"PROD001","quantity":5,"customer_id":"CUST001"}' | jq .

health:
	@echo "Order Service Health:"
	@curl -s http://localhost:8080/health || echo "FAILED"
	@echo ""
	@echo "RabbitMQ Management UI: http://localhost:15672 (guest/guest)"
	@echo "Inventory Service gRPC: localhost:50051"

proto-gen:
	@echo "Generating protocol buffer code..."
	protoc --go_out=. --go-grpc_out=. proto/ecommerce.proto
	cp proto/ecommerce*.pb.go order-service/pb/
	cp proto/ecommerce*.pb.go inventory-service/pb/

local-deps:
	@echo "Installing local Go dependencies..."
	cd order-service && go mod download && go mod tidy
	cd inventory-service && go mod download && go mod tidy

local-run-inventory:
	cd inventory-service && RABBITMQ_URI=amqp://guest:guest@localhost:5672/ go run .

local-run-order:
	cd order-service && INVENTORY_SERVICE_HOST=localhost INVENTORY_SERVICE_PORT=50051 RABBITMQ_URI=amqp://guest:guest@localhost:5672/ go run .

.DEFAULT_GOAL := help
