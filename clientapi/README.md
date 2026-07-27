# EndlessNet Client API

`clientapi` is the producer-owned, versioned Go contract between the EndlessNet
control plane and independently released clients.

The module contains:

- the module root: public HTTP DTOs, the strict control-plane HTTP SDK,
  map-stream v3 framing, identity proof binding, signed map and
  node-credential verification;
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

Releases use submodule tags in the form `clientapi/v2.0.0`. A breaking contract
change requires a new Go module major version and an explicit product migration.
