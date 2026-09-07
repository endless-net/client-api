package clientapi

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	wgkeys "github.com/endless-net/client-api/clientapi/wireguard"
)

const (
	ShareInitiationRecipientOnly = "recipient_only"
	MaxSharePeerGrantTTL         = 2 * time.Minute
)

// SharePeerGrant is an effective, short-lived permission in a signed map.
// Both endpoints are exact identities; neither account membership nor subnet
// routes are implied. Only the recipient may initiate new flows. The source
// must enforce stateful reply-only traffic independently of the recipient.
type SharePeerGrant struct {
	GrantID             string         `json:"grant_id"`
	RecipientNetworkID  string         `json:"recipient_network_id"`
	RecipientNodeID     string         `json:"recipient_node_id"`
	RecipientPublicKey  string         `json:"recipient_public_key"`
	RecipientAllowedIPs []string       `json:"recipient_allowed_ips"`
	SourceNetworkID     string         `json:"source_network_id"`
	SourceNodeID        string         `json:"source_node_id"`
	SourcePublicKey     string         `json:"source_public_key"`
	SourceAllowedIPs    []string       `json:"source_allowed_ips"`
	Rights              []ShareTraffic `json:"rights"`
	Initiation          string         `json:"initiation"`
	Revision            uint64         `json:"revision"`
	IssuedAt            time.Time      `json:"issued_at"`
	ExpiresAt           time.Time      `json:"expires_at"`
}

// ShareTraffic permits destination ports on the source machine. ICMP uses
// zero ports. Empty rights never mean unrestricted access.
type ShareTraffic struct {
	Protocol  string `json:"protocol"`
	FirstPort uint32 `json:"first_port"`
	LastPort  uint32 `json:"last_port"`
}

func (grant SharePeerGrant) Validate() error {
	for field, value := range map[string]string{
		"grant_id": grant.GrantID, "recipient_network_id": grant.RecipientNetworkID,
		"recipient_node_id": grant.RecipientNodeID, "source_network_id": grant.SourceNetworkID,
		"source_node_id": grant.SourceNodeID,
	} {
		if value == "" || value != strings.TrimSpace(value) || len(value) > 128 {
			return fmt.Errorf("sharing %s is invalid", field)
		}
		if err := validateMapText(field, value); err != nil {
			return err
		}
	}
	if grant.SourceNodeID == grant.RecipientNodeID || grant.SourcePublicKey == grant.RecipientPublicKey {
		return fmt.Errorf("sharing endpoints must be distinct")
	}
	for _, key := range []string{grant.SourcePublicKey, grant.RecipientPublicKey} {
		if err := wgkeys.ValidatePublicKey(key); err != nil {
			return err
		}
	}
	if grant.Initiation != ShareInitiationRecipientOnly || grant.Revision == 0 {
		return fmt.Errorf("sharing initiation or revision is invalid")
	}
	if grant.IssuedAt.IsZero() || !grant.ExpiresAt.After(grant.IssuedAt) || grant.ExpiresAt.Sub(grant.IssuedAt) > MaxSharePeerGrantTTL {
		return fmt.Errorf("sharing lease is invalid")
	}
	left, err := shareHostPrefixes(grant.SourceAllowedIPs)
	if err != nil {
		return err
	}
	right, err := shareHostPrefixes(grant.RecipientAllowedIPs)
	if err != nil {
		return err
	}
	for _, a := range left {
		for _, b := range right {
			if a.Overlaps(b) {
				return fmt.Errorf("sharing endpoint addresses overlap")
			}
		}
	}
	if len(grant.Rights) == 0 || len(grant.Rights) > 128 {
		return fmt.Errorf("sharing rights are missing or excessive")
	}
	for i, right := range grant.Rights {
		switch right.Protocol {
		case "tcp", "udp":
			if right.FirstPort < 1 || right.LastPort < right.FirstPort || right.LastPort > 65535 {
				return fmt.Errorf("sharing port range is invalid")
			}
		case "icmp":
			if right.FirstPort != 0 || right.LastPort != 0 {
				return fmt.Errorf("sharing ICMP ports must be zero")
			}
		default:
			return fmt.Errorf("sharing protocol is invalid")
		}
		for _, other := range grant.Rights[:i] {
			if right.Protocol == other.Protocol && right.FirstPort <= other.LastPort && other.FirstPort <= right.LastPort {
				return fmt.Errorf("sharing rights overlap")
			}
		}
	}
	return nil
}

// ValidateSharePeerGrantsAt checks map bindings and current leases. Consumers
// must also remove access at ExpiresAt when no new map arrives; receiving a
// heartbeat or keeping an established flow does not renew a grant.
func ValidateSharePeerGrantsAt(snapshot NetworkMapSnapshot, now time.Time) error {
	if err := validateSharePeerGrants(snapshot); err != nil {
		return err
	}
	for _, grant := range snapshot.Network.SharePeerGrants {
		if now.Before(grant.IssuedAt) || !now.Before(grant.ExpiresAt) {
			return fmt.Errorf("sharing grant %q is outside its lease", grant.GrantID)
		}
	}
	return nil
}

