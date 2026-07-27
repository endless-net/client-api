//go:build system

package systemtests

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGatewayCutoverSecurityAndMapProtocol(t *testing.T) {
	apiURL := requiredEnvironment(t, "ENDLESSNET_SYSTEM_API_URL")
	nodeID := requiredEnvironment(t, "ENDLESSNET_SYSTEM_NODE_ID")
	client := &http.Client{Timeout: 10 * time.Second}

	serverKey := request(t, client, http.MethodGet, apiURL+"/server-key", nil)
	if serverKey.StatusCode != http.StatusOK {
		t.Fatalf("/server-key status = %d", serverKey.StatusCode)
	}
	closeResponse(t, serverKey)

	oldProtocol := request(t, client, http.MethodGet,
		apiURL+"/maps/"+nodeID+"/stream?timeout=1s",
		map[string]string{"X-EndlessNet-Map-Protocol": "2"},
	)
	if oldProtocol.StatusCode != http.StatusUpgradeRequired {
		t.Fatalf("map protocol v2 status = %d, want 426", oldProtocol.StatusCode)
	}
	closeResponse(t, oldProtocol)

	v3WithoutCredential := request(t, client, http.MethodGet,
		apiURL+"/maps/"+nodeID+"/stream?timeout=1s",
		map[string]string{"X-EndlessNet-Map-Protocol": "3"},
	)
	if v3WithoutCredential.StatusCode != http.StatusUnauthorized {
		t.Fatalf("map v3 without node credential status = %d, want 401", v3WithoutCredential.StatusCode)
	}
	closeResponse(t, v3WithoutCredential)
}

func request(t *testing.T, client *http.Client, method, target string, headers map[string]string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(target, "/"), nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func closeResponse(t *testing.T, response *http.Response) {
	t.Helper()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
}

func requiredEnvironment(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		t.Fatalf("%s is required", name)
	}
	return strings.TrimRight(value, "/")
}
