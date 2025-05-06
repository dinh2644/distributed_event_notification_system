
DOCKER_COMPOSE = COMPOSE_BAKE=true docker compose
GO = go # Note that if you use alphine in dockerfile, statically linked libary (CGO_ENABLED will not work)
MODULE_PATH = github.com/dinh2644/distributed_event_notification

# Service names
SERVICES = orderservice inventoryservice paymentservice shippingservice notificationservice

# Default target
all: build

# Build all service binaries
build: $(SERVICES)

orderservice:
	@echo "Building Order Service..."
	$(GO) build -o build/orderservice ./cmd/orderservice

inventoryservice:
	@echo "Building Inventory Service..."
	$(GO) build -o build/inventoryservice ./cmd/inventoryservice

paymentservice:
	@echo "Building Payment Service..."
	$(GO) build -o build/paymentservice ./cmd/paymentservice

shippingservice:
	@echo "Building Shipping Service..."
	$(GO) build -o build/shippingservice ./cmd/shippingservice

notificationservice:
	@echo "Building Notification Service..."
	$(GO) build -o build/notificationservice ./cmd/notificationservice

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf build/

# Docker Compose targets
up:
	@echo "Starting services with Docker Compose..."
	$(DOCKER_COMPOSE) up --build -d --remove-orphans

down:
	@echo "Stopping services with Docker Compose..."
	$(DOCKER_COMPOSE) down

logs:
	@echo "Tailing logs from Docker Compose..."
	$(DOCKER_COMPOSE) logs -f

stop: down

start: up

restart: down all up

# Target to run go mod tidy
tidy:
	@echo "Running go mod tidy..."
	$(GO) mod tidy

.PHONY: all build clean up down logs stop start restart tidy $(SERVICES)
