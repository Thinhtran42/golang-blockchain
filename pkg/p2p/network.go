package p2p

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"golang-blockchain/pkg/blockchain"
	pb "golang-blockchain/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Node đại diện cho một validator node
type Node struct {
	ID       string
	Address  string
	Port     string
	IsLeader bool

	// gRPC server và clients
	server     *grpc.Server
	clients    map[string]pb.BlockchainServiceClient
	clientsMux sync.RWMutex

	// Blockchain components
	storage BlockStorage
	txPool  TransactionPool // Fix: Remove pointer

	// Consensus state
	currentHeight int
	isProposing   bool
	pendingVotes  map[string]*pb.Vote
	votesMux      sync.RWMutex

	// Peers
	peers    []string
	peersMux sync.RWMutex

	pb.UnimplementedBlockchainServiceServer
}

// BlockStorage interface để abstract storage layer
type BlockStorage interface {
	SaveBlock(block *blockchain.Block) error
	GetBlock(hash []byte) (*blockchain.Block, error)
	GetBlockByHeight(height int) (*blockchain.Block, error)
	GetLatestHeight() (int, error)
	GetLatestBlock() (*blockchain.Block, error)
	GetBlocksFromHeight(startHeight int) ([]*blockchain.Block, error)
}

// TransactionPool interface
type TransactionPool interface {
	AddTransaction(tx *blockchain.Transaction) error
	GetTransactions() []*blockchain.Transaction
	RemoveTransactions(txs []*blockchain.Transaction)
	Clear()
	GetTransactionCount() int // Add this method
}

// NewNode tạo node mới
func NewNode(id, address, port string, isLeader bool, storage BlockStorage, txPool TransactionPool) *Node {
	return &Node{
		ID:           id,
		Address:      address,
		Port:         port,
		IsLeader:     isLeader,
		clients:      make(map[string]pb.BlockchainServiceClient),
		storage:      storage,
		txPool:       txPool, // Fix: Remove pointer
		pendingVotes: make(map[string]*pb.Vote),
		peers:        make([]string, 0),
	}
}

// Start khởi động node
func (n *Node) Start() error {
	// Khởi động gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", n.Port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	n.server = grpc.NewServer()
	pb.RegisterBlockchainServiceServer(n.server, n)

	log.Printf("Node %s starting on %s:%s (Leader: %v)", n.ID, n.Address, n.Port, n.IsLeader)

	go func() {
		if err := n.server.Serve(lis); err != nil {
			log.Printf("Failed to serve: %v", err)
		}
	}()

	// Get current height
	height, err := n.storage.GetLatestHeight()
	if err != nil {
		n.currentHeight = -1
	} else {
		n.currentHeight = height
	}

	return nil
}

// AddPeer thêm peer vào danh sách
func (n *Node) AddPeer(peerAddress string) error {
	n.peersMux.Lock()
	defer n.peersMux.Unlock()

	// Kiểm tra duplicate
	for _, peer := range n.peers {
		if peer == peerAddress {
			return nil // Đã tồn tại
		}
	}

	n.peers = append(n.peers, peerAddress)

	// Tạo connection đến peer
	conn, err := grpc.Dial(peerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to peer %s: %w", peerAddress, err)
	}

	client := pb.NewBlockchainServiceClient(conn)

	n.clientsMux.Lock()
	n.clients[peerAddress] = client
	n.clientsMux.Unlock()

	log.Printf("Node %s connected to peer %s", n.ID, peerAddress)
	return nil
}

// GetPeers trả về danh sách peers
func (n *Node) GetPeers() []string {
	n.peersMux.RLock()
	defer n.peersMux.RUnlock()

	peers := make([]string, len(n.peers))
	copy(peers, n.peers)
	return peers
}

// BroadcastTransaction phát tán transaction đến tất cả peers
func (n *Node) BroadcastTransaction(tx *blockchain.Transaction) error {
	protoTx := &pb.Transaction{
		Sender:    tx.Sender,
		Receiver:  tx.Receiver,
		Amount:    tx.Amount,
		Timestamp: tx.Timestamp,
		Signature: tx.Signature,
	}

	req := &pb.SubmitTransactionRequest{
		Transaction: protoTx,
	}

	n.clientsMux.RLock()
	clients := make(map[string]pb.BlockchainServiceClient)
	for addr, client := range n.clients {
		clients[addr] = client
	}
	n.clientsMux.RUnlock()

	var wg sync.WaitGroup
	for peerAddr, client := range clients {
		wg.Add(1)
		go func(addr string, c pb.BlockchainServiceClient) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := c.SubmitTransaction(ctx, req)
			if err != nil {
				log.Printf("Failed to broadcast transaction to %s: %v", addr, err)
			}
		}(peerAddr, client)
	}

	wg.Wait()
	return nil
}

// ProposeBlock đề xuất block mới (chỉ Leader)
func (n *Node) ProposeBlockToNetwork(block *blockchain.Block) error {
	if !n.IsLeader {
		return fmt.Errorf("only leader can propose blocks")
	}

	n.isProposing = true
	defer func() { n.isProposing = false }()

	log.Printf("Leader %s proposing block at height %d", n.ID, block.Height)

	// Convert block to proto
	protoBlock := n.blockToProto(block)
	req := &pb.ProposeBlockRequest{
		Block:      protoBlock,
		ProposerId: n.ID,
	}

	// Gửi propose request đến tất cả peers
	n.clientsMux.RLock()
	clients := make(map[string]pb.BlockchainServiceClient)
	for addr, client := range n.clients {
		clients[addr] = client
	}
	n.clientsMux.RUnlock()

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
	majority := (totalPeers / 2) + 1

	if successCount >= majority {
		log.Printf("Block approved by majority (%d/%d), committing...", successCount, totalPeers)
		return n.commitBlockToNetwork(block)
	} else {
		log.Printf("Block rejected by majority (%d/%d)", successCount, totalPeers)
		return fmt.Errorf("block rejected by majority")
	}
}

// commitBlockToNetwork broadcast commit to all peers
func (n *Node) commitBlockToNetwork(block *blockchain.Block) error {
	// First commit locally
	err := n.storage.SaveBlock(block)
	if err != nil {
		return fmt.Errorf("failed to commit block locally: %w", err)
	}

	// Remove transactions from pool
	n.txPool.RemoveTransactions(block.Transactions)
	n.currentHeight = block.Height

	// Broadcast commit to peers
	protoBlock := n.blockToProto(block)
	req := &pb.CommitBlockRequest{
		Block: protoBlock,
	}

	n.clientsMux.RLock()
	clients := make(map[string]pb.BlockchainServiceClient)
	for addr, client := range n.clients {
		clients[addr] = client
	}
	n.clientsMux.RUnlock()

	var wg sync.WaitGroup
	for peerAddr, client := range clients {
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
	log.Printf("Leader %s committed block at height %d", n.ID, block.Height)
	return nil
}

// Helper functions
func (n *Node) blockToProto(block *blockchain.Block) *pb.Block {
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

func (n *Node) protoToBlock(protoBlock *pb.Block) *blockchain.Block {
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

// Stop dừng node
func (n *Node) Stop() {
	if n.server != nil {
		n.server.GracefulStop()
	}

	// Close all client connections
	n.clientsMux.Lock()
	for _, client := range n.clients {
		// Note: gRPC client connections will be closed when server stops
		_ = client
	}
	n.clientsMux.Unlock()
}
