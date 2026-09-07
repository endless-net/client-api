package clientapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterNodeSDKValidatesOperationBinding(t *testing.T) {
	req := renewalRequestFixture(t)
	for _, mismatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "bound", true: "wrong-operation"}[mismatch], func(t *testing.T) {
			response := renewalResponseFixture(t, req)
			if mismatch {
				response.IdempotencyID = "another-registration-operation"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/nodes/register" {
					t.Errorf("unexpected procedure: %s %s", r.Method, r.URL.Path)
				}
				decoded, err := DecodeRegisterNodeRequest(r.Body)
				if err != nil || decoded.IdempotencyID != req.IdempotencyID {
					t.Errorf("request: %+v, %v", decoded, err)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			_, err := NewAPI(server.URL, "").RegisterNode(req)
			if mismatch && (err == nil || !strings.Contains(err.Error(), "idempotency_id")) {
				t.Fatalf("mismatched operation error: %v", err)
			}
			if !mismatch && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRegistrationRejectsSupersededBodyBeforeTransport(t *testing.T) {
	req := renewalRequestFixture(t)
	raw, err := MarshalRegisterNodeRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(string(raw), "idempotency_id", "idempotency_key", 1)
	if _, err := DecodeRegisterNodeRequest(strings.NewReader(body)); err == nil {
		t.Fatal("superseded field accepted")
	}
	req.SchemaVersion = 0
	if _, err := NewAPI("https://invalid.example", "").RegisterNode(req); err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("invalid request must fail before transport: %v", err)
	}
}
