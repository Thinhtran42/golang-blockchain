# 🚀 Auto-Scaling Blockchain System

## Tổng quan

Hệ thống blockchain auto-scaling này có khả năng:

- **Tự động phát hiện khi nodes gặp sự cố**
- **Tự động tạo nodes mới để duy trì consensus**
- **Node discovery tự động để nodes mới tìm peers**
- **Sync tự động với blockchain hiện tại**

## 🏗️ Kiến trúc hệ thống

```
┌─────────────────────────────────────────────────────────────┐
│                   AUTO-SCALING BLOCKCHAIN                  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐    ┌─────────────────────────────────────┐ │
│  │   Registry  │◄──►│          Docker Manager            │ │
│  │   Service   │    │                                     │ │
│  │             │    │  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐│ │
│  │ - Discovery │    │  │Node1│  │Node2│  │Node3│  │Node4││ │
│  │ - Health    │    │  │Lead │  │Foll │  │Foll │  │Foll ││ │
│  │ - Scaling   │    │  └─────┘  └─────┘  └─────┘  └─────┘│ │
│  └─────────────┘    └─────────────────────────────────────┘ │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## 🔧 Components

### 1. Registry Service (`pkg/registry/`)

- **Node Discovery**: Quản lý danh sách nodes và trạng thái
- **Health Monitoring**: Theo dõi sức khỏe của từng node
- **Auto-scaling Logic**: Quyết định khi nào cần tạo/xóa nodes

### 2. Docker Manager (`pkg/docker/`)

- **Container Management**: Tạo/xóa Docker containers
- **Network Configuration**: Tự động cấu hình mạng
- **Volume Management**: Quản lý persistent storage

### 3. Discovery Client (`pkg/discovery/`)

- **Node Registration**: Đăng ký node với registry
- **Peer Discovery**: Tìm kiếm peers từ registry
- **Heartbeat**: Gửi tín hiệu sống định kỳ

## 🚀 Cách chạy hệ thống

### Bước 1: Chuẩn bị

```bash
# Đảm bảo Docker đang chạy
docker --version
docker-compose --version

# Cấp quyền cho script
chmod +x scripts/start-auto-scaling.sh
```

### Bước 2: Khởi động hệ thống

```bash
# Chạy script auto-scaling
./scripts/start-auto-scaling.sh
```

### Bước 3: Theo dõi hệ thống

```bash
# Xem trạng thái registry
curl -s http://localhost:9090/status | jq

# Xem logs
docker-compose -f docker-compose.registry.yml logs -f
```

## 🧪 Testing Auto-scaling

### Test 1: Node Failure Recovery

```bash
# Dừng một node để test auto-scaling
docker-compose -f docker-compose.registry.yml stop node2

# Theo dõi logs registry để xem quá trình tạo node mới
docker-compose -f docker-compose.registry.yml logs -f registry
```

### Test 2: Consensus Failure

```bash
# Dừng nhiều nodes để gây consensus failure
docker-compose -f docker-compose.registry.yml stop node2 node3

# Registry sẽ tự động tạo nodes mới
curl -s http://localhost:9090/status | jq
```

### Test 3: Manual Scaling

```bash
# Tạo node mới thủ công qua API
curl -X POST http://localhost:9090/scale \
  -H "Content-Type: application/json" \
  -d '{"target_nodes": 5}'
