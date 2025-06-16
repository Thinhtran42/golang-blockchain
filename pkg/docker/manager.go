package docker

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// Manager handles Docker operations for auto-scaling nodes
type Manager struct {
	network      string
	image        string
	portStart    int
	registryAddr string
}

// NewManager creates a new Docker manager
func NewManager(network, image string, portStart int, registryAddr string) *Manager {
	return &Manager{
		network:      network,
		image:        image,
		portStart:    portStart,
		registryAddr: registryAddr,
	}
}

// CreateNode creates a new Docker container for a blockchain node
func (m *Manager) CreateNode(nodeID string) error {
	log.Printf("Creating Docker container for node: %s", nodeID)

	// Check if container already exists
	if exists, err := m.containerExists(nodeID); err != nil {
		return fmt.Errorf("failed to check container existence: %w", err)
	} else if exists {
		log.Printf("Container %s already exists, skipping creation", nodeID)
		return nil
	}

	// Get next available node number if nodeID conflicts
	finalNodeID := m.getAvailableNodeID(nodeID)

	// Calculate ports for the new node
	nodeNumber := m.extractNodeNumber(finalNodeID)
	grpcPort := m.portStart + nodeNumber
	httpPort := 8080 + nodeNumber

	// Check for port conflicts and adjust if needed
	grpcPort, httpPort = m.getAvailablePorts(grpcPort, httpPort)

	// Use the actual network name created by docker-compose
	actualNetwork := m.network
	if actualNetwork == "golang-blockchain_blockchain_network" {
		// Docker-compose prefixes with directory name
		actualNetwork = "golang-blockchain_blockchain_network"
	}

	// Docker run command
	cmd := exec.Command("docker", "run", "-d",
		"--name", finalNodeID,
		"--network", actualNetwork,
		"-e", fmt.Sprintf("NODE_ID=%s", finalNodeID),
		"-e", "IS_LEADER=false", // New nodes are always followers
		"-e", fmt.Sprintf("PORT=%d", 50051), // Internal port
		"-e", fmt.Sprintf("REGISTRY_URL=http://%s", m.registryAddr),
		"-e", "AUTO_DISCOVER=true",
		"-p", fmt.Sprintf("%d:50051", grpcPort),
		"-p", fmt.Sprintf("%d:8080", httpPort),
		"-v", fmt.Sprintf("%s_data:/app/data", finalNodeID),
		"--restart", "unless-stopped",
		m.image,
	)

	log.Printf("Executing: %s", strings.Join(cmd.Args, " "))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create container %s: %v - %s", finalNodeID, err, string(output))
	}

	log.Printf("Container %s created successfully: %s", finalNodeID, strings.TrimSpace(string(output)))

	// Wait a bit for the container to start
	time.Sleep(5 * time.Second)

	// Create volume if it doesn't exist
	m.createVolume(finalNodeID)

	return nil
}

// containerExists checks if a container with given name exists
func (m *Manager) containerExists(name string) (bool, error) {
	cmd := exec.Command("docker", "ps", "-a", "--filter", fmt.Sprintf("name=^%s$", name), "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == name {
			return true, nil
		}
	}
	return false, nil
}

// getAvailableNodeID finds next available node ID
func (m *Manager) getAvailableNodeID(preferredID string) string {
	// If preferred ID is available, use it
	if exists, _ := m.containerExists(preferredID); !exists {
		return preferredID
	}

	// Find next available node number
	for i := 1; i <= 100; i++ { // Reasonable limit
		candidateID := fmt.Sprintf("node%d", i)
		if exists, _ := m.containerExists(candidateID); !exists {
			return candidateID
		}
	}

	// Fallback to timestamp-based naming
	return fmt.Sprintf("node_%d", time.Now().Unix())
}

// getAvailablePorts finds available ports starting from the given ones
func (m *Manager) getAvailablePorts(startGrpc, startHttp int) (int, int) {
	for offset := 0; offset < 100; offset++ {
		grpcPort := startGrpc + offset
		httpPort := startHttp + offset

		if m.isPortAvailable(grpcPort) && m.isPortAvailable(httpPort) {
			return grpcPort, httpPort
		}
	}

	// Return original ports as fallback
	return startGrpc, startHttp
}

// isPortAvailable checks if a port is available
func (m *Manager) isPortAvailable(port int) bool {
	cmd := exec.Command("docker", "ps", "--format", "{{.Ports}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return true // Assume available if can't check
	}

	portStr := fmt.Sprintf(":%d->", port)
	return !strings.Contains(string(output), portStr)
}

// RemoveNode removes a Docker container
func (m *Manager) RemoveNode(nodeID string) error {
	log.Printf("Removing Docker container: %s", nodeID)

	// Stop the container
	stopCmd := exec.Command("docker", "stop", nodeID)
	if output, err := stopCmd.CombinedOutput(); err != nil {
		log.Printf("Warning: failed to stop container %s: %v - %s", nodeID, err, string(output))
	}

	// Remove the container
	rmCmd := exec.Command("docker", "rm", "-f", nodeID)
	output, err := rmCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove container %s: %v - %s", nodeID, err, string(output))
	}

	log.Printf("Container %s removed successfully", nodeID)
	return nil
}

// GetNodeStatus returns the status of a Docker container
func (m *Manager) GetNodeStatus(nodeID string) (string, error) {
	cmd := exec.Command("docker", "inspect", "--format={{.State.Status}}", nodeID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get status for %s: %v", nodeID, err)
	}

	return strings.TrimSpace(string(output)), nil
}

