package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"golang-blockchain/pkg/blockchain"
	"golang-blockchain/pkg/storage"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Database Viewer - Commands:")
		fmt.Println("  list [db_path]                    - List all blocks")
		fmt.Println("  block [db_path] [height]          - View specific block")
		fmt.Println("  stats [db_path]                   - Database statistics")
		fmt.Println("  export [db_path] [output.json]    - Export all blocks to JSON")
		fmt.Println("  latest [db_path]                  - View latest block")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  go run cmd/db-viewer/main.go list data/leader_db")
		fmt.Println("  go run cmd/db-viewer/main.go block data/leader_db 0")
		fmt.Println("  go run cmd/db-viewer/main.go stats data/leader_db")
		return
	}

	command := os.Args[1]

	switch command {
	case "list":
		if len(os.Args) != 3 {
			log.Fatal("Usage: list [db_path]")
		}
		listBlocks(os.Args[2])

	case "block":
		if len(os.Args) != 4 {
			log.Fatal("Usage: block [db_path] [height]")
		}
		height, err := strconv.Atoi(os.Args[3])
		if err != nil {
			log.Fatal("Invalid height:", err)
		}
		viewBlock(os.Args[2], height)

	case "stats":
		if len(os.Args) != 3 {
			log.Fatal("Usage: stats [db_path]")
		}
		showStats(os.Args[2])

	case "export":
		if len(os.Args) != 4 {
			log.Fatal("Usage: export [db_path] [output.json]")
		}
		exportBlocks(os.Args[2], os.Args[3])

	case "latest":
		if len(os.Args) != 3 {
			log.Fatal("Usage: latest [db_path]")
		}
		viewLatest(os.Args[2])

	default:
		fmt.Printf("Unknown command: %s\n", command)
	}
}

