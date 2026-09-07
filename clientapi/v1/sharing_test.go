package clientapi

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

func sharingSnapshot(now time.Time) NetworkMapSnapshot {
	snapshot := validMapStreamSnapshot()
	key := make([]byte, 32)
	key[0] = 1
	remoteKey := base64.StdEncoding.EncodeToString(key)
	snapshot.Peers = []Peer{{ID: "shared", NetworkID: "remote", Hostname: "shared", PublicKey: remoteKey, AllowedIPs: []string{"100.65.0.1/32"}}}
	snapshot.Network.SharePeerGrants = []SharePeerGrant{{
		GrantID: "grant", RecipientNetworkID: snapshot.Network.ID, RecipientNodeID: snapshot.Node.ID,
		RecipientPublicKey: snapshot.Node.PublicKey, RecipientAllowedIPs: []string{"100.64.0.1/32"},
		SourceNetworkID: "remote", SourceNodeID: "shared", SourcePublicKey: remoteKey, SourceAllowedIPs: []string{"100.65.0.1/32"},
		Rights: []ShareTraffic{{Protocol: "tcp", FirstPort: 443, LastPort: 443}}, Initiation: ShareInitiationRecipientOnly,
		Revision: 1, IssuedAt: now, ExpiresAt: now.Add(time.Minute),
	}}
	return snapshot
}

func TestSharingMapBindings(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := sharingSnapshot(now)
	if err := ValidateNetworkMapSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*NetworkMapSnapshot){
		"missing grant":            func(s *NetworkMapSnapshot) { s.Network.SharePeerGrants = nil },
		"missing peer network":     func(s *NetworkMapSnapshot) { s.Peers[0].NetworkID = "" },
		"wrong peer network":       func(s *NetworkMapSnapshot) { s.Peers[0].NetworkID = s.Network.ID },
		"recipient key rotation":   func(s *NetworkMapSnapshot) { s.Node.PublicKey = s.Peers[0].PublicKey },
		"source key rotation":      func(s *NetworkMapSnapshot) { s.Peers[0].PublicKey = s.Node.PublicKey },
		"recipient address change": func(s *NetworkMapSnapshot) { s.Node.AssignedIP = "100.64.0.2" },
		"source address change":    func(s *NetworkMapSnapshot) { s.Peers[0].AllowedIPs[0] = "100.65.0.2/32" },
		"subnet route":             func(s *NetworkMapSnapshot) { s.Peers[0].AllowedIPs = append(s.Peers[0].AllowedIPs, "10.0.0.0/8") },
		"duplicate grant": func(s *NetworkMapSnapshot) {
			s.Network.SharePeerGrants = append(s.Network.SharePeerGrants, s.Network.SharePeerGrants[0])
		},
		"local network overlap": func(s *NetworkMapSnapshot) { s.Network.CIDR = "100.64.0.0/10" },
		"another peer overlap": func(s *NetworkMapSnapshot) {
			p := s.Peers[0]
			p.ID = "other"
			p.NetworkID = s.Network.ID
			p.PublicKey = s.Node.PublicKey
			s.Peers = append(s.Peers, p)
		},
		"unrelated grant": func(s *NetworkMapSnapshot) { s.Network.SharePeerGrants[0].RecipientNodeID = "unrelated" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneNetworkMapSnapshot(snapshot)
			mutate(&candidate)
			if err := ValidateNetworkMapSnapshot(candidate); err == nil {
				t.Fatal("invalid sharing map accepted")
			}
		})
	}
	if err := ValidateSharePeerGrantsAt(snapshot, now); err != nil {
		t.Fatalf("clone mutated original: %v", err)
	}
	// The source receives the same signed grant, with the recipient as its peer.
	source := cloneNetworkMapSnapshot(snapshot)
	source.Network.ID = "remote"
	source.Network.CIDR = "100.65.0.0/24"
	source.Node.ID = "shared"
	source.Node.NetworkID = "remote"
	source.Node.PublicKey = snapshot.Peers[0].PublicKey
	source.Node.AssignedIP = "100.65.0.1"
	source.Peers = []Peer{{ID: snapshot.Node.ID, NetworkID: snapshot.Network.ID, Hostname: "recipient", PublicKey: snapshot.Node.PublicKey, AllowedIPs: []string{"100.64.0.1/32"}}}
	if err := ValidateNetworkMapSnapshot(source); err != nil {
		t.Fatal(err)
	}
}

