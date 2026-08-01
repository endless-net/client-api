// Package clientapi defines the v2 public recovery contract shared by the
// EndlessNet control plane and independently released clients.
package clientapi

import (
	v1 "github.com/endless-net/client-api/clientapi/v1"
	relayauth "github.com/endless-net/relay/protocol/v1"
)

// SchemaVersion is carried by every v2 recovery DTO.
const SchemaVersion = 2

// Unchanged network-map types remain wire-compatible with clientapi/v1. The
// aliases let v2 registration responses use the established signed-map shape
// without maintaining a second copy of that contract.
type (
	Network              = v1.Network
	Node                 = v1.Node
	Peer                 = v1.Peer
	ACLPort              = v1.ACLPort
	STUNEndpoint         = v1.STUNEndpoint
	MapRevision          = v1.MapRevision
	MapSignature         = v1.MapSignature
	NodeCredentialClaims = v1.NodeCredentialClaims
)

// RegisterNodeRequest is the v2 request body for POST /nodes/register.
//
// A request with NodeCredential set is an idempotent credential renewal of the
// existing node. IdempotencyID must be durably stored by the caller and reused
// with the same old credential until a validated response is committed.
type RegisterNodeRequest struct {
	SchemaVersion       int      `json:"schema_version"`
	IdempotencyID       string   `json:"idempotency_id"`
	NetworkID           string   `json:"network_id"`
	NetworkName         string   `json:"network_name"`
	AccountID           string   `json:"account_id,omitempty"`
	CellID              string   `json:"cell_id,omitempty"`
	JoinToken           string   `json:"join_token,omitempty"`
	NodeCredential      string   `json:"node_credential,omitempty"`
	RegistrationBinding string   `json:"registration_binding,omitempty"`
	SessionTokenBinding string   `json:"session_token_binding,omitempty"`
	Hostname            string   `json:"hostname"`
	ClientVersion       string   `json:"client_version,omitempty"`
	IdentityPublicKey   string   `json:"identity_public_key"`
	IdentitySignature   string   `json:"identity_signature"`
	PublicKey           string   `json:"public_key"`
	DeviceFingerprint   string   `json:"device_fingerprint"`
	Endpoint            string   `json:"endpoint,omitempty"`
	EndpointGeneration  uint64   `json:"generation,omitempty"`
	EndpointCandidates  []string `json:"candidates,omitempty"`
	EndpointTTL         string   `json:"ttl,omitempty"`
	AdvertisedIPs       []string `json:"advertised_ips"`
	Tags                []string `json:"tags"`
}

// IsCredentialRenewal reports whether the registration request renews an
// existing node credential instead of enrolling a new node.
func (r RegisterNodeRequest) IsCredentialRenewal() bool {
	return r.NodeCredential != ""
}

// RegisterNodeResponse is the v2 success body for POST /nodes/register.
// IdempotencyID echoes the request identity and identifies the durable result
// returned by every retry of the same operation.
type RegisterNodeResponse struct {
	SchemaVersion       int                   `json:"schema_version"`
	IdempotencyID       string                `json:"idempotency_id"`
	Revision            MapRevision           `json:"revision"`
	Network             Network               `json:"network"`
	Node                Node                  `json:"node"`
	Peers               []Peer                `json:"peers"`
	RegistrationBinding string                `json:"registration_binding"`
	NodeCredential      string                `json:"node_credential"`
	STUNEndpoints       []STUNEndpoint        `json:"stun_endpoints,omitempty"`
	Relays              []relayauth.Endpoint  `json:"relays"`
	RelayCredential     *relayauth.Credential `json:"relay_credential,omitempty"`
	MapSignature        *MapSignature         `json:"map_signature"`
}

// NetworkMap returns the established v1 signed-map projection carried by a v2
// registration response. It does not downgrade or decode a v2 wire response.
func (r RegisterNodeResponse) NetworkMap() v1.RegisterNodeResponse {
	return v1.RegisterNodeResponse{
		Revision:            r.Revision,
		Network:             r.Network,
		Node:                r.Node,
		Peers:               r.Peers,
		RegistrationBinding: r.RegistrationBinding,
		NodeCredential:      r.NodeCredential,
		STUNEndpoints:       r.STUNEndpoints,
		Relays:              r.Relays,
		RelayCredential:     r.RelayCredential,
		MapSignature:        r.MapSignature,
	}
}
