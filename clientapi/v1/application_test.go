package clientapi

import (
	"strings"
	"testing"
	"time"
)

func TestApplicationTargetRejectsUnenforceableScope(t *testing.T) {
	for _, target := range []struct{ kind, value string }{
		{"url", "https://example.com/admin"}, {"url", "https://user:password@example.com"}, {"url", "https://example.com?secret=1"},
		{"domain", "*.example.com"}, {"cidr", "0.0.0.0/0"}, {"cidr", "127.0.0.0/8"}, {"cidr", "169.254.0.0/16"}, {"cidr", "10.0.0.1/24"},
		{"cidr", "128.0.0.0/1"}, {"cidr", "126.0.0.0/7"}, {"cidr", "169.0.0.0/8"}, {"cidr", "fe00::/8"}, {"cidr", "255.255.255.255/32"},
	} {
		if _, err := ParseApplicationTarget(target.kind, target.value); err == nil {
			t.Fatalf("unenforceable target accepted: %v", target)
		}
	}
	for _, target := range []struct{ kind, value string }{{"url", "https://example.com:8443"}, {"domain", "private.example.com"}, {"cidr", "10.0.0.0/24"}} {
		if _, err := ParseApplicationTarget(target.kind, target.value); err != nil {
			t.Fatal(err)
		}
	}
}

func TestApplicationRouteBindingsFailClosed(t *testing.T) {
	snapshot := validMapStreamSnapshot()
	binding := ServiceHost{NodeID: snapshot.Node.ID, PublicKey: snapshot.Node.PublicKey}
	snapshot.Network.Applications = []Application{{ID: "app", Name: "app", TargetType: "domain", Target: "private.example.com", PolicyHash: strings.Repeat("a", 64), Sources: []ServiceHost{binding}, Connectors: []ServiceHost{binding}, Routes: []ApplicationRoute{{Connector: binding, CIDRs: []string{"10.0.0.5/32"}, ExpiresAt: time.Now().Add(time.Minute)}}}}
	if err := ValidateNetworkMapSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Application){
		"source":    func(a *Application) { a.Sources[0].NodeID = "foreign" },
		"connector": func(a *Application) { a.Routes[0].Connector.PublicKey = "rotated" },
		"scope":     func(a *Application) { a.Routes[0].CIDRs = []string{"10.0.0.0/8"} },
		"expiry":    func(a *Application) { a.Routes[0].ExpiresAt = time.Time{} },
		"policy":    func(a *Application) { a.PolicyHash = "" },
		"overlay":   func(a *Application) { a.Routes[0].CIDRs = []string{snapshot.Node.AssignedIP + "/32"} },
	} {
		t.Run(name, func(t *testing.T) {
			clone := cloneNetworkMapSnapshot(snapshot)
			mutate(&clone.Network.Applications[0])
			if err := ValidateNetworkMapSnapshot(clone); err == nil {
				t.Fatal("invalid binding accepted")
			}
		})
	}
	if err := ValidateNetworkMapSnapshot(snapshot); err != nil {
		t.Fatal("clone shares authorization", err)
	}
}
