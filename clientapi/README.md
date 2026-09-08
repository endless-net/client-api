# EndlessNet Client API

`clientapi` contains the producer-owned Go contracts between the EndlessNet
control plane and independently released clients.

The module contains:

- the `v1` package: public HTTP DTOs, the strict control-plane HTTP SDK,
  map-stream v3 framing, identity proof binding, signed map and
  node-credential verification;
- `v1` also owns strict recovery errors and idempotent registration/renewal;
- `v1/clientrpc` and `v1/clientrpc/clientrpcconnect`: client-owned protobuf DTOs
  and Connect clients/handlers generated from `proto/client/v1/client.proto`;
- `wireguard`: shared WireGuard key, address, prefix and endpoint validation used
  on both sides of the contract.

## Method reference

Each remote method documents its purpose, input scope, result and caller
responsibilities at its source declaration:

- [HTTP SDK](v1/api.go): server key, network creation/listing, node listing,
  join tokens, direct registration/renewal, browser enrollment creation/status/
  completion, endpoint updates, map streaming and logout (13 methods).
- [Protobuf RPC contract](proto/client/v1/client.proto): account/network/route
  selection, billing plans/subscription/usage/checkout/invoices, application
  discovery and flow-log policy/reporting (11 methods).
- [Generated Connect interfaces](v1/clientrpc/clientrpcconnect/client.connect.go)
  carry the RPC descriptions into Go client and handler documentation. Edit the
  proto source and regenerate using [the documented command](UNIFIED-CONTRACT.md#client-rpc-ownership).

The HTTP SDK and RPC clients have separate transports: configuring `API.Token`,
`API.NodeCredential` or HTTP failover does not configure a generated Connect
client. Supply the appropriate authorization to its requests separately. HTTP
network/node lists return arrays; paginated RPCs use opaque continuation tokens.

Comments describe this contract and SDK behavior, not a new verification of
deployed producers. The schema does not enumerate every RPC error code, billing
period value, currency amount unit, pagination limit or discovery TTL bound;
consumers must not invent those guarantees from field types. Producer conformance
and system acceptance retain the scope recorded in
[cutover evidence](UNIFIED-CONTRACT.md#implementation-evidence-and-separate-system-follow-up).

## Contract boundaries

The private control plane owns the producer behavior for this module. Client,
MCP and compatibility gates consume the public versioned module without
importing `internal/control` or another backend-internal package.

Map-stream v3 uses a `(network, global)` revision vector. Every delta identifies
its base hash and carries a signature for the complete resulting map. Clients
reconstruct into a copy, validate and authenticate it, and only then replace
cache and WireGuard state atomically. A mismatch requires a full snapshot; no
personalized delta history or explicit ACK is part of the protocol.

[Signed machine sharing](SHARING.md) defines exact endpoint grants, recipient-only
initiation, short leases and the enforcement requirements for consumers.

The sole Go module is `github.com/endless-net/client-api/clientapi`.
The nested recovery module has been removed. This is an explicitly authorized
breaking change within v1, with no compatibility aliases or wire fallback.
Schema and proof generation numbers retain their existing values.
See [recovery](RECOVERY.md) and [cutover requirements](UNIFIED-CONTRACT.md).
Module release and consumer pin changes require separate version approval.
