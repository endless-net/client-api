package clientapi

import (
	"errors"
	"fmt"
	"strings"
	"time"

	relayauth "github.com/endless-net/relay/protocol/v1"
)

var (
	ErrMapStreamResyncRequired      = errors.New("map stream resync required")
	ErrMapStreamEventAlreadyApplied = errors.New("map stream event already applied")
)

// ApplyMapStreamEvent reconstructs and authenticates the full map represented
// by an event. The caller must atomically replace its cache and network state
// only after this function succeeds.
func ApplyMapStreamEvent(current NetworkMapSnapshot, event MapStreamEvent, trust SigningTrustBundle, now time.Time) (NetworkMapSnapshot, error) {
	if err := validateMapStreamEvent(event); err != nil {
		return NetworkMapSnapshot{}, err
	}
	if current.Revision.Equal(event.To) && current.MapSignature != nil &&
		event.ResultSignature != nil &&
		current.MapSignature.PayloadHash == event.ResultSignature.PayloadHash {
		return cloneNetworkMapSnapshot(current), ErrMapStreamEventAlreadyApplied
	}

	switch event.Type {
	case "heartbeat", "checkpoint":
		if !current.Revision.Equal(event.To) {
			return NetworkMapSnapshot{}, fmt.Errorf("%w: checkpoint revision does not match cached revision", ErrMapStreamResyncRequired)
		}
		if event.BaseHash != "" && event.BaseHash != currentMapHash(current) {
			return NetworkMapSnapshot{}, fmt.Errorf("%w: checkpoint base hash does not match cached map", ErrMapStreamResyncRequired)
		}
		return cloneNetworkMapSnapshot(current), nil
	case "snapshot", "resync":
		next := cloneNetworkMapSnapshot(*event.Snapshot)
		next.Revision = event.To
		next.MapSignature = cloneMapSignature(event.ResultSignature)
		return verifyMapStreamResult(next, trust, now)
	case "delta":
		if !current.Revision.Equal(event.From) {
			return NetworkMapSnapshot{}, fmt.Errorf("%w: delta base revision does not match cached revision", ErrMapStreamResyncRequired)
		}
		if event.BaseHash != currentMapHash(current) {
			return NetworkMapSnapshot{}, fmt.Errorf("%w: delta base hash does not match cached map", ErrMapStreamResyncRequired)
		}
		next, err := applyMapDelta(current, *event.Delta)
		if err != nil {
			return NetworkMapSnapshot{}, err
		}
		next.Revision = event.To
		next.MapSignature = cloneMapSignature(event.ResultSignature)
		return verifyMapStreamResult(next, trust, now)
	default:
		return NetworkMapSnapshot{}, fmt.Errorf("unsupported map stream event type %q", event.Type)
	}
}

func verifyMapStreamResult(snapshot NetworkMapSnapshot, trust SigningTrustBundle, now time.Time) (NetworkMapSnapshot, error) {
	if err := ValidateNetworkMapSnapshot(snapshot); err != nil {
		return NetworkMapSnapshot{}, fmt.Errorf("validate map stream result: %w", err)
	}
	if snapshot.MapSignature == nil {
		return NetworkMapSnapshot{}, errors.New("map stream result signature is missing")
	}
	key, err := trust.Resolve(snapshot.MapSignature.KeyID, now.UTC())
	if err != nil {
		return NetworkMapSnapshot{}, err
	}
	if err := VerifyNetworkMapSnapshotSignatureAt(snapshot, key.PublicKey, now.UTC()); err != nil {
		return NetworkMapSnapshot{}, err
	}
	return snapshot, nil
}

func applyMapDelta(current NetworkMapSnapshot, delta MapDelta) (NetworkMapSnapshot, error) {
	next := cloneNetworkMapSnapshot(current)
	if delta.Network != nil {
		next.Network = *delta.Network
	}
	if delta.Node != nil {
		next.Node = *delta.Node
	}
	if delta.STUNEndpoints != nil {
		next.STUNEndpoints = append([]STUNEndpoint(nil), (*delta.STUNEndpoints)...)
	}
	if delta.RelayCredential != nil {
		credential := *delta.RelayCredential
		next.RelayCredential = &credential
	}

	peers, err := mergePeers(next.Peers, delta.PeerUpserts, delta.PeerRemoveIDs)
	if err != nil {
		return NetworkMapSnapshot{}, err
	}
	next.Peers = peers
	relays, err := mergeRelays(next.Relays, delta.RelayUpserts, delta.RelayRemoveIDs)
	if err != nil {
		return NetworkMapSnapshot{}, err
	}
	next.Relays = relays
	return cloneNetworkMapSnapshot(next), nil
}

