package clientapi

import (
	"crypto/rand"
	"encoding/base64"
)

const createIdempotencyKeyBytes = 32

func NewCreateIdempotencyKey() (string, error) {
	raw := make([]byte, createIdempotencyKeyBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