// Helper functions
func (m *Manager) extractNodeNumber(nodeID string) int {
	// Extract number from nodeID like "node4" -> 4
	if len(nodeID) > 4 && strings.HasPrefix(nodeID, "node") {
		var num int
		fmt.Sscanf(nodeID[4:], "%d", &num)
		return num
	}
	return 1
}

func (m *Manager) createVolume(nodeID string) {
	volumeName := fmt.Sprintf("%s_data", nodeID)
	cmd := exec.Command("docker", "volume", "create", volumeName)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Warning: failed to create volume %s: %v - %s", volumeName, err, string(output))
	} else {
		log.Printf("Created volume: %s", volumeName)
	}
}

// ScaleNodes scales the number of nodes to the target count
func (m *Manager) ScaleNodes(targetCount int) error {
	// Get current running containers
	cmd := exec.Command("docker", "ps", "--filter", "name=node", "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to list containers: %v", err)
	}

	currentNodes := strings.Fields(string(output))
	currentCount := len(currentNodes)

	log.Printf("Current nodes: %d, Target: %d", currentCount, targetCount)

	if currentCount < targetCount {
		// Scale up
		for i := currentCount + 1; i <= targetCount; i++ {
			nodeID := fmt.Sprintf("node%d", i)
			if err := m.CreateNode(nodeID); err != nil {
				log.Printf("Failed to create node %s: %v", nodeID, err)
				continue
			}
		}
	} else if currentCount > targetCount {
		// Scale down (remove excess nodes, prefer non-leader nodes)
		excess := currentCount - targetCount
		for i := 0; i < excess && i < len(currentNodes); i++ {
			nodeID := currentNodes[len(currentNodes)-1-i] // Remove from the end
			if err := m.RemoveNode(nodeID); err != nil {
				log.Printf("Failed to remove node %s: %v", nodeID, err)
				continue
			}
		}
	}

	return nil
}

// GetRunningNodes returns list of currently running node containers
func (m *Manager) GetRunningNodes() ([]string, error) {
	// Get containers with 'node' in name (blockchain nodes)
	cmd := exec.Command("docker", "ps", "--filter", "name=node", "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}

	nodes := strings.Fields(string(output))
	
	// Filter out non-blockchain containers
	var blockchainNodes []string
	for _, node := range nodes {
		// Include containers that are blockchain nodes
		if strings.Contains(node, "blockchain") || strings.HasPrefix(node, "node") {
			blockchainNodes = append(blockchainNodes, node)
		}
	}
	
	return blockchainNodes, nil
}

// RestartNode restarts a specific node container
func (m *Manager) RestartNode(nodeID string) error {
	log.Printf("Restarting Docker container: %s", nodeID)

	cmd := exec.Command("docker", "restart", nodeID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart container %s: %v - %s", nodeID, err, string(output))
	}

	log.Printf("Container %s restarted successfully", nodeID)
	return nil
}

// BuildImage builds the blockchain Docker image
func (m *Manager) BuildImage(dockerfilePath, imageName string) error {
	log.Printf("Building Docker image: %s", imageName)

	cmd := exec.Command("docker", "build", "-t", imageName, dockerfilePath)
	cmd.Dir = dockerfilePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to build image %s: %v - %s", imageName, err, string(output))
	}

	log.Printf("Image %s built successfully", imageName)
	return nil
}

// CleanupOrphanedVolumes removes volumes for containers that no longer exist
func (m *Manager) CleanupOrphanedVolumes() error {
	log.Printf("Cleaning up orphaned volumes...")

	// Get all node volumes
	volumeCmd := exec.Command("docker", "volume", "ls", "--filter", "name=node", "--format", "{{.Name}}")
	volumeOutput, err := volumeCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to list volumes: %v", err)
	}

	volumes := strings.Fields(string(volumeOutput))

	// Get current containers
	containerCmd := exec.Command("docker", "ps", "-a", "--filter", "name=node", "--format", "{{.Names}}")
	containerOutput, err := containerCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to list containers: %v", err)
	}

	containers := strings.Fields(string(containerOutput))

	// Find orphaned volumes
	for _, volume := range volumes {
		isOrphaned := true
		expectedContainer := strings.TrimSuffix(volume, "_data")

		for _, container := range containers {
			if container == expectedContainer {
				isOrphaned = false
				break
			}
		}

		if isOrphaned {
			log.Printf("Removing orphaned volume: %s", volume)
			rmVolCmd := exec.Command("docker", "volume", "rm", volume)
			if output, err := rmVolCmd.CombinedOutput(); err != nil {
				log.Printf("Warning: failed to remove volume %s: %v - %s", volume, err, string(output))
			}
		}
	}

	return nil
}

// MonitorHealth monitors the health of all node containers
func (m *Manager) MonitorHealth(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkContainerHealth()
		}
	}
}

func (m *Manager) checkContainerHealth() {
	nodes, err := m.GetRunningNodes()
	if err != nil {
		log.Printf("Failed to get running nodes: %v", err)
		return
	}

	for _, nodeID := range nodes {
		status, err := m.GetNodeStatus(nodeID)
		if err != nil {
			log.Printf("Failed to get status for %s: %v", nodeID, err)
			continue
		}

		if status != "running" {
			log.Printf("Node %s is not running (status: %s), attempting restart...", nodeID, status)
			if err := m.RestartNode(nodeID); err != nil {
				log.Printf("Failed to restart %s: %v", nodeID, err)
			}
		}
	}
}