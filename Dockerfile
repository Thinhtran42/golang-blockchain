# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /go-blockchain ./cmd/node
# Run stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /go-blockchain .
# Tạo thư mục data cho LevelDB
RUN mkdir -p /app/data
CMD ["./go-blockchain"]