func TestShareGrantRejectsBroadenedRightsAndLeases(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := sharingSnapshot(now)
	for name, mutate := range map[string]func(*SharePeerGrant){
		"empty rights":   func(g *SharePeerGrant) { g.Rights = nil },
		"all protocol":   func(g *SharePeerGrant) { g.Rights[0].Protocol = "all" },
		"zero TCP port":  func(g *SharePeerGrant) { g.Rights[0].FirstPort = 0 },
		"excess port":    func(g *SharePeerGrant) { g.Rights[0].LastPort = 65536 },
		"inverted ports": func(g *SharePeerGrant) { g.Rights[0].LastPort = 1 },
		"ICMP port":      func(g *SharePeerGrant) { g.Rights[0].Protocol = "icmp" },
		"overlap":        func(g *SharePeerGrant) { g.Rights = append(g.Rights, g.Rights[0]) },
		"symmetric":      func(g *SharePeerGrant) { g.Initiation = "both" },
		"no revision":    func(g *SharePeerGrant) { g.Revision = 0 },
		"long lease":     func(g *SharePeerGrant) { g.ExpiresAt = now.Add(MaxSharePeerGrantTTL + time.Nanosecond) },
		"empty lease":    func(g *SharePeerGrant) { g.ExpiresAt = now },
		"subnet":         func(g *SharePeerGrant) { g.SourceAllowedIPs[0] = "100.65.0.0/24" },
		"shared IP":      func(g *SharePeerGrant) { g.SourceAllowedIPs[0] = g.RecipientAllowedIPs[0] },
		"loopback":       func(g *SharePeerGrant) { g.SourceAllowedIPs[0] = "127.0.0.1/32" },
		"IPv4 mapped":    func(g *SharePeerGrant) { g.SourceAllowedIPs[0] = "::ffff:100.65.0.1/128" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneNetworkMapSnapshot(snapshot)
			mutate(&candidate.Network.SharePeerGrants[0])
			if err := candidate.Network.SharePeerGrants[0].Validate(); err == nil {
				t.Fatal("invalid grant accepted")
			}
		})
	}
	for _, instant := range []time.Time{now.Add(-time.Nanosecond), now.Add(time.Minute), now.Add(time.Hour)} {
		if err := ValidateSharePeerGrantsAt(snapshot, instant); err == nil {
			t.Fatal("grant accepted outside lease")
		}
	}
	if err := ValidateSharePeerGrantsAt(snapshot, now.Add(time.Minute-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
}

func TestSharingSignatureCoversBothIdentitiesAndExpires(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := sharingSnapshot(now)
	snapshot.MapSignature, err = SignNetworkMapSnapshotAt(private, snapshot, now, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	anchor := base64.RawURLEncoding.EncodeToString(public)
	if err := VerifyNetworkMapSnapshotSignatureAt(snapshot, anchor, now); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*NetworkMapSnapshot){
		"remove grant":   func(s *NetworkMapSnapshot) { s.Network.SharePeerGrants = nil },
		"recipient key":  func(s *NetworkMapSnapshot) { s.Network.SharePeerGrants[0].RecipientPublicKey = s.Peers[0].PublicKey },
		"source address": func(s *NetworkMapSnapshot) { s.Network.SharePeerGrants[0].SourceAllowedIPs[0] = "100.65.0.2/32" },
		"rights":         func(s *NetworkMapSnapshot) { s.Network.SharePeerGrants[0].Rights[0].LastPort = 444 },
		"lease":          func(s *NetworkMapSnapshot) { s.Network.SharePeerGrants[0].ExpiresAt = now.Add(2 * time.Minute) },
		"revision":       func(s *NetworkMapSnapshot) { s.Network.SharePeerGrants[0].Revision++ },
		"network":        func(s *NetworkMapSnapshot) { s.Peers[0].NetworkID = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneNetworkMapSnapshot(snapshot)
			mutate(&candidate)
			if err := VerifyNetworkMapSnapshotSignatureAt(candidate, anchor, now); err == nil {
				t.Fatal("tampered sharing accepted")
			}
		})
	}
	if err := VerifyNetworkMapSnapshotSignatureAt(snapshot, anchor, now.Add(time.Minute)); err == nil {
		t.Fatal("map signature extended grant lease")
	}
}
