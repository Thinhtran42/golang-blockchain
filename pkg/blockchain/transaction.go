package blockchain

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
)

// Transaction struct theo đề bài
type Transaction struct {
	Sender    []byte  `json:"sender"`    // Địa chỉ công khai của người gửi
	Receiver  []byte  `json:"receiver"`  // Địa chỉ công khai của người nhận
	Amount    float64 `json:"amount"`    // Số lượng tiền được chuyển
	Timestamp int64   `json:"timestamp"` // Thời điểm tạo giao dịch
	Signature []byte  `json:"signature"` // Chữ ký điện tử của giao dịch
}

// Hash tính hash của transaction (theo code mẫu đề bài)
func (t *Transaction) Hash() []byte {
	// Create a hashable representation of the transaction
	txCopy := *t
	txCopy.Signature = nil // Exclude signature from hash
	data, _ := json.Marshal(txCopy)
	hash := sha256.Sum256(data)
	return hash[:]
}

// SignTransaction ký transaction (theo code mẫu đề bài)
func SignTransaction(tx *Transaction, privKey *ecdsa.PrivateKey) error {
	txHash := tx.Hash()
	r, s, err := ecdsa.Sign(rand.Reader, privKey, txHash)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}
	// Store R and S as a concatenated byte slice
	tx.Signature = append(r.Bytes(), s.Bytes()...)
	return nil
}

// VerifyTransaction xác thực chữ ký (theo code mẫu đề bài)
func VerifyTransaction(tx *Transaction, pubKey *ecdsa.PublicKey) bool {
	txHash := tx.Hash()
	// Assume signature is r and s concatenated, parse them back to big.Int
	if len(tx.Signature) == 0 || len(tx.Signature)%2 != 0 {
		return false
	}

	mid := len(tx.Signature) / 2
	r := new(big.Int).SetBytes(tx.Signature[:mid])
	s := new(big.Int).SetBytes(tx.Signature[mid:])
	return ecdsa.Verify(pubKey, txHash, r, s)
}
