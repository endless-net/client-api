package clientapi

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newAPIWithNodeCredentialForTest(baseURL, token, nodeCredential string) *API {
	return NewAPIWithNodeCredentialURLs([]string{baseURL}, token, nodeCredential)
}

type testMeResponse struct {
	Email string `json:"email"`
}

func requestMeForTest(api *API) (testMeResponse, error) {
	var out testMeResponse
	return out, api.request(http.MethodGet, "/auth/me", nil, &out)
}

func TestNewControlPlaneHTTPClientRequiresTLS13(t *testing.T) {
	httpClient := NewControlPlaneHTTPClient(15*time.Second, nil)
	transport, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("control-plane transport = %T, want *http.Transport", httpClient.Transport)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("control-plane TLS config is nil")
	}
	if transport.TLSClientConfig.MinVersion != tls.VersionTLS13 {
		t.Fatalf("control-plane TLS min version = %#x, want TLS 1.3", transport.TLSClientConfig.MinVersion)
	}
	if transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("control-plane TLS config enables InsecureSkipVerify")
	}
}

func TestAPIRejectsTLS12ControlPlane(t *testing.T) {
	called := false
	server := newControlPlaneTLSServer(t, tls.VersionTLS12, tls.VersionTLS12, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"email":"user@example.test"}`))
	})
	defer server.Close()

	api := NewAPI(server.URL, "session-token")
	api.HTTPClient = NewControlPlaneHTTPClient(2*time.Second, trustedServerRoots(server))
	_, err := requestMeForTest(api)
	if err == nil {
		t.Fatal("API Me succeeded against TLS 1.2-only control-plane")
	}
	if called {
		t.Fatal("TLS 1.2-only control-plane handler was reached")
	}
}

func TestAPIRejectsUntrustedTLS13ControlPlaneCertificate(t *testing.T) {
	called := false
	server := newControlPlaneTLSServer(t, tls.VersionTLS13, tls.VersionTLS13, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"email":"mitm@example.test"}`))
	})
	defer server.Close()

	api := NewAPI(server.URL, "session-token")
	api.HTTPClient = NewControlPlaneHTTPClient(2*time.Second, nil)
	_, err := requestMeForTest(api)
	if err == nil {
		t.Fatal("API Me succeeded against untrusted TLS 1.3 control-plane certificate")
	}
	if called {
		t.Fatal("untrusted TLS 1.3 control-plane handler was reached")
	}
}

func TestAPIAcceptsTLS13ControlPlane(t *testing.T) {
	server := newControlPlaneTLSServer(t, tls.VersionTLS13, tls.VersionTLS13, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/me" {
			t.Fatalf("request path = %q, want /auth/me", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer session-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"email":"user@example.test"}`))
	})
	defer server.Close()

	api := NewAPI(server.URL, "session-token")
	api.HTTPClient = NewControlPlaneHTTPClient(2*time.Second, trustedServerRoots(server))
	me, err := requestMeForTest(api)
	if err != nil {
		t.Fatalf("API Me failed against TLS 1.3 control-plane: %v", err)
	}
	if me.Email != "user@example.test" {
		t.Fatalf("me = %#v, want email from TLS 1.3 server", me)
	}
}

func TestAPIRejectsUnknownAndTrailingResponseJSON(t *testing.T) {
	for index, response := range []string{
		`{"email":"user@example.test","removed_public_key":"legacy"}`,
		`{"email":"user@example.test"} {"email":"second@example.test"}`,
	} {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, response)
			}))
			defer server.Close()

			api := NewAPI(server.URL, "session-token")
			if _, err := requestMeForTest(api); err == nil {
				t.Fatalf("API accepted non-canonical response %s", response)
			}
		})
	}
}

func TestAPIFailsOverToNextCoordinator(t *testing.T) {
	primaryCalls := 0
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls++
		http.Error(w, "primary unavailable", http.StatusServiceUnavailable)
	}))
	defer primary.Close()

	secondaryCalls := 0
	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondaryCalls++
		if r.URL.Path != "/auth/me" {
			t.Fatalf("request path = %q, want /auth/me", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer session-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"email":"fallback@example.test"}`))
	}))
	defer secondary.Close()

	api := NewAPIWithBaseURLs([]string{primary.URL, secondary.URL}, "session-token")
	me, err := requestMeForTest(api)
	if err != nil {
		t.Fatal(err)
	}
	if me.Email != "fallback@example.test" {
		t.Fatalf("me = %#v, want fallback email", me)
	}
	if primaryCalls != 1 || secondaryCalls != 1 {
		t.Fatalf("calls primary=%d secondary=%d, want 1/1", primaryCalls, secondaryCalls)
	}
	if api.BaseURL != secondary.URL {
		t.Fatalf("active base URL = %q, want fallback %q", api.BaseURL, secondary.URL)
	}

	_, err = requestMeForTest(api)
	if err != nil {
		t.Fatal(err)
	}
	if primaryCalls != 1 || secondaryCalls != 2 {
		t.Fatalf("calls after active fallback primary=%d secondary=%d, want 1/2", primaryCalls, secondaryCalls)
	}
}

