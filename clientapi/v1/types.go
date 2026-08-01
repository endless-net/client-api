// Package clientapi defines the versioned HTTP and wire contract shared by
// EndlessNet control-plane producers and remote clients.
package clientapi

import (
	"strings"
	"time"

	relayauth "github.com/endless-net/relay/protocol/v1"
)

const (
	MapStreamProtocolVersion       = 3
	MapStreamCapabilitySnapshot    = "snapshot"
	MapStreamCapabilitySignedDelta = "signed-delta"
	MapStreamCapabilityCheckpoint  = "checkpoint"
	MapStreamCapabilityResync      = "resync"
	MapStreamCapabilityHeartbeat   = "heartbeat"
	NodeStatusOnline               = "online"
	NodeStatusOffline              = "offline"
	NodeApprovalPending            = "pending"
	NodeApprovalApproved           = "approved"
	NodeApprovalRejected           = "rejected"
	NodeEnrollmentRequestPending   = "pending"
	NodeEnrollmentRequestApproved  = "approved"
	NodeEnrollmentRequestRejected  = "rejected"
	NodeEnrollmentRequestExpired   = "expired"
	NodeEnrollmentRequestEnrolled  = "enrolled"
)

func MapStreamSupportedCapabilities() []string {
	return []string{
		MapStreamCapabilitySnapshot,
		MapStreamCapabilitySignedDelta,
		MapStreamCapabilityCheckpoint,
		MapStreamCapabilityResync,
		MapStreamCapabilityHeartbeat,
	}
}

type Network struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	CIDR                string    `json:"cidr"`
	IPv6CIDR            string    `json:"ipv6_cidr,omitempty"`
	DNS                 []string  `json:"dns,omitempty"`
	CellID              string    `json:"cell_id,omitempty"`
	AuthoritativeCellID string    `json:"authoritative_cell_id,omitempty"`
	MigratingToCellID   string    `json:"migrating_to_cell_id,omitempty"`
	Revision            uint64    `json:"revision"`
	OwnerID             string    `json:"owner_id"`
	AccountID           string    `json:"account_id,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

type Node struct {
	ID                 string     `json:"id"`
	NetworkID          string     `json:"network_id"`
	UserID             string     `json:"user_id"`
	Hostname           string     `json:"hostname"`
	IdentityPublicKey  string     `json:"identity_public_key,omitempty"`
	PublicKey          string     `json:"public_key"`
	DeviceFingerprint  string     `json:"device_fingerprint,omitempty"`
	AssignedIP         string     `json:"assigned_ip"`
	AssignedIPv6       string     `json:"assigned_ipv6,omitempty"`
	Endpoint           string     `json:"endpoint,omitempty"`
	EndpointGeneration uint64     `json:"endpoint_generation,omitempty"`
	EndpointCandidates []string   `json:"endpoint_candidates,omitempty"`
	EndpointExpiresAt  *time.Time `json:"endpoint_expires_at,omitempty"`
	Status             string     `json:"status"`
	AdvertisedIPs      []string   `json:"advertised_ips,omitempty"`
	RequestedTags      []string   `json:"requested_tags,omitempty"`
	Tags               []string   `json:"tags,omitempty"`
	ApprovalState      string     `json:"approval_state"`
	Ephemeral          bool       `json:"ephemeral"`
	KeyExpiryEnabled   bool       `json:"key_expiry_enabled"`
	KeyExpiresAt       *time.Time `json:"key_expires_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastSeen           time.Time  `json:"last_seen"`
}

type CreateNetworkRequest struct {
	Name           string   `json:"name"`
	CIDR           string   `json:"cidr"`
	IPv6CIDR       string   `json:"ipv6_cidr,omitempty"`
	DNS            []string `json:"dns"`
	CellID         string   `json:"cell_id,omitempty"`
	AccountID      string   `json:"account_id,omitempty"`
	IdempotencyKey string   `json:"idempotency_key,omitempty"`
}

