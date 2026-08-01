package clientapi

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"reflect"
	"strings"
	"testing"
	"time"

	v1 "github.com/endless-net/client-api/clientapi/v1"
)

func TestRegisterNodeRenewalStrictRoundTrip(t *testing.T) {
	req := renewalRequestFixture(t)
	if !req.IsCredentialRenewal() {
		t.Fatal("fixture is not a credential renewal")
	}
	raw, err := MarshalRegisterNodeRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		`"schema_version":2`, `"idempotency_id":`, `"node_credential":`,
		`"registration_binding":`, `"identity_public_key":`, `"identity_signature":`,
		`"public_key":`, `"device_fingerprint":`,
	} {
		if !bytes.Contains(raw, []byte(field)) {
			t.Errorf("request JSON lacks %s: %s", field, raw)
		}
	}
	decoded, err := DecodeRegisterNodeRequest(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, req) {
		t.Fatalf("round trip = %#v, want %#v", decoded, req)
	}

	unknown := append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`,"legacy_idempotency_key":"old"}`)...)
	if _, err := DecodeRegisterNodeRequest(bytes.NewReader(unknown)); err == nil {
		t.Fatal("request decoder accepted an unknown legacy field")
	}
	if _, err := DecodeRegisterNodeRequest(strings.NewReader(string(raw) + `{}`)); err == nil {
		t.Fatal("request decoder accepted trailing JSON")
	}

	mutated := req
	mutated.IdempotencyID = "reg_ffffffffffffffffffffffffffffffff"
	if err := mutated.Validate(); err == nil || !strings.Contains(err.Error(), "identity signature") {
		t.Fatalf("mutated proof error = %v", err)
	}
}

func TestRegisterNodeEnrollmentUsesSessionBindingWithoutRenewalState(t *testing.T) {
	privateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, ed25519.SeedSize))
	req := renewalRequestFixture(t)
	req.NodeCredential = ""
	req.RegistrationBinding = ""
	req.SessionTokenBinding = RegistrationSessionTokenBinding("session-secret")
	if err := SetRegisterNodeIdentityProof(&req, privateKey); err != nil {
		t.Fatal(err)
	}
	if req.IsCredentialRenewal() {
		t.Fatal("enrollment request was classified as credential renewal")
	}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}

	mixed := renewalRequestFixture(t)
	mixed.SessionTokenBinding = RegistrationSessionTokenBinding("session-secret")
	if err := SetRegisterNodeIdentityProof(&mixed, privateKey); err != nil {
		t.Fatal(err)
	}
	if err := mixed.Validate(); err == nil || !strings.Contains(err.Error(), "must not mix") {
		t.Fatalf("mixed authorization error = %v", err)
	}
}

func TestRegistrationProofNeverContainsBearerMaterial(t *testing.T) {
	req := renewalRequestFixture(t)
	payload := RegistrationIdentityProofPayload(req)
	if bytes.Contains(payload, []byte(req.NodeCredential)) {
		t.Fatalf("proof payload contains node credential: %s", payload)
	}
	if !bytes.Contains(payload, []byte(`"node_credential_binding":`)) {
		t.Fatalf("proof payload lacks credential binding: %s", payload)
	}
}

func TestRegistrationSessionTokenBindingDoesNotExposeToken(t *testing.T) {
	token := "bearer-session-secret"
	binding := RegistrationSessionTokenBinding(token)
	if binding == "" || strings.Contains(binding, token) {
		t.Fatalf("session token binding = %q", binding)
	}
	if err := validateDigest("session_token_binding", binding, true); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterNodeResponseStrictRoundTripAndRequestBinding(t *testing.T) {
	req := renewalRequestFixture(t)
	response := renewalResponseFixture(t, req)
	if err := response.ValidateForRequest(req); err != nil {
		t.Fatal(err)
	}
	raw, err := MarshalRegisterNodeResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeRegisterNodeResponse(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, response) {
		t.Fatalf("round trip = %#v, want %#v", decoded, response)
	}
	if err := decoded.ValidateForRequest(req); err != nil {
		t.Fatal(err)
	}

	mismatch := response
	mismatch.IdempotencyID = "reg_ffffffffffffffffffffffffffffffff"
	if err := mismatch.ValidateForRequest(req); err == nil || !strings.Contains(err.Error(), "idempotency_id") {
		t.Fatalf("idempotency mismatch error = %v", err)
	}
	mismatch = response
	mismatch.RegistrationBinding = digestFixture(9)
	if err := mismatch.ValidateForRequest(req); err == nil || !strings.Contains(err.Error(), "registration_binding") {
		t.Fatalf("binding mismatch error = %v", err)
	}

	unknown := append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`,"result_text":"renewed"}`)...)
	if _, err := DecodeRegisterNodeResponse(bytes.NewReader(unknown)); err == nil {
		t.Fatal("response decoder accepted an unknown field")
	}
}

