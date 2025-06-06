# 🔗 Golang Blockchain

A simple yet complete blockchain implementation in Go featuring multi-node consensus, P2P networking, and ECDSA digital signatures.

[![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)](https://golang.org)
[![Docker](https://img.shields.io/badge/Docker-Supported-brightgreen.svg)](https://docker.com)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## ✨ Features

- 🔐 **ECDSA Digital Signatures** - P-256 curve with SHA-256 hashing
- 🌳 **Merkle Tree** - Efficient transaction verification
- 💾 **LevelDB Storage** - Persistent blockchain data
- 🌐 **P2P Network** - gRPC-based node communication
- ⚡ **Leader-Follower Consensus** - Byzantine fault tolerant
- 🔄 **Node Recovery** - Automatic sync on restart
- 🐳 **Docker Support** - Easy multi-node deployment
- 📱 **CLI Interface** - User-friendly blockchain interaction

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Node 1 (Leader) │◄──►│ Node 2          │◄──►│ Node 3          │
│ :50051          │    │ :50052          │    │ :50053          │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
            ┌─────────────────────▼─────────────────────┐
            │               Client Layer                │
            │  ┌─────────────┐    ┌─────────────┐      │
            │  │ CLI Tool    │    │ gRPC Client │      │
            │  └─────────────┘    └─────────────┘      │
            └───────────────────────────────────────────┘
```

### Consensus Flow

```
Leader                  Followers
  │                        │
  ├─ 1. Collect TXs        │
  ├─ 2. Create Block       │
  ├─ 3. Propose ──────────►├─ 4. Validate
  ├─ 5. Collect Votes ◄────├─ 6. Vote
  ├─ 7. Check Majority     │
  ├─ 8. Commit & Broadcast►├─ 9. Save Block
  │                        │
```

## 🚀 Quick Start

### Prerequisites

- **Go 1.23+**
- Docker & Docker Compose

### 1. Setup & Run

```bash
# Clone repository
git clone https://github.com/Thinhtran42/golang-blockchain.git
cd golang-blockchain

# Remove docker-compose version warning (optional)
sed -i '1d' docker-compose.yml

# Start 3-node blockchain network
docker-compose up --build -d

# Check nodes are running
docker-compose ps
```

### 2. Test Blockchain

```bash
# Enter node container
docker-compose exec node1 sh

# Create users
./blockchain-cli create-user Alice
./blockchain-cli create-user Bob

# Send transaction
./blockchain-cli send Alice Bob 10.5

# Check blockchain status
./blockchain-cli status

# Exit container
exit
```

## 📱 CLI Commands

| Command       | Description                 | Example                                |
| ------------- | --------------------------- | -------------------------------------- |
| `create-user` | Create user with ECDSA keys | `./blockchain-cli create-user Alice`   |
| `list-users`  | Show all users              | `./blockchain-cli list-users`          |
| `send`        | Send transaction            | `./blockchain-cli send Alice Bob 10.5` |
| `status`      | Get blockchain status       | `./blockchain-cli status`              |

**Note:** CLI runs inside containers. For host usage:

```bash
go build -o blockchain-cli ./cmd/cli
./blockchain-cli status --node localhost:50051
```

## 🔧 Development

### Project Structure

```
golang-blockchain/
├── cmd/
│   ├── node/           # Validator node
│   └── cli/            # CLI tool
├── pkg/
│   ├── blockchain/     # Core blockchain logic
│   ├── wallet/         # ECDSA key management
│   ├── p2p/           # Network communication
│   ├── consensus/     # Consensus engine
│   └── storage/       # LevelDB operations
├── proto/             # gRPC definitions
├── Dockerfile
├── docker-compose.yml
└── README.md
```

### Local Development

```bash
# Install dependencies
go mod tidy

# Generate protobuf files (if modified)
protoc --go_out=. --go-grpc_out=. proto/blockchain.proto

# Build components
go build -o validator ./cmd/node
go build -o blockchain-cli ./cmd/cli

# Run single node (development)
./validator --node-id=node1 --is-leader=true --port=50051
```

### Testing

```bash
# Unit tests
go test ./pkg/...

# Integration test
docker-compose up -d
sleep 10
docker-compose exec node1 sh -c "
  ./blockchain-cli create-user Test1 &&
  ./blockchain-cli create-user Test2 &&
  ./blockchain-cli send Test1 Test2 5.0 &&
  sleep 8 &&
  ./blockchain-cli status
"
```

## 🔒 Security Features

### Cryptography

- **Algorithm:** ECDSA with P-256 elliptic curve
- **Hash Function:** SHA-256 for addresses and signatures
- **Key Format:** ASN.1 DER encoding
- **Address:** First 20 bytes of SHA-256(PublicKey)

### Network Security

- **Development:** Plain gRPC (localhost only)
- **Production Ready:** TLS/mTLS support planned

## 📊 Performance

| Metric         | Value                |
| -------------- | -------------------- |
| Block Time     | ~5 seconds           |
| Throughput     | ~10-20 TPS           |
| Memory Usage   | ~50-100MB per node   |
| Storage Growth | ~1KB per transaction |

## 🔄 Node Recovery

**Automatic Sync Process:**

1. Node restarts and connects to peers
2. Compares local height with network
3. Downloads missing blocks
4. Validates and commits blocks
5. Rejoins consensus

**Test Recovery:**

```bash
# Stop a node
docker-compose stop node2

# Send transactions
docker-compose exec node1 ./blockchain-cli send Alice Bob 3.0

# Restart and observe sync
docker-compose start node2
docker-compose logs node2 | tail -10
```

## 🐛 Troubleshooting

### Common Issues

**Docker Compose Warning:**

```bash
# Fix version warning
sed -i '1d' docker-compose.yml
```

**CLI Not Found:**

```bash
# Build CLI in container
docker-compose exec node1 sh
go build -o blockchain-cli ./cmd/cli
```

**Port Conflicts:**

```bash
# Kill existing processes
lsof -ti:50051 | xargs kill -9
```

**Permission Issues:**

```bash
# Fix data directory permissions
sudo chown -R $USER:$USER data/
```

## 🏭 Production Considerations

### Security Hardening

- [ ] Implement mTLS for P2P communication
- [ ] Add rate limiting and DDoS protection
- [ ] Use Hardware Security Modules (HSM) for key management
- [ ] Enable audit logging

### Scalability

- [ ] Horizontal scaling (5+ validator nodes)
- [ ] Parallel transaction validation
- [ ] Connection pooling and load balancing
- [ ] Sharding for increased throughput

### Monitoring

- [ ] Prometheus metrics collection
- [ ] Grafana dashboards
- [ ] ELK stack for centralized logging
- [ ] Alerting for critical failures

### Infrastructure

- [ ] Kubernetes deployment
- [ ] Infrastructure as Code (Terraform)
- [ ] CI/CD pipeline with security scanning
- [ ] Multi-region disaster recovery

## 🔮 Roadmap

### Phase 1 (Next 3 months)

- [ ] TLS/mTLS implementation
- [ ] Web dashboard
- [ ] Enhanced CLI commands
- [ ] Performance optimization

### Phase 2 (3-6 months)

- [ ] Smart contract support (WASM)
- [ ] Advanced consensus (PBFT)
- [ ] Cross-chain communication
- [ ] Mobile SDK

### Phase 3 (6-12 months)

- [ ] Enterprise key management
- [ ] Regulatory compliance tools
- [ ] Sharding implementation
- [ ] Advanced analytics

## 🤝 Contributing

1. Fork the repository
2. Create feature branch: `git checkout -b feature/amazing-feature`
3. Commit changes: `git commit -m 'Add amazing feature'`
4. Push to branch: `git push origin feature/amazing-feature`
5. Open Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with Go, gRPC, and Docker
- Inspired by Bitcoin and Ethereum architectures
- Uses LevelDB for efficient storage

---

**Made with ❤️ for the blockchain community**

For questions or support, please open an issue on GitHub.
