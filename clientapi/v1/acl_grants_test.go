package clientapi

import (
	"bytes"
	"testing"
	"time"
)

func TestACLGrantsStayWithinPeerRoutes(t *testing.T) {
	for _, mode := range []string{"valid", "unrestricted", "mixed", "outside", "hostbits", "missing", "badport", "badprotocol"} {
		peer := Peer{AllowedIPs: []string{"10.0.0.0/8"}, ACLRestricted: true, ACLGrants: []ACLGrant{{DestinationCIDRs: []string{"10.1.0.0/16"}, AllowedPorts: []ACLPort{{Protocol: "tcp", Port: 443}}}}}
		switch mode {
		case "unrestricted":
			peer.ACLRestricted = false
		case "mixed":
			peer.AllowedPorts = []ACLPort{{Protocol: "tcp", Port: 22}}
		case "outside":
			peer.ACLGrants[0].DestinationCIDRs[0] = "192.0.2.0/24"
		case "hostbits":
			peer.ACLGrants[0].DestinationCIDRs[0] = "10.1.1.1/16"
		case "missing":
			peer.ACLGrants[0].DestinationCIDRs = nil
		case "badport":
			peer.ACLGrants[0].AllowedPorts[0].Port = 65536
		case "badprotocol":
			peer.ACLGrants[0].AllowedPorts[0].Protocol = "all"
		}
		err := validateACLGrants(peer)
		if (err == nil) != (mode == "valid") {
			t.Fatalf("%s: %v", mode, err)
		}
	}
}

func TestACLGrantFieldsAreCoveredByCanonicalSigningPayload(t *testing.T) {
	snapshot := validMapStreamSnapshot()
	snapshot.Peers = []Peer{{ID: "peer", ACLRestricted: true, AllowedIPs: []string{"10.0.0.0/8"}, ACLGrants: []ACLGrant{{DestinationCIDRs: []string{"10.1.0.0/16"}, AllowedPorts: []ACLPort{{Protocol: "tcp", Port: 443}}}}}}
	now := time.Unix(1000, 0)
	original, err := canonicalNetworkMapPayload(snapshot, "test-key", now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Peer){
		func(peer *Peer) { peer.ACLGrants[0].AllowedPorts[0].Port = 22 },
		func(peer *Peer) { peer.ACLGrants[0].AllowedPorts[0].Protocol = "udp" },
		func(peer *Peer) { peer.ACLGrants[0].DestinationCIDRs[0] = "10.2.0.0/16" },
		func(peer *Peer) { peer.ACLGrants = nil },
	} {
		snapshot.Peers[0].ACLGrants = []ACLGrant{{DestinationCIDRs: []string{"10.1.0.0/16"}, AllowedPorts: []ACLPort{{Protocol: "tcp", Port: 443}}}}
		mutate(&snapshot.Peers[0])
		next, err := canonicalNetworkMapPayload(snapshot, "test-key", now, now.Add(time.Hour))
		if err != nil || bytes.Equal(original, next) {
			t.Fatal("ACL grant mutation omitted from signed payload", err)
		}
	}
}
