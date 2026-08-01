package clientapi

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	SigningTrustBundleVersion  = 1
	SigningKeyAlgorithmEd25519 = "ed25519"
)

type SigningTrustKey struct {
	KeyID           string     `json:"key_id"`
	Algorithm       string     `json:"algorithm"`
	PublicKey       string     `json:"public_key"`
	Provider        string     `json:"provider,omitempty"`
	ProviderKey     string     `json:"provider_key,omitempty"`
	ProviderVersion int        `json:"provider_version,omitempty"`
	NotBefore       *time.Time `json:"not_before,omitempty"`
	NotAfter        *time.Time `json:"not_after,omitempty"`
}

type SigningTrustBundle struct {
	Version     int               `json:"version"`
	ActiveKeyID string            `json:"active_key_id"`
	Keys        []SigningTrustKey `json:"keys"`
}

func SigningKeyID(publicKey string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(publicKey))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return "", errors.New("invalid Ed25519 signing public key")
	}
	sum := sha256.Sum256(raw)
	return "ed25519:" + base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func NewSigningTrustBundle(publicKey string) (SigningTrustBundle, error) {
	publicKey = strings.TrimSpace(publicKey)
	keyID, err := SigningKeyID(publicKey)
	if err != nil {
		return SigningTrustBundle{}, err
	}
	return SigningTrustBundle{
		Version:     SigningTrustBundleVersion,
		ActiveKeyID: keyID,
		Keys: []SigningTrustKey{{
			KeyID:     keyID,
			Algorithm: SigningKeyAlgorithmEd25519,
			PublicKey: publicKey,
		}},
	}, nil
}

func (b SigningTrustBundle) Validate() error {
	if b.Version != SigningTrustBundleVersion {
		return fmt.Errorf("unsupported signing trust bundle version %d", b.Version)
	}
	if strings.TrimSpace(b.ActiveKeyID) == "" {
		return errors.New("signing trust bundle active_key_id is required")
	}
	if len(b.Keys) == 0 {
		return errors.New("signing trust bundle keys are required")
	}
	seen := make(map[string]struct{}, len(b.Keys))
	activeFound := false
	for _, key := range b.Keys {
		keyID := strings.TrimSpace(key.KeyID)
		if keyID == "" {
			return errors.New("signing trust key id is required")
		}
		if _, exists := seen[keyID]; exists {
			return fmt.Errorf("signing trust key id %s is duplicated", keyID)
		}
		seen[keyID] = struct{}{}
		if strings.TrimSpace(key.Algorithm) != SigningKeyAlgorithmEd25519 {
			return fmt.Errorf("signing trust key %s uses unsupported algorithm", keyID)
		}
		derived, err := SigningKeyID(key.PublicKey)
		if err != nil {
			return fmt.Errorf("signing trust key %s: %w", keyID, err)
		}
		if derived != keyID {
			return fmt.Errorf("signing trust key %s does not match its public key", keyID)
		}
		if key.NotBefore != nil && key.NotAfter != nil && !key.NotAfter.After(*key.NotBefore) {
			return fmt.Errorf("signing trust key %s has an invalid validity window", keyID)
		}
		activeFound = activeFound || keyID == strings.TrimSpace(b.ActiveKeyID)
	}
	if !activeFound {
		return errors.New("signing trust bundle active key is missing")
	}
	return nil
}

func (b SigningTrustBundle) Resolve(keyID string, at time.Time) (SigningTrustKey, error) {
	if err := b.Validate(); err != nil {
		return SigningTrustKey{}, err
	}
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return SigningTrustKey{}, errors.New("signing key id is required")
	}
	for _, key := range b.Keys {
		if key.KeyID != keyID {
			continue
		}
		at = at.UTC()
		if key.NotBefore != nil && at.Before(key.NotBefore.UTC()) {
			return SigningTrustKey{}, errors.New("signing key is not yet trusted")
		}
		if key.NotAfter != nil && !at.Before(key.NotAfter.UTC()) {
			return SigningTrustKey{}, errors.New("signing key trust has expired")
		}
		return key, nil
	}
	return SigningTrustKey{}, fmt.Errorf("signing key id %s is not trusted", keyID)
}

func (r ServerKeyResponse) SigningTrustBundle() (SigningTrustBundle, error) {
	return signingTrustBundleFromResponse("map", r.TrustBundle)
}

func (r ServerKeyResponse) NodeCredentialSigningTrustBundle() (SigningTrustBundle, error) {
	return signingTrustBundleFromResponse("node credential", r.NodeCredentialTrustBundle)
}

func (r ServerKeyResponse) RelaySigningTrustBundle() (SigningTrustBundle, error) {
	return signingTrustBundleFromResponse("relay", r.RelayTrustBundle)
}

func signingTrustBundleFromResponse(label string, responseBundle SigningTrustBundle) (SigningTrustBundle, error) {
	bundle := CloneSigningTrustBundle(responseBundle)
	if err := bundle.Validate(); err != nil {
		return SigningTrustBundle{}, fmt.Errorf("%s signing trust bundle: %w", label, err)
	}
	return bundle, nil
}

func CloneServerKeyResponse(response ServerKeyResponse) ServerKeyResponse {
	copy := response
	copy.TrustBundle = CloneSigningTrustBundle(response.TrustBundle)
	copy.NodeCredentialTrustBundle = CloneSigningTrustBundle(response.NodeCredentialTrustBundle)
	copy.RelayTrustBundle = CloneSigningTrustBundle(response.RelayTrustBundle)
	return copy
}

func CloneSigningTrustBundle(bundle SigningTrustBundle) SigningTrustBundle {
	copy := bundle
	copy.Keys = make([]SigningTrustKey, len(bundle.Keys))
	for index, key := range bundle.Keys {
		keyCopy := key
		if key.NotBefore != nil {
			value := key.NotBefore.UTC()
			keyCopy.NotBefore = &value
		}
		if key.NotAfter != nil {
			value := key.NotAfter.UTC()
			keyCopy.NotAfter = &value
		}
		copy.Keys[index] = keyCopy
	}
	return copy
}
