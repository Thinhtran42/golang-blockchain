package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	pb "golang-blockchain/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NodeInfo represents information about a blockchain node
type NodeInfo struct {
	ID         string    `json:"id"`
	Address    string    `json:"address"`
	Port       string    `json:"port"`
	IsLeader   bool      `json:"is_leader"`
	IsHealthy  bool      `json:"is_healthy"`
	LastSeen   time.Time `json:"last_seen"`
	Height     int32     `json:"height"`
	TxPoolSize int32     `json:"tx_pool_size"`
}

// Registry manages node discovery and health monitoring
type Registry struct {
	nodes    map[string]*NodeInfo
	nodesMux sync.RWMutex
	server   *http.Server

	// Health check configuration
	healthCheckInterval time.Duration
	unhealthyThreshold  time.Duration
	consensusTimeout    time.Duration

	// Auto-scaling configuration
	minNodes       int
	maxNodes       int
	scaleThreshold int // Minimum nodes needed for consensus

	// Docker management
	dockerClient DockerManager
}

// DockerManager interface for Docker operations
type DockerManager interface {
	CreateNode(nodeID string) error
	RemoveNode(nodeID string) error
	GetNodeStatus(nodeID string) (string, error)
	GetRunningNodes() ([]string, error)
}

// NewRegistry creates a new node registry
func NewRegistry(minNodes, maxNodes int, dockerClient DockerManager) *Registry {
	return &Registry{
		nodes:               make(map[string]*NodeInfo),
		healthCheckInterval: 5 * time.Second,
		unhealthyThreshold:  15 * time.Second,
		consensusTimeout:    10 * time.Second,
		minNodes:            minNodes,
		maxNodes:            maxNodes,
		scaleThreshold:      (minNodes/2 + 1), // Majority needed
		dockerClient:        dockerClient,
	}
}

// Start starts the registry service
func (r *Registry) Start(port string) error {
	// Setup HTTP server for node registration and discovery
	mux := http.NewServeMux()
	mux.HandleFunc("/register", r.handleRegister)
	mux.HandleFunc("/discover", r.handleDiscover)
	mux.HandleFunc("/health", r.handleHealthCheck)
	mux.HandleFunc("/status", r.handleStatus)
	mux.HandleFunc("/consensus-failure", r.handleConsensusFailure)
	mux.HandleFunc("/leadership-request", r.handleLeadershipRequest)

	r.server = &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Discover existing nodes from Docker at startup
	go r.discoverExistingNodes()

	// Start health monitoring
	go r.startHealthMonitoring()

	// Start auto-scaling monitoring
	go r.autoScalingLoop()

	log.Printf("Registry service starting on port %s", port)
	return r.server.ListenAndServe()
}

// Stop stops the registry service
func (r *Registry) Stop() error {
	if r.server != nil {
		return r.server.Shutdown(context.Background())
	}
	return nil
}

// RegisterNode registers a new node
func (r *Registry) RegisterNode(info *NodeInfo) {
	r.nodesMux.Lock()
	defer r.nodesMux.Unlock()

	info.LastSeen = time.Now()
	info.IsHealthy = true
	r.nodes[info.ID] = info

	log.Printf("Node %s registered: %s:%s (Leader: %v)",
		info.ID, info.Address, info.Port, info.IsLeader)
}

// GetHealthyNodes returns list of healthy nodes
func (r *Registry) GetHealthyNodes() []*NodeInfo {
	r.nodesMux.RLock()
	defer r.nodesMux.RUnlock()

	var healthyNodes []*NodeInfo
	for _, node := range r.nodes {
		if node.IsHealthy {
			healthyNodes = append(healthyNodes, node)
		}
	}

	return healthyNodes
}

// GetAllNodes returns all registered nodes
func (r *Registry) GetAllNodes() []*NodeInfo {
	r.nodesMux.RLock()
	defer r.nodesMux.RUnlock()

	var allNodes []*NodeInfo
	for _, node := range r.nodes {
		allNodes = append(allNodes, node)
	}

	return allNodes
}

