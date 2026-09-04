# EndlessNet public contracts and integration gates

This public repository owns public/client and browser-facing contracts plus
their cross-repository contract gates. It intentionally contains no runtime
binary and is not the server release-control owner.

- `clientapi/` — canonical v1 plus the independently pinnable
  `github.com/endless-net/client-api/clientapi/v2` recovery contract;
- `contracts/` — browser OpenAPI/auth contracts and the public-site runtime
  configuration contract consumed by `endless-net/front`;
- `architecture/` — cross-service boundaries and interaction rules;
- `release/` — frozen legacy server-release records and the versioned handoff
  inventory for migration to private `endless-net/releases`;
- `systemtests/` — cross-repository contract and system gates.

The private [`endless-net/releases`](https://github.com/endless-net/releases)
repository owns one-component server candidates, affected-edge evidence,
approvals, promotion, and immutable released-component records. A released
component references one exact tested candidate; it is not a mandatory
seven-component server snapshot. Service implementations remain in their
producer-owned repositories; Infrastructure alone owns production desired state
and deployment execution after receiving the released-component signal by
digest.

The records under `release/` remain byte-for-byte available so current consumers
are not broken while their cutovers are proved. They are a frozen compatibility
source, not an authorized production input. New server-release consumers must
use component-scoped candidate/released records from `endless-net/releases`;
public and browser contracts remain owned here.

The superseded full-server-set destination implementation is pinned at
[`endless-net/releases@89e6129dd7304a05bb2b7f18c771d776058b3dcc`](https://github.com/endless-net/releases/tree/89e6129dd7304a05bb2b7f18c771d776058b3dcc).
The ownership handoff is implemented, but production cutover remains incomplete
until System Tests and Infrastructure provide the evidence listed in
`release/migration/v2/inventory.json`.
