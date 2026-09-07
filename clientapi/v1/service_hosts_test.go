package clientapi

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

func serviceHostSnapshot() NetworkMapSnapshot {
	snapshot := validMapStreamSnapshot()
	snapshot.Network.Services = []AdvertisedService{{ID: "svc", Name: "db", DNSName: "db.example", Ports: []ServicePort{{Protocol: "tcp", Port: 5432}}, ApprovalMode: "manual", ApprovalStatus: "approved", Hosts: []ServiceHost{{NodeID: snapshot.Node.ID, PublicKey: snapshot.Node.PublicKey}}}}
	return snapshot
}

func TestServiceHostIdentityValidation(t *testing.T) {
	snapshot := serviceHostSnapshot()
	if err := ValidateNetworkMapSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*AdvertisedService){
		"unapproved":   func(s *AdvertisedService) { s.ApprovalStatus = "pending" },
		"unknown node": func(s *AdvertisedService) { s.Hosts[0].NodeID = "other" },
		"changed key":  func(s *AdvertisedService) { s.Hosts[0].PublicKey = "changed" },
		"empty key":    func(s *AdvertisedService) { s.Hosts[0].PublicKey = "" },
		"duplicate":    func(s *AdvertisedService) { s.Hosts = append(s.Hosts, s.Hosts[0]) },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneNetworkMapSnapshot(snapshot)
			mutate(&candidate.Network.Services[0])
			if err := ValidateNetworkMapSnapshot(candidate); err == nil {
				t.Fatal("invalid host accepted")
			}
		})
	}
	if err := ValidateNetworkMapSnapshot(snapshot); err != nil {
		t.Fatalf("clone mutation changed original: %v", err)
	}
}

func TestServiceHostsRequireAuthenticMap(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := serviceHostSnapshot()
	snapshot.MapSignature, err = SignNetworkMapSnapshotAt(private, snapshot, now, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	anchor := base64.RawURLEncoding.EncodeToString(public)
	if err := VerifyNetworkMapSnapshotSignatureAt(snapshot, anchor, now); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*AdvertisedService){
		"remove approval": func(s *AdvertisedService) { s.Hosts = nil },
		"change host":     func(s *AdvertisedService) { s.Hosts[0].NodeID = "other" },
		"change identity": func(s *AdvertisedService) { s.Hosts[0].PublicKey = "other" },
		"change port":     func(s *AdvertisedService) { s.Ports[0].Port = 22 },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneNetworkMapSnapshot(snapshot)
			mutate(&candidate.Network.Services[0])
			if err := VerifyNetworkMapSnapshotSignatureAt(candidate, anchor, now); err == nil {
				t.Fatal("tampered host authorization accepted")
			}
		})
	}
}
