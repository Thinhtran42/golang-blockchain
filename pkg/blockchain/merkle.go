package blockchain

import (
	"crypto/sha256"
	"fmt"
)

// MerkleNode đại diện cho một node trong Merkle Tree
type MerkleNode struct {
	Left  *MerkleNode
	Right *MerkleNode
	Hash  []byte
}

// MerkleTree đại diện cho Merkle Tree
type MerkleTree struct {
	Root *MerkleNode
}

// NewMerkleTree tạo Merkle Tree từ danh sách transactions
func NewMerkleTree(transactions []*Transaction) *MerkleTree {
	var nodes []*MerkleNode

	// Tạo leaf nodes từ transaction hashes
	for _, tx := range transactions {
		node := &MerkleNode{
			Hash: tx.Hash(),
		}
		nodes = append(nodes, node)
	}

	// Nếu số nodes lẻ, duplicate node cuối
	if len(nodes)%2 == 1 {
		nodes = append(nodes, &MerkleNode{Hash: nodes[len(nodes)-1].Hash})
	}

	// Xây dựng tree từ bottom lên top
	for len(nodes) > 1 {
		var nextLevel []*MerkleNode

		for i := 0; i < len(nodes); i += 2 {
			left := nodes[i]
			right := nodes[i+1]

			// Tạo parent node
			parent := &MerkleNode{
				Left:  left,
				Right: right,
				Hash:  hashPair(left.Hash, right.Hash),
			}

			nextLevel = append(nextLevel, parent)
		}

		// Nếu next level có số nodes lẻ, duplicate node cuối
		if len(nextLevel)%2 == 1 && len(nextLevel) > 1 {
			nextLevel = append(nextLevel, &MerkleNode{Hash: nextLevel[len(nextLevel)-1].Hash})
		}

		nodes = nextLevel
	}

	return &MerkleTree{Root: nodes[0]}
}

// hashPair hash 2 byte arrays với nhau
func hashPair(left, right []byte) []byte {
	combined := append(left, right...)
	hash := sha256.Sum256(combined)
	return hash[:]
}

// GetRootHash trả về root hash của Merkle Tree
func (mt *MerkleTree) GetRootHash() []byte {
	if mt.Root == nil {
		return nil
	}
	return mt.Root.Hash
}

// VerifyTransaction kiểm tra transaction có trong tree không
func (mt *MerkleTree) VerifyTransaction(txHash []byte) bool {
	return mt.searchHash(mt.Root, txHash)
}

// searchHash tìm kiếm hash trong tree
func (mt *MerkleTree) searchHash(node *MerkleNode, targetHash []byte) bool {
	if node == nil {
		return false
	}

	// So sánh hash
	if bytesEqual(node.Hash, targetHash) {
		return true
	}

	// Tìm kiếm trong left và right subtree
	return mt.searchHash(node.Left, targetHash) || mt.searchHash(node.Right, targetHash)
}

// bytesEqual so sánh 2 byte arrays
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

// PrintTree in cấu trúc tree (để debug)
func (mt *MerkleTree) PrintTree() {
	fmt.Println("Merkle Tree Structure:")
	mt.printNode(mt.Root, 0)
}

// printNode in node với indentation
func (mt *MerkleTree) printNode(node *MerkleNode, level int) {
	if node == nil {
		return
	}

	indent := ""
	for i := 0; i < level; i++ {
		indent += "  "
	}

	fmt.Printf("%sHash: %x\n", indent, node.Hash[:8]) // Chỉ in 8 bytes đầu

	if node.Left != nil || node.Right != nil {
		mt.printNode(node.Left, level+1)
		mt.printNode(node.Right, level+1)
	}
}
