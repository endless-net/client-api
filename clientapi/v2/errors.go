package clientapi

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorCode is a closed public control-plane error classification. Consumers
// must reject values not returned by KnownErrorCodes; diagnostic text and HTTP
// status are never compatibility fallbacks for an absent or unknown code.
type ErrorCode string

const (
	ErrorCodeNodeCredentialRenewalRequired ErrorCode = "node_credential_renewal_required"
	ErrorCodeNodeCredentialUnknown         ErrorCode = "node_credential_unknown"
	ErrorCodeNodeCredentialRevoked         ErrorCode = "node_credential_revoked"
	ErrorCodeNodeCredentialExpired         ErrorCode = "node_credential_expired"
	ErrorCodeNodeCredentialInvalid         ErrorCode = "node_credential_invalid"
	ErrorCodeNodeIdentityBindingMismatch   ErrorCode = "node_identity_binding_mismatch"
	ErrorCodeAuthenticationRequired        ErrorCode = "authentication_required"
	ErrorCodeAuthorizationDenied           ErrorCode = "authorization_denied"
	ErrorCodeTemporarilyUnavailable        ErrorCode = "temporarily_unavailable"
)

var knownErrorCodes = []ErrorCode{
	ErrorCodeNodeCredentialRenewalRequired,
	ErrorCodeNodeCredentialUnknown,
	ErrorCodeNodeCredentialRevoked,
	ErrorCodeNodeCredentialExpired,
	ErrorCodeNodeCredentialInvalid,
	ErrorCodeNodeIdentityBindingMismatch,
	ErrorCodeAuthenticationRequired,
	ErrorCodeAuthorizationDenied,
	ErrorCodeTemporarilyUnavailable,
}

// KnownErrorCodes returns the complete v2 wire enum in stable declaration
// order. The returned slice is independent and may be modified by the caller.
func KnownErrorCodes() []ErrorCode {
	return append([]ErrorCode(nil), knownErrorCodes...)
}

// Valid reports whether c belongs to the closed v2 wire enum.
func (c ErrorCode) Valid() bool {
	_, ok := c.HTTPStatus()
	return ok
}

// HTTPStatus returns the one status assigned to the code by the public
// recovery contract. Transport status remains observable, but clients branch
// only after successfully decoding and validating ErrorCode.
func (c ErrorCode) HTTPStatus() (int, bool) {
	switch c {
	case ErrorCodeNodeCredentialRenewalRequired, ErrorCodeNodeIdentityBindingMismatch:
		return http.StatusConflict, true
	case ErrorCodeNodeCredentialUnknown, ErrorCodeNodeCredentialRevoked,
		ErrorCodeNodeCredentialExpired, ErrorCodeNodeCredentialInvalid,
		ErrorCodeAuthenticationRequired:
		return http.StatusUnauthorized, true
	case ErrorCodeAuthorizationDenied:
		return http.StatusForbidden, true
	case ErrorCodeTemporarilyUnavailable:
		return http.StatusServiceUnavailable, true
	default:
		return 0, false
	}
}

// RequiresReEnrollment reports the only terminal public error semantics that
// authorize a client to clear node-bound enrollment state. Invalid, malformed,
// or unknown codes are deliberately non-terminal.
func (c ErrorCode) RequiresReEnrollment() bool {
	switch c {
	case ErrorCodeNodeCredentialUnknown, ErrorCodeNodeCredentialRevoked, ErrorCodeNodeCredentialExpired:
		return true
	default:
		return false
	}
}

// PublicError is the machine-readable body returned for public node-route and
// edge failures. DiagnosticMessage must be safe for support diagnostics, but
// is never a localization key, UI string, or recovery decision input.
type PublicError struct {
	SchemaVersion     int       `json:"schema_version"`
	ErrorCode         ErrorCode `json:"error_code"`
	DiagnosticMessage string    `json:"diagnostic_message"`
	RequestID         string    `json:"request_id"`
}

// NewPublicError constructs and validates a v2 error body.
func NewPublicError(code ErrorCode, diagnosticMessage, requestID string) (PublicError, error) {
	value := PublicError{
		SchemaVersion:     SchemaVersion,
		ErrorCode:         code,
		DiagnosticMessage: diagnosticMessage,
		RequestID:         requestID,
	}
	if err := value.Validate(); err != nil {
		return PublicError{}, err
	}
	return value, nil
}

// Validate checks the closed code and all required safe wire fields.
func (e PublicError) Validate() error {
	if e.SchemaVersion != SchemaVersion {
		return fmt.Errorf("public error schema_version = %d, want %d", e.SchemaVersion, SchemaVersion)
	}
	if !e.ErrorCode.Valid() {
		return fmt.Errorf("unknown public error_code %q", e.ErrorCode)
	}
	if err := validateSafeString("diagnostic_message", e.DiagnosticMessage, 1, maxDiagnosticMessageBytes); err != nil {
		return err
	}
	if err := validateIdentifier("request_id", e.RequestID, 1, maxRequestIDBytes); err != nil {
		return err
	}
	return nil
}

// ValidateHTTPStatus ensures that a producer did not attach valid domain
// semantics to the wrong HTTP status.
func (e PublicError) ValidateHTTPStatus(status int) error {
	if err := e.Validate(); err != nil {
		return err
	}
	expected, _ := e.ErrorCode.HTTPStatus()
	if status != expected {
		return fmt.Errorf("public error_code %q requires HTTP %d, got %d", e.ErrorCode, expected, status)
	}
	return nil
}

// ValidateHTTPResponse validates both status semantics and the edge
// X-Request-ID correlation value associated with the body.
func (e PublicError) ValidateHTTPResponse(status int, requestID string) error {
	if err := e.ValidateHTTPStatus(status); err != nil {
		return err
	}
	if err := validateIdentifier("X-Request-ID", requestID, 1, maxRequestIDBytes); err != nil {
		return err
	}
	if e.RequestID != requestID {
		return errors.New("public error request_id does not match X-Request-ID")
	}
	return nil
}
