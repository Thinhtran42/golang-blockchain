package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"golang-blockchain/pkg/docker"
	"golang-blockchain/pkg/registry"
)

func main() {
	// Command line flags
	var (
		port        = flag.String("port", "9090", "Registry service port")
		minNodes    = flag.Int("min-nodes", 2, "Minimum number of nodes for consensus")
		maxNodes    = flag.Int("max-nodes", 10, "Maximum number of nodes")
		dockerNet   = flag.String("docker-network", "golang-blockchain_blockchain_network", "Docker network name")
		dockerImage = flag.String("docker-image", "golang-blockchain-node", "Docker image name")
		portStart   = flag.Int("port-start", 50054, "Starting port for new nodes")
	)
	flag.Parse()

	// Override with environment variables if present
	if envPort := os.Getenv("PORT"); envPort != "" {
		*port = envPort
	}
	if envMinNodes := os.Getenv("MIN_NODES"); envMinNodes != "" {
		if val, err := strconv.Atoi(envMinNodes); err == nil {
			*minNodes = val
		}
	}
	if envMaxNodes := os.Getenv("MAX_NODES"); envMaxNodes != "" {
		if val, err := strconv.Atoi(envMaxNodes); err == nil {
			*maxNodes = val
		}
	}
	if envDockerNet := os.Getenv("DOCKER_NETWORK"); envDockerNet != "" {
		*dockerNet = envDockerNet
	}
	if envDockerImage := os.Getenv("DOCKER_IMAGE"); envDockerImage != "" {
		*dockerImage = envDockerImage
	}
	if envPortStart := os.Getenv("PORT_START"); envPortStart != "" {
		if val, err := strconv.Atoi(envPortStart); err == nil {
			*portStart = val
		}
	}

	log.Printf("🚀 Starting Auto-Scaling Registry Service")
	log.Printf("   Port: %s", *port)
	log.Printf("   Min nodes: %d", *minNodes)
	log.Printf("   Max nodes: %d", *maxNodes)
	log.Printf("   Docker network: %s", *dockerNet)
	log.Printf("   Docker image: %s", *dockerImage)

	// Create Docker manager
	registryAddr := "registry:" + *port
	dockerManager := docker.NewManager(*dockerNet, *dockerImage, *portStart, registryAddr)

	// Create registry
	reg := registry.NewRegistry(*minNodes, *maxNodes, dockerManager)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Docker health monitoring
	go dockerManager.MonitorHealth(ctx)

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Printf("Received shutdown signal, stopping registry...")
		reg.Stop()
		cancel()
	}()

	// Start registry service
	log.Printf("Registry service running on :%s", *port)
	if err := reg.Start(*port); err != nil {
		log.Fatalf("Failed to start registry: %v", err)
	}
} 