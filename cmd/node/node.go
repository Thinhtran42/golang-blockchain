package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang-blockchain/pkg/blockchain"
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

func main() {
	// Parse environment variables
	nodeID := getEnvOrDefault("NODE_ID", "node1")
	isLeaderStr := getEnvOrDefault("IS_LEADER", "false")
	port := getEnvOrDefault("PORT", "50051")
	peersStr := getEnvOrDefault("PEERS", "")

	isLeader, _ := strconv.ParseBool(isLeaderStr)

	// Parse peers
	var peers []string
	if peersStr != "" {
		peers = strings.Split(peersStr, ",")
	}

	// Tạo database path
	dbPath := fmt.Sprintf("data/%s_db", nodeID)
	os.MkdirAll("data", 0755)

	log.Printf("Starting validator node %s (Leader: %v) on port %s", nodeID, isLeader, port)

	// Tạo validator node
	node, err := NewValidatorNode(nodeID, "0.0.0.0", port, isLeader, dbPath)
	if err != nil {
		log.Fatal("Failed to create validator node:", err)
	}

	// Start node
	if err := node.Start(); err != nil {
		log.Fatal("Failed to start node:", err)
	}

	log.Printf("Node %s started successfully", nodeID)

	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Connect to peers
	if len(peers) > 0 {
		log.Printf("Connecting to peers: %v", peers)
		if err := node.ConnectToPeers(peers); err != nil {
			log.Printf("Warning: Failed to connect to some peers: %v", err)
		}
	}

	// Sync with peers if not leader
	if !isLeader && len(peers) > 0 {
		time.Sleep(3 * time.Second) // Wait for connections
		if err := node.SyncWithPeers(); err != nil {
			log.Printf("Warning: Sync failed: %v", err)
		}
	}

	// Start consensus for leader
	node.StartConsensusLoop()

	log.Printf("Validator node %s is ready", nodeID)

	// Wait for shutdown signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Printf("Shutting down validator node %s", nodeID)
	node.Stop()
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
