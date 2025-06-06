package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
)

// GenerateKeyPair tạo cặp khóa ECDSA (theo code mẫu đề bài)
func GenerateKeyPair() (*ecdsa.PrivateKey, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}
	return privKey, nil
}

// PublicKeyToAddress chuyển public key thành dạng địa chỉ (theo đề bài)
func PublicKeyToAddress(pubKey *ecdsa.PublicKey) []byte {
	// Hash public key để tạo address
	pubKeyBytes := append(pubKey.X.Bytes(), pubKey.Y.Bytes()...)
	hash := sha256.Sum256(pubKeyBytes)
	return hash[:20] // Lấy 20 bytes đầu như Ethereum
}

// User struct để lưu thông tin user với keys
type User struct {
	Name       string            `json:"name"`
	PrivateKey *ecdsa.PrivateKey `json:"-"` // Không serialize private key
	PublicKey  *ecdsa.PublicKey  `json:"-"`
	Address    []byte            `json:"address"`
	// Để serialize keys, chuyển sang hex
	PrivateKeyHex string `json:"private_key_hex"`
	PublicKeyHex  string `json:"public_key_hex"`
}

// CreateUser tạo user mới với cặp khóa ECDSA
func CreateUser(name string) (*User, error) {
	// Generate key pair
	privateKey, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair for %s: %w", name, err)
	}

	publicKey := &privateKey.PublicKey
	address := PublicKeyToAddress(publicKey)

	user := &User{
		Name:       name,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Address:    address,
		// Convert keys to hex for JSON storage
		PrivateKeyHex: fmt.Sprintf("%x", privateKey.D.Bytes()),
		PublicKeyHex:  fmt.Sprintf("%x%x", publicKey.X.Bytes(), publicKey.Y.Bytes()),
	}

	return user, nil
}

// SaveUser lưu user vào file JSON
func SaveUser(user *User, filename string) error {
	data, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// LoadUser load user từ file JSON
func LoadUser(filename string) (*User, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read user file: %w", err)
	}

	var user User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	// Reconstruct private key từ hex
	privateKeyBytes := make([]byte, 0)
	for i := 0; i < len(user.PrivateKeyHex); i += 2 {
		b := user.PrivateKeyHex[i : i+2]
		val := new(big.Int)
		val.SetString(b, 16)
		privateKeyBytes = append(privateKeyBytes, byte(val.Uint64()))
	}

	user.PrivateKey = &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: elliptic.P256(),
		},
		D: new(big.Int).SetBytes(privateKeyBytes),
	}

	// Reconstruct public key
	user.PrivateKey.PublicKey.X, user.PrivateKey.PublicKey.Y =
		elliptic.P256().ScalarBaseMult(privateKeyBytes)
	user.PublicKey = &user.PrivateKey.PublicKey

	return &user, nil
}
