package blockchain

import (
	"fmt"
	"sync"
)

// TransactionPool quản lý pending transactions
type TransactionPool struct {
	transactions map[string]*Transaction // key: transaction hash
	txMux        sync.RWMutex
	maxSize      int
}

// NewTransactionPool tạo transaction pool mới
func NewTransactionPool(maxSize int) *TransactionPool {
	return &TransactionPool{
		transactions: make(map[string]*Transaction),
		maxSize:      maxSize,
	}
}

// AddTransaction thêm transaction vào pool
func (tp *TransactionPool) AddTransaction(tx *Transaction) error {
	tp.txMux.Lock()
	defer tp.txMux.Unlock()

	// Kiểm tra pool size
	if len(tp.transactions) >= tp.maxSize {
		return fmt.Errorf("transaction pool is full")
	}

	// Validate transaction
	if err := tp.validateTransaction(tx); err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}

	// Tạo hash key
	hashKey := string(tx.Hash())

	// Kiểm tra duplicate
	if _, exists := tp.transactions[hashKey]; exists {
		return fmt.Errorf("transaction already exists in pool")
	}

	// Thêm vào pool
	tp.transactions[hashKey] = tx

	return nil
}

// GetTransactions trả về tất cả transactions trong pool
func (tp *TransactionPool) GetTransactions() []*Transaction {
	tp.txMux.RLock()
	defer tp.txMux.RUnlock()

	transactions := make([]*Transaction, 0, len(tp.transactions))
	for _, tx := range tp.transactions {
		transactions = append(transactions, tx)
	}

	return transactions
}

// RemoveTransactions xóa transactions khỏi pool
func (tp *TransactionPool) RemoveTransactions(txs []*Transaction) {
	tp.txMux.Lock()
	defer tp.txMux.Unlock()

	for _, tx := range txs {
		hashKey := string(tx.Hash())
		delete(tp.transactions, hashKey)
	}
}

// RemoveTransaction xóa một transaction
func (tp *TransactionPool) RemoveTransaction(tx *Transaction) {
	tp.txMux.Lock()
	defer tp.txMux.Unlock()

	hashKey := string(tx.Hash())
	delete(tp.transactions, hashKey)
}

// Clear xóa tất cả transactions
func (tp *TransactionPool) Clear() {
	tp.txMux.Lock()
	defer tp.txMux.Unlock()

	tp.transactions = make(map[string]*Transaction)
}

// GetTransactionCount trả về số lượng transactions
func (tp *TransactionPool) GetTransactionCount() int {
	tp.txMux.RLock()
	defer tp.txMux.RUnlock()

	return len(tp.transactions)
}

// HasTransaction kiểm tra transaction có trong pool không
func (tp *TransactionPool) HasTransaction(tx *Transaction) bool {
	tp.txMux.RLock()
	defer tp.txMux.RUnlock()

	hashKey := string(tx.Hash())
	_, exists := tp.transactions[hashKey]
	return exists
}

// GetTransactionByHash lấy transaction bằng hash
func (tp *TransactionPool) GetTransactionByHash(hash []byte) (*Transaction, bool) {
	tp.txMux.RLock()
	defer tp.txMux.RUnlock()

	hashKey := string(hash)
	tx, exists := tp.transactions[hashKey]
	return tx, exists
}

// GetLimitedTransactions lấy số lượng transactions giới hạn (cho block creation)
func (tp *TransactionPool) GetLimitedTransactions(limit int) []*Transaction {
	tp.txMux.RLock()
	defer tp.txMux.RUnlock()

	transactions := make([]*Transaction, 0, limit)
	count := 0

	for _, tx := range tp.transactions {
		if count >= limit {
			break
		}
		transactions = append(transactions, tx)
		count++
	}

	return transactions
}

// validateTransaction validate transaction cơ bản
func (tp *TransactionPool) validateTransaction(tx *Transaction) error {
	if len(tx.Sender) == 0 {
		return fmt.Errorf("sender address is required")
	}

	if len(tx.Receiver) == 0 {
		return fmt.Errorf("receiver address is required")
	}

	if tx.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	if string(tx.Sender) == string(tx.Receiver) {
		return fmt.Errorf("sender and receiver cannot be the same")
	}

	if len(tx.Signature) == 0 {
		return fmt.Errorf("transaction must be signed")
	}

	if tx.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be positive")
	}

	return nil
}

// GetPoolStatus trả về thông tin về pool
func (tp *TransactionPool) GetPoolStatus() map[string]interface{} {
	tp.txMux.RLock()
	defer tp.txMux.RUnlock()

	return map[string]interface{}{
		"transaction_count": len(tp.transactions),
		"max_size":          tp.maxSize,
		"usage_percentage":  float64(len(tp.transactions)) / float64(tp.maxSize) * 100,
	}
}