```

## 📊 Monitoring

### Registry API Endpoints

- `GET /status` - Overall system status
- `GET /discover` - List healthy nodes
- `GET /health?node_id=X` - Node heartbeat
- `POST /register` - Node registration
- `POST /consensus-failure` - Report consensus issues

### Node Health Monitoring

- **Health Check Interval**: 5 seconds
- **Unhealthy Threshold**: 15 seconds
- **Auto-scaling Check**: 10 seconds

### Scaling Parameters

- **Minimum Nodes**: 2 (for consensus)
- **Maximum Nodes**: 10
- **Scale Threshold**: Majority needed (min_nodes/2 + 1)

## 🔧 Configuration

### Environment Variables

#### Registry Service

```bash
PORT=9090                                    # Registry port
MIN_NODES=2                                  # Minimum nodes for consensus
MAX_NODES=10                                 # Maximum nodes
DOCKER_NETWORK=golang-blockchain_network     # Docker network
DOCKER_IMAGE=golang-blockchain-node         # Node image
PORT_START=50054                            # Starting port for new nodes
```

#### Node Configuration

```bash
NODE_ID=nodeX                               # Unique node ID
IS_LEADER=false                             # Leadership status
PORT=50051                                  # Internal gRPC port
REGISTRY_URL=http://registry:9090           # Registry service URL
AUTO_DISCOVER=true                          # Enable auto-discovery
```

## 🎯 Auto-scaling Logic

### Scale Up Triggers

1. **Insufficient Nodes**: Khi số nodes healthy < minimum threshold
2. **Consensus Failure**: Khi consensus không thể đạt được
3. **Health Check Failure**: Khi nodes không phản hồi health check

### Scale Down Triggers

1. **Excess Nodes**: Khi số nodes > maximum limit
2. **Resource Optimization**: Tối ưu tài nguyên (optional)

### Node Selection

- **Scale Up**: Tạo nodes mới với ID tự động tăng
- **Scale Down**: Ưu tiên xóa follower nodes (giữ leader)

## 🔄 Node Discovery Process

### For New Nodes

1. **Registration**: Node đăng ký với Registry
2. **Discovery**: Lấy danh sách peers từ Registry
3. **Connection**: Kết nối tới các peers
4. **Sync**: Đồng bộ blockchain data
5. **Participation**: Tham gia consensus

### For Existing Nodes

1. **Heartbeat**: Gửi tín hiệu sống định kỳ
2. **Peer Updates**: Cập nhật danh sách peers
3. **Health Reporting**: Báo cáo trạng thái của node

## 🛠️ Troubleshooting

### Common Issues

#### Registry không khởi động

```bash
# Kiểm tra logs
docker-compose -f docker-compose.registry.yml logs registry

# Kiểm tra Docker daemon
sudo systemctl status docker
```

#### Nodes không tự tạo

```bash
# Kiểm tra quyền Docker socket
ls -la /var/run/docker.sock

# Kiểm tra network
docker network ls | grep blockchain
```

#### Consensus không hoạt động

```bash
# Kiểm tra số nodes healthy
curl -s http://localhost:9090/status | jq '.healthy_nodes'

# Xem logs các nodes
docker-compose -f docker-compose.registry.yml logs node1
```

### Debug Commands

```bash
# Xem tất cả containers
docker ps -a

# Xem logs chi tiết
docker-compose -f docker-compose.registry.yml logs -f --tail=100

# Kiểm tra network connectivity
docker exec node1 nc -zv registry 9090
```

## 🎉 Features

### ✅ Implemented

- [x] Node health monitoring
- [x] Auto-scaling up/down
- [x] Node discovery service
- [x] Docker container management
- [x] Consensus failure detection
- [x] Automatic peer discovery
- [x] Blockchain sync for new nodes

### 🔄 Advanced Features (Future)

- [ ] Leader election when leader fails
- [ ] Geographic distribution
- [ ] Resource-based scaling
- [ ] Kubernetes support
- [ ] Metrics and alerting
- [ ] Rolling updates

## 📈 Performance

### Scaling Performance

- **Node Creation Time**: ~10-15 seconds
- **Discovery Time**: ~5 seconds
- **Sync Time**: Depends on blockchain size
- **Health Check Response**: <1 second

### Resource Requirements

- **Registry Service**: ~50MB RAM
- **Per Node**: ~100MB RAM, 1GB storage
- **Network**: Internal Docker network

## 🔐 Security Considerations

1. **Docker Socket Access**: Registry cần quyền Docker socket
2. **Network Isolation**: Nodes chỉ giao tiếp trong Docker network
3. **No External Access**: Chỉ expose ports cần thiết
4. **Resource Limits**: Giới hạn số nodes tối đa

---

**🎯 Kết quả**: Hệ thống blockchain có khả năng tự phục hồi và mở rộng, đảm bảo tính khả dụng cao và consensus luôn hoạt động!
