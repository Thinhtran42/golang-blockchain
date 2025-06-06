package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"golang-blockchain/pkg/blockchain"
	"golang-blockchain/pkg/wallet"
	pb "golang-blockchain/proto"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	nodeAddress string
	client      pb.BlockchainServiceClient
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "blockchain-cli",
		Short: "CLI tool for interacting with the blockchain",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Connect to node
			conn, err := grpc.Dial(nodeAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("Failed to connect to node: %v", err)
			}
			client = pb.NewBlockchainServiceClient(conn)
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&nodeAddress, "node", "localhost:50051", "Node address to connect to")

	// Add commands
	rootCmd.AddCommand(createUserCmd())
	rootCmd.AddCommand(sendTransactionCmd())
	rootCmd.AddCommand(getStatusCmd())
	rootCmd.AddCommand(getLatestBlockCmd())
	rootCmd.AddCommand(listUsersCmd())

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// createUserCmd tạo user mới
func createUserCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create-user [name]",
		Short: "Create a new user with ECDSA key pair",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			fmt.Printf("Creating user: %s\n", name)

			// Tạo user
			user, err := wallet.CreateUser(name)
			if err != nil {
				log.Fatalf("Failed to create user: %v", err)
			}

			// Save user to file
			filename := fmt.Sprintf("%s.json", name)
			err = wallet.SaveUser(user, filename)
			if err != nil {
				log.Fatalf("Failed to save user: %v", err)
			}

			fmt.Printf("✅ User created successfully!\n")
			fmt.Printf("   Name: %s\n", user.Name)
			fmt.Printf("   Address: %x\n", user.Address)
			fmt.Printf("   Saved to: %s\n", filename)
			fmt.Printf("   Private Key: %s\n", user.PrivateKeyHex)
			fmt.Printf("   Public Key: %s\n", user.PublicKeyHex)
		},
	}
}

// sendTransactionCmd gửi transaction
func sendTransactionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send [from] [to] [amount]",
		Short: "Send transaction from one user to another",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			fromUser := args[0]
			toUser := args[1]
			amount, err := strconv.ParseFloat(args[2], 64)
			if err != nil {
				log.Fatalf("Invalid amount: %v", err)
			}

			fmt.Printf("Sending %.2f from %s to %s\n", amount, fromUser, toUser)

			// Load sender
			sender, err := wallet.LoadUser(fmt.Sprintf("%s.json", fromUser))
			if err != nil {
				log.Fatalf("Failed to load sender %s: %v", fromUser, err)
			}

			// Load receiver
			receiver, err := wallet.LoadUser(fmt.Sprintf("%s.json", toUser))
			if err != nil {
				log.Fatalf("Failed to load receiver %s: %v", toUser, err)
			}

			// Tạo transaction
			tx := &blockchain.Transaction{
				Sender:    sender.Address,
				Receiver:  receiver.Address,
				Amount:    amount,
				Timestamp: time.Now().Unix(),
			}

			// Ký transaction
			err = blockchain.SignTransaction(tx, sender.PrivateKey)
			if err != nil {
				log.Fatalf("Failed to sign transaction: %v", err)
			}

			// Convert to proto
			protoTx := &pb.Transaction{
				Sender:    tx.Sender,
				Receiver:  tx.Receiver,
				Amount:    tx.Amount,
				Timestamp: tx.Timestamp,
				Signature: tx.Signature,
			}

			// Submit to node
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			resp, err := client.SubmitTransaction(ctx, &pb.SubmitTransactionRequest{
				Transaction: protoTx,
			})
			if err != nil {
				log.Fatalf("Failed to submit transaction: %v", err)
			}

			if resp.Success {
				fmt.Printf("✅ Transaction submitted successfully!\n")
				fmt.Printf("   Message: %s\n", resp.Message)
				fmt.Printf("   From: %s (%x)\n", fromUser, sender.Address[:8])
				fmt.Printf("   To: %s (%x)\n", toUser, receiver.Address[:8])
				fmt.Printf("   Amount: %.2f\n", amount)
			} else {
				fmt.Printf("❌ Transaction failed: %s\n", resp.Message)
			}
		},
	}

	return cmd
}

