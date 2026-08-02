# Frozen server release migration source

Server release-control ownership moved to the private
[`endless-net/releases`](https://github.com/endless-net/releases) repository.
Under architecture decisions D-020 and D-025, `client-api` continues to own
public/client and browser contracts plus contract gates, but it no longer owns
server manifest schemas, candidates, evidence, approval, promotion, or released
envelopes.

Authority is pinned in the versioned migration inventory to architecture commit
`a4a4798de03ca93d626dd242b55884aa3d478c67`. The target repository baseline is
`endless-net/releases@51775a764544b8aad44a9fc44a9e67da543034cf`.

## Migration state

State is `copy_pending`; production is not authorized.

Creating the private repository established the ownership boundary but did not
complete the migration. The following evidence is still required:

1. Releases copies every listed record byte-for-byte and verifies its digest.
2. Releases provides protected append-only validation and approval-aware
   promotion preserving the exact tested server component set.
3. System Tests resolves a candidate from Releases by digest and publishes
   passing evidence bound to that digest.
4. Infrastructure accepts only a released envelope from Releases and rejects
   these legacy candidates at production entry points.
5. Client API retires its retained release tooling only after the preceding
   copy and consumer evidence exists.

The machine-readable source of truth is
[`migration/v1/inventory.json`](migration/v1/inventory.json), validated by
[`migration/v1/inventory.schema.json`](migration/v1/inventory.schema.json).
Every gate is explicitly `pending`, and its evidence is `null`.

## Frozen compatibility source

The existing files remain at their historical paths to avoid breaking strict
consumers during the copy phase:

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
commit-addressed `server-release-migration-source` artifact. It verifies each
inventory digest, copies the frozen records and retained validator sources,
writes `SHA256SUMS`, and marks `production_authorized: false`.

This artifact exists only so Releases can perform and prove the copy. It is not
a candidate publication, approval, promotion, released manifest, Infrastructure
desired state, or production deployment request.

## Retained legacy tooling

`systemtests/releasecontract`, `systemtests/cmd/releasecontract`, and their exact
fixtures/tests remain temporarily so current gates keep working and Releases can
port the semantic checks. They are deprecated migration sources. Client API no
longer runs candidate publication or server promotion automation.

Do not remove the records or retained gates until immutable evidence completes
every inventory cutover gate. Do not interpret the ownership documentation, the
migration artifact, or a green client-api CI run as proof that Releases,
System Tests, Infrastructure, or production cutover is complete.

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