// HTTP Handlers
func (r *Registry) handleRegister(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var nodeInfo NodeInfo
	if err := json.NewDecoder(req.Body).Decode(&nodeInfo); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	r.nodesMux.Lock()
	nodeInfo.IsHealthy = true
	nodeInfo.LastSeen = time.Now()

	// Check for leadership conflicts
	if nodeInfo.IsLeader {
		// Check if we already have a healthy leader
		hasHealthyLeader := false
		for _, node := range r.nodes {
			if node.IsHealthy && node.IsLeader && node.ID != nodeInfo.ID {
				hasHealthyLeader = true
				break
			}
		}

		if hasHealthyLeader {
			log.Printf("Leadership conflict: Node %s claims leadership but we already have a leader, demoting %s", nodeInfo.ID, nodeInfo.ID)
			nodeInfo.IsLeader = false

			// Notify the node about demotion
			go func() {
				r.nodesMux.RUnlock()
				r.notifyLeadershipDemotion(&nodeInfo)
				r.nodesMux.RLock()
			}()
		}
	}

	r.nodes[nodeInfo.ID] = &nodeInfo
	r.nodesMux.Unlock()

	log.Printf("Node %s registered: %s:%s (Leader: %v)", nodeInfo.ID, nodeInfo.Address, nodeInfo.Port, nodeInfo.IsLeader)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "registered",
		"node_id": nodeInfo.ID,
	})
}

func (r *Registry) handleDiscover(w http.ResponseWriter, req *http.Request) {
	nodes := r.GetHealthyNodes()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
		"count": len(nodes),
	})
}

func (r *Registry) handleHealthCheck(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		// Handle POST with JSON payload
		var heartbeat struct {
			NodeID     string `json:"node_id"`
			Height     int32  `json:"height"`
			TxPoolSize int32  `json:"tx_pool_size"`
		}

		if err := json.NewDecoder(req.Body).Decode(&heartbeat); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		r.nodesMux.Lock()
		defer r.nodesMux.Unlock()

		if node, exists := r.nodes[heartbeat.NodeID]; exists {
			node.LastSeen = time.Now()
			node.IsHealthy = true
			node.Height = heartbeat.Height
			node.TxPoolSize = heartbeat.TxPoolSize
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	// Handle GET with query parameter (legacy)
	nodeID := req.URL.Query().Get("node_id")
	if nodeID == "" {
		http.Error(w, "node_id required", http.StatusBadRequest)
		return
	}

	r.nodesMux.Lock()
	defer r.nodesMux.Unlock()

	if node, exists := r.nodes[nodeID]; exists {
		node.LastSeen = time.Now()
		node.IsHealthy = true
	}

	w.WriteHeader(http.StatusOK)
}

func (r *Registry) handleStatus(w http.ResponseWriter, req *http.Request) {
	r.nodesMux.RLock()
	defer r.nodesMux.RUnlock()

	healthyCount := 0
	for _, node := range r.nodes {
		if node.IsHealthy {
			healthyCount++
		}
	}

	status := map[string]interface{}{
		"total_nodes":   len(r.nodes),
		"healthy_nodes": healthyCount,
		"min_nodes":     r.minNodes,
		"max_nodes":     r.maxNodes,
		"consensus_possible": healthyCount >= r.scaleThreshold,
		"nodes": r.nodes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Health Monitoring
func (r *Registry) startHealthMonitoring() {
	ticker := time.NewTicker(r.healthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		r.checkNodeHealth()
	}
}

func (r *Registry) checkNodeHealth() {
	r.nodesMux.Lock()
	defer r.nodesMux.Unlock()

	now := time.Now()
	for nodeID, node := range r.nodes {
		if now.Sub(node.LastSeen) > r.unhealthyThreshold {
			if node.IsHealthy {
				log.Printf("Node %s marked as unhealthy (last seen: %v ago)",
					nodeID, now.Sub(node.LastSeen))
				node.IsHealthy = false
			}
		}

		// Try to ping node directly via gRPC
		if !node.IsHealthy {
			if r.pingNode(node) {
				log.Printf("Node %s recovered and marked as healthy", nodeID)
				node.IsHealthy = true
				node.LastSeen = now
			}
		}
	}
}

func (r *Registry) pingNode(node *NodeInfo) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx,
		fmt.Sprintf("%s:%s", node.Address, node.Port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return false
	}
	defer conn.Close()

	client := pb.NewBlockchainServiceClient(conn)
	resp, err := client.GetStatus(ctx, &pb.StatusRequest{NodeId: "registry"})
	if err != nil {
		return false
	}

	// Update node info including actual leadership status
	node.Height = resp.CurrentHeight
	node.TxPoolSize = resp.TxPoolSize

	// Check for leadership conflicts
	actualIsLeader := resp.IsLeader
	if actualIsLeader != node.IsLeader {
		log.Printf("Leadership conflict detected: Node %s registry=%v actual=%v", node.ID, node.IsLeader, actualIsLeader)

		if actualIsLeader && !node.IsLeader {
			// Node thinks it's leader but registry says it's not
			// Check if we have another healthy leader
			hasOtherLeader := false
			for _, otherNode := range r.nodes {
				if otherNode.IsHealthy && otherNode.IsLeader && otherNode.ID != node.ID {
					hasOtherLeader = true
					break
				}
			}

			if hasOtherLeader {
				log.Printf("Demoting node %s - already have another leader", node.ID)
				r.notifyLeadershipDemotion(node)
			} else {
				log.Printf("Accepting node %s as leader - no other healthy leader", node.ID)
				node.IsLeader = true
			}
		}
	}

	return true
}

// Auto-scaling
func (r *Registry) autoScalingLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check node health first
			r.checkNodeHealth()

			// Perform leader election if needed
			r.performLeaderElection()

		// Count healthy nodes
		healthyCount := len(r.GetHealthyNodes())

		log.Printf("Auto-scaling check: %d healthy nodes (min: %d, threshold: %d)",
			healthyCount, r.minNodes, r.scaleThreshold)

		// Scale up if below threshold
		if healthyCount < r.scaleThreshold {
			log.Printf("Need to scale up! Current: %d, Required: %d", healthyCount, r.scaleThreshold)

			// Calculate how many nodes to create
			nodesToCreate := r.scaleThreshold - healthyCount

			for i := 0; i < nodesToCreate; i++ {
				r.nodesMux.RLock()
				totalNodes := len(r.nodes)
				r.nodesMux.RUnlock()

				newNodeID := fmt.Sprintf("node%d", totalNodes+1)
				log.Printf("Creating new node: %s", newNodeID)

				if err := r.dockerClient.CreateNode(newNodeID); err != nil {
					log.Printf("Failed to create new node %s: %v", newNodeID, err)
					continue
				}

				log.Printf("Successfully triggered creation of node %s", newNodeID)
			}
		}
		}
	}
}