type CreateJoinTokenRequest struct {
	NetworkID      string   `json:"network_id"`
	NetworkName    string   `json:"network_name"`
	AccountID      string   `json:"account_id,omitempty"`
	TTL            string   `json:"ttl"`
	Reusable       bool     `json:"reusable,omitempty"`
	Ephemeral      bool     `json:"ephemeral,omitempty"`
	Preauthorized  bool     `json:"preauthorized,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	IdempotencyKey string   `json:"idempotency_key,omitempty"`
}

type CreateJoinTokenResponse struct {
	ID            string    `json:"id"`
	Token         string    `json:"token"`
	AccountID     string    `json:"account_id,omitempty"`
	NetworkID     string    `json:"network_id,omitempty"`
	NetworkName   string    `json:"network_name,omitempty"`
	Reusable      bool      `json:"reusable,omitempty"`
	Ephemeral     bool      `json:"ephemeral,omitempty"`
	Preauthorized bool      `json:"preauthorized,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type RegisterNodeRequest struct {
	NetworkID           string   `json:"network_id"`
	NetworkName         string   `json:"network_name"`
	AccountID           string   `json:"account_id,omitempty"`
	CellID              string   `json:"cell_id,omitempty"`
	IdempotencyKey      string   `json:"idempotency_key,omitempty"`
	JoinToken           string   `json:"join_token,omitempty"`
	NodeCredential      string   `json:"node_credential,omitempty"`
	SessionTokenBinding string   `json:"session_token_binding,omitempty"`
	Hostname            string   `json:"hostname"`
	ClientVersion       string   `json:"client_version,omitempty"`
	IdentityPublicKey   string   `json:"identity_public_key,omitempty"`
	IdentitySignature   string   `json:"identity_signature,omitempty"`
	PublicKey           string   `json:"public_key"`
	DeviceFingerprint   string   `json:"device_fingerprint,omitempty"`
	Endpoint            string   `json:"endpoint"`
	EndpointGeneration  uint64   `json:"generation,omitempty"`
	EndpointCandidates  []string `json:"candidates,omitempty"`
	EndpointTTL         string   `json:"ttl,omitempty"`
	AdvertisedIPs       []string `json:"advertised_ips"`
	Tags                []string `json:"tags"`
}

type NodeEnrollmentRequest struct {
	ID                string     `json:"id"`
	Status            string     `json:"status"`
	AccountID         string     `json:"account_id,omitempty"`
	NetworkID         string     `json:"network_id,omitempty"`
	NetworkName       string     `json:"network_name,omitempty"`
	Hostname          string     `json:"hostname"`
	PublicKey         string     `json:"public_key"`
	IdentityPublicKey string     `json:"identity_public_key,omitempty"`
	DeviceFingerprint string     `json:"device_fingerprint,omitempty"`
	Tags              []string   `json:"tags,omitempty"`
	ApprovalURL       string     `json:"approval_url,omitempty"`
	ApprovedByUserID  string     `json:"approved_by_user_id,omitempty"`
	RejectedByUserID  string     `json:"rejected_by_user_id,omitempty"`
	RejectionReason   string     `json:"rejection_reason,omitempty"`
	NodeID            string     `json:"node_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	ExpiresAt         time.Time  `json:"expires_at"`
	ApprovedAt        *time.Time `json:"approved_at,omitempty"`
	RejectedAt        *time.Time `json:"rejected_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
}

type CreateNodeEnrollmentRequestResponse struct {
	Request          NodeEnrollmentRequest `json:"request"`
	PollToken        string                `json:"poll_token"`
	PollAfterSeconds int                   `json:"poll_after_seconds"`
}

type NodeEnrollmentRequestStatusResponse struct {
	Request          NodeEnrollmentRequest `json:"request"`
	PollAfterSeconds int                   `json:"poll_after_seconds,omitempty"`
}

type CompleteNodeEnrollmentRequestResponse struct {
	Request      NodeEnrollmentRequest `json:"request"`
	Registration *RegisterNodeResponse `json:"registration,omitempty"`
}

type UpdateNodeEndpointRequest struct {
	Endpoint      string   `json:"endpoint"`
	Generation    uint64   `json:"generation,omitempty"`
	Candidates    []string `json:"candidates,omitempty"`
	TTL           string   `json:"ttl,omitempty"`
	Status        string   `json:"status,omitempty"`
	ClientVersion string   `json:"client_version,omitempty"`
}

