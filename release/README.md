# Frozen server release migration source

## Current D-025 component contract

Architecture commit
[`a7a42114af6f38b389ffbce85678999e69021e70`](https://github.com/endless-net/architecture/tree/a7a42114af6f38b389ffbce85678999e69021e70)
supersedes the full server-set rollout model retained below as historical
context. A new candidate and released record each describe one deployable
producer component only. No component waits for a mandatory seven-component
production snapshot.

The component record binds its exact commit, immutable GitHub Release archive
digest, module/contract pins, provenance, and the consumer/provider gates that
the component actually affects. Promotion binds the released record to that
exact candidate digest. A changed artifact digest requires a new candidate and
fresh affected-edge evidence.

Releases sends Infrastructure only the component, immutable released-record
digest, manifest commit, and event ID. Infrastructure deduplicates on
`(environment, component, released record digest, config generation)` and owns
desired state, plan/apply, host access, probe policy, rollback, inventory and
secrets. Candidates, arbitrary artifact overrides, host targets, commands,
`skip_checks`, and credentials are never producer inputs to deployment.

The executable schema and validation fixtures for this handoff are in
[`migration/v3`](migration/v3/). They are documentation and compatibility
fixtures, not new production release records. `client-api` continues to own
public client/browser contracts; it does not own Infrastructure deployment or
server release-control records.

## Superseded full-server-set handoff

Server release-control ownership moved to the private
[`endless-net/releases`](https://github.com/endless-net/releases) repository.
Under architecture decisions D-020 and D-025, `client-api` continues to own
public/client and browser contracts plus contract gates, but it no longer owns
server manifest schemas, candidates, evidence, approval, promotion, or released
envelopes.

The superseded handoff authority is pinned in the versioned migration inventory to architecture commit
`a4a4798de03ca93d626dd242b55884aa3d478c67`. Its Releases implementation is
pinned at
[`89e6129dd7304a05bb2b7f18c771d776058b3dcc`](https://github.com/endless-net/releases/tree/89e6129dd7304a05bb2b7f18c771d776058b3dcc).

## Migration state

State is `destination_implemented_consumer_cutover_pending`; production is not
authorized.

Releases now owns the v1 schemas, semantic validator, exact fixtures,
append-only history gate, actor-recording protected promotion workflow, and the
`gateway-23121de-d025-pilot-rc1` candidate with its provenance. Client API's
active candidate publication and promotion ownership is stopped.

The remaining work is consumer cutover, not recreation of the contract here:

1. System Tests resolves a candidate from Releases by digest and publishes
   passing evidence bound to that digest.
2. Infrastructure accepts only a released envelope from Releases and rejects
   these legacy candidates at production entry points.
3. Client API retires its retained release tooling only after both consumer
   cutovers have immutable evidence.

The current machine-readable source of truth is
[`migration/v2/inventory.json`](migration/v2/inventory.json), validated by
[`migration/v2/inventory.schema.json`](migration/v2/inventory.schema.json). It
records exact destination file digests, the consumer resolution contract, and
which gates are verified or pending. The original
[`migration/v1/inventory.json`](migration/v1/inventory.json) remains unchanged
as the historical pre-implementation handoff.

## Canonical consumer contract

Consumers must use the Releases v1 schemas and fixtures from the same pinned
revision; they must not copy schemas into their own repositories.

- Validation consumers resolve
  [`gateway-23121de-d025-pilot-rc1`](https://github.com/endless-net/releases/blob/89e6129dd7304a05bb2b7f18c771d776058b3dcc/candidates/gateway-23121de-d025-pilot-rc1.json)
  and its
  [candidate provenance](https://github.com/endless-net/releases/blob/89e6129dd7304a05bb2b7f18c771d776058b3dcc/candidate-provenance/gateway-23121de-d025-pilot-rc1.json)
  by the digests in the v2 inventory. A candidate is valid only for the
  `validation` environment class.
- Production consumers resolve only an immutable record under the Releases
  `released/` path and then use `resolve-release`. At the pinned revision there
  is no released record for the pilot, so it is not a production input.
- Cross-repository decoder tests use
  [`fixtures/v1`](https://github.com/endless-net/releases/tree/89e6129dd7304a05bb2b7f18c771d776058b3dcc/fixtures/v1)
  as the versioned handoff contract.

## Frozen compatibility source

The existing files remain at their historical paths to avoid breaking strict
consumers during the evidenced cutover:

- `manifest.schema.json` and `evidence.schema.json`;
- `candidates/` and `evidence/` operational records;
- `schemas/v1/` D-025 schema handoff;
- `fixtures/v1/` exact decoder and policy fixtures.

These paths are frozen. CI rejects additions, edits, renames, or deletions under
the legacy server-release source. A new server candidate, schema version,
evidence record, approval, or released envelope belongs only in Releases.

There are no operational candidate-provenance records, System Tests promotion
evidence records, approvals, or released envelopes in this repository. The
released envelope under `fixtures/v1/` is synthetic test data and cannot
authorize deployment. Existing candidates remain `candidate` and validation
only.

## Handoff publication

After a green `main` revision adds a versioned migration inventory, the workflow
in `.github/workflows/publish-release-migration-source.yml` publishes a
commit-addressed migration audit artifact. It verifies the referenced historical
inventory and frozen source digests, includes retained compatibility gates,
writes `SHA256SUMS`, and marks `production_authorized: false`.

This artifact preserves an auditable handoff and rollback reference. It is not
a canonical server contract, candidate publication, approval, promotion,
released manifest, Infrastructure desired state, or production deployment
request. Canonical server release-control records now come from Releases.

## Retained legacy tooling

`systemtests/releasecontract`, `systemtests/cmd/releasecontract`, and their exact
fixtures/tests remain temporarily so existing Client API gates and consumers
keep working during cutover. They are deprecated compatibility sources; the
implemented owner is Releases. Client API no longer runs candidate publication
or server promotion automation.

Do not remove the records or retained gates until immutable evidence completes
the System Tests and Infrastructure cutover gates. Do not interpret the
ownership documentation, the migration artifact, or a green Client API or
Releases CI run as proof that consumer or production cutover is complete.

## Historical record notes

The independent Admin Web artifact remains a first-class historical `admin`
component; its runtime policy is pinned separately as `infrastructure`.

The `client-signing-identity-recovery` contract declaration pins architecture
commit `6cf37091846920e238bef631ef8951d395c084a1` and requires the checksummed
`github.com/endless-net/client-api/clientapi/v2` module plus exact participating
component commits.

The `gateway-browser-login-v2-rc1` candidate and matching evidence preserve the
exact Gateway, Admin, Identity, supporting service, client, and OIDC fixture
revisions accepted by the historical System Tests consumer at
`f4ff29a13d973c4067c0a3787355bc197dd8c40a`. Their continued presence proves
only legacy candidate compatibility, not promotion or production readiness.
