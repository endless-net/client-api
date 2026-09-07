package clientapi

import "testing"

func TestNetworkMapValidatesEffectiveDNSConfig(t *testing.T) {
	snapshot := validMapStreamSnapshot()
	snapshot.Network.DNSConfig = &DNSConfig{
		MagicDNSEnabled:  true,
		OverrideLocalDNS: true,
		Suffix:           "prod.endlessnet",
		Nameservers: []DNSNameserver{
			{ID: "global", Address: "192.0.2.53", Scope: "global", Priority: 100},
			{ID: "split", Address: "2001:db8::53", Scope: "split", Priority: 10, SplitDomains: []string{"corp.example"}},
		},
		SearchDomains: []string{"prod.endlessnet", "corp.example"},
	}
	if err := ValidateNetworkMapSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}

	for name, mutate := range map[string]func(*DNSConfig){
		"invalid suffix":          func(value *DNSConfig) { value.Suffix = "bad domain" },
		"duplicate id":            func(value *DNSConfig) { value.Nameservers[1].ID = "global" },
		"noncanonical address":    func(value *DNSConfig) { value.Nameservers[0].Address = "192.000.2.53" },
		"global split domain":     func(value *DNSConfig) { value.Nameservers[0].SplitDomains = []string{"corp.example"} },
		"split without domain":    func(value *DNSConfig) { value.Nameservers[1].SplitDomains = nil },
		"duplicate search domain": func(value *DNSConfig) { value.SearchDomains = []string{"corp.example", "CORP.EXAMPLE."} },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneNetworkMapSnapshot(snapshot)
			mutate(candidate.Network.DNSConfig)
			if err := ValidateNetworkMapSnapshot(candidate); err == nil {
				t.Fatal("invalid DNS configuration accepted")
			}
		})
	}
}

func TestNetworkMapCloneDetachesDNSConfig(t *testing.T) {
	snapshot := validMapStreamSnapshot()
	snapshot.Network.DNSConfig = &DNSConfig{
		Nameservers:   []DNSNameserver{{ID: "split", Address: "192.0.2.53", Scope: "split", SplitDomains: []string{"corp.example"}}},
		SearchDomains: []string{"corp.example"},
	}
	clone := cloneNetworkMapSnapshot(snapshot)
	clone.Network.DNSConfig.Nameservers[0].SplitDomains[0] = "changed.example"
	clone.Network.DNSConfig.SearchDomains[0] = "changed.example"
	if snapshot.Network.DNSConfig.Nameservers[0].SplitDomains[0] != "corp.example" || snapshot.Network.DNSConfig.SearchDomains[0] != "corp.example" {
		t.Fatal("verified DNS configuration aliases mutable input")
	}
}
