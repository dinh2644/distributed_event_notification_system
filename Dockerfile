# Dockerfile

# --- Builder Stage ---
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build each service binary
RUN mkdir -p build
RUN go build -o build/orderservice ./cmd/orderservice
RUN go build -o build/inventoryservice ./cmd/inventoryservice
RUN go build -o build/paymentservice ./cmd/paymentservice
RUN go build -o build/shippingservice ./cmd/shippingservice
RUN go build -o build/notificationservice ./cmd/notificationservice


# --- Runtime Stages ---

# --- Order Service ---
FROM alpine:latest AS orderservice
WORKDIR /app
COPY --from=builder /app/build/orderservice /app/
EXPOSE 8080
CMD ["./orderservice"]

# --- Inventory Service ---
FROM alpine:latest AS inventoryservice
WORKDIR /app
COPY --from=builder /app/build/inventoryservice /app/
EXPOSE 8081
CMD ["./inventoryservice"]

# --- Payment Service ---
FROM alpine:latest AS paymentservice
WORKDIR /app
COPY --from=builder /app/build/paymentservice /app/
EXPOSE 8082
CMD ["./paymentservice"]

# --- Shipping Service ---
FROM alpine:latest AS shippingservice
WORKDIR /app
COPY --from=builder /app/build/shippingservice /app/
EXPOSE 8083
CMD ["./shippingservice"]

# --- Notification Service ---
FROM alpine:latest AS notificationservice
WORKDIR /app
COPY --from=builder /app/build/notificationservice /app/
EXPOSE 8084
CMD ["./notificationservice"]


# Note: The final images are picked by target in docker-compose.yml