// performLeaderElection checks if we need to elect a new leader
func (r *Registry) performLeaderElection() {
	r.nodesMux.Lock()
	defer r.nodesMux.Unlock()

	// First, clear any unhealthy leaders
	for _, node := range r.nodes {
		if !node.IsHealthy && node.IsLeader {
			log.Printf("Removing unhealthy leader: %s", node.ID)
			node.IsLeader = false
		}
	}

	// Check current leadership state
	var healthyLeaders []*NodeInfo
	var healthyFollowers []*NodeInfo

	for _, node := range r.nodes {
		if node.IsHealthy {
			if node.IsLeader {
				healthyLeaders = append(healthyLeaders, node)
			} else {
				healthyFollowers = append(healthyFollowers, node)
			}
		}
	}

	// Handle different leadership scenarios
	switch len(healthyLeaders) {
	case 0:
		// No leader - elect new one
		if len(healthyFollowers) > 0 {
			r.electNewLeader(healthyFollowers)
		}
	case 1:
		// Normal case - one leader
		log.Printf("Leader status OK: %s", healthyLeaders[0].ID)
	default:
		// Multiple leaders - resolve conflict
		log.Printf("Leadership conflict detected: %d leaders found", len(healthyLeaders))
		r.resolveLeadershipConflict(healthyLeaders)
	}
}

