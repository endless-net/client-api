package clientapi

import (
	"bytes"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestPublicErrorClosedCodesRoundTripAndSemantics(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		status   int
		terminal bool
	}{
		{ErrorCodeNodeCredentialRenewalRequired, http.StatusConflict, false},
		{ErrorCodeNodeCredentialUnknown, http.StatusUnauthorized, true},
		{ErrorCodeNodeCredentialRevoked, http.StatusUnauthorized, true},
		{ErrorCodeNodeCredentialExpired, http.StatusUnauthorized, true},
		{ErrorCodeNodeCredentialInvalid, http.StatusUnauthorized, false},
		{ErrorCodeNodeIdentityBindingMismatch, http.StatusConflict, false},
		{ErrorCodeAuthenticationRequired, http.StatusUnauthorized, false},
		{ErrorCodeAuthorizationDenied, http.StatusForbidden, false},
		{ErrorCodeTemporarilyUnavailable, http.StatusServiceUnavailable, false},
	}
	if got := KnownErrorCodes(); !reflect.DeepEqual(got, []ErrorCode{
		ErrorCodeNodeCredentialRenewalRequired,
		ErrorCodeNodeCredentialUnknown,
		ErrorCodeNodeCredentialRevoked,
		ErrorCodeNodeCredentialExpired,
		ErrorCodeNodeCredentialInvalid,
		ErrorCodeNodeIdentityBindingMismatch,
		ErrorCodeAuthenticationRequired,
		ErrorCodeAuthorizationDenied,
		ErrorCodeTemporarilyUnavailable,
	}) {
		t.Fatalf("KnownErrorCodes() = %v", got)
	}
	for _, test := range tests {
		t.Run(string(test.code), func(t *testing.T) {
			value, err := NewPublicError(test.code, "safe diagnostic", "edge:req-123")
			if err != nil {
				t.Fatal(err)
			}
			if err := value.ValidateHTTPStatus(test.status); err != nil {
				t.Fatal(err)
			}
			if err := value.ValidateHTTPResponse(test.status, value.RequestID); err != nil {
				t.Fatal(err)
			}
			if value.ErrorCode.RequiresReEnrollment() != test.terminal {
				t.Fatalf("RequiresReEnrollment() = %t, want %t", value.ErrorCode.RequiresReEnrollment(), test.terminal)
			}
			raw, err := MarshalPublicError(value)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodePublicError(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			if decoded != value {
				t.Fatalf("round trip = %#v, want %#v", decoded, value)
			}
		})
	}

	got := KnownErrorCodes()
	got[0] = "mutated"
	if KnownErrorCodes()[0] != ErrorCodeNodeCredentialRenewalRequired {
		t.Fatal("KnownErrorCodes returned mutable package state")
	}
}

func TestPublicErrorRejectsProtocolAndLegacyFallbacks(t *testing.T) {
	valid := `{"schema_version":2,"error_code":"node_credential_unknown","diagnostic_message":"not found","request_id":"req-1"}`
	tests := map[string]string{
		"plain text":         "forbidden",
		"legacy object":      `{"error":"forbidden"}`,
		"unknown field":      strings.TrimSuffix(valid, "}") + `,"message":"legacy"}`,
		"unknown code":       strings.Replace(valid, "node_credential_unknown", "forbidden", 1),
		"wrong schema":       strings.Replace(valid, `"schema_version":2`, `"schema_version":1`, 1),
		"missing request id": strings.Replace(valid, `,"request_id":"req-1"`, "", 1),
		"unsafe diagnostic":  strings.Replace(valid, "not found", `not\nfound`, 1),
		"trailing JSON":      valid + `{}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if value, err := DecodePublicError(strings.NewReader(raw)); err == nil {
				t.Fatalf("DecodePublicError accepted %#v", value)
			}
		})
	}

	unknown := ErrorCode("future_code")
	if unknown.Valid() || unknown.RequiresReEnrollment() {
		t.Fatal("unknown error code acquired recovery semantics")
	}
	value, err := NewPublicError(ErrorCodeAuthorizationDenied, "denied", "req-403")
	if err != nil {
		t.Fatal(err)
	}
	if err := value.ValidateHTTPStatus(http.StatusUnauthorized); err == nil {
		t.Fatal("ValidateHTTPStatus accepted mismatched status")
	}
	if err := value.ValidateHTTPResponse(http.StatusForbidden, "different-request"); err == nil {
		t.Fatal("ValidateHTTPResponse accepted mismatched request ID")
	}
}
