package clientapi

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"strings"
)

const IdentityPublicKeyPrefix = "enp_"

func DecodeIdentityPublicKey(value string) (ed25519.PublicKey, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, IdentityPublicKeyPrefix) {
		return nil, errors.New("invalid identity public key")
	}
	encoded := strings.TrimPrefix(value, IdentityPublicKeyPrefix)
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) != ed25519.PublicKeySize || base64.RawURLEncoding.EncodeToString(raw) != encoded {
		return nil, errors.New("invalid identity public key")
	}
	return ed25519.PublicKey(raw), nil
}
