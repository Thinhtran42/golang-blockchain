package blockchain

import (
	"fmt"

	"github.com/cbergoon/merkletree"
)

// TransactionContent implements the Content interface required by cbergoon/merkletree
type TransactionContent struct {
	tx *Transaction
}

// CalculateHash implements merkletree.Content interface
func (tc TransactionContent) CalculateHash() ([]byte, error) {
	return tc.tx.Hash(), nil
}

// Equals implements merkletree.Content interface
func (tc TransactionContent) Equals(other merkletree.Content) (bool, error) {
	otherTc, ok := other.(TransactionContent)
	if !ok {
		return false, nil
	}
	return bytesEqual(tc.tx.Hash(), otherTc.tx.Hash()), nil
}

// MerkleTree wrapper struct to maintain compatibility with existing code
type MerkleTree struct {
	tree         *merkletree.MerkleTree
	transactions []*Transaction
}

// NewMerkleTree creates a new Merkle Tree using cbergoon/merkletree library
// Maintains the same API interface as the original implementation
func NewMerkleTree(transactions []*Transaction) *MerkleTree {
	// Handle empty transactions
	if len(transactions) == 0 {
		// Create a dummy transaction for empty case
		return &MerkleTree{
			tree:         nil,
			transactions: transactions,
		}
	}

	// Convert transactions to Content interface
	var contents []merkletree.Content
	for _, tx := range transactions {
		contents = append(contents, TransactionContent{tx: tx})
	}

	// Create the merkle tree
	tree, err := merkletree.NewTree(contents)
	if err != nil {
		// Fallback to nil tree if creation fails
		return &MerkleTree{
			tree:         nil,
			transactions: transactions,
		}
	}

	return &MerkleTree{
		tree:         tree,
		transactions: transactions,
	}
}

// GetRootHash returns the root hash of the Merkle Tree
func (mt *MerkleTree) GetRootHash() []byte {
	if mt.tree == nil {
		// Return empty hash for nil tree
		return make([]byte, 32)
	}
	return mt.tree.MerkleRoot()
}

// VerifyTransaction checks if a transaction exists in the tree
func (mt *MerkleTree) VerifyTransaction(txHash []byte) bool {
	if mt.tree == nil {
		return false
	}

	// Find the transaction with matching hash
	for _, tx := range mt.transactions {
		if bytesEqual(tx.Hash(), txHash) {
			content := TransactionContent{tx: tx}
			verified, err := mt.tree.VerifyContent(content)
			return err == nil && verified
		}
	}
	return false
}

// PrintTree prints the tree structure (for debugging)
func (mt *MerkleTree) PrintTree() {
	if mt.tree == nil {
		fmt.Println("Merkle Tree: nil")
		return
	}

	fmt.Println("Merkle Tree Structure:")
	fmt.Printf("Root Hash: %x\n", mt.GetRootHash()[:8])
	fmt.Printf("Number of transactions: %d\n", len(mt.transactions))

	// Print some transaction hashes
	for i, tx := range mt.transactions {
		if i >= 3 { // Show max 3 transactions
			fmt.Printf("... and %d more transactions\n", len(mt.transactions)-3)
			break
		}
		fmt.Printf("TX%d Hash: %x\n", i+1, tx.Hash()[:8])
	}
}

// bytesEqual compares two byte arrays for equality
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
