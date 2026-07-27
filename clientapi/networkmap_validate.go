package clientapi

import (
	"fmt"
	"strings"

	protocolv1 "github.com/unng-lab/endlessnet-relay/protocol/v1"
	wgkeys "github.com/unng-lab/endlessnet/clientapi/v2/wireguard"
)

// ValidateNetworkMap validates the complete untrusted map boundary before a
// client is allowed to cache, render, or apply it.
func ValidateNetworkMap(response RegisterNodeResponse) error {
	return ValidateNetworkMapSnapshot(response.Snapshot())
}

// ValidateNetworkMapSnapshot validates a stream snapshot or reconstructed
// delta result before it is cached or applied to the local network stack.
func ValidateNetworkMapSnapshot(response NetworkMapSnapshot) error {
	networkID := strings.TrimSpace(response.Network.ID)
	if networkID == "" {
		return fmt.Errorf("network map network_id is missing")
	}
	if err := validateMapText("network.id", response.Network.ID); err != nil {
		return err
	}
	for field, value := range map[string]string{
		"network.name":                  response.Network.Name,
		"network.cell_id":               response.Network.CellID,
		"network.authoritative_cell_id": response.Network.AuthoritativeCellID,
		"network.migrating_to_cell_id":  response.Network.MigratingToCellID,
		"network.owner_id":              response.Network.OwnerID,
		"network.account_id":            response.Network.AccountID,
		"registration_binding":          response.RegistrationBinding,
	} {
		if err := validateMapText(field, value); err != nil {
			return err
		}
	}
	if err := wgkeys.ValidatePrefix("network.cidr", response.Network.CIDR); err != nil {
		return err
	}
	if response.Network.IPv6CIDR != "" {
		if err := wgkeys.ValidatePrefix("network.ipv6_cidr", response.Network.IPv6CIDR); err != nil {
			return err
		}
	}
	for i, value := range response.Network.DNS {
		if err := wgkeys.ValidateDNSIP(value); err != nil {
			return fmt.Errorf("network.dns[%d]: %w", i, err)
		}
	}
	if err := validateMapNode(response.Node, networkID); err != nil {
		return err
	}
	seenPeers := make(map[string]struct{}, len(response.Peers))
	for i, peer := range response.Peers {
		if err := validateMapPeer(peer); err != nil {
			return fmt.Errorf("peers[%d]: %w", i, err)
		}
		if _, exists := seenPeers[peer.ID]; exists {
			return fmt.Errorf("peers[%d]: duplicate peer id %q", i, peer.ID)
		}
		seenPeers[peer.ID] = struct{}{}
	}
	for i, endpoint := range response.STUNEndpoints {
		if strings.TrimSpace(endpoint.ID) == "" {
			return fmt.Errorf("stun_endpoints[%d].id is missing", i)
		}
		if err := validateMapText(fmt.Sprintf("stun_endpoints[%d].id", i), endpoint.ID); err != nil {
			return err
		}
		if err := wgkeys.ValidateEndpoint(endpoint.Addr); err != nil {
			return fmt.Errorf("stun_endpoints[%d].addr: %w", i, err)
		}
	}
	if err := (protocolv1.EndpointSnapshot{Version: 1, Endpoints: response.Relays}).Validate(); err != nil {
		return fmt.Errorf("relays: %w", err)
	}
	for i, endpoint := range response.Relays {
		if strings.TrimSpace(endpoint.ID) == "" {
			return fmt.Errorf("relays[%d].id is missing", i)
		}
		for field, value := range map[string]string{
			"id": endpoint.ID, "protocol": endpoint.Protocol, "region": endpoint.Region,
		} {
			if err := validateMapText(fmt.Sprintf("relays[%d].%s", i, field), value); err != nil {
				return err
			}
		}
		if err := wgkeys.ValidateEndpoint(endpoint.Addr); err != nil {
			return fmt.Errorf("relays[%d].addr: %w", i, err)
		}
	}
	if credential := response.RelayCredential; credential != nil {
		for field, value := range map[string]string{
			"relay_credential.algorithm":  credential.Algorithm,
			"relay_credential.key_id":     credential.KeyID,
			"relay_credential.network_id": credential.NetworkID,
			"relay_credential.node_id":    credential.NodeID,
			"relay_credential.signature":  credential.Signature,
		} {
			if err := validateMapText(field, value); err != nil {
				return err
			}
		}
		if strings.TrimSpace(credential.NetworkID) != networkID {
			return fmt.Errorf("relay credential network_id %q does not match network %q", credential.NetworkID, networkID)
		}
		if strings.TrimSpace(credential.NodeID) != strings.TrimSpace(response.Node.ID) {
			return fmt.Errorf("relay credential node_id %q does not match node %q", credential.NodeID, response.Node.ID)
		}
	}
	if signature := response.MapSignature; signature != nil {
		for field, value := range map[string]string{
			"map_signature.algorithm":    signature.Algorithm,
			"map_signature.key_id":       signature.KeyID,
			"map_signature.payload_hash": signature.PayloadHash,
			"map_signature.signature":    signature.Signature,
		} {
			if err := validateMapText(field, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateMapNode(node Node, networkID string) error {
	if strings.TrimSpace(node.ID) == "" {
		return fmt.Errorf("network map node_id is missing")
	}
	if strings.TrimSpace(node.NetworkID) != networkID {
		return fmt.Errorf("network map node network_id %q does not match network %q", node.NetworkID, networkID)
	}
	for field, value := range map[string]string{
		"node.id": node.ID, "node.network_id": node.NetworkID, "node.user_id": node.UserID,
		"node.identity_public_key": node.IdentityPublicKey, "node.device_fingerprint": node.DeviceFingerprint,
		"node.status": node.Status,
	} {
		if err := validateMapText(field, value); err != nil {
			return err
		}
	}
	if err := wgkeys.ValidateHostname(node.Hostname); err != nil {
		return fmt.Errorf("node.hostname: %w", err)
	}
	if err := wgkeys.ValidatePublicKey(node.PublicKey); err != nil {
		return fmt.Errorf("node.public_key: %w", err)
	}
	if err := wgkeys.ValidateAddr("node.assigned_ip", node.AssignedIP); err != nil {
		return err
	}
	if node.AssignedIPv6 != "" {
		if err := wgkeys.ValidateAddr("node.assigned_ipv6", node.AssignedIPv6); err != nil {
			return err
		}
	}
	if err := validateOptionalEndpoint("node.endpoint", node.Endpoint); err != nil {
		return err
	}
	for i, endpoint := range node.EndpointCandidates {
		if err := wgkeys.ValidateEndpoint(endpoint); err != nil {
			return fmt.Errorf("node.endpoint_candidates[%d]: %w", i, err)
		}
	}
	for i, prefix := range node.AdvertisedIPs {
		if err := wgkeys.ValidatePrefix(fmt.Sprintf("node.advertised_ips[%d]", i), prefix); err != nil {
			return err
		}
	}
	for i, tag := range node.Tags {
		if err := validateMapText(fmt.Sprintf("node.tags[%d]", i), tag); err != nil {
			return err
		}
	}
	return nil
}

func validateMapPeer(peer Peer) error {
	if strings.TrimSpace(peer.ID) == "" {
		return fmt.Errorf("peer_id is missing")
	}
	for field, value := range map[string]string{"id": peer.ID, "status": peer.Status} {
		if err := validateMapText("peer."+field, value); err != nil {
			return err
		}
	}
	if err := wgkeys.ValidateHostname(peer.Hostname); err != nil {
		return fmt.Errorf("hostname: %w", err)
	}
	if err := wgkeys.ValidatePublicKey(peer.PublicKey); err != nil {
		return fmt.Errorf("public_key: %w", err)
	}
	if err := validateOptionalEndpoint("endpoint", peer.Endpoint); err != nil {
		return err
	}
	for i, endpoint := range peer.EndpointCandidates {
		if err := wgkeys.ValidateEndpoint(endpoint); err != nil {
			return fmt.Errorf("endpoint_candidates[%d]: %w", i, err)
		}
	}
	for i, prefix := range peer.AllowedIPs {
		if err := wgkeys.ValidatePrefix(fmt.Sprintf("allowed_ips[%d]", i), prefix); err != nil {
			return err
		}
	}
	for i, port := range peer.AllowedPorts {
		if port.Protocol != "tcp" && port.Protocol != "udp" && port.Protocol != "icmp" {
			return fmt.Errorf("allowed_ports[%d].protocol is invalid", i)
		}
		if (port.Protocol == "icmp" && port.Port != 0) || (port.Protocol != "icmp" && (port.Port < 1 || port.Port > 65535)) {
			return fmt.Errorf("allowed_ports[%d].port is invalid", i)
		}
	}
	for i, tag := range peer.Tags {
		if err := validateMapText(fmt.Sprintf("tags[%d]", i), tag); err != nil {
			return err
		}
	}
	return nil
}

func validateOptionalEndpoint(field, value string) error {
	if value == "" {
		return nil
	}
	if err := wgkeys.ValidateEndpoint(value); err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	return nil
}

func validateMapText(field, value string) error {
	if err := wgkeys.ValidateSafeText(field, value); err != nil {
		return err
	}
	return nil
}
