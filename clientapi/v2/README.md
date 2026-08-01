# EndlessNet Client API v2 recovery contract

Module: `github.com/endless-net/client-api/clientapi/v2`

This module owns the public wire contract for node credential recovery after a
server signing-identity change. It implements the contract accepted at
`endless-net/architecture@6cf37091846920e238bef631ef8951d395c084a1` without a
plain-text, message-matching, or HTTP-status-only compatibility path.

## Public errors

`PublicError` is the only recovery error body. Its required JSON fields are
`schema_version`, `error_code`, `diagnostic_message`, and `request_id`.
`diagnostic_message` is safe support context; consumers must never branch on it,
localize it, or show it directly to users. `request_id` must equal the edge
`X-Request-ID` value.

| `ErrorCode` constant | JSON value | HTTP | Clears node-bound state |
| --- | --- | ---: | --- |
| `ErrorCodeNodeCredentialRenewalRequired` | `node_credential_renewal_required` | 409 | no |
| `ErrorCodeNodeCredentialUnknown` | `node_credential_unknown` | 401 | yes |
| `ErrorCodeNodeCredentialRevoked` | `node_credential_revoked` | 401 | yes |
| `ErrorCodeNodeCredentialExpired` | `node_credential_expired` | 401 | yes |
| `ErrorCodeNodeCredentialInvalid` | `node_credential_invalid` | 401 | no |
| `ErrorCodeNodeIdentityBindingMismatch` | `node_identity_binding_mismatch` | 409 | no |
| `ErrorCodeAuthenticationRequired` | `authentication_required` | 401 | no |
| `ErrorCodeAuthorizationDenied` | `authorization_denied` | 403 | no |
| `ErrorCodeTemporarilyUnavailable` | `temporarily_unavailable` | 503 | no |

Only `ErrorCode.RequiresReEnrollment()` may authorize automatic removal of
node-bound enrollment state. It returns true only for unknown, revoked, and
expired credentials. An absent/unknown code, malformed DTO, generic 403, invalid
credential, binding mismatch, policy denial, or unavailable dependency is
non-terminal and must preserve enrollment.

Producers construct errors with `NewPublicError` and serialize them with
`MarshalPublicError`. Consumers use `DecodePublicError`, then
`ValidateHTTPResponse` to check the code/status/request-ID relationship.
`KnownErrorCodes` is the exhaustive closed enum. Gateway proxy/upstream failures
for which no authoritative decision was obtained use
`ErrorCodeTemporarilyUnavailable`/503. CORS rejection, Host/SNI mismatch, and an
unmatched route do not impersonate recovery domain errors.

## Credential renewal and re-registration

`RegisterNodeRequest` and `RegisterNodeResponse` are the v2 bodies for
`POST /nodes/register`. Both require `schema_version` and the same
`idempotency_id`. The client creates the ID with
`NewRegistrationIdempotencyID`, saves it durably before the request, and reuses
it with the same old credential until the validated response is committed. The
producer stores the result durably and returns that exact result for every
retry, including after the old credential has been superseded.

A request with `node_credential` is renewal (`IsCredentialRenewal`); it also
requires the saved `registration_binding`, `network_id`, device identity proof,
WireGuard public key, and installation fingerprint. Renewal must not mix the old
credential with join/session enrollment authorization. A success returns the
echoed ID, current `node_credential`, request-bound `registration_binding`, and
signed network map.

Client proof helpers are:

- `RegistrationSessionTokenBinding` for direct enrollment session binding;
- `RegistrationIdentityProofPayload` and
  `RegistrationIdentityProofBinding` for the canonical request binding;
- `SetRegisterNodeIdentityProof` for signing with the device Ed25519 key;
- `VerifyRegisterNodeIdentityProof` for producer verification.

The proof contains only digests of bearer credentials. It covers the durable
idempotency ID, old registration binding, network, device identity/WireGuard
key, installation fingerprint, and endpoint projection. Possession of the old
node credential alone is therefore insufficient for renewal.

Use `MarshalRegisterNodeRequest`/`DecodeRegisterNodeRequest` and
`MarshalRegisterNodeResponse`/`DecodeRegisterNodeResponse` at wire boundaries.
The decoders reject unknown fields, trailing JSON, oversized bodies, invalid
schema versions, and invalid DTOs. Before committing a response, the client also
calls `RegisterNodeResponse.ValidateForRequest` and verifies the returned node
credential and map signatures with its pinned trust bundles.
