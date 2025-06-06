# Go Blockchain - Simple Blockchain Implementation

Hệ thống blockchain đơn giản được viết bằng Go, hỗ trợ chuyển tiền giữa users với các tính năng:

- ✅ **ECDSA Digital Signatures** - Ký số giao dịch đảm bảo tính toàn vẹn
- ✅ **Merkle Tree** - Xác thực hiệu quả các transactions trong block
- ✅ **LevelDB Storage** - Lưu trữ blocks bền vững
- ✅ **P2P Network** - Giao tiếp giữa nodes qua gRPC
- ✅ **Leader-Follower Consensus** - Cơ chế đồng thuận đa node
- ✅ **Node Recovery** - Tự động sync khi node restart
- ✅ **Docker Support** - Chạy 3 validators dễ dàng
- ✅ **CLI Tool** - Tương tác với blockchain

## 🏗️ Kiến trúc hệ thống

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Node 1        │    │   Node 2        │    │   Node 3        │
│   (Leader)      │◄──►│   (Follower)    │◄──►│   (Follower)    │
│                 │    │                 │    │                 │
│ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │
│ │ Consensus   │ │    │ │ Consensus   │ │    │ │ Consensus   │ │
│ │ Engine      │ │    │ │ Engine      │ │    │ │ Engine      │ │
│ └─────────────┘ │    │ └─────────────┘ │    │ └─────────────┘ │
│ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │
│ │ P2P Network │ │    │ │ P2P Network │ │    │ │ P2P Network │ │
│ └─────────────┘ │    │ └─────────────┘ │    │ └─────────────┘ │
│ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │
│ │ LevelDB     │ │    │ │ LevelDB     │ │    │ │ LevelDB     │ │
│ └─────────────┘ │    │ └─────────────┘ │    │ └─────────────┘ │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Consensus Flow:

1. **Leader** thu thập pending transactions
2. **Leader** tạo block và propose đến followers
3. **Followers** validate block và gửi vote
4. **Leader** thu thập votes, nếu majority approve thì commit
5. **Leader** broadcast commit đến tất cả followers

## 🚀 Quick Start

### 1. Build và chạy với Docker

```bash
# Clone repo
git clone <repo-url>
cd go-blockchain

# Build và start 3 nodes
docker-compose up --build

# Kiểm tra logs
docker-compose logs -f node1
docker-compose logs -f node2
docker-compose logs -f node3
```

### 2. Sử dụng CLI Tool

```bash
# Build CLI tool
go build -o blockchain-cli ./cmd/cli

# Tạo users
./blockchain-cli create-user Alice
./blockchain-cli create-user Bob

# Xem danh sách users
./blockchain-cli list-users

# Gửi transaction
./blockchain-cli send Alice Bob 10.5

# Xem trạng thái blockchain
./blockchain-cli status

# Xem block cụ thể
./blockchain-cli get-block 0
```

## 📁 Cấu trúc dự án

```
go-blockchain/
├── cmd/
│   ├── node/           # Validator node main
│   └── cli/            # CLI tool
├── pkg/
│   ├── blockchain/     # Core blockchain (Transaction, Block, Merkle)
│   ├── wallet/         # ECDSA key management
│   ├── p2p/           # Network communication
│   ├── consensus/     # Consensus engine
│   └── storage/       # LevelDB operations
├── proto/             # gRPC protobuf definitions
├── Dockerfile         # Docker image
├── docker-compose.yml # Multi-node setup
└── README.md
```

## 🔧 Development Setup

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- Protocol Buffers compiler

```bash
# Install protoc
# macOS
brew install protobuf

# Ubuntu
sudo apt-get install protobuf-compiler

# Install Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### Build từ source

```bash
# Clone và setup
git clone <repo-url>
cd go-blockchain
go mod tidy

# Generate protobuf files
protoc --go_out=. --go-grpc_out=. proto/blockchain.proto

# Build validator node
go build -o validator ./cmd/node

# Build CLI
go build -o blockchain-cli ./cmd/cli

# Run tests
go run cmd/test/test_basic.go
```

## 🔑 ECDSA Key Management

### Tạo Users

```bash
# Tạo Alice
./blockchain-cli create-user Alice
# Output: Alice.json file với private/public keys

