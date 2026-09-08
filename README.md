# EndlessNet public contracts and integration gates

This public repository owns public/client and browser-facing contracts plus
their cross-repository contract gates. It intentionally contains no runtime
binary and is not the server release-control owner.

- `clientapi/` — the single v1 client contract, including recovery and node RPCs;
- `contracts/` — browser-auth semantics and the public-site runtime
  configuration contract consumed by `endless-net/front`; Management owns the
  browser API as Protobuf/Connect and publishes its generated SDK;
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

## Continuous integration

[CI](https://github.com/endless-net/client-api/actions/workflows/ci.yml) is the
single workflow entry point. Pull requests, pushes to `main`, and manual
`operation: check` runs execute these checks for **each** Go module (`clientapi`,
`contracts`, and `systemtests`):

| Job | Required checks |
| --- | --- |
| `format` | `goimports` formatting and import order |
| `lint` | `go vet` and the shared standard `golangci-lint` configuration |
| `unit-test` | Full `go test -count=1 ./...`, without `-short` |
| `race` | Full tests with the Go race detector |
| `vulnerability-scan` | `govulncheck -test` including test dependencies |
| `modules-licenses` | Module integrity and dependency licenses, including test dependencies |
| `build` | Build every Go package |

`release-history` rejects edits to frozen release records. `protobuf` lints and
builds the public Protobuf contract, then reports breaking changes against the
newest stable `clientapi/v*` tag with the same Go module path. The compatibility
comparison is advisory; its artifact records the exact baseline or an explicit
skip if no released Protobuf baseline exists. It does not check Go source API
compatibility or HTTP/JSON behavior.

CodeQL runs as `analyze`; pull requests also require DCO `signoff`.
`test-complete` requires every applicable job to succeed. The shared lint config
checks existing as well as new code; only the frozen migration CLI and its audit
tests may import the deprecated release-control archive.

After successful checks on a `main` push, `publish-migration-source` publishes a
digest-verified migration audit if that commit changes a migration inventory.
Audits reference their own CI run and do not authorize production deployment.

Manual `operation: system` runs only the cutover system suite using the
`cutover-candidate` environment and its API URL and node ID. The weekly schedule
runs CodeQL only. Normal Go tests do not enable the `system` build tag.
