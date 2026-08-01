package clientapi

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	v1 "github.com/endless-net/client-api/clientapi/v1"
	wgkeys "github.com/endless-net/client-api/clientapi/wireguard"
)

const (
	registrationProofDomain   = "endlessnet-register-identity-v4"
	idempotencyEntropyBytes   = 32
	maxIdentifierBytes        = 128
	maxRequestIDBytes         = 128
	maxDiagnosticMessageBytes = 1024
	maxOpaqueCredentialBytes  = 16 << 10
	maxSafeFieldBytes         = 1024
	maxEndpointCandidates     = 32
	maxAdvertisedIPs          = 256
	maxTags                   = 256
	maxEndpointTTL            = 24 * time.Hour
)

// NewRegistrationIdempotencyID returns a high-entropy operation identity. The
// caller must persist it before sending a request and reuse it for every retry.
func NewRegistrationIdempotencyID() (string, error) {
	raw := make([]byte, idempotencyEntropyBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "reg_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

// RegistrationSessionTokenBinding returns the response-safe digest used to
// bind a direct enrollment proof to the session that authorizes it.
func RegistrationSessionTokenBinding(token string) string {
	return secretBinding(token)
}

// RegistrationIdentityProofPayload is the canonical v2 payload signed by the
// device identity key. Raw bearer values are represented only by digests.
func RegistrationIdentityProofPayload(req RegisterNodeRequest) []byte {
	canonical := struct {
		SchemaVersion         int      `json:"schema_version"`
		Proof                 string   `json:"proof"`
		IdempotencyID         string   `json:"idempotency_id"`
		NetworkID             string   `json:"network_id"`
		NetworkName           string   `json:"network_name"`
		AccountID             string   `json:"account_id"`
		CellID                string   `json:"cell_id"`
		JoinTokenBinding      string   `json:"join_token_binding"`
		NodeCredentialBinding string   `json:"node_credential_binding"`
		RegistrationBinding   string   `json:"registration_binding"`
		SessionTokenBinding   string   `json:"session_token_binding"`
		Hostname              string   `json:"hostname"`
		ClientVersion         string   `json:"client_version"`
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
		SchemaVersion:         req.SchemaVersion,
		Proof:                 registrationProofDomain,
		IdempotencyID:         strings.TrimSpace(req.IdempotencyID),
		NetworkID:             strings.TrimSpace(req.NetworkID),
		NetworkName:           strings.TrimSpace(req.NetworkName),
		AccountID:             strings.TrimSpace(req.AccountID),
		CellID:                strings.TrimSpace(req.CellID),
		JoinTokenBinding:      secretBinding(req.JoinToken),
		NodeCredentialBinding: secretBinding(req.NodeCredential),
		RegistrationBinding:   strings.TrimSpace(req.RegistrationBinding),
		SessionTokenBinding:   strings.TrimSpace(req.SessionTokenBinding),
		Hostname:              strings.TrimSpace(req.Hostname),
		ClientVersion:         strings.TrimSpace(req.ClientVersion),
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

// RegistrationIdentityProofBinding is the response-safe digest of the exact
// request authorized by the device identity key.
func RegistrationIdentityProofBinding(req RegisterNodeRequest) string {
	sum := sha256.Sum256(RegistrationIdentityProofPayload(req))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// SetRegisterNodeIdentityProof fills the public key and signature using the
// supplied device identity private key.
func SetRegisterNodeIdentityProof(req *RegisterNodeRequest, privateKey ed25519.PrivateKey) error {
	if req == nil {
		return errors.New("register node request is required")
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return errors.New("invalid identity private key")
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return errors.New("invalid identity public key")
	}
	req.IdentityPublicKey = v1.IdentityPublicKeyPrefix + base64.RawURLEncoding.EncodeToString(publicKey)
	req.IdentitySignature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, RegistrationIdentityProofPayload(*req)))
	return nil
}

// VerifyRegisterNodeIdentityProof verifies the canonical v2 device proof.
func VerifyRegisterNodeIdentityProof(req RegisterNodeRequest) error {
	publicKey, err := v1.DecodeIdentityPublicKey(req.IdentityPublicKey)
	if err != nil {
		return err
	}
	encoded := strings.TrimSpace(req.IdentitySignature)
	signature, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(signature) != ed25519.SignatureSize || base64.RawURLEncoding.EncodeToString(signature) != encoded {
		return errors.New("invalid identity signature")
	}
	if !ed25519.Verify(publicKey, RegistrationIdentityProofPayload(req), signature) {
		return errors.New("invalid identity signature")
	}
	return nil
}

// Validate checks v2 registration shape and proof semantics. Credential
// verification, revocation, expiry, scope, and authoritative binding lookup
// remain producer responsibilities.
func (r RegisterNodeRequest) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("register request schema_version = %d, want %d", r.SchemaVersion, SchemaVersion)
	}
	if err := validateIdentifier("idempotency_id", r.IdempotencyID, 16, maxIdentifierBytes); err != nil {
		return err
	}
	if strings.TrimSpace(r.NetworkID) == "" && strings.TrimSpace(r.NetworkName) == "" {
		return errors.New("network_id or network_name is required")
	}
	for field, value := range map[string]string{
		"network_id": r.NetworkID, "network_name": r.NetworkName, "account_id": r.AccountID,
		"cell_id": r.CellID, "client_version": r.ClientVersion, "device_fingerprint": r.DeviceFingerprint,
	} {
		if err := validateSafeString(field, value, 0, maxSafeFieldBytes); err != nil {
			return err
		}
	}
	if strings.TrimSpace(r.DeviceFingerprint) == "" {
		return errors.New("device_fingerprint is required")
	}
	if err := wgkeys.ValidateHostname(r.Hostname); err != nil {
		return fmt.Errorf("hostname: %w", err)
	}
	if err := wgkeys.ValidatePublicKey(r.PublicKey); err != nil {
		return fmt.Errorf("public_key: %w", err)
	}
	if r.Endpoint != "" {
		if err := wgkeys.ValidateEndpoint(r.Endpoint); err != nil {
			return fmt.Errorf("endpoint: %w", err)
		}
	}
	if len(r.EndpointCandidates) > maxEndpointCandidates {
		return fmt.Errorf("candidates contains %d values, maximum is %d", len(r.EndpointCandidates), maxEndpointCandidates)
	}
	for i, endpoint := range r.EndpointCandidates {
		if err := wgkeys.ValidateEndpoint(endpoint); err != nil {
			return fmt.Errorf("candidates[%d]: %w", i, err)
		}
	}
	if r.EndpointTTL != "" {
		ttl, err := time.ParseDuration(r.EndpointTTL)
		if err != nil || ttl <= 0 || ttl > maxEndpointTTL || ttl.String() != r.EndpointTTL {
			return fmt.Errorf("ttl must be a canonical positive duration no greater than %s", maxEndpointTTL)
		}
	}
	if len(r.AdvertisedIPs) > maxAdvertisedIPs {
		return fmt.Errorf("advertised_ips contains %d values, maximum is %d", len(r.AdvertisedIPs), maxAdvertisedIPs)
	}
	for i, prefix := range r.AdvertisedIPs {
		if err := wgkeys.ValidatePrefix(fmt.Sprintf("advertised_ips[%d]", i), prefix); err != nil {
			return err
		}
	}
	if len(r.Tags) > maxTags {
		return fmt.Errorf("tags contains %d values, maximum is %d", len(r.Tags), maxTags)
	}
	for i, tag := range r.Tags {
		if err := validateSafeString(fmt.Sprintf("tags[%d]", i), tag, 1, maxSafeFieldBytes); err != nil {
			return err
		}
	}
	if err := validateOpaqueCredential("join_token", r.JoinToken); err != nil {
		return err
	}
	if err := validateOpaqueCredential("node_credential", r.NodeCredential); err != nil {
		return err
	}
	if r.IsCredentialRenewal() {
		if strings.TrimSpace(r.NetworkID) == "" {
			return errors.New("network_id is required for credential renewal")
		}
		if r.JoinToken != "" || r.SessionTokenBinding != "" {
			return errors.New("credential renewal must not mix node credential with enrollment authorization")
		}
		if err := validateDigest("registration_binding", r.RegistrationBinding, true); err != nil {
			return err
		}
	} else {
		if r.RegistrationBinding != "" {
			return errors.New("registration_binding is only valid for credential renewal")
		}
		if r.JoinToken == "" && r.SessionTokenBinding == "" {
			return errors.New("join_token or session_token_binding is required for enrollment")
		}
	}
	if err := validateDigest("session_token_binding", r.SessionTokenBinding, false); err != nil {
		return err
	}
	if _, err := v1.DecodeIdentityPublicKey(r.IdentityPublicKey); err != nil {
		return err
	}
	return VerifyRegisterNodeIdentityProof(r)
}

// Validate checks a standalone v2 registration result.
func (r RegisterNodeResponse) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("register response schema_version = %d, want %d", r.SchemaVersion, SchemaVersion)
	}
	if err := validateIdentifier("idempotency_id", r.IdempotencyID, 16, maxIdentifierBytes); err != nil {
		return err
	}
	if err := validateDigest("registration_binding", r.RegistrationBinding, true); err != nil {
		return err
	}
	if err := validateOpaqueCredential("node_credential", r.NodeCredential); err != nil {
		return err
	}
	if r.NodeCredential == "" {
		return errors.New("node_credential is required")
	}
	claims, err := v1.DecodeNodeCredential(r.NodeCredential)
	if err != nil {
		return fmt.Errorf("node_credential: %w", err)
	}
	if claims.NetworkID != strings.TrimSpace(r.Network.ID) || claims.NodeID != strings.TrimSpace(r.Node.ID) {
		return errors.New("node_credential identity does not match registration result")
	}
	if r.MapSignature == nil {
		return errors.New("map_signature is required")
	}
	if err := v1.ValidateNetworkMap(r.NetworkMap()); err != nil {
		return fmt.Errorf("network map: %w", err)
	}
	return nil
}