func mergePeers(current, upserts []Peer, removeIDs []string) ([]Peer, error) {
	remove, err := normalizedIDSet("peer_remove_ids", removeIDs)
	if err != nil {
		return nil, err
	}
	result := make([]Peer, 0, len(current)+len(upserts))
	index := make(map[string]int, len(current)+len(upserts))
	for _, peer := range current {
		if _, removed := remove[peer.ID]; removed {
			continue
		}
		index[peer.ID] = len(result)
		result = append(result, peer)
	}
	seen := make(map[string]struct{}, len(upserts))
	for _, peer := range upserts {
		id := strings.TrimSpace(peer.ID)
		if id == "" {
			return nil, errors.New("peer_upserts contains an empty id")
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("peer_upserts contains duplicate id %q", id)
		}
		seen[id] = struct{}{}
		if position, exists := index[id]; exists {
			result[position] = peer
			continue
		}
		index[id] = len(result)
		result = append(result, peer)
	}
	return result, nil
}

func mergeRelays(current, upserts []relayauth.Endpoint, removeIDs []string) ([]relayauth.Endpoint, error) {
	remove, err := normalizedIDSet("relay_remove_ids", removeIDs)
	if err != nil {
		return nil, err
	}
	result := make([]relayauth.Endpoint, 0, len(current)+len(upserts))
	index := make(map[string]int, len(current)+len(upserts))
	for _, endpoint := range current {
		if _, removed := remove[endpoint.ID]; removed {
			continue
		}
		index[endpoint.ID] = len(result)
		result = append(result, endpoint)
	}
	seen := make(map[string]struct{}, len(upserts))
	for _, endpoint := range upserts {
		id := strings.TrimSpace(endpoint.ID)
		if id == "" {
			return nil, errors.New("relay_upserts contains an empty id")
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("relay_upserts contains duplicate id %q", id)
		}
		seen[id] = struct{}{}
		if position, exists := index[id]; exists {
			result[position] = endpoint
			continue
		}
		index[id] = len(result)
		result = append(result, endpoint)
	}
	return result, nil
}

func normalizedIDSet(field string, values []string) (map[string]struct{}, error) {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			return nil, fmt.Errorf("%s contains an empty id", field)
		}
		if _, duplicate := result[id]; duplicate {
			return nil, fmt.Errorf("%s contains duplicate id %q", field, id)
		}
		result[id] = struct{}{}
	}
	return result, nil
}

func cloneNetworkMapSnapshot(snapshot NetworkMapSnapshot) NetworkMapSnapshot {
	clone := snapshot
	clone.Network.DNS = append([]string(nil), snapshot.Network.DNS...)
	clone.Network.DNSConfig = cloneDNSConfig(snapshot.Network.DNSConfig)
	clone.Node.EndpointCandidates = append([]string(nil), snapshot.Node.EndpointCandidates...)
	clone.Node.AdvertisedIPs = append([]string(nil), snapshot.Node.AdvertisedIPs...)
	clone.Node.RequestedTags = append([]string(nil), snapshot.Node.RequestedTags...)
	clone.Node.Tags = append([]string(nil), snapshot.Node.Tags...)
	clone.Node.EndpointExpiresAt = cloneMapTime(snapshot.Node.EndpointExpiresAt)
	clone.Node.KeyExpiresAt = cloneMapTime(snapshot.Node.KeyExpiresAt)
	clone.Peers = append([]Peer(nil), snapshot.Peers...)
	for index := range clone.Peers {
		peer := &clone.Peers[index]
		peer.EndpointCandidates = append([]string(nil), peer.EndpointCandidates...)
		peer.AllowedIPs = append([]string(nil), peer.AllowedIPs...)
		peer.AllowedPorts = append([]ACLPort(nil), peer.AllowedPorts...)
		peer.Tags = append([]string(nil), peer.Tags...)
		peer.EndpointExpiresAt = cloneMapTime(peer.EndpointExpiresAt)
		peer.ACLGrants = append([]ACLGrant(nil), peer.ACLGrants...)
		for grant := range peer.ACLGrants {
			peer.ACLGrants[grant].DestinationCIDRs = append([]string(nil), peer.ACLGrants[grant].DestinationCIDRs...)
			peer.ACLGrants[grant].AllowedPorts = append([]ACLPort(nil), peer.ACLGrants[grant].AllowedPorts...)
		}
	}
	clone.STUNEndpoints = append([]STUNEndpoint(nil), snapshot.STUNEndpoints...)
	clone.Relays = append([]relayauth.Endpoint(nil), snapshot.Relays...)
	if snapshot.RelayCredential != nil {
		credential := *snapshot.RelayCredential
		clone.RelayCredential = &credential
	}
	clone.MapSignature = cloneMapSignature(snapshot.MapSignature)
	return clone
}

func cloneDNSConfig(value *DNSConfig) *DNSConfig {
	if value == nil {
		return nil
	}
	clone := *value
	clone.SearchDomains = append([]string(nil), value.SearchDomains...)
	clone.Nameservers = append([]DNSNameserver(nil), value.Nameservers...)
	for index := range clone.Nameservers {
		clone.Nameservers[index].SplitDomains = append([]string(nil), value.Nameservers[index].SplitDomains...)
	}
	return &clone
}

func cloneMapTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneMapSignature(signature *MapSignature) *MapSignature {
	if signature == nil {
		return nil
	}
	clone := *signature
	return &clone
}

func currentMapHash(snapshot NetworkMapSnapshot) string {
	if snapshot.MapSignature == nil {
		return ""
	}
	return strings.TrimSpace(snapshot.MapSignature.PayloadHash)
}