func listBlocks(dbPath string) {
	fmt.Printf("📂 Listing blocks in: %s\n", dbPath)

	storage, err := storage.NewBlockStorage(dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer storage.Close()

	latestHeight, err := storage.GetLatestHeight()
	if err != nil {
		fmt.Println("❌ No blocks found in database")
		return
	}

	fmt.Printf("📊 Total blocks: %d (heights 0 to %d)\n\n", latestHeight+1, latestHeight)

	for height := 0; height <= latestHeight; height++ {
		block, err := storage.GetBlockByHeight(height)
		if err != nil {
			fmt.Printf("❌ Error reading block %d: %v\n", height, err)
			continue
		}

		fmt.Printf("📦 Block %d:\n", height)
		fmt.Printf("   Hash: %x\n", block.GetHash()[:16])
		fmt.Printf("   Transactions: %d\n", len(block.Transactions))
		fmt.Printf("   Timestamp: %s\n", time.Unix(block.Timestamp, 0).Format("2006-01-02 15:04:05"))

		if len(block.Transactions) > 0 {
			totalAmount := 0.0
			for _, tx := range block.Transactions {
				totalAmount += tx.Amount
			}
			fmt.Printf("   Total Amount: %.2f\n", totalAmount)
		}
		fmt.Println()
	}
}

func viewBlock(dbPath string, height int) {
	fmt.Printf("🔍 Viewing block %d in: %s\n", height, dbPath)

	storage, err := storage.NewBlockStorage(dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer storage.Close()

	block, err := storage.GetBlockByHeight(height)
	if err != nil {
		log.Fatal("Failed to get block:", err)
	}

	fmt.Printf("\n📦 Block %d Details:\n", height)
	fmt.Printf("─────────────────────────────────────\n")
	fmt.Printf("Hash:           %x\n", block.GetHash())
	fmt.Printf("Previous Hash:  %x\n", block.PreviousBlockHash)
	fmt.Printf("Merkle Root:    %x\n", block.MerkleRoot)
	fmt.Printf("Timestamp:      %s\n", time.Unix(block.Timestamp, 0).Format("2006-01-02 15:04:05"))
	fmt.Printf("Height:         %d\n", block.Height)
	fmt.Printf("Transactions:   %d\n", len(block.Transactions))

	if len(block.Transactions) > 0 {
		fmt.Printf("\n💸 Transactions:\n")
		totalAmount := 0.0
		for i, tx := range block.Transactions {
			fmt.Printf("  [%d] Transaction:\n", i+1)
			fmt.Printf("      From:      %x\n", tx.Sender)
			fmt.Printf("      To:        %x\n", tx.Receiver)
			fmt.Printf("      Amount:    %.2f\n", tx.Amount)
			fmt.Printf("      Time:      %s\n", time.Unix(tx.Timestamp, 0).Format("2006-01-02 15:04:05"))
			fmt.Printf("      Signature: %x\n", tx.Signature[:32])
			fmt.Printf("      TX Hash:   %x\n", tx.Hash())
			totalAmount += tx.Amount
			fmt.Println()
		}
		fmt.Printf("📊 Total Amount Transferred: %.2f\n", totalAmount)
	}
}

func showStats(dbPath string) {
	fmt.Printf("📊 Database Statistics: %s\n", dbPath)

	storage, err := storage.NewBlockStorage(dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer storage.Close()

	latestHeight, err := storage.GetLatestHeight()
	if err != nil {
		fmt.Println("❌ No blocks found in database")
		return
	}

	totalBlocks := latestHeight + 1
	totalTransactions := 0
	totalAmount := 0.0
	var firstBlockTime, lastBlockTime int64

	// Analyze all blocks
	for height := 0; height <= latestHeight; height++ {
		block, err := storage.GetBlockByHeight(height)
		if err != nil {
			continue
		}

		totalTransactions += len(block.Transactions)

		for _, tx := range block.Transactions {
			totalAmount += tx.Amount
		}

		if height == 0 {
			firstBlockTime = block.Timestamp
		}
		if height == latestHeight {
			lastBlockTime = block.Timestamp
		}
	}

	fmt.Printf("─────────────────────────────────────\n")
	fmt.Printf("Total Blocks:       %d\n", totalBlocks)
	fmt.Printf("Total Transactions: %d\n", totalTransactions)
	fmt.Printf("Total Amount:       %.2f\n", totalAmount)
	fmt.Printf("First Block:        %s\n", time.Unix(firstBlockTime, 0).Format("2006-01-02 15:04:05"))
	fmt.Printf("Latest Block:       %s\n", time.Unix(lastBlockTime, 0).Format("2006-01-02 15:04:05"))

	if totalBlocks > 1 {
		avgTxPerBlock := float64(totalTransactions) / float64(totalBlocks)
		timeDiff := lastBlockTime - firstBlockTime
		fmt.Printf("Avg TX per Block:   %.2f\n", avgTxPerBlock)
		if timeDiff > 0 {
			fmt.Printf("Time Span:          %d seconds\n", timeDiff)
			fmt.Printf("Avg Block Time:     %.2f seconds\n", float64(timeDiff)/float64(totalBlocks-1))
		}
	}
}

func exportBlocks(dbPath, outputFile string) {
	fmt.Printf("📤 Exporting blocks from %s to %s\n", dbPath, outputFile)

	storage, err := storage.NewBlockStorage(dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer storage.Close()

	latestHeight, err := storage.GetLatestHeight()
	if err != nil {
		fmt.Println("❌ No blocks found in database")
		return
	}

	type ExportBlock struct {
		Height           int                      `json:"height"`
		Hash             string                   `json:"hash"`
		PreviousHash     string                   `json:"previous_hash"`
		MerkleRoot       string                   `json:"merkle_root"`
		Timestamp        int64                    `json:"timestamp"`
		TimestampHuman   string                   `json:"timestamp_human"`
		Transactions     []blockchain.Transaction `json:"transactions"`
		TransactionCount int                      `json:"transaction_count"`
	}

	var exportData []ExportBlock

	for height := 0; height <= latestHeight; height++ {
		block, err := storage.GetBlockByHeight(height)
		if err != nil {
			fmt.Printf("❌ Error reading block %d: %v\n", height, err)
			continue
		}

		// Convert []*blockchain.Transaction to []blockchain.Transaction
		transactions := make([]blockchain.Transaction, len(block.Transactions))
		for i, tx := range block.Transactions {
			transactions[i] = *tx
		}

		exportBlock := ExportBlock{
			Height:           height,
			Hash:             fmt.Sprintf("%x", block.GetHash()),
			PreviousHash:     fmt.Sprintf("%x", block.PreviousBlockHash),
			MerkleRoot:       fmt.Sprintf("%x", block.MerkleRoot),
			Timestamp:        block.Timestamp,
			TimestampHuman:   time.Unix(block.Timestamp, 0).Format("2006-01-02 15:04:05"),
			Transactions:     transactions,
			TransactionCount: len(block.Transactions),
		}

		exportData = append(exportData, exportBlock)
	}

	// Write to JSON file
	file, err := os.Create(outputFile)
	if err != nil {
		log.Fatal("Failed to create output file:", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(exportData)
	if err != nil {
		log.Fatal("Failed to write JSON:", err)
	}

	fmt.Printf("✅ Exported %d blocks to %s\n", len(exportData), outputFile)
}

func viewLatest(dbPath string) {
	fmt.Printf("🔍 Viewing latest block in: %s\n", dbPath)

	storage, err := storage.NewBlockStorage(dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer storage.Close()

	latestHeight, err := storage.GetLatestHeight()
	if err != nil {
		fmt.Println("❌ No blocks found in database")
		return
	}

	viewBlock(dbPath, latestHeight)
}
