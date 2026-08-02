# EndlessNet public contracts and integration gates

This public repository owns public/client and browser-facing contracts plus
their cross-repository contract gates. It intentionally contains no runtime
binary and is not the server release-control owner.

- `clientapi/` — canonical v1 plus the independently pinnable
  `github.com/endless-net/client-api/clientapi/v2` recovery contract;
- `contracts/` — browser OpenAPI and runtime configuration contracts;
- `architecture/` — cross-service boundaries and interaction rules;
- `release/` — frozen legacy server-release records and the versioned handoff
  inventory for migration to private `endless-net/releases`;
- `systemtests/` — cross-repository contract and system gates.

The private [`endless-net/releases`](https://github.com/endless-net/releases)
repository owns server manifest schemas, candidates, evidence, approvals,
promotion, and released envelopes. Service implementations remain in their
producer-owned repositories; Infrastructure alone owns production desired state
and deployment execution.

The records under `release/` remain byte-for-byte available so current consumers
are not broken while the copy and consumer cutover are proved. They are a
migration source, not an authorized production input. The ownership migration
is incomplete until Releases, System Tests, and Infrastructure provide the
evidence listed in `release/migration/v1/inventory.json`.
