# Multi-stage build for Inventory Service
# Stage 1: Build
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download 2>/dev/null || true

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o inventory-service .

# Stage 2: Runtime
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/inventory-service .

# Expose gRPC port
EXPOSE 50051

# Run the service
CMD ["./inventory-service"]
