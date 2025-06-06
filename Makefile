.PHONY: help build clean test docker-build docker-up docker-down proto-gen cli validator

# Default target
help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Build targets
build: proto-gen validator cli ## Build all binaries

validator: ## Build validator node
	@echo "Building validator node..."
	go build -o bin/validator ./cmd/node

cli: ## Build CLI tool
	@echo "Building CLI tool..."
	go build -o bin/blockchain-cli ./cmd/cli

# Protocol Buffers
proto-gen: ## Generate protobuf files
	@echo "Generating protobuf files..."
	protoc --go_out=. --go-grpc_out=. proto/blockchain.proto

# Testing
test: ## Run tests
	@echo "Running tests..."
	go run cmd/test/test_basic.go

test-integration: docker-up ## Run integration tests
	@echo "Running integration tests..."
	sleep 10  # Wait for containers to start
	@# Add integration test commands here
	$(MAKE) docker-down

# Docker targets
docker-build: ## Build Docker images
	@echo "Building Docker images..."
	docker-compose build

docker-up: ## Start all nodes
	@echo "Starting blockchain network..."
	docker-compose up -d
	@echo "Waiting for nodes to start..."
	sleep 5
	@echo "Blockchain network is ready!"
	@echo "- Node 1 (Leader): localhost:50051"
	@echo "- Node 2: localhost:50052"
	@echo "- Node 3: localhost:50053"

docker-down: ## Stop all nodes
	@echo "Stopping blockchain network..."
	docker-compose down

docker-logs: ## View logs from all nodes
	docker-compose logs -f

docker-restart: docker-down docker-up ## Restart all nodes

# Development targets
dev-setup: ## Setup development environment
	@echo "Setting up development environment..."
	go mod tidy
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	mkdir -p bin data

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf data/
	docker-compose down -v
	docker system prune -f

# Demo targets
demo: docker-up demo-users demo-transactions ## Run complete demo

demo-users: ## Create demo users
	@echo "Creating demo users..."
	sleep 8  # Wait for nodes to be ready
	./bin/blockchain-cli create-user Alice --node localhost:50051
	./bin/blockchain-cli create-user Bob --node localhost:50051
	./bin/blockchain-cli create-user Charlie --node localhost:50051
	@echo "Demo users created: Alice, Bob, Charlie"

demo-transactions: ## Send demo transactions
	@echo "Sending demo transactions..."
	./bin/blockchain-cli send Alice Bob 10.5 --node localhost:50051
	sleep 2
	./bin/blockchain-cli send Bob Charlie 5.0 --node localhost:50051
	sleep 2
	./bin/blockchain-cli send Charlie Alice 3.5 --node localhost:50051
	@echo "Demo transactions sent!"

demo-status: ## Show demo status
	@echo "=== Blockchain Status ==="
	./bin/blockchain-cli status --node localhost:50051
	@echo ""
	@echo "=== Latest Block ==="
	./bin/blockchain-cli get-block 1 --node localhost:50051 2>/dev/null || echo "No blocks yet"

# Utility targets
format: ## Format Go code
	go fmt ./...

lint: ## Run linter
	golangci-lint run

deps: ## Download dependencies
	go mod download
	go mod tidy

# Quick start
quick-start: build docker-build demo ## Quick start demo (build + run + demo)

# Node operations
node1-logs: ## View Node 1 logs
	docker-compose logs -f node1

node2-logs: ## View Node 2 logs
	docker-compose logs -f node2

node3-logs: ## View Node 3 logs
	docker-compose logs -f node3

# Recovery test
test-recovery: ## Test node recovery
	@echo "Testing node recovery..."
	docker-compose up -d
	sleep 10
	@echo "Stopping node2..."
	docker-compose stop node2
	sleep 5
	@echo "Starting transactions while node2 is down..."
	./bin/blockchain-cli send Alice Bob 1.0 --node localhost:50051 || true
	sleep 8
	@echo "Restarting node2..."
	docker-compose start node2
	sleep 10
	@echo "Checking if node2 synced..."
	./bin/blockchain-cli status --node localhost:50052
	@echo "Recovery test completed!"

# Network test
test-network: ## Test network connectivity
	@echo "Testing network connectivity..."
	@echo "Node 1 health:"
	./bin/blockchain-cli status --node localhost:50051 || echo "Node 1 unreachable"
	@echo "Node 2 health:"
	./bin/blockchain-cli status --node localhost:50052 || echo "Node 2 unreachable"
	@echo "Node 3 health:"
	./bin/blockchain-cli status --node localhost:50053 || echo "Node 3 unreachable"

# Benchmark
benchmark: ## Run performance benchmark
	@echo "Running benchmark..."
	@echo "Creating users for benchmark..."
	./bin/blockchain-cli create-user User1 --node localhost:50051
	./bin/blockchain-cli create-user User2 --node localhost:50051
	@echo "Sending multiple transactions..."
	for i in {1..10}; do \
		./bin/blockchain-cli send User1 User2 $i.0 --node localhost:50051; \
		sleep 1; \
	done
	@echo "Benchmark completed!"

# Install targets (for production)
install: build ## Install binaries to system
	sudo cp bin/validator /usr/local/bin/
	sudo cp bin/blockchain-cli /usr/local/bin/
	@echo "Binaries installed to /usr/local/bin/"

uninstall: ## Remove binaries from system
	sudo rm -f /usr/local/bin/validator
	sudo rm -f /usr/local/bin/blockchain-cli
	@echo "Binaries removed from system"