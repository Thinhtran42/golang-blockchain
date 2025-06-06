package consensus

import (
	"fmt"
	"log"
	"sync"
	"time"

	"golang-blockchain/pkg/blockchain"
	pb "golang-blockchain/proto"
)

// ConsensusEngine quản lý consensus process
type ConsensusEngine struct {
	node          ConsensusNode
	blockInterval time.Duration
	voteTimeout   time.Duration
	minVotes      int // Minimum votes needed (majority)

	// Consensus state
	isRunning    bool
	currentRound int
	pendingVotes map[string]*pb.Vote
	votesMux     sync.RWMutex
	roundMux     sync.Mutex
}

// ConsensusNode interface để abstract node operations
type ConsensusNode interface {
	GetID() string
	IsLeader() bool
	GetStorage() BlockStorage
	GetTxPool() TransactionPool
	GetPeers() []string
	BroadcastTransaction(tx *blockchain.Transaction) error
	ProposeBlock(block *blockchain.Block) error
	SendVote(vote *pb.Vote) error
	SendCommit(block *blockchain.Block) error
}

// BlockStorage interface (repeated for clarity)
type BlockStorage interface {
	SaveBlock(block *blockchain.Block) error
	GetLatestHeight() (int, error)
	GetLatestBlock() (*blockchain.Block, error)
}

// TransactionPool interface (repeated for clarity)
type TransactionPool interface {
	GetTransactions() []*blockchain.Transaction
	RemoveTransactions(txs []*blockchain.Transaction)
}

// NewConsensusEngine tạo consensus engine mới
func NewConsensusEngine(node ConsensusNode, totalNodes int) *ConsensusEngine {
	return &ConsensusEngine{
		node:          node,
		blockInterval: 5 * time.Second,      // Tạo block mỗi 5 giây
		voteTimeout:   3 * time.Second,      // Timeout chờ votes
		minVotes:      (totalNodes / 2) + 1, // Majority vote
		pendingVotes:  make(map[string]*pb.Vote),
	}
}

// Start khởi động consensus engine
func (ce *ConsensusEngine) Start() {
	ce.isRunning = true

	if ce.node.IsLeader() {
		log.Printf("Starting consensus engine as LEADER: %s", ce.node.GetID())
		go ce.leaderLoop()
	} else {
		log.Printf("Starting consensus engine as FOLLOWER: %s", ce.node.GetID())
		go ce.followerLoop()
	}
}

// Stop dừng consensus engine
func (ce *ConsensusEngine) Stop() {
	ce.isRunning = false
}

// leaderLoop chạy consensus loop cho Leader
func (ce *ConsensusEngine) leaderLoop() {
	ticker := time.NewTicker(ce.blockInterval)
	defer ticker.Stop()

	for ce.isRunning {
		select {
		case <-ticker.C:
			ce.proposeNewBlock()
		}
	}
}

// followerLoop chạy loop cho Follower (passive)
func (ce *ConsensusEngine) followerLoop() {
	// Followers chỉ respond to proposals từ Leader
	// Logic được handle trong gRPC service methods
	for ce.isRunning {
		time.Sleep(1 * time.Second)
		// Có thể thêm health check logic ở đây
	}
}

// proposeNewBlock tạo và propose block mới (chỉ Leader)
func (ce *ConsensusEngine) proposeNewBlock() {
	if !ce.node.IsLeader() {
		return
	}

	ce.roundMux.Lock()
	defer ce.roundMux.Unlock()

	log.Printf("Leader %s starting new consensus round %d", ce.node.GetID(), ce.currentRound)

	// Lấy pending transactions
	transactions := ce.node.GetTxPool().GetTransactions()
	if len(transactions) == 0 {
		log.Printf("No pending transactions, skipping block creation")
		return
	}

	// Lấy previous block hash
	var previousHash []byte
	height := 0

	latestHeight, err := ce.node.GetStorage().GetLatestHeight()
	if err == nil && latestHeight >= 0 {
		latestBlock, err := ce.node.GetStorage().GetLatestBlock()
		if err == nil {
			previousHash = latestBlock.GetHash()
			height = latestHeight + 1
		}
	}

	// Tạo block mới
	block := blockchain.NewBlock(transactions, previousHash, height)

	log.Printf("Leader %s created block at height %d with %d transactions",
		ce.node.GetID(), height, len(transactions))

	// Propose block to followers
	err = ce.node.ProposeBlock(block)
	if err != nil {
		log.Printf("Failed to propose block: %v", err)
		return
	}

	// Chờ votes từ followers
	votes := ce.collectVotes(block, ce.voteTimeout)

	// Kiểm tra có đủ votes không
	approveCount := ce.countApproveVotes(votes)
	totalVotes := len(votes)

	log.Printf("Leader %s received %d/%d votes (%d approvals)",
		ce.node.GetID(), totalVotes, len(ce.node.GetPeers()), approveCount)

	// Majority vote decision
	if approveCount >= ce.minVotes {
		log.Printf("Block approved by majority, committing...")
		ce.commitBlock(block)
	} else {
		log.Printf("Block rejected by majority, discarding...")
	}

	ce.currentRound++
}

