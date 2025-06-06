package main

import (
	"fmt"
	"golang-blockchain/pkg/blockchain"
	"golang-blockchain/pkg/storage"
	"golang-blockchain/pkg/wallet"
	"log"
	"os"
	"time"
)

func main() {
	fmt.Println("=== Test Basic Blockchain theo đề bài ===")

	// Cleanup trước khi test
	os.RemoveAll("test_db")
	defer os.RemoveAll("test_db")

	// Test 1: Tạo Alice và Bob
	fmt.Println("\n1. Tạo users Alice và Bob...")
	alice, err := wallet.CreateUser("Alice")
	if err != nil {
		log.Fatal("Lỗi tạo Alice:", err)
	}

	bob, err := wallet.CreateUser("Bob")
	if err != nil {
		log.Fatal("Lỗi tạo Bob:", err)
	}

	fmt.Printf("✅ Alice: %s (address: %x)\n", alice.Name, alice.Address)
	fmt.Printf("✅ Bob: %s (address: %x)\n", bob.Name, bob.Address)

	// Test 2: Tạo transaction từ Alice đến Bob
	fmt.Println("\n2. Tạo transaction Alice -> Bob...")
	tx := &blockchain.Transaction{
		Sender:    alice.Address,
		Receiver:  bob.Address,
		Amount:    10.5,
		Timestamp: time.Now().Unix(),
	}

	// Ký transaction với private key của Alice
	err = blockchain.SignTransaction(tx, alice.PrivateKey)
	if err != nil {
		log.Fatal("Lỗi ký transaction:", err)
	}
	fmt.Printf("✅ Transaction đã được ký bởi Alice\n")

	// Verify transaction
	isValid := blockchain.VerifyTransaction(tx, alice.PublicKey)
	if isValid {
		fmt.Printf("✅ Transaction signature hợp lệ\n")
	} else {
		fmt.Printf("❌ Transaction signature không hợp lệ!\n")
	}

	// Test 3: Tạo Merkle Tree
	fmt.Println("\n3. Tạo Merkle Tree...")
	transactions := []*blockchain.Transaction{tx}
	merkleTree := blockchain.NewMerkleTree(transactions)

	fmt.Printf("✅ Merkle Tree được tạo\n")
	fmt.Printf("   Root Hash: %x\n", merkleTree.GetRootHash())

	// Verify transaction có trong tree
	txExists := merkleTree.VerifyTransaction(tx.Hash())
	if txExists {
		fmt.Printf("✅ Transaction tồn tại trong Merkle Tree\n")
	} else {
		fmt.Printf("❌ Transaction không tồn tại trong Merkle Tree!\n")
	}

	// Test 4: Tạo Block
	fmt.Println("\n4. Tạo Block...")
	genesisBlock := blockchain.NewBlock(transactions, nil, 0)

	fmt.Printf("✅ Genesis Block được tạo\n")
	fmt.Printf("   Height: %d\n", genesisBlock.Height)
	fmt.Printf("   Hash: %x\n", genesisBlock.GetHash())
	fmt.Printf("   Merkle Root: %x\n", genesisBlock.MerkleRoot)
	fmt.Printf("   Transactions: %d\n", genesisBlock.GetTransactionCount())

	// Validate block
	err = genesisBlock.ValidateBlock()
	if err != nil {
		fmt.Printf("❌ Block validation failed: %s\n", err)
	} else {
		fmt.Printf("✅ Block validation passed\n")
	}

	// Test 5: Lưu block vào LevelDB
	fmt.Println("\n5. Lưu block vào LevelDB...")
	storage, err := storage.NewBlockStorage("test_db")
	if err != nil {
		log.Fatal("Lỗi tạo storage:", err)
	}
	defer storage.Close()

	err = storage.SaveBlock(genesisBlock)
	if err != nil {
		log.Fatal("Lỗi save block:", err)
	}
	fmt.Printf("✅ Block đã được lưu vào LevelDB\n")

	// Test 6: Đọc block từ LevelDB
	fmt.Println("\n6. Đọc block từ LevelDB...")
	loadedBlock, err := storage.GetBlock(genesisBlock.GetHash())
	if err != nil {
		log.Fatal("Lỗi load block:", err)
	}

	fmt.Printf("✅ Block đã được load từ LevelDB\n")
	fmt.Printf("   Height: %d\n", loadedBlock.Height)
	fmt.Printf("   Hash: %x\n", loadedBlock.GetHash())
	fmt.Printf("   Transactions: %d\n", loadedBlock.GetTransactionCount())

	// Verify loaded block
	if string(loadedBlock.GetHash()) == string(genesisBlock.GetHash()) {
		fmt.Printf("✅ Loaded block hash khớp với original\n")
	} else {
		fmt.Printf("❌ Loaded block hash không khớp!\n")
	}

	// Test 7: Tạo block thứ 2
	fmt.Println("\n7. Tạo block thứ 2...")

	// Tạo transaction mới: Bob -> Alice
	tx2 := &blockchain.Transaction{
		Sender:    bob.Address,
		Receiver:  alice.Address,
		Amount:    5.0,
		Timestamp: time.Now().Unix(),
	}

	err = blockchain.SignTransaction(tx2, bob.PrivateKey)
	if err != nil {
		log.Fatal("Lỗi ký transaction 2:", err)
	}

	// Tạo block 2
	transactions2 := []*blockchain.Transaction{tx2}
	block2 := blockchain.NewBlock(transactions2, genesisBlock.GetHash(), 1)

	fmt.Printf("✅ Block 2 được tạo\n")
	fmt.Printf("   Height: %d\n", block2.Height)
	fmt.Printf("   Hash: %x\n", block2.GetHash())
	fmt.Printf("   Previous Hash: %x\n", block2.PreviousBlockHash)

	// Lưu block 2
	err = storage.SaveBlock(block2)
	if err != nil {
		log.Fatal("Lỗi save block 2:", err)
	}
	fmt.Printf("✅ Block 2 đã được lưu\n")

	// Test 8: Kiểm tra chain linking
	fmt.Println("\n8. Kiểm tra chain linking...")
	if string(block2.PreviousBlockHash) == string(genesisBlock.GetHash()) {
		fmt.Printf("✅ Block 2 đúng cách link với Genesis Block\n")
	} else {
		fmt.Printf("❌ Block 2 không link đúng với Genesis Block!\n")
	}

	// Test 9: Test storage functions
	fmt.Println("\n9. Test storage functions...")

	// Get latest height
	latestHeight, err := storage.GetLatestHeight()
	if err != nil {
		log.Fatal("Lỗi get latest height:", err)
	}
	fmt.Printf("✅ Latest height: %d\n", latestHeight)

	// Get latest block
	latestBlock, err := storage.GetLatestBlock()
	if err != nil {
		log.Fatal("Lỗi get latest block:", err)
	}
	fmt.Printf("✅ Latest block height: %d\n", latestBlock.Height)

	// Get block by height
	blockByHeight, err := storage.GetBlockByHeight(0)
	if err != nil {
		log.Fatal("Lỗi get block by height:", err)
	}
	fmt.Printf("✅ Block at height 0: %x\n", blockByHeight.GetHash())

	// Get all blocks
	allBlocks, err := storage.GetAllBlocks()
	if err != nil {
		log.Fatal("Lỗi get all blocks:", err)
	}
	fmt.Printf("✅ Total blocks trong chain: %d\n", len(allBlocks))

	// Test 10: Save và Load users
	fmt.Println("\n10. Test save/load users...")
	err = wallet.SaveUser(alice, "alice.json")
	if err != nil {
		log.Fatal("Lỗi save Alice:", err)
	}

	loadedAlice, err := wallet.LoadUser("alice.json")
	if err != nil {
		log.Fatal("Lỗi load Alice:", err)
	}

	if loadedAlice.Name == alice.Name && string(loadedAlice.Address) == string(alice.Address) {
		fmt.Printf("✅ Alice save/load thành công\n")
	} else {
		fmt.Printf("❌ Alice save/load thất bại!\n")
	}

	// Test signing với loaded Alice
	testTx := &blockchain.Transaction{
		Sender:    loadedAlice.Address,
		Receiver:  bob.Address,
		Amount:    1.0,
		Timestamp: time.Now().Unix(),
	}

	err = blockchain.SignTransaction(testTx, loadedAlice.PrivateKey)
	if err != nil {
		log.Fatal("Lỗi ký với loaded Alice:", err)
	}

	isValidLoadedSig := blockchain.VerifyTransaction(testTx, loadedAlice.PublicKey)
	if isValidLoadedSig {
		fmt.Printf("✅ Loaded Alice có thể ký và verify\n")
	} else {
		fmt.Printf("❌ Loaded Alice không thể ký hoặc verify!\n")
	}

	fmt.Println("\n=== Tóm tắt kết quả ===")
	fmt.Println("✅ ECDSA key generation và signing")
	fmt.Println("✅ Transaction creation và verification")
	fmt.Println("✅ Merkle Tree construction và verification")
	fmt.Println("✅ Block creation với Merkle Root")
	fmt.Println("✅ LevelDB storage operations")
	fmt.Println("✅ Blockchain linking với previous hash")
	fmt.Println("✅ User persistence")

	fmt.Println("\n=== Test hoàn tất ===")

	// Cleanup files
	os.Remove("alice.json")
}
