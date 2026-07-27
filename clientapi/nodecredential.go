package clientapi

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"time"
)

const (
	nodeCredentialVersion       = 3
	nodeCredentialAlgorithm     = "ed25519-node-credential-v3"
	nodeCredentialPrefix        = "enc_"
	nodeCredentialTTL           = 365 * 24 * time.Hour
	nodeCredentialScopeRegister = "node:register"
	nodeCredentialScopeMap      = "node:map"
	nodeCredentialScopeEndpoint = "node:endpoint"
	nodeCredentialScopeDelete   = "node:delete"
)

type NodeCredentialClaims struct {
	Version   int       `json:"version"`
	KeyID     string    `json:"key_id"`
	Algorithm string    `json:"algorithm"`
	NetworkID string    `json:"network_id"`
	NodeID    string    `json:"node_id"`
	Scopes    []string  `json:"scopes"`
	ExpiresAt time.Time `json:"expires_at"`
	Signature string    `json:"signature"`
}

type nodeCredentialPayload struct {
	Schema    int       `json:"schema"`
	KeyID     string    `json:"key_id"`
	NetworkID string    `json:"network_id"`
	NodeID    string    `json:"node_id"`
	Scopes    []string  `json:"scopes"`
	ExpiresAt time.Time `json:"expires_at"`
}

func SignNodeCredential(privateKey ed25519.PrivateKey, networkID, nodeID string, scopes []string, expiresAt time.Time) (string, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", errors.New("invalid node credential private key")
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return "", errors.New("invalid node credential public key")
	}
	return SignNodeCredentialWithProvider(publicKey, networkID, nodeID, scopes, expiresAt, func(payload []byte) ([]byte, error) {
		return ed25519.Sign(privateKey, payload), nil
	})
}

func SignNodeCredentialWithProvider(publicKey ed25519.PublicKey, networkID, nodeID string, scopes []string, expiresAt time.Time, signer func([]byte) ([]byte, error)) (string, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return "", errors.New("invalid node credential public key")
	}
	if signer == nil {
		return "", errors.New("node credential signing provider is required")
	}
	publicKeyText := base64.RawURLEncoding.EncodeToString(publicKey)
	keyID, err := SigningKeyID(publicKeyText)
	if err != nil {
		return "", err
	}
	payload, err := nodeCredentialSigningPayload(keyID, networkID, nodeID, scopes, expiresAt)
	if err != nil {
		return "", err
	}
	signature, err := signer(payload)
	if err != nil {
		return "", err
	}
	if len(signature) != ed25519.SignatureSize {
		return "", errors.New("node credential signing provider returned an invalid signature")
	}
	claims := NodeCredentialClaims{
		Version:   nodeCredentialVersion,
		KeyID:     keyID,
		Algorithm: nodeCredentialAlgorithm,
		NetworkID: strings.TrimSpace(networkID),
		NodeID:    strings.TrimSpace(nodeID),
		Scopes:    cleanCredentialScopes(scopes),
		ExpiresAt: expiresAt.UTC(),
		Signature: base64.RawURLEncoding.EncodeToString(signature),
	}
	raw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return nodeCredentialPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeNodeCredential(credential string) (NodeCredentialClaims, error) {
	encoded := strings.TrimSpace(credential)
	if !strings.HasPrefix(encoded, nodeCredentialPrefix) {
		return NodeCredentialClaims{}, errors.New("unsupported node credential format")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(encoded, nodeCredentialPrefix))
	if err != nil {
		return NodeCredentialClaims{}, errors.New("invalid node credential encoding")
	}
	var claims NodeCredentialClaims
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&claims); err != nil {
		return NodeCredentialClaims{}, errors.New("invalid node credential payload")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return NodeCredentialClaims{}, errors.New("invalid node credential payload")
	}
	if claims.Algorithm != nodeCredentialAlgorithm {
		return NodeCredentialClaims{}, errors.New("unsupported node credential algorithm")
	}
	if claims.Version != nodeCredentialVersion {
		return NodeCredentialClaims{}, errors.New("unsupported node credential version")
	}
	if strings.TrimSpace(claims.KeyID) == "" {
		return NodeCredentialClaims{}, errors.New("node credential key id is missing")
	}
	return claims, nil
}

