package clientapi

import (
	"encoding/hex"
	"errors"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// ApplicationTarget defines L3 scope. URL paths, credentials and fragments are
// rejected because an IP route cannot enforce HTTP path or host isolation.
type ApplicationTarget struct {
	Domain  string
	Prefix  netip.Prefix
	TCPPort uint16
}

func ParseApplicationTarget(kind, target string) (ApplicationTarget, error) {
	switch kind {
	case "cidr":
		prefix, err := netip.ParsePrefix(target)
		if err != nil || prefix.Masked().String() != target || prefix.Bits() == 0 || !applicationAddressAllowed(prefix.Addr()) {
			return ApplicationTarget{}, errors.New("invalid application CIDR")
		}
		return ApplicationTarget{Prefix: prefix}, nil
	case "domain":
		if !validDNSDomain(target) || strings.Contains(target, "*") || target != strings.ToLower(target) {
			return ApplicationTarget{}, errors.New("application requires an exact canonical domain")
		}
		return ApplicationTarget{Domain: target}, nil
	case "url":
		value, err := url.Parse(target)
		if err != nil || (value.Scheme != "http" && value.Scheme != "https") || value.User != nil || (value.Path != "" && value.Path != "/") || value.RawQuery != "" || value.Fragment != "" || !validDNSDomain(value.Hostname()) {
			return ApplicationTarget{}, errors.New("application URL must identify an HTTP(S) origin without path, query or credentials")
		}
		port := uint64(80)
		if value.Scheme == "https" {
			port = 443
		}
		if value.Port() != "" {
			port, err = strconv.ParseUint(value.Port(), 10, 16)
			if err != nil || port == 0 {
				return ApplicationTarget{}, errors.New("invalid application URL port")
			}
		}
		return ApplicationTarget{Domain: strings.ToLower(value.Hostname()), TCPPort: uint16(port)}, nil
	default:
		return ApplicationTarget{}, errors.New("unsupported application target type")
	}
}

func applicationAddressAllowed(address netip.Addr) bool {
	return address.IsValid() && !address.IsLoopback() && !address.IsLinkLocalUnicast() && !address.IsLinkLocalMulticast() && !address.IsMulticast() && !address.IsUnspecified() && !address.Is4In6()
}

func ValidateApplicationAddress(address netip.Addr) error {
	if !applicationAddressAllowed(address) {
		return errors.New("application destination address is forbidden")
	}
	return nil
}

func validateApplicationBindings(snapshot NetworkMapSnapshot) error {
	identities := map[string]string{snapshot.Node.ID: snapshot.Node.PublicKey}
	for _, peer := range snapshot.Peers {
		identities[peer.ID] = peer.PublicKey
	}
	for _, app := range snapshot.Network.Applications {
		hash, err := hex.DecodeString(app.PolicyHash)
		if err != nil || len(hash) != 32 {
			return errors.New("application policy hash is missing or invalid")
		}
		for _, bindings := range [][]ServiceHost{app.Sources, app.Connectors} {
			seen := make(map[string]bool)
			for _, binding := range bindings {
				if binding.NodeID == "" || binding.PublicKey == "" || identities[binding.NodeID] != binding.PublicKey || seen[binding.NodeID] {
					return errors.New("invalid application node binding")
				}
				seen[binding.NodeID] = true
			}
		}
		target, err := ParseApplicationTarget(app.TargetType, app.Target)
		if err != nil {
			return err
		}
		seen := make(map[string]bool)
		for _, route := range app.Routes {
			if !slices.Contains(app.Connectors, route.Connector) || seen[route.Connector.NodeID] || route.ExpiresAt.IsZero() || len(route.CIDRs) == 0 || len(route.CIDRs) > 64 {
				return errors.New("invalid application route binding")
			}
			seen[route.Connector.NodeID] = true
			prefixes := make(map[string]bool)
			for _, value := range route.CIDRs {
				prefix, err := netip.ParsePrefix(value)
				if err != nil || prefix.Masked().String() != value || !applicationAddressAllowed(prefix.Addr()) || prefixes[value] {
					return errors.New("invalid application route destination")
				}
				if target.Prefix.IsValid() && prefix != target.Prefix || !target.Prefix.IsValid() && prefix.Bits() != prefix.Addr().BitLen() {
					return errors.New("application route exceeds target scope")
				}
				prefixes[value] = true
			}
		}
	}
	return nil
}
