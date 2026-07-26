package wgkeys

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestValidateSafeTextRejectsConfigurationSeparators(t *testing.T) {
	for _, value := range []string{"safe\nPostUp = evil", "safe\rpeer", "safe\x00peer", "safe\u0085peer", "safe\u2028peer", "safe\u2029peer"} {
		if err := ValidateSafeText("value", value); err == nil {
			t.Fatalf("ValidateSafeText(%q) accepted an unsafe value", value)
		}
	}
}

func TestValidatePublicKey(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString(make([]byte, 32))
	if err := ValidatePublicKey(valid); err != nil {
		t.Fatalf("ValidatePublicKey(valid) = %v", err)
	}
	for _, value := range []string{"", strings.TrimSuffix(valid, "="), "public-key", valid + "\n"} {
		if err := ValidatePublicKey(value); err == nil {
			t.Fatalf("ValidatePublicKey(%q) accepted an invalid key", value)
		}
	}
}

func TestValidateHostnameAndEndpoint(t *testing.T) {
	for _, hostname := range []string{"node-1", "node-1.example.test", "NODE.EXAMPLE"} {
		if err := ValidateHostname(hostname); err != nil {
			t.Fatalf("ValidateHostname(%q) = %v", hostname, err)
		}
	}
	for _, hostname := range []string{"-node", "node_1", "node.", "node\n.example"} {
		if err := ValidateHostname(hostname); err == nil {
			t.Fatalf("ValidateHostname(%q) accepted an invalid hostname", hostname)
		}
	}
	for _, endpoint := range []string{"node.example.test:51820", "192.0.2.1:1", "[2001:db8::1]:65535"} {
		if err := ValidateEndpoint(endpoint); err != nil {
			t.Fatalf("ValidateEndpoint(%q) = %v", endpoint, err)
		}
	}
	for _, endpoint := range []string{"2001:db8::1:51820", "node.example.test:0", "node.example.test:65536", "node.example.test:01", "node\n.example:51820"} {
		if err := ValidateEndpoint(endpoint); err == nil {
			t.Fatalf("ValidateEndpoint(%q) accepted an invalid endpoint", endpoint)
		}
	}
}
