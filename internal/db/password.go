package db

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GeneratePassword creates a secure random 20-character alphanumeric password
func GeneratePassword(length int) (string, error) {
	if length <= 0 {
		length = 20
	}
	bytes := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random byte: %w", err)
		}
		bytes[i] = charset[num.Int64()]
	}
	return string(bytes), nil
}
