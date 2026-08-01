package clientapi

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

const IdentityPublicKeyPrefix = "enp_"

func RegistrationIdentityProofPayload(req RegisterNodeRequest) []byte {
	canonical := struct {
		Proof                 string   `json:"proof"`
		NetworkID             string   `json:"network_id"`
		NetworkName           string   `json:"network_name"`
		AccountID             string   `json:"account_id"`
		CellID                string   `json:"cell_id"`
		IdempotencyKey        string   `json:"idempotency_key"`
		JoinTokenBinding      string   `json:"join_token_binding"`
		NodeCredentialBinding string   `json:"node_credential_binding"`
		SessionTokenBinding   string   `json:"session_token_binding"`
		Hostname              string   `json:"hostname"`
		IdentityPublicKey     string   `json:"identity_public_key"`
		PublicKey             string   `json:"public_key"`
		DeviceFingerprint     string   `json:"device_fingerprint"`
		Endpoint              string   `json:"endpoint"`
		EndpointGeneration    uint64   `json:"endpoint_generation"`
		EndpointCandidates    []string `json:"endpoint_candidates"`
		EndpointTTL           string   `json:"endpoint_ttl"`
		AdvertisedIPs         []string `json:"advertised_ips"`
		Tags                  []string `json:"tags"`
	}{
		Proof:                 "endlessnet-register-identity-v3",
		NetworkID:             strings.TrimSpace(req.NetworkID),
		NetworkName:           strings.TrimSpace(req.NetworkName),
		AccountID:             strings.TrimSpace(req.AccountID),
		CellID:                strings.TrimSpace(req.CellID),
		IdempotencyKey:        strings.TrimSpace(req.IdempotencyKey),
		JoinTokenBinding:      registrationSecretBinding(req.JoinToken),
		NodeCredentialBinding: registrationSecretBinding(req.NodeCredential),
		SessionTokenBinding:   strings.TrimSpace(req.SessionTokenBinding),
		Hostname:              strings.TrimSpace(req.Hostname),
		IdentityPublicKey:     strings.TrimSpace(req.IdentityPublicKey),
		PublicKey:             strings.TrimSpace(req.PublicKey),
		DeviceFingerprint:     strings.TrimSpace(req.DeviceFingerprint),
		Endpoint:              strings.TrimSpace(req.Endpoint),
		EndpointGeneration:    req.EndpointGeneration,
		EndpointCandidates:    append([]string(nil), req.EndpointCandidates...),
		EndpointTTL:           strings.TrimSpace(req.EndpointTTL),
		AdvertisedIPs:         append([]string(nil), req.AdvertisedIPs...),
		Tags:                  append([]string(nil), req.Tags...),
	}
	raw, _ := json.Marshal(canonical)
	return raw
}

// RegistrationSessionTokenBinding returns the response-safe digest used to
// bind a direct enrollment proof to the session that authorizes it.
func RegistrationSessionTokenBinding(token string) string {
	return registrationSecretBinding(token)
}

// RegistrationIdentityProofBinding is the response-safe digest of the exact
// enrollment payload authorized by the node identity key. Map signatures cover
// this value so a valid response from another enrollment cannot be replayed.
func RegistrationIdentityProofBinding(req RegisterNodeRequest) string {
	sum := sha256.Sum256(RegistrationIdentityProofPayload(req))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func VerifyRegisterNodeIdentityProof(req RegisterNodeRequest) error {
	identityPublicKey := strings.TrimSpace(req.IdentityPublicKey)
	if identityPublicKey == "" {
		if strings.TrimSpace(req.NodeCredential) != "" {
			return nil
		}
		return errors.New("identity_public_key is required for node enrollment")
	}
	return verifyRegisterNodeIdentityProof(req)
}

func verifyRegisterNodeIdentityProof(req RegisterNodeRequest) error {
	identityPublicKey := strings.TrimSpace(req.IdentityPublicKey)
	signature := strings.TrimSpace(req.IdentitySignature)
	if signature == "" {
		return errors.New("identity signature is required")
	}
	publicKey, err := DecodeIdentityPublicKey(identityPublicKey)
	if err != nil {
		return err
	}
	rawSignature, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || len(rawSignature) != ed25519.SignatureSize {
		return errors.New("invalid identity signature")
	}
	if !ed25519.Verify(publicKey, RegistrationIdentityProofPayload(req), rawSignature) {
		return errors.New("invalid identity signature")
	}
	return nil
}

func registrationSecretBinding(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

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