func validateSharePeerGrants(snapshot NetworkMapSnapshot) error {
	seen := make(map[string]bool)
	bound := make(map[string]bool)
	for _, grant := range snapshot.Network.SharePeerGrants {
		if err := grant.Validate(); err != nil {
			return err
		}
		if seen[grant.GrantID] {
			return fmt.Errorf("duplicate sharing grant")
		}
		seen[grant.GrantID] = true
		localNetwork, localNode, localKey, localIPs := grant.RecipientNetworkID, grant.RecipientNodeID, grant.RecipientPublicKey, grant.RecipientAllowedIPs
		remoteNetwork, remoteNode, remoteKey, remoteIPs := grant.SourceNetworkID, grant.SourceNodeID, grant.SourcePublicKey, grant.SourceAllowedIPs
		if snapshot.Node.ID == grant.SourceNodeID && snapshot.Network.ID == grant.SourceNetworkID {
			localNetwork, localNode, localKey, localIPs, remoteNetwork, remoteNode, remoteKey, remoteIPs = remoteNetwork, remoteNode, remoteKey, remoteIPs, localNetwork, localNode, localKey, localIPs
		}
		if localNetwork != snapshot.Network.ID || localNode != snapshot.Node.ID || localKey != snapshot.Node.PublicKey || !shareLocalIPsMatch(snapshot.Node, localIPs) {
			return fmt.Errorf("sharing local endpoint binding mismatch")
		}
		found := false
		for _, peer := range snapshot.Peers {
			if peer.ID != remoteNode {
				continue
			}
			if peer.NetworkID != remoteNetwork || peer.PublicKey != remoteKey || !shareIPsEqual(peer.AllowedIPs, remoteIPs) {
				return fmt.Errorf("sharing peer endpoint binding mismatch")
			}
			found = true
		}
		if !found {
			return fmt.Errorf("sharing peer is absent")
		}
		bound[remoteNode] = true
		for _, value := range remoteIPs {
			prefix, _ := netip.ParsePrefix(value) // validated above
			if remoteNetwork != localNetwork {
				for _, localCIDR := range []string{snapshot.Network.CIDR, snapshot.Network.IPv6CIDR} {
					if localPrefix, err := netip.ParsePrefix(localCIDR); err == nil && localPrefix.Overlaps(prefix) {
						return fmt.Errorf("sharing address overlaps local network")
					}
				}
			}
			for _, peer := range snapshot.Peers {
				if peer.ID == remoteNode {
					continue
				}
				if peer.PublicKey == remoteKey {
					return fmt.Errorf("sharing key belongs to multiple peers")
				}
				for _, other := range peer.AllowedIPs {
					if otherPrefix, err := netip.ParsePrefix(other); err == nil && otherPrefix.Overlaps(prefix) {
						return fmt.Errorf("sharing address overlaps another peer")
					}
				}
			}
		}
	}
	for _, peer := range snapshot.Peers {
		if peer.NetworkID != "" && peer.NetworkID != snapshot.Network.ID && !bound[peer.ID] {
			return fmt.Errorf("cross-network peer has no sharing grant")
		}
	}
	return nil
}

func shareHostPrefixes(values []string) ([]netip.Prefix, error) {
	if len(values) < 1 || len(values) > 2 {
		return nil, fmt.Errorf("sharing requires exact host addresses")
	}
	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil || prefix.Bits() != prefix.Addr().BitLen() || prefix.String() != value || prefix.Addr().Is4In6() || prefix.Addr().IsUnspecified() || prefix.Addr().IsLoopback() || prefix.Addr().IsMulticast() || prefix.Addr().IsLinkLocalUnicast() {
			return nil, fmt.Errorf("sharing address must be a canonical unicast host prefix")
		}
		for _, existing := range result {
			if existing.Addr().BitLen() == prefix.Addr().BitLen() {
				return nil, fmt.Errorf("sharing has duplicate address family")
			}
		}
		result = append(result, prefix)
	}
	return result, nil
}

func shareIPsEqual(left, right []string) bool {
	a, b := slices.Clone(left), slices.Clone(right)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

func shareLocalIPsMatch(node Node, values []string) bool {
	var local []string
	for _, value := range []string{node.AssignedIP, node.AssignedIPv6} {
		if value == "" {
			continue
		}
		addr, err := netip.ParseAddr(value)
		if err != nil {
			return false
		}
		local = append(local, netip.PrefixFrom(addr, addr.BitLen()).String())
	}
	return shareIPsEqual(local, values)
}

func cloneSharePeerGrants(values []SharePeerGrant) []SharePeerGrant {
	result := slices.Clone(values)
	for i := range result {
		result[i].SourceAllowedIPs = slices.Clone(values[i].SourceAllowedIPs)
		result[i].RecipientAllowedIPs = slices.Clone(values[i].RecipientAllowedIPs)
		result[i].Rights = slices.Clone(values[i].Rights)
	}
	return result
}