func TestAPIDoesNotFailOverAuthenticationFailure(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer primary.Close()
	secondaryCalls := 0
	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondaryCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer secondary.Close()

	api := NewAPIWithBaseURLs([]string{primary.URL, secondary.URL}, "bad-token")
	_, err := requestMeForTest(api)
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("Me error = %v, want 401", err)
	}
	if secondaryCalls != 0 {
		t.Fatalf("secondary calls = %d, want no auth failover", secondaryCalls)
	}
}

func TestAPIExposesControlPlaneStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"enrollment request not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	api := NewAPI(server.URL, "poll-token")
	_, err := api.NodeEnrollmentRequestStatus("ner_missing", "poll-token")
	if !IsControlPlaneStatus(err, http.StatusNotFound) {
		t.Fatalf("status error = %v, want typed 404", err)
	}
	var statusErr *ControlPlaneStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("status error type = %T, want *ControlPlaneStatusError", err)
	}
	if statusErr.Method != http.MethodGet || statusErr.Path != "/nodes/enrollment-requests/ner_missing" || statusErr.StatusCode != http.StatusNotFound {
		t.Fatalf("status error = %#v", statusErr)
	}
	if !strings.Contains(statusErr.Body, "enrollment request not found") || !strings.Contains(err.Error(), "404 Not Found") {
		t.Fatalf("status error text = %q body=%q", err, statusErr.Body)
	}
}

func TestReadMapStreamEventFailsOverToNextCoordinator(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "stream primary unavailable", http.StatusServiceUnavailable)
	}))
	defer primary.Close()

	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/maps/node-1/stream" {
			t.Fatalf("request path = %q, want map stream", r.URL.Path)
		}
		if got := r.Header.Get("X-EndlessNet-Node-Credential"); got != "credential" {
			t.Fatalf("node credential = %q, want credential", got)
		}
		if got, want := r.Header.Get("X-EndlessNet-Map-Protocol"), strconv.Itoa(MapStreamProtocolVersion); got != want {
			t.Fatalf("protocol header = %q, want %s", got, want)
		}
		if got, want := r.Header.Get("X-EndlessNet-Map-Capabilities"), strings.Join(MapStreamSupportedCapabilities(), ","); got != want {
			t.Fatalf("capabilities header = %q, want %q", got, want)
		}
		setMapStreamResponseHeaders(w)
		w.Header().Set("Content-Type", "application/x-ndjson")
		_ = json.NewEncoder(w).Encode(MapStreamEvent{
			Type:            "snapshot",
			ProtocolVersion: MapStreamProtocolVersion,
			Capabilities:    MapStreamSupportedCapabilities(),
			ToRevision:      9,
			Map: &RegisterNodeResponse{
				Network: Network{ID: "net-1", Revision: 9},
				Node:    Node{ID: "node-1", NetworkID: "net-1"},
			},
		})
	}))
	defer secondary.Close()

	api := NewAPIWithNodeCredentialURLs([]string{primary.URL, secondary.URL}, "", "credential")
	event, err := api.ReadMapStreamEvent("node-1", 7, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if event.ToRevision != 9 || event.Map.Network.ID != "net-1" {
		t.Fatalf("event = %#v, want fallback stream event", event)
	}
	if api.BaseURL != secondary.URL {
		t.Fatalf("active base URL = %q, want fallback %q", api.BaseURL, secondary.URL)
	}
}

func TestReadMapStreamEventRejectsMissingCapabilities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setMapStreamResponseHeaders(w)
		w.Header().Set("Content-Type", "application/x-ndjson")
		_ = json.NewEncoder(w).Encode(MapStreamEvent{
			Type:            "snapshot",
			ProtocolVersion: MapStreamProtocolVersion,
			ToRevision:      3,
			Map: &RegisterNodeResponse{
				Network: Network{ID: "net-1", Revision: 3},
				Node:    Node{ID: "node-1", NetworkID: "net-1"},
			},
		})
	}))
	defer server.Close()

	api := newAPIWithNodeCredentialForTest(server.URL, "", "credential")
	_, err := api.ReadMapStreamEvent("node-1", 0, time.Second)
	if err == nil || !strings.Contains(err.Error(), "event capabilities") {
		t.Fatalf("missing capabilities error = %v", err)
	}
}