type AdvertisedRoute struct {
	NetworkID string    `json:"network_id"`
	NodeID    string    `json:"node_id"`
	Hostname  string    `json:"hostname"`
	CIDR      string    `json:"cidr"`
	Approved  bool      `json:"approved"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type SetAdvertisedRouteApprovalRequest struct {
	NodeID   string `json:"node_id"`
	CIDR     string `json:"cidr"`
	Approved bool   `json:"approved"`
}

type SetAdvertisedRouteApprovalResponse struct {
	Route   AdvertisedRoute `json:"route"`
	Network Network         `json:"network"`
}

type Peer struct {
	ID                 string     `json:"id"`
	Hostname           string     `json:"hostname"`
	PublicKey          string     `json:"public_key"`
	Endpoint           string     `json:"endpoint,omitempty"`
	EndpointGeneration uint64     `json:"endpoint_generation,omitempty"`
	EndpointCandidates []string   `json:"endpoint_candidates,omitempty"`
	EndpointExpiresAt  *time.Time `json:"endpoint_expires_at,omitempty"`
	Status             string     `json:"status,omitempty"`
	AllowedIPs         []string   `json:"allowed_ips"`
	AllowedPorts       []ACLPort  `json:"allowed_ports,omitempty"`
	ACLRestricted      bool       `json:"acl_restricted,omitempty"`
	Tags               []string   `json:"tags,omitempty"`
}

type ACLPort struct {
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
}

type STUNEndpoint struct {
	ID   string `json:"id"`
	Addr string `json:"addr"`
}

type RegisterNodeResponse struct {
	Revision            MapRevision           `json:"revision"`
	Network             Network               `json:"network"`
	Node                Node                  `json:"node"`
	Peers               []Peer                `json:"peers"`
	RegistrationBinding string                `json:"registration_binding,omitempty"`
	NodeCredential      string                `json:"node_credential,omitempty"`
	STUNEndpoints       []STUNEndpoint        `json:"stun_endpoints,omitempty"`
	Relays              []relayauth.Endpoint  `json:"relays"`
	RelayCredential     *relayauth.Credential `json:"relay_credential,omitempty"`
	MapSignature        *MapSignature         `json:"map_signature,omitempty"`
}

// NetworkMapSnapshot is the complete, signed client projection. Enrollment
// credentials are deliberately excluded so a snapshot can be cached safely.
type NetworkMapSnapshot struct {
	Revision            MapRevision           `json:"revision"`
	Network             Network               `json:"network"`
	Node                Node                  `json:"node"`
	Peers               []Peer                `json:"peers"`
	RegistrationBinding string                `json:"registration_binding,omitempty"`
	STUNEndpoints       []STUNEndpoint        `json:"stun_endpoints,omitempty"`
	Relays              []relayauth.Endpoint  `json:"relays"`
	RelayCredential     *relayauth.Credential `json:"relay_credential,omitempty"`
	MapSignature        *MapSignature         `json:"map_signature,omitempty"`
}

func (r RegisterNodeResponse) Snapshot() NetworkMapSnapshot {
	return NetworkMapSnapshot{
		Revision:            r.Revision,
		Network:             r.Network,
		Node:                r.Node,
		Peers:               r.Peers,
		RegistrationBinding: r.RegistrationBinding,
		STUNEndpoints:       r.STUNEndpoints,
		Relays:              r.Relays,
		RelayCredential:     r.RelayCredential,
		MapSignature:        r.MapSignature,
	}
}

type MapRevision struct {
	Network uint64 `json:"network"`
	Global  uint64 `json:"global"`
}

func (r MapRevision) Equal(other MapRevision) bool {
	return r.Network == other.Network && r.Global == other.Global
}

type MapCursor struct {
	Revision MapRevision `json:"revision"`
	MapHash  string      `json:"map_hash,omitempty"`
}

type MapStreamEvent struct {
	Type            string              `json:"type"`
	ProtocolVersion int                 `json:"protocol_version"`
	Capabilities    []string            `json:"capabilities"`
	EventID         string              `json:"event_id"`
	From            MapRevision         `json:"from"`
	To              MapRevision         `json:"to"`
	BaseHash        string              `json:"base_hash,omitempty"`
	Reason          string              `json:"reason,omitempty"`
	Snapshot        *NetworkMapSnapshot `json:"snapshot,omitempty"`
	Delta           *MapDelta           `json:"delta,omitempty"`
	ResultSignature *MapSignature       `json:"result_signature,omitempty"`
}

func (e MapStreamEvent) HasFullMap() bool {
	return e.Snapshot != nil &&
		strings.TrimSpace(e.Snapshot.Network.ID) != "" &&
		strings.TrimSpace(e.Snapshot.Node.ID) != ""
}

type MapDelta struct {
	Network         *Network              `json:"network,omitempty"`
	Node            *Node                 `json:"node,omitempty"`
	PeerUpserts     []Peer                `json:"peer_upserts,omitempty"`
	PeerRemoveIDs   []string              `json:"peer_remove_ids,omitempty"`
	STUNEndpoints   *[]STUNEndpoint       `json:"stun_endpoints,omitempty"`
	RelayUpserts    []relayauth.Endpoint  `json:"relay_upserts,omitempty"`
	RelayRemoveIDs  []string              `json:"relay_remove_ids,omitempty"`
	RelayCredential *relayauth.Credential `json:"relay_credential,omitempty"`
}

type ServerKeyResponse struct {
	TrustBundle               SigningTrustBundle `json:"trust_bundle"`
	NodeCredentialTrustBundle SigningTrustBundle `json:"node_credential_trust_bundle"`
	RelayTrustBundle          SigningTrustBundle `json:"relay_trust_bundle"`
}

type MapSignature struct {
	Version     int       `json:"version"`
	KeyID       string    `json:"key_id"`
	Algorithm   string    `json:"algorithm"`
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	PayloadHash string    `json:"payload_hash"`
	Signature   string    `json:"signature"`
}