func ValidateNodeCredential(credential, trustedPublicKey, requiredScope string, now time.Time) (NodeCredentialClaims, error) {
	claims, err := DecodeNodeCredential(credential)
	if err != nil {
		return NodeCredentialClaims{}, err
	}
	if strings.TrimSpace(claims.NetworkID) == "" || strings.TrimSpace(claims.NodeID) == "" {
		return NodeCredentialClaims{}, errors.New("node credential is missing identity")
	}
	if !now.Before(claims.ExpiresAt) {
		return NodeCredentialClaims{}, errors.New("node credential is expired")
	}
	if required := strings.TrimSpace(requiredScope); required != "" && !CredentialHasScope(claims.Scopes, required) {
		return NodeCredentialClaims{}, errors.New("node credential is missing required scope")
	}
	publicKey := strings.TrimSpace(trustedPublicKey)
	if publicKey == "" {
		return NodeCredentialClaims{}, errors.New("node credential trust anchor is required")
	}
	keyID, err := SigningKeyID(publicKey)
	if err != nil {
		return NodeCredentialClaims{}, err
	}
	if claims.KeyID != keyID {
		return NodeCredentialClaims{}, errors.New("node credential key id mismatch")
	}
	publicKeyRaw, err := base64.RawURLEncoding.DecodeString(publicKey)
	if err != nil || len(publicKeyRaw) != ed25519.PublicKeySize {
		return NodeCredentialClaims{}, errors.New("invalid node credential public key")
	}
	signatureRaw, err := base64.RawURLEncoding.DecodeString(claims.Signature)
	if err != nil || len(signatureRaw) != ed25519.SignatureSize {
		return NodeCredentialClaims{}, errors.New("invalid node credential signature")
	}
	payload, err := nodeCredentialSigningPayload(claims.KeyID, claims.NetworkID, claims.NodeID, claims.Scopes, claims.ExpiresAt)
	if err != nil {
		return NodeCredentialClaims{}, err
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKeyRaw), payload, signatureRaw) {
		return NodeCredentialClaims{}, errors.New("node credential signature verification failed")
	}
	return claims, nil
}

func VerifyNodeCredentialWithTrustBundle(credential string, trust SigningTrustBundle, requiredScope string, now time.Time) (NodeCredentialClaims, error) {
	claims, err := DecodeNodeCredential(credential)
	if err != nil {
		return NodeCredentialClaims{}, err
	}
	key, err := trust.Resolve(claims.KeyID, now)
	if err != nil {
		return NodeCredentialClaims{}, err
	}
	return ValidateNodeCredential(credential, key.PublicKey, requiredScope, now)
}

func IsSignedNodeCredential(credential string) bool {
	_, err := DecodeNodeCredential(credential)
	return err == nil
}

func nodeCredentialSigningPayload(keyID, networkID, nodeID string, scopes []string, expiresAt time.Time) ([]byte, error) {
	if strings.TrimSpace(keyID) == "" {
		return nil, errors.New("node credential key id is required")
	}
	if strings.TrimSpace(networkID) == "" || strings.TrimSpace(nodeID) == "" {
		return nil, errors.New("node credential identity is required")
	}
	cleanScopes := cleanCredentialScopes(scopes)
	if len(cleanScopes) == 0 {
		return nil, errors.New("node credential scopes are required")
	}
	return json.Marshal(nodeCredentialPayload{
		Schema:    nodeCredentialVersion,
		KeyID:     strings.TrimSpace(keyID),
		NetworkID: strings.TrimSpace(networkID),
		NodeID:    strings.TrimSpace(nodeID),
		Scopes:    cleanScopes,
		ExpiresAt: expiresAt.UTC(),
	})
}

func cleanCredentialScopes(scopes []string) []string {
	out := cleanStrings(scopes)
	sort.Strings(out)
	return out
}

func CredentialHasScope(scopes []string, scope string) bool {
	scope = strings.TrimSpace(scope)
	for _, candidate := range scopes {
		if candidate == scope {
			return true
		}
	}
	return false
}

func cleanStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
