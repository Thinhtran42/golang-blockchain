#!/bin/bash

# Auto-Scaling Blockchain Startup Script
set -e

echo "🚀 Starting Auto-Scaling Blockchain System"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker is running
if ! docker info >/dev/null 2>&1; then
    print_error "Docker is not running. Please start Docker first."
    exit 1
fi

# Check if docker-compose is available
if ! command -v docker-compose >/dev/null 2>&1; then
    print_error "docker-compose is not installed. Please install docker-compose first."
    exit 1
fi

# Clean up any existing containers
print_status "Cleaning up existing containers..."
docker-compose -f docker-compose.registry.yml down --remove-orphans >/dev/null 2>&1 || true

# Build the images
print_status "Building Docker images..."
docker-compose -f docker-compose.registry.yml build --no-cache

# Start the registry service first
print_status "Starting Registry Service..."
docker-compose -f docker-compose.registry.yml up -d registry

# Wait for registry to be ready
echo -n "Waiting for registry to be ready"
for i in {1..30}; do
    if curl -s http://localhost:9090/status >/dev/null 2>&1; then
        echo -e "\n${GREEN}✓${NC} Registry is ready!"
        break
    fi
    echo -n "."
    sleep 2
done

if [ $i -eq 30 ]; then
    print_error "Registry failed to start within 60 seconds"
    exit 1
fi

# Start the initial nodes
print_status "Starting initial blockchain nodes..."
docker-compose -f docker-compose.registry.yml up -d node1 node2 node3

# Wait for nodes to be ready
echo -n "Waiting for nodes to be ready"
for i in {1..60}; do
    if [ $(docker-compose -f docker-compose.registry.yml ps -q node1 node2 node3 | wc -l) -eq 3 ]; then
        # Check if nodes are actually running
        running_nodes=$(docker-compose -f docker-compose.registry.yml ps | grep -c "Up" || true)
        if [ $running_nodes -ge 3 ]; then
            echo -e "\n${GREEN}✓${NC} All nodes are ready!"
            break
        fi
    fi
    echo -n "."
    sleep 2
done

if [ $i -eq 60 ]; then
    print_error "Nodes failed to start within 120 seconds"
    exit 1
fi

# Display system status
print_status "System Status:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Show registry status
echo -e "${BLUE}Registry Service:${NC}"
curl -s http://localhost:9090/status | jq '.' 2>/dev/null || echo "Registry status API not available"

echo
echo -e "${BLUE}Running Containers:${NC}"
docker-compose -f docker-compose.registry.yml ps

echo
echo -e "${BLUE}System URLs:${NC}"
echo "  Registry API:     http://localhost:9090"
echo "  Registry Status:  http://localhost:9090/status"
echo "  Node1 (Leader):   http://localhost:8081"
echo "  Node2:            http://localhost:8082"
echo "  Node3:            http://localhost:8083"

echo
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
print_status "Auto-Scaling Blockchain System is now running!"

echo
echo "🔥 Auto-scaling features:"
echo "  • Registry monitors node health every 5 seconds"
echo "  • Auto-scaling evaluation every 10 seconds"
echo "  • Minimum nodes: 2 (for consensus)"
echo "  • Maximum nodes: 10"
echo "  • New nodes will auto-discover peers"
echo "  • Failed nodes will be automatically replaced"

echo
echo "📊 Monitoring commands:"
echo "  Registry status:   curl -s http://localhost:9090/status | jq"
echo "  Node discovery:    curl -s http://localhost:9090/discover | jq"
echo "  System logs:       docker-compose -f docker-compose.registry.yml logs -f"

echo
echo "🧪 Testing auto-scaling:"
echo "  Stop a node:       docker-compose -f docker-compose.registry.yml stop node2"
echo "  Watch auto-scale:  docker-compose -f docker-compose.registry.yml logs -f registry"

echo
print_status "Press Ctrl+C to stop the system"

# Follow logs
trap 'docker-compose -f docker-compose.registry.yml down' EXIT
docker-compose -f docker-compose.registry.yml logs -f