package clientapi

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestApplyMapStreamDeltaVerifiesResultBeforeReturning(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	trust, err := NewSigningTrustBundle(base64.RawURLEncoding.EncodeToString(publicKey))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	current := validMapStreamSnapshot()
	current.MapSignature, err = SignNetworkMapSnapshotAt(privateKey, current, now.Add(-time.Minute), time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	peer := Peer{
		ID:         "peer-1",
		Hostname:   "peer-1",
		PublicKey:  base64.StdEncoding.EncodeToString(make([]byte, 32)),
		AllowedIPs: []string{"100.64.0.2/32"},
	}
	expected := cloneNetworkMapSnapshot(current)
	expected.Revision.Network = 2
	expected.Peers = append(expected.Peers, peer)
	expected.MapSignature = nil
	resultSignature, err := SignNetworkMapSnapshotAt(privateKey, expected, now.Add(-time.Minute), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	event := MapStreamEvent{
		Type:            "delta",
		ProtocolVersion: MapStreamProtocolVersion,
		Capabilities:    MapStreamSupportedCapabilities(),
		EventID:         "event-2",
		From:            current.Revision,
		To:              expected.Revision,
		BaseHash:        current.MapSignature.PayloadHash,
		Delta:           &MapDelta{PeerUpserts: []Peer{peer}},
		ResultSignature: resultSignature,
	}

	applied, err := ApplyMapStreamEvent(current, event, trust, now)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Revision.Equal(expected.Revision) || len(applied.Peers) != 1 || applied.Peers[0].ID != peer.ID {
		t.Fatalf("applied snapshot = %#v", applied)
	}
	if applied.MapSignature == nil || applied.MapSignature.PayloadHash != resultSignature.PayloadHash {
		t.Fatalf("applied signature = %#v", applied.MapSignature)
	}

	replayed, err := ApplyMapStreamEvent(applied, event, trust, now)
	if !errors.Is(err, ErrMapStreamEventAlreadyApplied) {
		t.Fatalf("duplicate event error = %v", err)
	}
	if replayed.MapSignature.PayloadHash != applied.MapSignature.PayloadHash || len(replayed.Peers) != len(applied.Peers) {
		t.Fatal("duplicate event changed the applied map")
	}
}

func TestApplyMapStreamDeltaRequiresResyncOnCursorMismatch(t *testing.T) {
	current := validMapStreamSnapshot()
	current.MapSignature = &MapSignature{PayloadHash: "current-hash"}
	event := MapStreamEvent{
		Type:            "delta",
		ProtocolVersion: MapStreamProtocolVersion,
		Capabilities:    MapStreamSupportedCapabilities(),
		EventID:         "event-3",
		From:            MapRevision{Network: 2},
		To:              MapRevision{Network: 3},
		BaseHash:        "different-hash",
		Delta:           &MapDelta{},
		ResultSignature: &MapSignature{PayloadHash: "result-hash"},
	}

	_, err := ApplyMapStreamEvent(current, event, SigningTrustBundle{}, time.Now())
	if !errors.Is(err, ErrMapStreamResyncRequired) {
		t.Fatalf("cursor mismatch error = %v", err)
	}
}

func validMapStreamSnapshot() NetworkMapSnapshot {
	publicKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	return NetworkMapSnapshot{
		Revision: MapRevision{Network: 1},
		Network: Network{
			ID:       "net-1",
			Name:     "test",
			CIDR:     "100.64.0.0/24",
			Revision: 1,
			OwnerID:  "user-1",
		},
		Node: Node{
			ID:            "node-1",
			NetworkID:     "net-1",
			UserID:        "user-1",
			Hostname:      "node-1",
			PublicKey:     publicKey,
			AssignedIP:    "100.64.0.1",
			ApprovalState: NodeApprovalApproved,
		},
		Peers: []Peer{},
	}
}