// electNewLeader elects a new leader from followers
func (r *Registry) electNewLeader(followers []*NodeInfo) {
	log.Printf("No healthy leader found, starting leader election")

	// First, demote any existing leaders (even unhealthy ones)
	for _, node := range r.nodes {
		if node.IsLeader {
			log.Printf("Demoting existing leader: %s", node.ID)
			node.IsLeader = false

			// If the node is healthy, notify about demotion
			if node.IsHealthy {
				r.notifyLeadershipDemotion(node)
			}
		}
	}

	// Select the follower with highest height as new leader
	var newLeader *NodeInfo
	maxHeight := int32(-1)

	for _, follower := range followers {
		if follower.Height > maxHeight {
			maxHeight = follower.Height
			newLeader = follower
		}
	}

	if newLeader != nil {
		log.Printf("Electing node %s as new leader (height: %d)", newLeader.ID, newLeader.Height)

		// Promote to leader
		newLeader.IsLeader = true

		// Notify the node about leadership change
		r.notifyLeadershipChange(newLeader)

		log.Printf("Successfully elected %s as new leader", newLeader.ID)
	}
}

// resolveLeadershipConflict resolves conflicts when multiple leaders exist
func (r *Registry) resolveLeadershipConflict(leaders []*NodeInfo) {
	// Find the leader with highest height (most up-to-date)
	var primaryLeader *NodeInfo
	maxHeight := int32(-1)

	for _, leader := range leaders {
		if leader.Height > maxHeight {
			maxHeight = leader.Height
			primaryLeader = leader
		}
	}

	if primaryLeader == nil {
		log.Printf("Cannot resolve leadership conflict - no valid leader found")
		return
	}

	log.Printf("Resolving leadership conflict: keeping %s as primary leader (height: %d)",
		primaryLeader.ID, primaryLeader.Height)

	// Demote other leaders to followers
	for _, leader := range leaders {
		if leader.ID != primaryLeader.ID {
			log.Printf("Demoting %s from leader to follower", leader.ID)
			leader.IsLeader = false

			// Notify the demoted node
			r.notifyLeadershipDemotion(leader)
		}
	}
}

// notifyLeadershipChange sends leadership change notification to the node
func (r *Registry) notifyLeadershipChange(node *NodeInfo) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Prepare notification payload
	payload := map[string]interface{}{
		"new_leader": true,
		"node_id":    node.ID,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal leadership notification: %v", err)
		return
	}

	// Send leadership notification to the node
	url := fmt.Sprintf("http://%s:8080/leadership", node.Address)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to create leadership notification request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send leadership notification to %s: %v", node.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Successfully notified %s about leadership change", node.ID)
	} else {
		log.Printf("Leadership notification failed for %s: HTTP %d", node.ID, resp.StatusCode)
	}
}

// notifyLeadershipDemotion sends leadership demotion notification to the node
func (r *Registry) notifyLeadershipDemotion(node *NodeInfo) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Prepare demotion payload
	payload := map[string]interface{}{
		"new_leader": false,
		"demoted":    true,
		"node_id":    node.ID,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal demotion notification: %v", err)
		return
	}

	// Send demotion notification to the node
	url := fmt.Sprintf("http://%s:8080/leadership", node.Address)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to create demotion notification request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send demotion notification to %s: %v", node.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Successfully notified %s about leadership demotion", node.ID)
	} else {
		log.Printf("Demotion notification failed for %s: HTTP %d", node.ID, resp.StatusCode)
	}
}

// Node Discovery for new nodes
func (r *Registry) GetPeerList(requestingNodeID string) []string {
	healthyNodes := r.GetHealthyNodes()
	var peers []string

	for _, node := range healthyNodes {
		if node.ID != requestingNodeID {
			peers = append(peers, fmt.Sprintf("%s:%s", node.Address, node.Port))
		}
	}

	return peers
}

// Consensus failure detection
func (r *Registry) ReportConsensusFailure(nodeID string, reason string) {
	log.Printf("Consensus failure reported by %s: %s", nodeID, reason)

	// Trigger immediate scaling evaluation
	go r.autoScalingLoop()
}

// Add consensus failure reporting endpoint
func (r *Registry) handleConsensusFailure(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var report struct {
		NodeID string `json:"node_id"`
		Reason string `json:"reason"`
	}

	if err := json.NewDecoder(req.Body).Decode(&report); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	r.ReportConsensusFailure(report.NodeID, report.Reason)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "reported",
		"message": "Consensus failure recorded",
	})
}

