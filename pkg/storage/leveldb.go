package storage

import (
	"encoding/json"
	"fmt"
	"golang-blockchain/pkg/blockchain"

	"github.com/syndtr/goleveldb/leveldb"
)

// BlockStorage quản lý việc lưu trữ blocks trong LevelDB
type BlockStorage struct {
	db *leveldb.DB
}

// NewBlockStorage tạo BlockStorage mới
func NewBlockStorage(dbPath string) (*BlockStorage, error) {
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open LevelDB: %w", err)
	}

	return &BlockStorage{db: db}, nil
}

// SaveBlock lưu block vào LevelDB (theo code mẫu đề bài)
func (bs *BlockStorage) SaveBlock(block *blockchain.Block) error {
	blockBytes, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}

	// Sử dụng block hash làm key
	key := block.GetHash()
	err = bs.db.Put(key, blockBytes, nil)
	if err != nil {
		return fmt.Errorf("failed to save block to LevelDB: %w", err)
	}

	// Cũng lưu mapping từ height -> hash để dễ truy cập
	heightKey := fmt.Sprintf("height_%d", block.Height)
	err = bs.db.Put([]byte(heightKey), key, nil)
	if err != nil {
		return fmt.Errorf("failed to save height mapping: %w", err)
	}

	// Update latest block height
	err = bs.db.Put([]byte("latest_height"), []byte(fmt.Sprintf("%d", block.Height)), nil)
	if err != nil {
		return fmt.Errorf("failed to update latest height: %w", err)
	}

	return nil
}

// GetBlock lấy block từ LevelDB bằng hash (theo code mẫu đề bài)
func (bs *BlockStorage) GetBlock(hash []byte) (*blockchain.Block, error) {
	blockBytes, err := bs.db.Get(hash, nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, fmt.Errorf("block not found")
		}
		return nil, fmt.Errorf("failed to get block from LevelDB: %w", err)
	}

	var block blockchain.Block
	if err := json.Unmarshal(blockBytes, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return &block, nil
}

// GetBlockByHeight lấy block bằng height
func (bs *BlockStorage) GetBlockByHeight(height int) (*blockchain.Block, error) {
	heightKey := fmt.Sprintf("height_%d", height)
	hash, err := bs.db.Get([]byte(heightKey), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, fmt.Errorf("block at height %d not found", height)
		}
		return nil, fmt.Errorf("failed to get block hash for height %d: %w", height, err)
	}

	return bs.GetBlock(hash)
}

// GetLatestHeight trả về height cao nhất
func (bs *BlockStorage) GetLatestHeight() (int, error) {
	heightBytes, err := bs.db.Get([]byte("latest_height"), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return -1, nil // Chưa có block nào
		}
		return -1, fmt.Errorf("failed to get latest height: %w", err)
	}

	var height int
	if _, err := fmt.Sscanf(string(heightBytes), "%d", &height); err != nil {
		return -1, fmt.Errorf("failed to parse latest height: %w", err)
	}

	return height, nil
}

// GetLatestBlock trả về block mới nhất
func (bs *BlockStorage) GetLatestBlock() (*blockchain.Block, error) {
	height, err := bs.GetLatestHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest height: %w", err)
	}

	if height == -1 {
		return nil, fmt.Errorf("no blocks found")
	}

	return bs.GetBlockByHeight(height)
}

// BlockExists kiểm tra block có tồn tại không
func (bs *BlockStorage) BlockExists(hash []byte) bool {
	_, err := bs.db.Get(hash, nil)
	return err == nil
}

// GetAllBlocks trả về tất cả blocks (để sync)
func (bs *BlockStorage) GetAllBlocks() ([]*blockchain.Block, error) {
	latestHeight, err := bs.GetLatestHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest height: %w", err)
	}

	if latestHeight == -1 {
		return []*blockchain.Block{}, nil
	}

	var blocks []*blockchain.Block
	for i := 0; i <= latestHeight; i++ {
		block, err := bs.GetBlockByHeight(i)
		if err != nil {
			return nil, fmt.Errorf("failed to get block at height %d: %w", i, err)
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

// Close đóng database
func (bs *BlockStorage) Close() error {
	return bs.db.Close()
}

// GetBlocksFromHeight lấy tất cả blocks từ height cụ thể (cho sync)
func (bs *BlockStorage) GetBlocksFromHeight(startHeight int) ([]*blockchain.Block, error) {
	latestHeight, err := bs.GetLatestHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest height: %w", err)
	}

	if latestHeight == -1 || startHeight > latestHeight {
		return []*blockchain.Block{}, nil
	}

	var blocks []*blockchain.Block
	for i := startHeight; i <= latestHeight; i++ {
		block, err := bs.GetBlockByHeight(i)
		if err != nil {
			return nil, fmt.Errorf("failed to get block at height %d: %w", i, err)
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}
