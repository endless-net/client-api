package clientapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAPIRejectsPlaintextRemoteControlPlane(t *testing.T) {
	api := newAPIWithNodeCredentialForTest("http://control.example.test", "session-secret", "node-secret")
	_, err := requestMeForTest(api)
	if err == nil || !strings.Contains(err.Error(), "must use HTTPS") {
		t.Fatalf("plaintext remote error = %v", err)
	}
}

func TestControlPlaneHTTPClientDoesNotFollowCredentialRedirects(t *testing.T) {
	var redirected atomic.Int64
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Add(1)
		if r.Header.Get("X-EndlessNet-Node-Credential") != "" || r.Header.Get("Authorization") != "" {
			t.Error("credentials reached redirect destination")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destination.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", destination.URL+"/capture")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer source.Close()

	api := newAPIWithNodeCredentialForTest(source.URL, "session-secret", "node-secret")
	api.HTTPClient = NewControlPlaneHTTPClient(time.Second, nil)
	if _, err := requestMeForTest(api); err == nil || !strings.Contains(err.Error(), http.StatusText(http.StatusTemporaryRedirect)) {
		t.Fatalf("redirect response error = %v", err)
	}
	if got := redirected.Load(); got != 0 {
		t.Fatalf("redirect destination requests = %d, want 0", got)
	}
}

func TestControlPlaneURLAllowsHTTPSAndLoopbackDevelopment(t *testing.T) {
	for _, rawURL := range []string{
		"https://control.example.test",
		"http://localhost:7070",
		"http://127.0.0.1:7070",
		"http://[::1]:7070",
	} {
		if err := validateControlPlaneBaseURL(rawURL); err != nil {
			t.Errorf("validateControlPlaneBaseURL(%q): %v", rawURL, err)
		}
	}
	for _, rawURL := range []string{
		"http://control.example.test",
		"http://localhost.example.test",
		"ftp://127.0.0.1",
		"https://user@example.test",
	} {
		if err := validateControlPlaneBaseURL(rawURL); err == nil {
			t.Errorf("validateControlPlaneBaseURL(%q) accepted an unsafe URL", rawURL)
		}
	}
}