func TestRegistrationIdempotencyIDShape(t *testing.T) {
	first, err := NewRegistrationIdempotencyID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRegistrationIdempotencyID()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "reg_") {
		t.Fatalf("generated ids = %q and %q", first, second)
	}
	if err := validateIdentifier("idempotency_id", first, 16, maxIdentifierBytes); err != nil {
		t.Fatal(err)
	}
}

func renewalRequestFixture(t *testing.T) RegisterNodeRequest {
	t.Helper()
	privateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, ed25519.SeedSize))
	req := RegisterNodeRequest{
		SchemaVersion:       SchemaVersion,
		IdempotencyID:       "reg_0123456789abcdef0123456789abcdef",
		NetworkID:           "network-1",
		NetworkName:         "production",
		AccountID:           "account-1",
		CellID:              "cell-1",
		NodeCredential:      "enc_old-credential",
		RegistrationBinding: digestFixture(2),
		Hostname:            "workstation-1",
		ClientVersion:       "2.0.0",
		PublicKey:           base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32)),
		DeviceFingerprint:   "device-fingerprint-1",
		Endpoint:            "192.0.2.10:51820",
		EndpointGeneration:  7,
		EndpointCandidates:  []string{"192.0.2.10:51820", "198.51.100.10:51820"},
		EndpointTTL:         (2 * time.Minute).String(),
		AdvertisedIPs:       []string{"10.20.0.0/16"},
		Tags:                []string{"tag:workstation"},
	}
	if err := SetRegisterNodeIdentityProof(&req, privateKey); err != nil {
		t.Fatal(err)
	}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	return req
}

func renewalResponseFixture(t *testing.T, req RegisterNodeRequest) RegisterNodeResponse {
	t.Helper()
	signingKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{4}, ed25519.SeedSize))
	credential, err := v1.SignNodeCredential(
		signingKey,
		req.NetworkID,
		"node-1",
		[]string{"node:register", "node:map"},
		time.Now().UTC().Add(24*time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	return RegisterNodeResponse{
		SchemaVersion: SchemaVersion,
		IdempotencyID: req.IdempotencyID,
		Revision:      MapRevision{Network: 8, Global: 3},
		Network: Network{
			ID: req.NetworkID, Name: req.NetworkName, CIDR: "100.64.0.0/24",
			Revision: 8, OwnerID: "owner-1", AccountID: req.AccountID,
		},
		Node: Node{
			ID: "node-1", NetworkID: req.NetworkID, UserID: "user-1", Hostname: req.Hostname,
			IdentityPublicKey: req.IdentityPublicKey, PublicKey: req.PublicKey,
			DeviceFingerprint: req.DeviceFingerprint, AssignedIP: "100.64.0.10",
			Endpoint: req.Endpoint, EndpointGeneration: req.EndpointGeneration,
			EndpointCandidates: append([]string(nil), req.EndpointCandidates...), Status: "online",
		},
		Peers:               []Peer{},
		RegistrationBinding: RegistrationIdentityProofBinding(req),
		NodeCredential:      credential,
		STUNEndpoints:       nil,
		Relays:              nil,
		MapSignature: &MapSignature{
			Version: 5, KeyID: "map-key-1", Algorithm: "ed25519-network-map-v5",
			IssuedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour),
			PayloadHash: "hash", Signature: "signature",
		},
	}
}

func digestFixture(value byte) string {
	return base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{value}, 32))
}
