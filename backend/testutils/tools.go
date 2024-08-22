package testutils

import (
	"crypto/rand"
	"fmt"
	"github.com/google/uuid"
	"math/big"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func CryptoRandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letterBytes))))
		if err != nil {
			return ""
		}
		b[i] = letterBytes[num.Int64()]
	}
	return string(b)
}

func GetRandomEmail(n int) string {
	domain := CryptoRandString(n)
	name := CryptoRandString(n)
	return fmt.Sprintf("%s@%s.com", name, domain)
}

func GenerateUUIDv7() string {
	id, err := uuid.NewV7()
	if err != nil {
		return ""
	}
	return id.String()
}
