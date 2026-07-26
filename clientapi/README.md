# EndlessNet Client API

`clientapi` is the producer-owned, versioned Go contract between the EndlessNet
control plane and independently released clients.

The module contains:

- `v1`: public HTTP DTOs, the strict control-plane HTTP SDK, map-stream framing,
  identity proof binding, signed map and node-credential verification;
- `wireguard`: shared WireGuard key, address, prefix and endpoint validation used
  on both sides of the contract.

The private control plane owns the producer behavior for this module. Client,
MCP and compatibility gates consume the public versioned module without
importing `internal/control` or another backend-internal package.

Releases use submodule tags in the form `clientapi/v1.1.0`. A breaking contract
change requires a new Go module major version and an explicit product migration;
moving or copying internal backend structs into a client repository is not a
supported compatibility mechanism.