func TestReadMapStreamEventSkipsHeartbeat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-EndlessNet-Map-Capabilities"); !strings.Contains(got, MapStreamCapabilityHeartbeat) {
			t.Fatalf("capabilities header = %q, want heartbeat opt-in", got)
		}
		setMapStreamResponseHeaders(w)
		w.Header().Set("Content-Type", "application/x-ndjson")
		encoder := json.NewEncoder(w)
		_ = encoder.Encode(MapStreamEvent{
			Type:            "heartbeat",
			ProtocolVersion: MapStreamProtocolVersion,
			Capabilities:    MapStreamSupportedCapabilities(),
			FromRevision:    3,
			ToRevision:      3,
		})
		_ = encoder.Encode(MapStreamEvent{
			Type:            "snapshot",
			ProtocolVersion: MapStreamProtocolVersion,
			Capabilities:    MapStreamSupportedCapabilities(),
			FromRevision:    3,
			ToRevision:      4,
			Map: &RegisterNodeResponse{
				Network: Network{ID: "net-1", Revision: 4},
				Node:    Node{ID: "node-1", NetworkID: "net-1"},
			},
		})
	}))
	defer server.Close()

	api := newAPIWithNodeCredentialForTest(server.URL, "", "credential")
	event, err := api.ReadMapStreamEvent("node-1", 3, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "snapshot" || event.FromRevision != 3 || event.ToRevision != 4 {
		t.Fatalf("event after heartbeat = %#v", event)
	}
}

func TestReadMapStreamEventRejectsExplicitCapabilitiesWithoutFullMap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setMapStreamResponseHeaders(w)
		w.Header().Set("Content-Type", "application/x-ndjson")
		_ = json.NewEncoder(w).Encode(MapStreamEvent{
			Type:            "snapshot",
			ProtocolVersion: MapStreamProtocolVersion,
			Capabilities:    []string{MapStreamCapabilityDelta},
			ToRevision:      3,
			Map: &RegisterNodeResponse{
				Network: Network{ID: "net-1", Revision: 3},
				Node:    Node{ID: "node-1", NetworkID: "net-1"},
			},
		})
	}))
	defer server.Close()

	api := newAPIWithNodeCredentialForTest(server.URL, "", "credential")
	_, err := api.ReadMapStreamEvent("node-1", 0, time.Second)
	if err == nil || !strings.Contains(err.Error(), "event capabilities") {
		t.Fatalf("ReadMapStreamEvent error = %v, want missing full-map rejection", err)
	}
}

func TestReadMapStreamEventRejectsUnknownCapability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setMapStreamResponseHeaders(w)
		w.Header().Set("Content-Type", "application/x-ndjson")
		capabilities := append(MapStreamSupportedCapabilities(), "future-v3")
		_ = json.NewEncoder(w).Encode(MapStreamEvent{
			Type:            "snapshot",
			ProtocolVersion: MapStreamProtocolVersion,
			Capabilities:    capabilities,
			Map: &RegisterNodeResponse{
				Network: Network{ID: "net-1", Revision: 1},
				Node:    Node{ID: "node-1", NetworkID: "net-1"},
			},
		})
	}))
	defer server.Close()

	api := newAPIWithNodeCredentialForTest(server.URL, "", "credential")
	_, err := api.ReadMapStreamEvent("node-1", 0, time.Second)
	if err == nil || !strings.Contains(err.Error(), "event capabilities") {
		t.Fatalf("ReadMapStreamEvent error = %v, want unknown capability rejection", err)
	}
}

func TestReadMapStreamEventRejectsRemovedRevisionField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setMapStreamResponseHeaders(w)
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, `{"type":"snapshot","protocol_version":2,"capabilities":["full-map","delta","resync","heartbeat"],"previous_revision":0,"to_revision":1,"map":{"network":{"id":"net-1","revision":1},"node":{"id":"node-1","network_id":"net-1"}}}`+"\n")
	}))
	defer server.Close()

	api := newAPIWithNodeCredentialForTest(server.URL, "", "credential")
	_, err := api.ReadMapStreamEvent("node-1", 0, time.Second)
	if err == nil || !strings.Contains(err.Error(), "previous_revision") {
		t.Fatalf("ReadMapStreamEvent error = %v, want removed field rejection", err)
	}
}

