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
