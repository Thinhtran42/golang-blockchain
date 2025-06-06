# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build cả node và CLI
RUN go build -o /go-blockchain ./cmd/node
RUN go build -o /blockchain-cli ./cmd/cli

# Run stage
FROM alpine:latest
WORKDIR /app
# Copy cả node và CLI
COPY --from=builder /go-blockchain .
COPY --from=builder /blockchain-cli .
# Copy db-viewer nếu có
COPY --from=builder /app/db-viewer ./db-viewer
# Tạo thư mục data cho LevelDB
RUN mkdir -p /app/data
CMD ["./go-blockchain"]