// ValidateForRequest additionally proves that a success is the durable result
// for the exact request and binding the caller is about to commit.
func (r RegisterNodeResponse) ValidateForRequest(req RegisterNodeRequest) error {
	if err := req.Validate(); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	if err := r.Validate(); err != nil {
		return err
	}
	if r.IdempotencyID != req.IdempotencyID {
		return errors.New("response idempotency_id does not match request")
	}
	if r.RegistrationBinding != RegistrationIdentityProofBinding(req) {
		return errors.New("response registration_binding does not match request proof")
	}
	if req.NetworkID != "" && r.Network.ID != req.NetworkID {
		return errors.New("response network does not match request")
	}
	if r.Node.NetworkID != r.Network.ID || r.Node.IdentityPublicKey != req.IdentityPublicKey ||
		r.Node.PublicKey != req.PublicKey || r.Node.DeviceFingerprint != req.DeviceFingerprint {
		return errors.New("response node binding does not match request")
	}
	return nil
}

func secretBinding(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func validateDigest(field, value string, required bool) error {
	if value == "" && !required {
		return nil
	}
	if value != strings.TrimSpace(value) || value == "" {
		return fmt.Errorf("%s must not be empty or contain surrounding whitespace", field)
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(raw) != sha256.Size || base64.RawURLEncoding.EncodeToString(raw) != value {
		return fmt.Errorf("%s must be canonical base64url encoding of a SHA-256 digest", field)
	}
	return nil
}

func validateOpaqueCredential(field, value string) error {
	if len(value) > maxOpaqueCredentialBytes {
		return fmt.Errorf("%s exceeds %d bytes", field, maxOpaqueCredentialBytes)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("%s contains surrounding whitespace", field)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s contains invalid UTF-8", field)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s contains a control character", field)
		}
	}
	return nil
}

func validateIdentifier(field, value string, minBytes, maxBytes int) error {
	if len(value) < minBytes || len(value) > maxBytes || value != strings.TrimSpace(value) {
		return fmt.Errorf("%s must contain between %d and %d canonical bytes", field, minBytes, maxBytes)
	}
	for _, c := range []byte(value) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '-' || c == '_' || c == '.' || c == ':' {
			continue
		}
		return fmt.Errorf("%s contains an invalid character", field)
	}
	return nil
}

func validateSafeString(field, value string, minBytes, maxBytes int) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s contains invalid UTF-8", field)
	}
	if value != strings.TrimSpace(value) || len(value) < minBytes || len(value) > maxBytes {
		return fmt.Errorf("%s must contain between %d and %d bytes without surrounding whitespace", field, minBytes, maxBytes)
	}
	for _, r := range value {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return fmt.Errorf("%s contains a control character", field)
		}
	}
	return nil
}
