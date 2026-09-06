package clientapi

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"
)

func TestACLStreamAuthenticatesAndDetachesGrantPayload(t *testing.T) {
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	trust, err := NewSigningTrustBundle(base64.RawURLEncoding.EncodeToString(public))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	current := validMapStreamSnapshot()
	current.MapSignature, err = SignNetworkMapSnapshotAt(private, current, now, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	peer := Peer{ID: "peer", Hostname: "peer", PublicKey: base64.StdEncoding.EncodeToString(make([]byte, 32)), AllowedIPs: []string{"100.64.0.2/32"}, ACLRestricted: true, ACLGrants: []ACLGrant{{DestinationCIDRs: []string{"100.64.0.2/32"}, AllowedPorts: []ACLPort{{Protocol: "tcp", Port: 443}}}}}
	next := cloneNetworkMapSnapshot(current)
	next.Revision.Network++
	next.Peers = []Peer{peer}
	signature, err := SignNetworkMapSnapshotAt(private, next, now, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	event := MapStreamEvent{Type: "delta", ProtocolVersion: MapStreamProtocolVersion, Capabilities: MapStreamSupportedCapabilities(), EventID: "grants", From: current.Revision, To: next.Revision, BaseHash: current.MapSignature.PayloadHash, Delta: &MapDelta{PeerUpserts: []Peer{peer}}, ResultSignature: signature}
	accepted, err := ApplyMapStreamEvent(current, event, trust, now)
	if err != nil {
		t.Fatal(err)
	}
	// Mutating the event after verification cannot mutate the accepted cache.
	event.Delta.PeerUpserts[0].ACLGrants[0].AllowedPorts[0].Port = 22
	if accepted.Peers[0].ACLGrants[0].AllowedPorts[0].Port != 443 {
		t.Fatal("verified ACL aliases the untrusted event")
	}
	if err := VerifyNetworkMapSnapshotSignatureAt(accepted, base64.RawURLEncoding.EncodeToString(public), now); err != nil {
		t.Fatal("accepted snapshot signature changed", err)
	}
	rejected, err := ApplyMapStreamEvent(current, event, trust, now)
	if err == nil || len(rejected.Peers) != 0 {
		t.Fatal("tampered ACL delta accepted")
	}
	// A separately cloned checkpoint snapshot must also own nested grant data.
	clone := cloneNetworkMapSnapshot(accepted)
	clone.Peers[0].ACLGrants[0].DestinationCIDRs[0] = "100.64.0.3/32"
	if accepted.Peers[0].ACLGrants[0].DestinationCIDRs[0] != "100.64.0.2/32" {
		t.Fatal("checkpoint clone aliases ACL prefixes")
	}
}
