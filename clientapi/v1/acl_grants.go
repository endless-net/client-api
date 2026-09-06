package clientapi

import (
	"fmt"
	"net/netip"
)

func validateACLGrants(peer Peer) error {
	if len(peer.ACLGrants) == 0 {
		return nil
	}
	if !peer.ACLRestricted || len(peer.AllowedPorts) != 0 {
		return fmt.Errorf("acl_grants require acl_restricted and cannot mix with peer-wide allowed_ports")
	}
	if len(peer.ACLGrants) > 1000 {
		return fmt.Errorf("at most 1000 acl_grants are allowed")
	}
	routes := make([]netip.Prefix, 0, len(peer.AllowedIPs))
	for _, value := range peer.AllowedIPs {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return fmt.Errorf("acl_grants require valid allowed_ips prefixes")
		}
		routes = append(routes, prefix.Masked())
	}
	for i, grant := range peer.ACLGrants {
		if len(grant.DestinationCIDRs) == 0 || len(grant.DestinationCIDRs) > 1000 {
			return fmt.Errorf("acl_grants[%d] requires 1..1000 destination CIDRs", i)
		}
		if len(grant.AllowedPorts) > 65535 {
			return fmt.Errorf("acl_grants[%d] has too many ports", i)
		}
		for _, cidr := range grant.DestinationCIDRs {
			prefix, err := netip.ParsePrefix(cidr)
			if err != nil || prefix.Addr().Is4In6() || prefix != prefix.Masked() {
				return fmt.Errorf("acl_grants[%d] destination must be a canonical IP prefix", i)
			}
			covered := false
			for _, route := range routes {
				if route.Addr().BitLen() == prefix.Addr().BitLen() && route.Bits() <= prefix.Bits() && route.Contains(prefix.Addr()) {
					covered = true
					break
				}
			}
			if !covered {
				return fmt.Errorf("acl_grants[%d] destination is outside peer allowed_ips", i)
			}
		}
		for _, port := range grant.AllowedPorts {
			if port.Protocol != "tcp" && port.Protocol != "udp" && port.Protocol != "icmp" {
				return fmt.Errorf("acl_grants[%d] protocol is invalid", i)
			}
			if (port.Protocol == "icmp" && port.Port != 0) || (port.Protocol != "icmp" && (port.Port < 1 || port.Port > 65535)) {
				return fmt.Errorf("acl_grants[%d] port is invalid", i)
			}
		}
	}
	return nil
}
