package p2p

import (
	"context"
	"fmt"
	"log"

	"golang-blockchain/pkg/blockchain"
	pb "golang-blockchain/proto"
)

// SubmitTransaction xử lý transaction mới
func (n *Node) SubmitTransaction(ctx context.Context, req *pb.SubmitTransactionRequest) (*pb.SubmitTransactionResponse, error) {
	log.Printf("Node %s received transaction submission", n.ID)

	// Convert proto transaction to internal transaction
	tx := &blockchain.Transaction{
		Sender:    req.Transaction.Sender,
		Receiver:  req.Transaction.Receiver,
		Amount:    req.Transaction.Amount,
		Timestamp: req.Transaction.Timestamp,
		Signature: req.Transaction.Signature,
	}

	// Validate transaction
	if tx.Amount <= 0 {
		return &pb.SubmitTransactionResponse{
			Success: false,
			Message: "Invalid transaction amount",
		}, nil
	}

	if len(tx.Signature) == 0 {
		return &pb.SubmitTransactionResponse{
			Success: false,
			Message: "Transaction not signed",
		}, nil
	}

	// Thêm vào transaction pool
	err := n.txPool.AddTransaction(tx)
	if err != nil {
		return &pb.SubmitTransactionResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to add transaction: %v", err),
		}, nil
	}

	log.Printf("Node %s added transaction to pool", n.ID)

	return &pb.SubmitTransactionResponse{
		Success: true,
		Message: "Transaction added to pool",
	}, nil
}

// ProposeBlock xử lý block proposal từ Leader
func (n *Node) ProposeBlock(ctx context.Context, req *pb.ProposeBlockRequest) (*pb.ProposeBlockResponse, error) {
	log.Printf("Node %s received block proposal from %s", n.ID, req.ProposerId)

	// Convert proto block to internal block
	block := n.protoToBlock(req.Block)

	// Validate proposed block
	err := block.ValidateBlock()
	if err != nil {
		log.Printf("Node %s rejected invalid block: %v", n.ID, err)
		return &pb.ProposeBlockResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid block: %v", err),
		}, nil
	}

	// Kiểm tra block height
	latestHeight, err := n.storage.GetLatestHeight()
	if err != nil {
		latestHeight = -1
	}

	expectedHeight := latestHeight + 1
	if block.Height != expectedHeight {
		return &pb.ProposeBlockResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid block height. Expected %d, got %d", expectedHeight, block.Height),
		}, nil
	}

	// Kiểm tra previous block hash
	if latestHeight >= 0 {
		latestBlock, err := n.storage.GetLatestBlock()
		if err != nil {
			return &pb.ProposeBlockResponse{
				Success: false,
				Message: "Failed to get latest block",
			}, nil
		}

		if string(block.PreviousBlockHash) != string(latestBlock.GetHash()) {
			return &pb.ProposeBlockResponse{
				Success: false,
				Message: "Invalid previous block hash",
			}, nil
		}
	}

	log.Printf("Node %s validated block proposal successfully", n.ID)

	// Return success (voting logic sẽ được handle riêng)
	return &pb.ProposeBlockResponse{
		Success: true,
		Message: "Block proposal accepted",
	}, nil
}

// SubmitVote xử lý votes từ các nodes
func (n *Node) SubmitVote(ctx context.Context, req *pb.VoteRequest) (*pb.VoteResponse, error) {
	if !n.IsLeader {
		return &pb.VoteResponse{
			Success: false,
			Message: "Only leader can collect votes",
		}, nil
	}

	vote := req.Vote
	log.Printf("Leader %s received vote from %s: %v", n.ID, vote.NodeId, vote.Approve)

	// Store vote
	n.votesMux.Lock()
	voteKey := fmt.Sprintf("%s_%x", vote.NodeId, vote.BlockHash)
	n.pendingVotes[voteKey] = vote
	n.votesMux.Unlock()

	return &pb.VoteResponse{
		Success: true,
		Message: "Vote recorded",
	}, nil
}

// CommitBlock xử lý commit block từ Leader
func (n *Node) CommitBlock(ctx context.Context, req *pb.CommitBlockRequest) (*pb.CommitBlockResponse, error) {
	log.Printf("Node %s received commit block request", n.ID)

	block := n.protoToBlock(req.Block)

	// Validate block một lần nữa
	err := block.ValidateBlock()
	if err != nil {
		return &pb.CommitBlockResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid block: %v", err),
		}, nil
	}

	// Lưu block vào storage
	err = n.storage.SaveBlock(block)
	if err != nil {
		return &pb.CommitBlockResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to save block: %v", err),
		}, nil
	}

	// Update current height
	n.currentHeight = block.Height

	// Remove transactions từ pool
	n.txPool.RemoveTransactions(block.Transactions)

	log.Printf("Node %s committed block at height %d", n.ID, block.Height)

	return &pb.CommitBlockResponse{
		Success: true,
		Message: "Block committed successfully",
	}, nil
}

// GetLatestBlock trả về block mới nhất
func (n *Node) GetLatestBlock(ctx context.Context, req *pb.GetLatestBlockRequest) (*pb.GetLatestBlockResponse, error) {
	latestBlock, err := n.storage.GetLatestBlock()
	if err != nil {
		return &pb.GetLatestBlockResponse{
			Success: false,
		}, nil
	}

	protoBlock := n.blockToProto(latestBlock)

	return &pb.GetLatestBlockResponse{
		Block:   protoBlock,
		Height:  int32(latestBlock.Height),
		Success: true,
	}, nil
}

// GetStatus trả về trạng thái node (MISSING METHOD - đây là nguyên nhân lỗi)
func (n *Node) GetStatus(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	// Get current height
	latestHeight, err := n.storage.GetLatestHeight()
	if err != nil {
		latestHeight = -1
	}

	// Get transaction pool size
	txPoolSize := 0
	if n.txPool != nil {
		txPoolSize = n.txPool.GetTransactionCount()
	}

	return &pb.StatusResponse{
		NodeId:        n.ID,
		IsLeader:      n.IsLeader,
		CurrentHeight: int32(latestHeight),
		TxPoolSize:    int32(txPoolSize),
	}, nil
}

// Helper method: Convert proto transaction to internal transaction
func (n *Node) protoToTransaction(protoTx *pb.Transaction) *blockchain.Transaction {
	return &blockchain.Transaction{
		Sender:    protoTx.Sender,
		Receiver:  protoTx.Receiver,
		Amount:    protoTx.Amount,
		Timestamp: protoTx.Timestamp,
		Signature: protoTx.Signature,
	}
}

// Helper method: Convert internal transaction to proto
func (n *Node) transactionToProto(tx *blockchain.Transaction) *pb.Transaction {
	return &pb.Transaction{
		Sender:    tx.Sender,
		Receiver:  tx.Receiver,
		Amount:    tx.Amount,
		Timestamp: tx.Timestamp,
		Signature: tx.Signature,
	}
}
