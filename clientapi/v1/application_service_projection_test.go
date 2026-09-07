package clientapi

import (
	"strings"
	"testing"
)

func TestNetworkMapValidatesApplicationAndServiceProjection(t *testing.T) {
	snapshot := validMapStreamSnapshot()
	snapshot.Network.Applications = []Application{{ID: "app-1", Name: "portal", TargetType: "url", Target: "https://portal.example", PolicyHash: strings.Repeat("a", 64), Sources: []ServiceHost{{NodeID: snapshot.Node.ID, PublicKey: snapshot.Node.PublicKey}}, DNSEnabled: true}}
	snapshot.Network.Services = []AdvertisedService{{ID: "service-1", Name: "database", DNSName: "database.example", Ports: []ServicePort{{Protocol: "tcp", Port: 5432}}, ApprovalMode: "manual", ApprovalStatus: "approved", Health: "healthy"}}
	if err := ValidateNetworkMapSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}

	for name, mutate := range map[string]func(*Network){
		"duplicate application": func(network *Network) { network.Applications = append(network.Applications, network.Applications[0]) },
		"invalid service port":  func(network *Network) { network.Services[0].Ports[0].Port = 0 },
		"invalid service dns":   func(network *Network) { network.Services[0].DNSName = "bad domain" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneNetworkMapSnapshot(snapshot)
			mutate(&candidate.Network)
			if err := ValidateNetworkMapSnapshot(candidate); err == nil {
				t.Fatal("invalid projection accepted")
			}
		})
	}
}

func TestNetworkMapCloneDetachesApplicationAndServiceProjection(t *testing.T) {
	snapshot := validMapStreamSnapshot()
	snapshot.Network.Applications = []Application{{ID: "app-1", Name: "portal", TargetType: "url", Target: "https://portal.example", PolicyHash: strings.Repeat("a", 64), Sources: []ServiceHost{{NodeID: snapshot.Node.ID, PublicKey: snapshot.Node.PublicKey}}}}
	snapshot.Network.Services = []AdvertisedService{{ID: "service-1", Name: "database", DNSName: "database.example", Ports: []ServicePort{{Protocol: "tcp", Port: 5432}}, Tags: []string{"database"}, ApprovalMode: "manual", ApprovalStatus: "approved", Health: "healthy"}}
	clone := cloneNetworkMapSnapshot(snapshot)
	clone.Network.Applications[0].Sources[0].NodeID = "changed"
	clone.Network.Services[0].Ports[0].Port = 443
	clone.Network.Services[0].Tags[0] = "changed"
	if snapshot.Network.Applications[0].Sources[0].NodeID != snapshot.Node.ID || snapshot.Network.Services[0].Ports[0].Port != 5432 || snapshot.Network.Services[0].Tags[0] != "database" {
		t.Fatal("clone shares application or service projection storage")
	}
}