// getStatusCmd lấy trạng thái node
func getStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Get node status and blockchain info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Getting status from node: %s\n", nodeAddress)

			// Get status using available method
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := client.GetStatus(ctx, &pb.StatusRequest{
				NodeId: "cli",
			})
			if err != nil {
				log.Fatalf("Failed to get status: %v", err)
			}

			fmt.Printf("📊 Node Status:\n")
			fmt.Printf("   Node ID: %s\n", resp.NodeId)
			fmt.Printf("   Is Leader: %v\n", resp.IsLeader)
			fmt.Printf("   Current Height: %d\n", resp.CurrentHeight)
			fmt.Printf("   Transaction Pool Size: %d\n", resp.TxPoolSize)

			// Get latest block for more info
			ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel2()

			blockResp, err := client.GetLatestBlock(ctx2, &pb.GetLatestBlockRequest{
				NodeId: "cli",
			})

			if err != nil {
				fmt.Printf("❌ Failed to get latest block: %v\n", err)
				return
			}

			if blockResp.Success {
				block := blockResp.Block
				fmt.Printf("📦 Latest Block:\n")
				fmt.Printf("   Height: %d\n", block.Height)
				fmt.Printf("   Hash: %x\n", block.CurrentBlockHash[:16])
				fmt.Printf("   Previous Hash: %x\n", block.PreviousBlockHash[:16])
				fmt.Printf("   Merkle Root: %x\n", block.MerkleRoot[:16])
				fmt.Printf("   Transactions: %d\n", len(block.Transactions))
				fmt.Printf("   Timestamp: %s\n", time.Unix(block.Timestamp, 0).Format("2006-01-02 15:04:05"))

				if len(block.Transactions) > 0 {
					fmt.Printf("   Recent Transactions:\n")
					for i, tx := range block.Transactions {
						if i >= 3 { // Show max 3 transactions
							fmt.Printf("     ... and %d more\n", len(block.Transactions)-3)
							break
						}
						fmt.Printf("     TX%d: %x -> %x (%.2f)\n",
							i+1, tx.Sender[:8], tx.Receiver[:8], tx.Amount)
					}
				}
			} else {
				fmt.Printf("❌ No blocks found\n")
			}
		},
	}
}

// getLatestBlockCmd lấy thông tin block mới nhất
func getLatestBlockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "latest-block",
		Short: "Get latest block information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Getting latest block from node: %s\n", nodeAddress)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := client.GetLatestBlock(ctx, &pb.GetLatestBlockRequest{
				NodeId: "cli",
			})
			if err != nil {
				log.Fatalf("Failed to get latest block: %v", err)
			}

			if !resp.Success {
				fmt.Printf("❌ No blocks found\n")
				return
			}

			block := resp.Block
			fmt.Printf("📦 Latest Block (Height %d):\n", block.Height)
			fmt.Printf("   Block Hash: %x\n", block.CurrentBlockHash)
			fmt.Printf("   Previous Hash: %x\n", block.PreviousBlockHash)
			fmt.Printf("   Merkle Root: %x\n", block.MerkleRoot)
			fmt.Printf("   Timestamp: %s\n", time.Unix(block.Timestamp, 0).Format("2006-01-02 15:04:05"))
			fmt.Printf("   Transactions (%d):\n", len(block.Transactions))

			if len(block.Transactions) == 0 {
				fmt.Printf("     No transactions in this block\n")
			} else {
				for i, tx := range block.Transactions {
					fmt.Printf("     TX%d:\n", i+1)
					fmt.Printf("       From: %x\n", tx.Sender)
					fmt.Printf("       To: %x\n", tx.Receiver)
					fmt.Printf("       Amount: %.2f\n", tx.Amount)
					fmt.Printf("       Time: %s\n", time.Unix(tx.Timestamp, 0).Format("2006-01-02 15:04:05"))
					fmt.Printf("       Signature: %x\n", tx.Signature[:16])
					fmt.Printf("       ---\n")
				}
			}
		},
	}
}

// listUsersCmd liệt kê users có sẵn
func listUsersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-users",
		Short: "List all available users",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Available users:")

			files, err := os.ReadDir(".")
			if err != nil {
				log.Fatalf("Failed to read directory: %v", err)
			}

			userCount := 0
			for _, file := range files {
				if !file.IsDir() && len(file.Name()) > 5 && file.Name()[len(file.Name())-5:] == ".json" {
					userName := file.Name()[:len(file.Name())-5]

					// Try to load user to get address
					user, err := wallet.LoadUser(file.Name())
					if err != nil {
						fmt.Printf("   ❌ %s (failed to load: %v)\n", userName, err)
						continue
					}

					fmt.Printf("   ✅ %s\n", userName)
					fmt.Printf("      Address: %x\n", user.Address)
					fmt.Printf("      File: %s\n", file.Name())
					userCount++
				}
			}

			if userCount == 0 {
				fmt.Println("   No users found. Use 'create-user' command to create users.")
				fmt.Println("\n💡 Example: blockchain-cli create-user Alice")
			} else {
				fmt.Printf("\nTotal users: %d\n", userCount)
				fmt.Println("\n💡 Usage examples:")
				fmt.Println("   blockchain-cli send Alice Bob 10.5")
				fmt.Println("   blockchain-cli status")
				fmt.Println("   blockchain-cli latest-block")
			}
		},
	}
}