// collectVotes thu thập votes từ followers
func (ce *ConsensusEngine) collectVotes(block *blockchain.Block, timeout time.Duration) []*pb.Vote {
	// Clear previous votes
	ce.votesMux.Lock()
	ce.pendingVotes = make(map[string]*pb.Vote)
	ce.votesMux.Unlock()

	// Wait for votes
	time.Sleep(timeout)

	// Collect all votes
	ce.votesMux.RLock()
	votes := make([]*pb.Vote, 0, len(ce.pendingVotes))
	for _, vote := range ce.pendingVotes {
		votes = append(votes, vote)
	}
	ce.votesMux.RUnlock()

	return votes
}

// countApproveVotes đếm số votes approve
func (ce *ConsensusEngine) countApproveVotes(votes []*pb.Vote) int {
	count := 0
	for _, vote := range votes {
		if vote.Approve {
			count++
		}
	}
	return count
}

// commitBlock commit block sau khi có consensus
func (ce *ConsensusEngine) commitBlock(block *blockchain.Block) {
	// Lưu block vào storage của Leader
	err := ce.node.GetStorage().SaveBlock(block)
	if err != nil {
		log.Printf("Leader failed to save block: %v", err)
		return
	}

	// Remove transactions từ pool
	ce.node.GetTxPool().RemoveTransactions(block.Transactions)

	// Broadcast commit to all followers
	err = ce.node.SendCommit(block)
	if err != nil {
		log.Printf("Failed to broadcast commit: %v", err)
	}

	log.Printf("Leader %s committed block at height %d", ce.node.GetID(), block.Height)
}

// ReceiveVote xử lý vote từ follower (called by gRPC service)
func (ce *ConsensusEngine) ReceiveVote(vote *pb.Vote) error {
	if !ce.node.IsLeader() {
		return fmt.Errorf("only leader can receive votes")
	}

	ce.votesMux.Lock()
	defer ce.votesMux.Unlock()

	voteKey := fmt.Sprintf("%s_%x", vote.NodeId, vote.BlockHash)
	ce.pendingVotes[voteKey] = vote

	log.Printf("Leader %s received vote from %s: %v",
		ce.node.GetID(), vote.NodeId, vote.Approve)

	return nil
}

// ValidateProposal xử lý block proposal (called by follower)
func (ce *ConsensusEngine) ValidateProposal(block *blockchain.Block, proposerID string) (*pb.Vote, error) {
	log.Printf("Follower %s validating proposal from %s", ce.node.GetID(), proposerID)

	// Validate block
	err := block.ValidateBlock()
	if err != nil {
		log.Printf("Block validation failed: %v", err)
		return &pb.Vote{
			NodeId:    ce.node.GetID(),
			BlockHash: block.GetHash(),
			Approve:   false,
			Timestamp: time.Now().Unix(),
		}, nil
	}

	// Kiểm tra height
	latestHeight, err := ce.node.GetStorage().GetLatestHeight()
	if err != nil {
		latestHeight = -1
	}

	expectedHeight := latestHeight + 1
	if block.Height != expectedHeight {
		log.Printf("Invalid block height: expected %d, got %d", expectedHeight, block.Height)
		return &pb.Vote{
			NodeId:    ce.node.GetID(),
			BlockHash: block.GetHash(),
			Approve:   false,
			Timestamp: time.Now().Unix(),
		}, nil
	}

	// Kiểm tra previous hash
	if latestHeight >= 0 {
		latestBlock, err := ce.node.GetStorage().GetLatestBlock()
		if err == nil {
			if string(block.PreviousBlockHash) != string(latestBlock.GetHash()) {
				log.Printf("Invalid previous block hash")
				return &pb.Vote{
					NodeId:    ce.node.GetID(),
					BlockHash: block.GetHash(),
					Approve:   false,
					Timestamp: time.Now().Unix(),
				}, nil
			}
		}
	}

	log.Printf("Follower %s approved proposal", ce.node.GetID())

	return &pb.Vote{
		NodeId:    ce.node.GetID(),
		BlockHash: block.GetHash(),
		Approve:   true,
		Timestamp: time.Now().Unix(),
	}, nil
}

// CommitBlock xử lý commit request từ Leader
func (ce *ConsensusEngine) CommitBlock(block *blockchain.Block) error {
	log.Printf("Follower %s committing block at height %d", ce.node.GetID(), block.Height)

	// Final validation
	err := block.ValidateBlock()
	if err != nil {
		return fmt.Errorf("final validation failed: %w", err)
	}

	// Save block
	err = ce.node.GetStorage().SaveBlock(block)
	if err != nil {
		return fmt.Errorf("failed to save block: %w", err)
	}

	// Remove transactions from pool
	ce.node.GetTxPool().RemoveTransactions(block.Transactions)

	log.Printf("Follower %s successfully committed block", ce.node.GetID())
	return nil
}

// GetConsensusStatus trả về trạng thái consensus
func (ce *ConsensusEngine) GetConsensusStatus() map[string]interface{} {
	ce.roundMux.Lock()
	defer ce.roundMux.Unlock()

	ce.votesMux.RLock()
	voteCount := len(ce.pendingVotes)
	ce.votesMux.RUnlock()

	return map[string]interface{}{
		"is_running":     ce.isRunning,
		"is_leader":      ce.node.IsLeader(),
		"current_round":  ce.currentRound,
		"pending_votes":  voteCount,
		"min_votes":      ce.minVotes,
		"block_interval": ce.blockInterval.String(),
	}
}
