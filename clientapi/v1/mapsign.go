package clientapi

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	relayauth "github.com/endless-net/relay/protocol/v1"
)

const (
	mapSignatureVersion             = 4
	mapSignatureAlgorithm           = "ed25519-network-map-v4"
	DefaultNetworkMapSignatureTTL   = 24 * time.Hour
	maxNetworkMapSignatureClockSkew = 5 * time.Minute
)

type networkMapSigningPayload struct {
	Schema              int                  `json:"schema"`
	KeyID               string               `json:"key_id"`
	IssuedAt            time.Time            `json:"issued_at"`
	ExpiresAt           time.Time            `json:"expires_at"`
	RegistrationBinding string               `json:"registration_binding,omitempty"`
	Network             Network              `json:"network"`
	Node                Node                 `json:"node"`
	Peers               []Peer               `json:"peers"`
	STUNEndpoints       []STUNEndpoint       `json:"stun_endpoints,omitempty"`
	Relays              []relayauth.Endpoint `json:"relays,omitempty"`
}

func SignNetworkMap(privateKey ed25519.PrivateKey, response RegisterNodeResponse) (*MapSignature, error) {
	return SignNetworkMapAt(privateKey, response, time.Now().UTC(), DefaultNetworkMapSignatureTTL)
}

func SignNetworkMapAt(privateKey ed25519.PrivateKey, response RegisterNodeResponse, issuedAt time.Time, ttl time.Duration) (*MapSignature, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid map signing private key")
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("invalid map signing public key")
	}
	return SignNetworkMapWithProvider(publicKey, response, issuedAt, ttl, func(payload []byte) ([]byte, error) {
		return ed25519.Sign(privateKey, payload), nil
	})
}

func SignNetworkMapWithProvider(publicKey ed25519.PublicKey, response RegisterNodeResponse, issuedAt time.Time, ttl time.Duration, signer func([]byte) ([]byte, error)) (*MapSignature, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, errors.New("invalid map signing public key")
	}
	if signer == nil {
		return nil, errors.New("map signing provider is required")
	}
	if ttl <= 0 {
		return nil, errors.New("network map signature ttl must be positive")
	}
	issuedAt = issuedAt.UTC().Truncate(time.Second)
	expiresAt := issuedAt.Add(ttl).UTC()
	publicKeyText := base64.RawURLEncoding.EncodeToString(publicKey)
	keyID, err := SigningKeyID(publicKeyText)
	if err != nil {
		return nil, err
	}
	payload, err := canonicalNetworkMapPayload(response, keyID, issuedAt, expiresAt)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(payload)
	signature, err := signer(payload)
	if err != nil {
		return nil, fmt.Errorf("sign network map: %w", err)
	}
	if len(signature) != ed25519.SignatureSize {
		return nil, errors.New("map signing provider returned an invalid signature")
	}
	return &MapSignature{
		Version:     mapSignatureVersion,
		KeyID:       keyID,
		Algorithm:   mapSignatureAlgorithm,
		IssuedAt:    issuedAt,
		ExpiresAt:   expiresAt,
		PayloadHash: hex.EncodeToString(sum[:]),
		Signature:   base64.RawURLEncoding.EncodeToString(signature),
	}, nil
}

func VerifyNetworkMapSignatureWithTrustBundle(response RegisterNodeResponse, trust SigningTrustBundle) error {
	if response.MapSignature == nil {
		return errors.New("network map signature is missing")
	}
	key, err := trust.Resolve(response.MapSignature.KeyID, time.Now().UTC())
	if err != nil {
		return err
	}
	return VerifyNetworkMapSignatureAt(response, key.PublicKey, time.Now().UTC())
}

func VerifyNetworkMapSignatureAt(response RegisterNodeResponse, expectedPublicKey string, now time.Time) error {
	if response.MapSignature == nil {
		return errors.New("network map signature is missing")
	}
	signature := response.MapSignature
	if signature.Version != mapSignatureVersion {
		return errors.New("unsupported network map signature version")
	}
	if signature.Algorithm != mapSignatureAlgorithm {
		return errors.New("unsupported network map signature algorithm")
	}
	publicKey := strings.TrimSpace(expectedPublicKey)
	if publicKey == "" {
		return errors.New("network map signing trust anchor is required")
	}
	keyID, err := SigningKeyID(publicKey)
	if err != nil {
		return err
	}
	if strings.TrimSpace(signature.KeyID) == "" || signature.KeyID != keyID {
		return errors.New("network map signing key id mismatch")
	}
	publicKeyRaw, err := base64.RawURLEncoding.DecodeString(publicKey)
	if err != nil || len(publicKeyRaw) != ed25519.PublicKeySize {
		return errors.New("invalid network map signing public key")
	}
	signatureRaw, err := base64.RawURLEncoding.DecodeString(signature.Signature)
	if err != nil || len(signatureRaw) != ed25519.SignatureSize {
		return errors.New("invalid network map signature")
	}
	if signature.IssuedAt.IsZero() {
		return errors.New("network map signature issued_at is missing")
	}
	if signature.ExpiresAt.IsZero() {
		return errors.New("network map signature expires_at is missing")
	}
	issuedAt := signature.IssuedAt.UTC()
	expiresAt := signature.ExpiresAt.UTC()
	if !expiresAt.After(issuedAt) {
		return errors.New("network map signature expires_at must be after issued_at")
	}
	now = now.UTC()
	if issuedAt.After(now.Add(maxNetworkMapSignatureClockSkew)) {
		return errors.New("network map signature is not yet valid")
	}
	if !now.Before(expiresAt) {
		return errors.New("network map signature expired")
	}
	payload, err := canonicalNetworkMapPayload(response, signature.KeyID, issuedAt, expiresAt)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(payload)
	if signature.PayloadHash != hex.EncodeToString(sum[:]) {
		return errors.New("network map payload hash mismatch")
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKeyRaw), payload, signatureRaw) {
		return errors.New("network map signature verification failed")
	}
	return nil
}

func canonicalNetworkMapPayload(response RegisterNodeResponse, keyID string, issuedAt, expiresAt time.Time) ([]byte, error) {
	if strings.TrimSpace(keyID) == "" {
		return nil, errors.New("network map signing key id is required")
	}
	peers := append([]Peer(nil), response.Peers...)
	stunEndpoints := append([]STUNEndpoint(nil), response.STUNEndpoints...)
	relays := append([]relayauth.Endpoint(nil), response.Relays...)
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].ID == peers[j].ID {
			return peers[i].Hostname < peers[j].Hostname
		}
		return peers[i].ID < peers[j].ID
	})
	sort.Slice(relays, func(i, j int) bool {
		if relays[i].ID == relays[j].ID {
			return relays[i].Addr < relays[j].Addr
		}
		return relays[i].ID < relays[j].ID
	})
	sort.Slice(stunEndpoints, func(i, j int) bool {
		if stunEndpoints[i].ID == stunEndpoints[j].ID {
			return stunEndpoints[i].Addr < stunEndpoints[j].Addr
		}
		return stunEndpoints[i].ID < stunEndpoints[j].ID
	})
	payload := networkMapSigningPayload{
		Schema:              mapSignatureVersion,
		KeyID:               strings.TrimSpace(keyID),
		IssuedAt:            issuedAt.UTC(),
		ExpiresAt:           expiresAt.UTC(),
		RegistrationBinding: strings.TrimSpace(response.RegistrationBinding),
		Network:             response.Network,
		Node:                response.Node,
		Peers:               peers,
		STUNEndpoints:       stunEndpoints,
		Relays:              relays,
	}
	return json.Marshal(payload)
}