# File Alice.json:
{
  "name": "Alice",
  "address": "a1b2c3d4e5f6...",
  "private_key_hex": "abcdef123456...",
  "public_key_hex": "fedcba654321..."
}
```

### Security Features

- **P-256 Curve**: NIST P-256 elliptic curve
- **SHA-256 Hashing**: Secure hash cho addresses và signatures
- **ASN.1 DER Encoding**: Standard signature format
- **Address Derivation**: Address = SHA-256(PublicKey)[:20]

## 📦 Block Structure

```json
{
  "transactions": [...],
  "merkle_root": "abc123...",
  "previous_block_hash": "def456...",
  "current_block_hash": "ghi789...",
  "timestamp": 1698765432,
  "height": 42
}
```

### Merkle Tree Features

- **Binary Tree**: Efficient transaction verification
- **Hash Pairs**: SHA-256(left + right)
- **Odd Handling**: Duplicate last node if odd count
- **Proof Generation**: Verify single transaction without full tree

## 🌐 Network Protocol (gRPC)

### Core Services

```protobuf
service BlockchainService {
  // Consensus
  rpc ProposeBlock(ProposeBlockRequest) returns (ProposeBlockResponse);
  rpc SubmitVote(VoteRequest) returns (VoteResponse);
  rpc CommitBlock(CommitBlockRequest) returns (CommitBlockResponse);

  // Transactions
  rpc SubmitTransaction(SubmitTransactionRequest) returns (SubmitTransactionResponse);

  // Sync
  rpc GetLatestBlock(GetLatestBlockRequest) returns (GetLatestBlockResponse);
  rpc GetBlocksFromHeight(GetBlocksFromHeightRequest) returns (GetBlocksFromHeightResponse);
}
```

### Port Mapping

- **Node 1 (Leader)**: localhost:50051
- **Node 2 (Follower)**: localhost:50052
- **Node 3 (Follower)**: localhost:50053

## 🔄 Consensus Mechanism

### Leader-Follower Model

1. **Leader Election**: Static configuration (Node 1 = Leader)
2. **Block Interval**: 5 seconds
3. **Vote Timeout**: 3 seconds
4. **Majority Rule**: 2/3 nodes must approve
5. **Failure Handling**: Leader proposes new block if previous fails

### Consensus Steps

```
Leader:               Followers:
  │                     │
  ├─ Collect TXs       │
  ├─ Create Block      │
  ├─ Propose ────────► ├─ Validate
  │                    ├─ Vote ──────┐
  ├─ Collect Votes ◄───┘              │
  ├─ Check Majority                   │
  ├─ Commit Block                     │
  ├─ Broadcast ──────► ├─ Save Block
  │                    │
```

## 🔧 Node Recovery

### Automatic Sync Process

1. **Startup**: Node checks latest height vs peers
2. **Height Comparison**: Find highest peer
3. **Block Download**: Get missing blocks từ best peer
4. **Validation**: Verify each block before saving
5. **Resume**: Join consensus after sync complete

### Recovery Scenarios

- **Network Partition**: Auto-reconnect and sync
- **Node Restart**: Resume từ last saved block
- **Corrupted Data**: Re-sync từ peers

## 📊 Monitoring & Debugging

### Health Checks

```bash
# Check node health
./blockchain-cli status --node localhost:50051

# View specific block
./blockchain-cli get-block 5 --node localhost:50051

# Monitor logs
docker-compose logs -f node1
```

### Debug Information

- **Consensus Status**: Current round, pending votes
- **Transaction Pool**: Pending transactions count
- **Sync Status**: Current height, sync progress
- **Peer Connections**: Connected peers list

## 🧪 Testing

### Unit Tests

```bash
# Test core components
go run cmd/test/test_basic.go

# Test wallet functionality
go test ./pkg/wallet/...

# Test blockchain components
go test ./pkg/blockchain/...
```

### Integration Tests

```bash
# Start test environment
docker-compose -f docker-compose.test.yml up

# Run integration tests
go test -tags=integration ./test/...
```

## 🐛 Troubleshooting

### Common Issues

**1. Port Already in Use**

```bash
# Kill existing processes
lsof -ti:50051 | xargs kill -9
```

**2. Permission Denied (LevelDB)**

```bash
# Fix data directory permissions
sudo chown -R $USER:$USER data/
```

**3. Peer Connection Failed**

```bash
# Check network connectivity
docker network ls
docker network inspect go-blockchain_blockchain_network
```

**4. Transaction Stuck in Pool**

```bash
# Check transaction signature
./blockchain-cli status
# Verify user key files exist and are valid
```

### Debug Mode

```bash
# Enable debug logging
export LOG_LEVEL=debug
docker-compose up
```

## 🚀 Production Considerations

### Security Enhancements Needed

- [ ] **TLS/mTLS**: Encrypt P2P communication
- [ ] **Key Rotation**: Periodic key updates
- [ ] **Rate Limiting**: Prevent spam attacks
- [ ] **Access Control**: Authentication for CLI access

### Performance Optimizations

- [ ] **Batch Processing**: Multiple transactions per block
- [ ] **Parallel Validation**: Concurrent signature verification
- [ ] **Caching**: Block and transaction caching
- [ ] **Database Optimization**: Custom LevelDB tuning

### Monitoring & Observability

- [ ] **Metrics**: Prometheus/Grafana integration
- [ ] **Logging**: Structured logging with correlation IDs
- [ ] **Alerting**: Critical failure notifications
- [ ] **Distributed Tracing**: Request flow tracking

## 📝 API Reference

### CLI Commands

| Command       | Description          | Example                                |
| ------------- | -------------------- | -------------------------------------- |
| `create-user` | Create new user      | `./blockchain-cli create-user Alice`   |
| `send`        | Send transaction     | `./blockchain-cli send Alice Bob 10.5` |
| `status`      | Get node status      | `./blockchain-cli status`              |
| `get-block`   | Get block info       | `./blockchain-cli get-block 5`         |
| `list-users`  | List available users | `./blockchain-cli list-users`          |

### Environment Variables

| Variable    | Description               | Default |
| ----------- | ------------------------- | ------- |
| `NODE_ID`   | Unique node identifier    | `node1` |
| `IS_LEADER` | Leader flag               | `false` |
| `PORT`      | gRPC port                 | `50051` |
| `PEERS`     | Comma-separated peer list | `""`    |

## 🤝 Contributing

1. Fork the repository
2. Create feature branch: `git checkout -b feature/amazing-feature`
3. Commit changes: `git commit -m 'Add amazing feature'`
4. Push to branch: `git push origin feature/amazing-feature`
5. Open Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**Made with ❤️ using Go, gRPC, and Docker**
