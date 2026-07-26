package wgkeys

import (
	"crypto/rand"
	"encoding/base64"
	"errors"

	"golang.org/x/crypto/curve25519"
)

func GeneratePrivateKey() (string, error) {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return "", err
	}
	key[0] &= 248
	key[31] &= 127
	key[31] |= 64
	return base64.StdEncoding.EncodeToString(key[:]), nil
}

func PublicKey(privateKey string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(privateKey)
	if err != nil {
		return "", err
	}
	if len(raw) != curve25519.ScalarSize {
		return "", errors.New("wireguard private key must decode to 32 bytes")
	}
	pub, err := curve25519.X25519(raw, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(pub), nil
}
