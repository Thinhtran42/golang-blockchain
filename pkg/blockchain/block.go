package blockchain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// Block struct theo đề bài
type Block struct {
	Transactions      []*Transaction `json:"transactions"`        // Danh sách giao dịch
	MerkleRoot        []byte         `json:"merkle_root"`         // Root của Merkle Tree
	PreviousBlockHash []byte         `json:"previous_block_hash"` // Hash của block trước
	CurrentBlockHash  []byte         `json:"current_block_hash"`  // Hash của block hiện tại
	Timestamp         int64          `json:"timestamp"`           // Thời gian tạo block
	Height            int            `json:"height"`              // Chiều cao của block trong chain
}

// NewBlock tạo block mới
func NewBlock(transactions []*Transaction, previousBlockHash []byte, height int) *Block {
	block := &Block{
		Transactions:      transactions,
		PreviousBlockHash: previousBlockHash,
		Timestamp:         time.Now().Unix(),
		Height:            height,
	}

	// Tính Merkle Root
	merkleTree := NewMerkleTree(transactions)
	block.MerkleRoot = merkleTree.GetRootHash()

	// Tính hash của block hiện tại
	block.CurrentBlockHash = block.calculateHash()

	return block
}

// calculateHash tính hash của block
func (b *Block) calculateHash() []byte {
	// Tạo copy block để hash (không bao gồm CurrentBlockHash)
	blockCopy := &Block{
		Transactions:      b.Transactions,
		MerkleRoot:        b.MerkleRoot,
		PreviousBlockHash: b.PreviousBlockHash,
		Timestamp:         b.Timestamp,
		Height:            b.Height,
		// CurrentBlockHash không được include trong hash calculation
	}

	data, _ := json.Marshal(blockCopy)
	hash := sha256.Sum256(data)
	return hash[:]
}

// GetHash trả về hash của block
func (b *Block) GetHash() []byte {
	return b.CurrentBlockHash
}

// ValidateBlock kiểm tra tính hợp lệ của block
func (b *Block) ValidateBlock() error {
	// Kiểm tra Merkle Root
	merkleTree := NewMerkleTree(b.Transactions)
	expectedMerkleRoot := merkleTree.GetRootHash()

	if !bytesEqual(b.MerkleRoot, expectedMerkleRoot) {
		return fmt.Errorf("invalid Merkle root")
	}

	// Kiểm tra hash của block
	expectedHash := b.calculateHash()
	if !bytesEqual(b.CurrentBlockHash, expectedHash) {
		return fmt.Errorf("invalid block hash")
	}

	// Kiểm tra tất cả transactions trong block
	for i, tx := range b.Transactions {
		if tx.Signature == nil || len(tx.Signature) == 0 {
			return fmt.Errorf("transaction %d is not signed", i)
		}

		if tx.Amount <= 0 {
			return fmt.Errorf("transaction %d has invalid amount", i)
		}

		if bytesEqual(tx.Sender, tx.Receiver) {
			return fmt.Errorf("transaction %d: sender cannot be receiver", i)
		}
	}

	return nil
}

// AddTransaction thêm transaction vào block (chỉ dùng khi tạo block)
func (b *Block) AddTransaction(tx *Transaction) {
	b.Transactions = append(b.Transactions, tx)

	// Recalculate Merkle Root và Block Hash
	merkleTree := NewMerkleTree(b.Transactions)
	b.MerkleRoot = merkleTree.GetRootHash()
	b.CurrentBlockHash = b.calculateHash()
}

// GetTransactionCount trả về số lượng transactions trong block
func (b *Block) GetTransactionCount() int {
	return len(b.Transactions)
}

// HasTransaction kiểm tra block có chứa transaction không
func (b *Block) HasTransaction(txHash []byte) bool {
	merkleTree := NewMerkleTree(b.Transactions)
	return merkleTree.VerifyTransaction(txHash)
}

// ToJSON convert block sang JSON string
func (b *Block) ToJSON() (string, error) {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal block: %w", err)
	}
	return string(data), nil
}

// FromJSON tạo block từ JSON string
func BlockFromJSON(jsonStr string) (*Block, error) {
	var block Block
	if err := json.Unmarshal([]byte(jsonStr), &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}
	return &block, nil
}
