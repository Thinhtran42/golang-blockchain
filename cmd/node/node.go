package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang-blockchain/pkg/blockchain"
	"golang-blockchain/pkg/discovery"
	"golang-blockchain/pkg/p2p"
	"golang-blockchain/pkg/storage"
	pb "golang-blockchain/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ValidatorNode implementation đầy đủ
type ValidatorNode struct {
	*p2p.Node
	storage *storage.BlockStorage
	txPool  *blockchain.TransactionPool

	// gRPC clients for sending votes/commits
	clients    map[string]pb.BlockchainServiceClient
	clientsMux sync.RWMutex

	// Consensus control
	isProposing  bool
	proposingMux sync.RWMutex

	// Discovery client for auto-discovery
	discoveryClient *discovery.Client
}

// NewValidatorNode tạo validator node mới
func NewValidatorNode(id, address, port string, isLeader bool, dbPath string) (*ValidatorNode, error) {
	// Khởi tạo storage
	storage, err := storage.NewBlockStorage(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Khởi tạo transaction pool
	txPool := blockchain.NewTransactionPool(1000) // Max 1000 transactions

	// Tạo p2p node
	p2pNode := p2p.NewNode(id, address, port, isLeader, storage, txPool)

	node := &ValidatorNode{
		Node:    p2pNode,
		storage: storage,
		txPool:  txPool,
		clients: make(map[string]pb.BlockchainServiceClient),
	}

	return node, nil
}

// GetID returns node ID
func (vn *ValidatorNode) GetID() string {
	return vn.ID
}

// IsLeaderNode returns if this node is leader
func (vn *ValidatorNode) IsLeaderNode() bool {
	return vn.Node.IsLeader
}

// PromoteToLeader promotes this node to leader
func (vn *ValidatorNode) PromoteToLeader() {
	vn.proposingMux.Lock()
	defer vn.proposingMux.Unlock()

	vn.Node.IsLeader = true
	log.Printf("Node %s promoted to leader", vn.GetID())
}

// DemoteToFollower demotes this node to follower
func (vn *ValidatorNode) DemoteToFollower() {
	vn.proposingMux.Lock()
	defer vn.proposingMux.Unlock()

	vn.Node.IsLeader = false
	vn.isProposing = false // Stop any ongoing consensus activities
	log.Printf("Node %s demoted to follower", vn.GetID())
}

// GetStorageInstance returns storage instance
func (vn *ValidatorNode) GetStorageInstance() *storage.BlockStorage {
	return vn.storage
}

// GetTxPoolInstance returns transaction pool instance
func (vn *ValidatorNode) GetTxPoolInstance() *blockchain.TransactionPool {
	return vn.txPool
}

// SendVote sends vote to leader (simplified implementation)
func (vn *ValidatorNode) SendVote(vote *pb.Vote) error {
	// Find leader and send vote
	peers := vn.GetPeers()
	for _, peerAddr := range peers {
		client, exists := vn.getClient(peerAddr)
		if !exists {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := client.SubmitVote(ctx, &pb.VoteRequest{Vote: vote})
		cancel()

		if err != nil {
			log.Printf("Failed to send vote to %s: %v", peerAddr, err)
		}
	}

	return nil
}

// SendCommit sends commit to all peers
func (vn *ValidatorNode) SendCommit(block *blockchain.Block) error {
	protoBlock := vn.blockToProto(block)
	req := &pb.CommitBlockRequest{Block: protoBlock}

	peers := vn.GetPeers()
	var wg sync.WaitGroup

	for _, peerAddr := range peers {
		client, exists := vn.getClient(peerAddr)
		if !exists {
			continue
		}

		wg.Add(1)
		go func(addr string, c pb.BlockchainServiceClient) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			_, err := c.CommitBlock(ctx, req)
			if err != nil {
				log.Printf("Failed to send commit to %s: %v", addr, err)
			}
		}(peerAddr, client)
	}

	wg.Wait()
	return nil
}

func (vn *ValidatorNode) getClient(address string) (pb.BlockchainServiceClient, bool) {
	vn.clientsMux.RLock()
	client, exists := vn.clients[address]
	vn.clientsMux.RUnlock()
	return client, exists
}

// ConnectToPeers kết nối đến các peers
func (vn *ValidatorNode) ConnectToPeers(peerAddresses []string) error {
	for _, peerAddr := range peerAddresses {
		if err := vn.AddPeer(peerAddr); err != nil {
			log.Printf("Failed to connect to peer %s: %v", peerAddr, err)
			continue
		}

		// Tạo gRPC client
		conn, err := grpc.Dial(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Failed to create gRPC client for %s: %v", peerAddr, err)
			continue
		}

		client := pb.NewBlockchainServiceClient(conn)

		vn.clientsMux.Lock()
		vn.clients[peerAddr] = client
		vn.clientsMux.Unlock()

		log.Printf("Connected to peer %s", peerAddr)
	}

	return nil
}

// SyncWithPeers đồng bộ với peers (simplified - since we don't have these gRPC methods)
func (vn *ValidatorNode) SyncWithPeers() error {
	log.Printf("Node %s starting basic sync with peers", vn.ID)

	// Get current height
	currentHeight, err := vn.storage.GetLatestHeight()
	if err != nil {
		currentHeight = -1
	}

	log.Printf("Node %s current height: %d", vn.ID, currentHeight)

	// For now, just try to get latest block from each peer
	peers := vn.GetPeers()
	for _, peerAddr := range peers {
		client, exists := vn.getClient(peerAddr)
		if !exists {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := client.GetLatestBlock(ctx, &pb.GetLatestBlockRequest{NodeId: vn.ID})
		cancel()

		if err != nil {
			log.Printf("Failed to get latest block from %s: %v", peerAddr, err)
			continue
		}

		if resp.Success && resp.Height > int32(currentHeight) {
			log.Printf("Peer %s has higher block at height %d", peerAddr, resp.Height)
			// In a real implementation, we would sync the missing blocks
		}
	}

	log.Printf("Node %s sync completed", vn.ID)
	return nil
}

// StartConsensusLoop starts consensus process for leader - FIXED IMPLEMENTATION
func (vn *ValidatorNode) StartConsensusLoop() {
	if !vn.Node.IsLeader {
		log.Printf("Node %s is not leader, not starting consensus loop", vn.ID)
		return
	}

	log.Printf("Leader %s starting consensus loop", vn.ID)

	// Start consensus loop in goroutine
	go func() {
		ticker := time.NewTicker(15 * time.Second) // Create block every 15 seconds
		defer ticker.Stop()

		for range ticker.C {
			vn.proposingMux.RLock()
			isProposing := vn.isProposing
			vn.proposingMux.RUnlock()

			if isProposing {
				continue // Skip if already proposing
			}

			vn.proposeNewBlock()
		}
	}()
}

// proposeNewBlock tạo và đề xuất block mới
func (vn *ValidatorNode) proposeNewBlock() {
	vn.proposingMux.Lock()
	vn.isProposing = true
	vn.proposingMux.Unlock()

	defer func() {
		vn.proposingMux.Lock()
		vn.isProposing = false
		vn.proposingMux.Unlock()
	}()

	// Get pending transactions
	transactions := vn.txPool.GetTransactions()
	if len(transactions) == 0 {
		log.Printf("Leader %s: No pending transactions", vn.ID)
		return
	}

	// Limit transactions per block
	maxTxPerBlock := 10
	if len(transactions) > maxTxPerBlock {
		transactions = transactions[:maxTxPerBlock]
	}

	// Get previous block hash
	var previousHash []byte
	height := 0

	latestHeight, err := vn.storage.GetLatestHeight()
	if err == nil && latestHeight >= 0 {
		latestBlock, err := vn.storage.GetLatestBlock()
		if err == nil {
			previousHash = latestBlock.GetHash()
			height = latestHeight + 1
		}
	}

	// Create new block
	block := blockchain.NewBlock(transactions, previousHash, height)

	log.Printf("Leader %s: Proposing block at height %d with %d transactions",
		vn.ID, height, len(transactions))

	// Propose block to network
	err = vn.ProposeBlockToNetwork(block)
	if err != nil {
		log.Printf("Leader %s: Failed to propose block: %v", vn.ID, err)
	}
}

// ProposeBlockToNetwork đề xuất block tới network
func (vn *ValidatorNode) ProposeBlockToNetwork(block *blockchain.Block) error {
	log.Printf("Leader %s proposing block at height %d", vn.ID, block.Height)

	// Convert block to proto
	protoBlock := vn.blockToProto(block)
	req := &pb.ProposeBlockRequest{
		Block:      protoBlock,
		ProposerId: vn.ID,
	}

	// Gửi propose request đến tất cả peers
	vn.clientsMux.RLock()
	clients := make(map[string]pb.BlockchainServiceClient)
	for addr, client := range vn.clients {
		clients[addr] = client
	}
	vn.clientsMux.RUnlock()

	var wg sync.WaitGroup
	successCount := 0
	var successMux sync.Mutex

	for peerAddr, client := range clients {
		wg.Add(1)
		go func(addr string, c pb.BlockchainServiceClient) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			resp, err := c.ProposeBlock(ctx, req)
			if err != nil {
				log.Printf("Failed to propose block to %s: %v", addr, err)
				return
			}

			if resp.Success {
				log.Printf("Peer %s accepted proposal", addr)
				successMux.Lock()
				successCount++
				successMux.Unlock()
			}
		}(peerAddr, client)
	}

	wg.Wait()

	// Check if majority approved
	totalPeers := len(clients)
	if totalPeers == 0 {
		// No peers, commit locally (single node mode)
		log.Printf("No peers, committing block locally")
		return vn.commitBlockLocally(block)
	}

	majority := (totalPeers / 2) + 1

	if successCount >= majority {
		log.Printf("Block approved by majority (%d/%d), committing...", successCount, totalPeers)
		return vn.commitBlockLocally(block)
	} else {
		log.Printf("Block rejected by majority (%d/%d)", successCount, totalPeers)
		return fmt.Errorf("block rejected by majority")
	}
}

// commitBlockLocally commit block locally and broadcast
func (vn *ValidatorNode) commitBlockLocally(block *blockchain.Block) error {
	// First commit locally
	err := vn.storage.SaveBlock(block)
	if err != nil {
		return fmt.Errorf("failed to commit block locally: %w", err)
	}

	// Remove transactions from pool
	vn.txPool.RemoveTransactions(block.Transactions)

	log.Printf("Leader %s committed block at height %d", vn.ID, block.Height)

	// Broadcast commit to peers
	vn.SendCommit(block)

	return nil
}

// Helper methods
func (vn *ValidatorNode) blockToProto(block *blockchain.Block) *pb.Block {
	var protoTxs []*pb.Transaction
	for _, tx := range block.Transactions {
		protoTx := &pb.Transaction{
			Sender:    tx.Sender,
			Receiver:  tx.Receiver,
			Amount:    tx.Amount,
			Timestamp: tx.Timestamp,
			Signature: tx.Signature,
		}
		protoTxs = append(protoTxs, protoTx)
	}

	return &pb.Block{
		Transactions:      protoTxs,
		MerkleRoot:        block.MerkleRoot,
		PreviousBlockHash: block.PreviousBlockHash,
		CurrentBlockHash:  block.CurrentBlockHash,
		Timestamp:         block.Timestamp,
		Height:            int32(block.Height),
	}
}

func (vn *ValidatorNode) protoToBlock(protoBlock *pb.Block) *blockchain.Block {
	var txs []*blockchain.Transaction
	for _, protoTx := range protoBlock.Transactions {
		tx := &blockchain.Transaction{
			Sender:    protoTx.Sender,
			Receiver:  protoTx.Receiver,
			Amount:    protoTx.Amount,
			Timestamp: protoTx.Timestamp,
			Signature: protoTx.Signature,
		}
		txs = append(txs, tx)
	}

	return &blockchain.Block{
		Transactions:      txs,
		MerkleRoot:        protoBlock.MerkleRoot,
		PreviousBlockHash: protoBlock.PreviousBlockHash,
		CurrentBlockHash:  protoBlock.CurrentBlockHash,
		Timestamp:         protoBlock.Timestamp,
		Height:            int(protoBlock.Height),
	}
}

// setupAutoDiscovery initializes auto-discovery for the node
func (vn *ValidatorNode) setupAutoDiscovery(registryURL string) error {
	log.Printf("Setting up auto-discovery with registry: %s", registryURL)

	// Get current height and pool size
	currentHeight, _ := vn.storage.GetLatestHeight()
	txPoolSize := int32(vn.txPool.GetTransactionCount())

	// Create node info for registration
	nodeInfo := &discovery.NodeInfo{
		ID:         vn.GetID(),
		Address:    vn.GetID(), // Use node ID as address in Docker network
		Port:       vn.Port,
		IsLeader:   vn.IsLeaderNode(),
		Height:     int32(currentHeight),
		TxPoolSize: txPoolSize,
	}

	// Create discovery client
	vn.discoveryClient = discovery.NewClient(registryURL, nodeInfo)

	// Start auto-discovery loop
	connectCallback := func(peers []string) error {
		log.Printf("Discovered peers: %v", peers)
		return vn.ConnectToPeers(peers)
	}

	go vn.discoveryClient.AutoDiscoveryLoop(15*time.Second, connectCallback)

	// Setup leadership change endpoint
	vn.setupLeadershipEndpoint()

	log.Printf("Auto-discovery setup completed")
	return nil
}

// setupLeadershipEndpoint sets up HTTP endpoint for leadership notifications
func (vn *ValidatorNode) setupLeadershipEndpoint() {
	http.HandleFunc("/leadership", vn.handleLeadershipChange)
	log.Printf("Leadership endpoint setup at :8080/leadership")
}

// handleLeadershipChange handles leadership change notifications from registry
func (vn *ValidatorNode) handleLeadershipChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var notification struct {
		NewLeader bool   `json:"new_leader"`
		Demoted   bool   `json:"demoted"`
		NodeID    string `json:"node_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Check if this node is being promoted to leader
	if notification.NewLeader && notification.NodeID == vn.GetID() {
		log.Printf("🎯 Received leadership promotion notification!")

		// Promote this node to leader
		vn.PromoteToLeader()

		// Start consensus loop
		go vn.StartConsensusLoop()

		log.Printf("✅ Successfully promoted to leader and started consensus")
	}

	// Check if this node is being demoted from leader
	if notification.Demoted && notification.NodeID == vn.GetID() {
		log.Printf("⬇️ Received leadership demotion notification!")

		// Demote this node to follower
		vn.DemoteToFollower()

		log.Printf("✅ Successfully demoted to follower")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "received",
		"node_id": vn.GetID(),
	})
}

// performAutoSync performs automatic synchronization with discovered peers
func (vn *ValidatorNode) performAutoSync() {
	log.Printf("Starting auto-sync process...")

	// Wait for initial peer discovery
	time.Sleep(10 * time.Second)

	// Try to sync every 30 seconds until successful, then every 60 seconds for maintenance
	syncTicker := time.NewTicker(30 * time.Second)
	defer syncTicker.Stop()

	isInitialSync := true

	for {
		select {
		case <-syncTicker.C:
			// Update discovery client with current status
			if vn.discoveryClient != nil {
				currentHeight, _ := vn.storage.GetLatestHeight()
				txPoolSize := int32(vn.txPool.GetTransactionCount())
				vn.discoveryClient.UpdateNodeInfo(int32(currentHeight), txPoolSize)
			}

			// Try to sync with peers
			peers := vn.GetPeers()
			if len(peers) > 0 {
				log.Printf("Attempting sync with %d peers", len(peers))

				// Check if we need aggressive sync for restarted nodes
				needsAggressiveSync := vn.checkNeedsAggressiveSync(peers)

				var err error
				if needsAggressiveSync {
					log.Printf("Detected restart or outdated state, performing aggressive sync")
					err = vn.aggressiveSyncFromPeers(peers)
				} else {
					err = vn.syncBlockchainFromPeers()
				}

				if err != nil {
					log.Printf("Sync attempt failed: %v", err)
				} else {
					log.Printf("Sync completed successfully")
					if isInitialSync {
						// After successful initial sync, check less frequently
						syncTicker.Reset(60 * time.Second)
						isInitialSync = false
					}
				}
			} else {
				log.Printf("No peers available for sync, waiting...")
			}
		}
	}
}

// checkNeedsAggressiveSync determines if this node needs aggressive sync
func (vn *ValidatorNode) checkNeedsAggressiveSync(peers []string) bool {
	currentHeight, err := vn.storage.GetLatestHeight()
	if err != nil {
		// If we can't get height, we probably need to sync
		return true
	}

	// Check if we're significantly behind any peer
	for _, peerAddr := range peers {
		client, exists := vn.getClient(peerAddr)
		if !exists {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := client.GetLatestBlock(ctx, &pb.GetLatestBlockRequest{NodeId: vn.GetID()})
		cancel()

		if err != nil {
			continue
		}

		if resp.Success {
			peerHeight := int(resp.Height)
			// If peer is more than 1 block ahead, we need aggressive sync
			if peerHeight > currentHeight+1 {
				log.Printf("Peer %s has height %d, we have %d - need aggressive sync", peerAddr, peerHeight, currentHeight)
				return true
			}
		}
	}

	return false
}

// aggressiveSyncFromPeers performs complete blockchain sync for restarted nodes
func (vn *ValidatorNode) aggressiveSyncFromPeers(peers []string) error {
	log.Printf("Starting aggressive sync to catch up with network")

	// Find the peer with highest height
	var bestPeer string
	var highestHeight int = -1

	for _, peerAddr := range peers {
		client, exists := vn.getClient(peerAddr)
		if !exists {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		resp, err := client.GetLatestBlock(ctx, &pb.GetLatestBlockRequest{NodeId: vn.GetID()})
		cancel()

		if err != nil {
			log.Printf("Failed to get latest block from %s: %v", peerAddr, err)
			continue
		}

		if resp.Success && int(resp.Height) > highestHeight {
			highestHeight = int(resp.Height)
			bestPeer = peerAddr
		}
	}

	if bestPeer == "" {
		return fmt.Errorf("no peers available for aggressive sync")
	}

	log.Printf("Best peer for sync: %s with height %d", bestPeer, highestHeight)

	// Get our current height
	currentHeight, err := vn.storage.GetLatestHeight()
	if err != nil {
		currentHeight = -1
	}

	// If we're already up to date, no need to sync
	if currentHeight >= highestHeight {
		log.Printf("Already synced with best peer (our height: %d, peer height: %d)", currentHeight, highestHeight)
		return nil
	}

	// For aggressive sync, we'll sync all missing blocks
	// In this implementation, we'll at least sync the latest block
	client, exists := vn.getClient(bestPeer)
	if !exists {
		return fmt.Errorf("lost connection to best peer during sync")
	}

	// Sync the latest block (in a full implementation, you'd sync all missing blocks)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	resp, err := client.GetLatestBlock(ctx, &pb.GetLatestBlockRequest{NodeId: vn.GetID()})
	cancel()

	if err != nil {
		return fmt.Errorf("failed to get latest block during aggressive sync: %v", err)
	}

	if !resp.Success || resp.Block == nil {
		return fmt.Errorf("peer returned unsuccessful response during aggressive sync")
	}

	// Convert and validate the block
	block := vn.protoToBlock(resp.Block)
	if err := block.ValidateBlock(); err != nil {
		return fmt.Errorf("invalid block received during aggressive sync: %v", err)
	}

	// Save the block
	if err := vn.storage.SaveBlock(block); err != nil {
		return fmt.Errorf("failed to save block during aggressive sync: %v", err)
	}

	log.Printf("Aggressive sync completed: synced to height %d from %s", block.Height, bestPeer)

	// Force update our status with discovery client
	if vn.discoveryClient != nil {
		newHeight, _ := vn.storage.GetLatestHeight()
		txPoolSize := int32(vn.txPool.GetTransactionCount())
		vn.discoveryClient.UpdateNodeInfo(int32(newHeight), txPoolSize)
		log.Printf("Updated registry with new height: %d", newHeight)
	}

	return nil
}

// syncBlockchainFromPeers syncs blockchain data from available peers
func (vn *ValidatorNode) syncBlockchainFromPeers() error {
	peers := vn.GetPeers()
	if len(peers) == 0 {
		return fmt.Errorf("no peers available for sync")
	}

	// Get current height
	currentHeight, err := vn.storage.GetLatestHeight()
	if err != nil {
		currentHeight = -1
	}

	log.Printf("Current blockchain height: %d", currentHeight)

	// Try to sync from each peer
	for _, peerAddr := range peers {
		client, exists := vn.getClient(peerAddr)
		if !exists {
			continue
		}

		// Get latest block from peer
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		resp, err := client.GetLatestBlock(ctx, &pb.GetLatestBlockRequest{NodeId: vn.GetID()})
		cancel()

		if err != nil {
			log.Printf("Failed to get latest block from %s: %v", peerAddr, err)
			continue
		}

		if !resp.Success {
			log.Printf("Peer %s returned unsuccessful response", peerAddr)
			continue
		}

		peerHeight := int(resp.Height)
		log.Printf("Peer %s has height: %d", peerAddr, peerHeight)

		// If peer has newer blocks, sync them
		if peerHeight > currentHeight {
			log.Printf("Syncing blocks from height %d to %d", currentHeight+1, peerHeight)

			// For now, just sync the latest block
			// In a full implementation, you'd sync all missing blocks
			if resp.Block != nil {
				block := vn.protoToBlock(resp.Block)

				// Validate and save the block
				if err := block.ValidateBlock(); err != nil {
					log.Printf("Invalid block from peer %s: %v", peerAddr, err)
					continue
				}

				// Save the block
				if err := vn.storage.SaveBlock(block); err != nil {
					log.Printf("Failed to save synced block: %v", err)
					continue
				}

				log.Printf("Successfully synced block at height %d from %s", block.Height, peerAddr)
				return nil
			}
		} else if peerHeight == currentHeight {
			log.Printf("Already synced with peer %s", peerAddr)
			return nil
		}
	}

	return fmt.Errorf("failed to sync from any peer")
}

// startHTTPServer starts HTTP server for handling leadership notifications
func (vn *ValidatorNode) startHTTPServer() {
	log.Printf("Starting HTTP server on :8080")

	// Setup health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "healthy",
			"node_id": vn.GetID(),
			"is_leader": vn.IsLeaderNode(),
		})
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: nil,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("HTTP server error: %v", err)
	}
}

// requestLeadershipFromRegistry requests leadership status from registry
func (vn *ValidatorNode) requestLeadershipFromRegistry(registryURL string) {
	// Wait a bit for node to be fully registered
	time.Sleep(3 * time.Second)
	
	log.Printf("🎯 Requesting leadership from registry...")
	
	// Create HTTP client
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	// Prepare request payload
	payload := map[string]interface{}{
		"node_id": vn.GetID(),
		"action":  "request_leadership",
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal leadership request: %v", err)
		return
	}
	
	// Send leadership request
	url := fmt.Sprintf("%s/leadership-request", registryURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to create leadership request: %v", err)
		return
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send leadership request: %v", err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusOK {
		log.Printf("✅ Leadership request sent successfully")
	} else {
		log.Printf("❌ Leadership request failed: HTTP %d", resp.StatusCode)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Printf("🚀 STARTING NODE MAIN FUNCTION - DEBUG VERSION")

	// Read environment variables
	nodeIDEnv := os.Getenv("NODE_ID")
	if nodeIDEnv == "" {
		log.Fatal("NODE_ID environment variable is required")
	}

	portEnv := os.Getenv("PORT")
	if portEnv == "" {
		portEnv = "50051"
	}

	// Parse peers from environment or use auto-discovery
	var peers []string
	peersEnv := os.Getenv("PEERS")
	registryURL := os.Getenv("REGISTRY_URL")

	isLeaderStr := os.Getenv("IS_LEADER")
	autoDiscover := os.Getenv("AUTO_DISCOVER") == "true"
	
	log.Printf("🔍 DEBUG: isLeaderStr='%s', autoDiscover=%v, registryURL='%s'", isLeaderStr, autoDiscover, registryURL)
	
	// Smart leadership determination
	var isLeader bool
	if autoDiscover && registryURL != "" {
		// In auto-discovery mode, let registry determine leadership
		// Node starts as follower and can be promoted later
		isLeader = false
		log.Printf("Auto-discovery mode: Starting as follower, registry will manage leadership")
	} else {
		// In static mode, use environment variable
		isLeader = isLeaderStr == "true"
		log.Printf("Static mode: Using configured leadership status: %v", isLeader)
	}

	if autoDiscover && registryURL != "" {
		// Use auto-discovery mode - ignore static peers
		log.Printf("Auto-discovery mode enabled, ignoring static peers")
	} else if peersEnv != "" {
		// Use static peer configuration
		peers = strings.Split(peersEnv, ",")
		log.Printf("Using static peers: %v", peers)
	}

	// Create database path
	dbPath := fmt.Sprintf("./data/%s", nodeIDEnv)

	log.Printf("🚀 Starting Validator Node %s", nodeIDEnv)
	log.Printf("   Leader: %v", isLeader)
	log.Printf("   Port: %s", portEnv)
	log.Printf("   Database: %s", dbPath)
	log.Printf("   Registry URL: %s", registryURL)
	log.Printf("   Auto-discover: %v", autoDiscover)

	// Create validator node
	node, err := NewValidatorNode(nodeIDEnv, "0.0.0.0", portEnv, isLeader, dbPath)
	if err != nil {
		log.Fatalf("Failed to create validator node: %v", err)
	}

	// Start the node
	err = node.Start()
	if err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}

	// Start HTTP server for all nodes (for leadership notifications)
	go node.startHTTPServer()

	// Setup auto-discovery if enabled
	if autoDiscover && registryURL != "" {
		err = node.setupAutoDiscovery(registryURL)
		if err != nil {
			log.Printf("Warning: Failed to setup auto-discovery: %v", err)
		}
		
		// If this was originally a leader node, request leadership from registry
		if isLeaderStr == "true" {
			log.Printf("Node was originally configured as leader, requesting leadership from registry...")
			go node.requestLeadershipFromRegistry(registryURL)
		}
	} else {
		// Setup leadership endpoint even for static nodes
		node.setupLeadershipEndpoint()
	}

	// Connect to peers
	if len(peers) > 0 {
		err = node.ConnectToPeers(peers)
		if err != nil {
			log.Printf("Warning: Failed to connect to some peers: %v", err)
		}
	}

	// Sync with network if auto-discovery is enabled
	if autoDiscover {
		go node.performAutoSync()
	} else {
		// Traditional sync for static configuration
		err = node.SyncWithPeers()
		if err != nil {
			log.Printf("Warning: Failed to sync with peers: %v", err)
		}
	}

	// Start consensus if leader
	if isLeader {
		node.StartConsensusLoop()
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Node %s is running. Press Ctrl+C to stop.", nodeIDEnv)
	<-sigChan

	log.Printf("Shutting down node %s...", nodeIDEnv)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