func TestReadMapStreamEventRejectsRemovedEmbeddedSigningKeys(t *testing.T) {
	for name, mapJSON := range map[string]string{
		"map-signature":    `{"network":{"id":"net-1","revision":1},"node":{"id":"node-1","network_id":"net-1"},"map_signature":{"public_key":"removed"}}`,
		"relay-credential": `{"network":{"id":"net-1","revision":1},"node":{"id":"node-1","network_id":"net-1"},"relay_credential":{"public_key":"removed"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				setMapStreamResponseHeaders(w)
				w.Header().Set("Content-Type", "application/x-ndjson")
				_, _ = io.WriteString(w, `{"type":"snapshot","protocol_version":2,"capabilities":["full-map","delta","resync","heartbeat"],"from_revision":0,"to_revision":1,"map":`+mapJSON+`}`+"\n")
			}))
			defer server.Close()

			api := newAPIWithNodeCredentialForTest(server.URL, "", "credential")
			if _, err := api.ReadMapStreamEvent("node-1", 0, time.Second); err == nil || !strings.Contains(err.Error(), "public_key") {
				t.Fatalf("ReadMapStreamEvent error = %v, want removed public_key rejection", err)
			}
		})
	}
}

func TestUpdateNodeEndpointStateSendsCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s, want PATCH", r.Method)
		}
		var req UpdateNodeEndpointRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Endpoint != "192.168.55.12:51820" || req.Generation != 7 || req.TTL != "2m" || !sameStrings(req.Candidates, []string{"192.168.55.12:51820", "198.51.100.12:51820"}) {
			t.Fatalf("endpoint state request = %#v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RegisterNodeResponse{
			Network: Network{ID: "net-1", Revision: 8},
			Node: Node{
				ID:                 "node-1",
				NetworkID:          "net-1",
				Endpoint:           req.Endpoint,
				EndpointGeneration: req.Generation,
				EndpointCandidates: req.Candidates,
			},
		})
	}))
	defer server.Close()

	api := newAPIWithNodeCredentialForTest(server.URL, "", "credential")
	response, err := api.UpdateNodeEndpointState("node-1", UpdateNodeEndpointRequest{
		Endpoint:   "192.168.55.12:51820",
		Generation: 7,
		Candidates: []string{"192.168.55.12:51820", "198.51.100.12:51820"},
		TTL:        "2m",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Node.EndpointGeneration != 7 || !sameStrings(response.Node.EndpointCandidates, []string{"192.168.55.12:51820", "198.51.100.12:51820"}) {
		t.Fatalf("response = %#v", response.Node)
	}
}

func TestUpdateNodeEndpointStatePublishesOfflineStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/nodes/node-1/endpoint" {
			t.Fatalf("path = %q, want endpoint route", r.URL.Path)
		}
		if got := r.Header.Get("X-EndlessNet-Node-Credential"); got != "credential" {
			t.Fatalf("node credential = %q, want credential", got)
		}
		var req UpdateNodeEndpointRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Endpoint != "" || req.Status != NodeStatusOffline {
			t.Fatalf("status request = %#v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RegisterNodeResponse{
			Network: Network{ID: "net-1", Revision: 4},
			Node:    Node{ID: "node-1", NetworkID: "net-1", Status: req.Status},
		})
	}))
	defer server.Close()

	api := newAPIWithNodeCredentialForTest(server.URL, "", "credential")
	response, err := api.UpdateNodeEndpointState("node-1", UpdateNodeEndpointRequest{Status: NodeStatusOffline})
	if err != nil {
		t.Fatal(err)
	}
	if response.Node.Status != NodeStatusOffline {
		t.Fatalf("response = %#v", response)
	}
}

func TestReadMapStreamEventRejectsOversizedControlFrame(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setMapStreamResponseHeaders(w)
		w.Header().Set("Content-Type", "application/x-ndjson")
		chunk := strings.Repeat("x", 32*1024)
		for written := 0; written <= MaxControlMapStreamEventBytes; written += len(chunk) {
			if _, err := io.WriteString(w, chunk); err != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}))
	defer server.Close()

	api := newAPIWithNodeCredentialForTest(server.URL, "", "credential")
	api.HTTPClient.Timeout = 2 * time.Second
	_, err := api.ReadMapStreamEvent("node-1", 0, time.Second)
	if err == nil || !strings.Contains(err.Error(), "control map stream event exceeds") {
		t.Fatalf("ReadMapStreamEvent error = %v, want oversized control frame rejection", err)
	}
}

func setMapStreamResponseHeaders(w http.ResponseWriter) {
	w.Header().Set("X-EndlessNet-Map-Protocol", strconv.Itoa(MapStreamProtocolVersion))
	w.Header().Set("X-EndlessNet-Map-Capabilities", strings.Join(MapStreamSupportedCapabilities(), ","))
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func newControlPlaneTLSServer(t *testing.T, minVersion, maxVersion uint16, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{
		MinVersion: minVersion,
		MaxVersion: maxVersion,
	}
	server.StartTLS()
	return server
}

func trustedServerRoots(server *httptest.Server) *x509.CertPool {
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	return roots
}