// discoverExistingNodes discovers nodes already running in Docker
func (r *Registry) discoverExistingNodes() {
	log.Printf("Discovering existing Docker containers...")

	// Wait a bit for Docker to be ready
	time.Sleep(3 * time.Second)

	runningNodes, err := r.dockerClient.GetRunningNodes()
	if err != nil {
		log.Printf("Failed to get running nodes: %v", err)
		return
	}

	for _, nodeID := range runningNodes {
		// Skip registry container
		if strings.Contains(nodeID, "registry") {
			continue
		}

		// Extract clean node ID
		cleanNodeID := r.extractCleanNodeID(nodeID)

		// Try to determine if it's a leader (node1 is usually leader)
		isLeader := strings.Contains(cleanNodeID, "node1")

		// Create node info for discovered container
		nodeInfo := &NodeInfo{
			ID:         cleanNodeID,
			Address:    cleanNodeID, // Use container name as address in Docker network
			Port:       "50051",     // Standard internal port
			IsLeader:   isLeader,
			IsHealthy:  true,
			LastSeen:   time.Now(),
			Height:     0,
			TxPoolSize: 0,
		}

		// Register the discovered node
		r.RegisterNode(nodeInfo)
	}

	log.Printf("Discovered %d existing nodes", len(runningNodes))
}

// extractCleanNodeID extracts clean node ID from Docker container name
func (r *Registry) extractCleanNodeID(containerName string) string {
	// Handle docker-compose naming: "golang-blockchain-node1-1" -> "node1"
	if strings.Contains(containerName, "golang-blockchain-") && strings.Contains(containerName, "node") {
		parts := strings.Split(containerName, "-")
		for _, part := range parts {
			if strings.HasPrefix(part, "node") {
				return strings.Split(part, "-")[0] // Remove any trailing parts
			}
		}
	}

	// Handle direct naming: "node1" -> "node1"
	if strings.HasPrefix(containerName, "node") {
		return containerName
	}

	// Fallback
	return containerName
}

func (r *Registry) handleLeadershipRequest(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		NodeID string `json:"node_id"`
	}

	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	r.handleLeadershipRequestInternal(request.NodeID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "request_received",
	})
}

func (r *Registry) handleLeadershipRequestInternal(nodeID string) {
	r.nodesMux.Lock()
	defer r.nodesMux.Unlock()

	log.Printf("🎯 Processing leadership request from node %s", nodeID)

	node, exists := r.nodes[nodeID]
	if !exists {
		log.Printf("❌ Node %s not found in registry", nodeID)
		return
	}

	// Check current leadership state
	currentLeaders := 0
	var currentLeader *NodeInfo

	for _, n := range r.nodes {
		if n.IsHealthy && n.IsLeader {
			currentLeaders++
			currentLeader = n
		}
	}

	switch currentLeaders {
	case 0:
		// No current leader - grant leadership
		log.Printf("✅ No current leader, granting leadership to %s", nodeID)
		node.IsLeader = true
		r.notifyLeadershipChange(node)

	case 1:
		if currentLeader.ID == nodeID {
			log.Printf("ℹ️ Node %s is already the leader", nodeID)
		} else {
			// Compare heights to determine who should be leader
			if node.Height > currentLeader.Height {
				log.Printf("🔄 Node %s has higher height (%d vs %d), transferring leadership",
					nodeID, node.Height, currentLeader.Height)

				// Demote current leader
				currentLeader.IsLeader = false
				r.notifyLeadershipDemotion(currentLeader)

				// Promote requesting node
				node.IsLeader = true
				r.notifyLeadershipChange(node)
			} else {
				log.Printf("❌ Leadership request denied: current leader %s has higher or equal height (%d vs %d)",
					currentLeader.ID, currentLeader.Height, node.Height)
			}
		}

	default:
		// Multiple leaders - resolve conflict
		log.Printf("⚠️ Multiple leaders detected during request, resolving...")
		leaders := []*NodeInfo{node}
		for _, n := range r.nodes {
			if n.IsHealthy && n.IsLeader && n.ID != nodeID {
				leaders = append(leaders, n)
			}
		}
		r.resolveLeadershipConflict(leaders)
	}
}
