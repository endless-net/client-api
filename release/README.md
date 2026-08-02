# Compatibility manifests

CI assembles a candidate from exact Git commits, module checksums, and immutable
artifact digests. A candidate is content-addressed as the SHA-256 of its exact
bytes and is usable only by an isolated validation environment. After the
cross-repository system suite passes those exact bytes, promotion creates a new
immutable released envelope. Production never consumes a candidate directly.

## D-025 contract layout

The original `manifest.schema.json` and candidate object remain unchanged for
strict existing consumers. Its historical `released` enum value remains in the
schema for compatibility, but is not an operational production input. D-025 is
an additive versioned contract under `schemas/v1/`:

- `candidate.schema.json` narrows the legacy manifest to `status: candidate`;
- `candidate-provenance.schema.json` binds every resolved component and module
  to producer build provenance;
- `system-test-evidence.schema.json` binds a passing System Tests run to the
  exact candidate digest;
- `promotion-request.schema.json` supplies immutable references to those three
  records and promotion provenance;
- `released-envelope.schema.json` carries the unchanged resolved set and the
  exact tested candidate digest;
- `resolution.schema.json` is the normalized machine contract consumed by
  Infrastructure.

New operational records use these paths:

```text
release/candidates/<release>.json
release/candidate-provenance/<release>.json
release/system-test-evidence/<release>.json
release/releases/<release>.json
```

All four paths are append-only. CI rejects modifying, renaming, or deleting an
existing record. A corrected component, digest, provenance statement, or test
run therefore creates a new release name and candidate. The promotion writer
also opens its output with create-only semantics and cannot overwrite a record.

## Promotion and resolution

The semantic validator in `systemtests/releasecontract` supplements JSON Schema
where cross-document equality is required. Promotion recomputes the digest of
each input, checks complete component/module provenance, requires a passing
System Tests result, rejects mutable artifact tags, and copies the candidate's
`components`, `modules`, and `contracts` arrays without reinterpretation.

From `systemtests/`, an operator or CI job creates a released envelope with:

```text
go run ./cmd/releasecontract promote \
  --candidate ../release/candidates/<release>.json \
  --candidate-provenance ../release/candidate-provenance/<release>.json \
  --system-test-evidence ../release/system-test-evidence/<release>.json \
  --request <promotion-request.json> \
  --output ../release/releases/<release>.json
```

`resolve-candidate` emits a `validation` resolution and accepts only a
candidate plus its complete provenance. `resolve-release` revalidates every
referenced record and emits a `production` resolution only from a released
envelope. Infrastructure must verify the source record digest before using the
normalized resolved set; it must reject `environment_class: validation` at all
production entry points.

The exact handoff fixtures are in `fixtures/v1/`. Infrastructure and System
Tests should run their decoders and policy gates against those commit-addressed
fixtures. They should consume the schemas from the same published contract
bundle rather than maintain copied schemas in their repositories.

The independent Admin Web artifact is a first-class `admin` component. Its
runtime policy is pinned separately as `infrastructure`; it is never folded into
the `management` component or selected by a Management release.

Schema version 1 uses canonical `endless-net/*` repository coordinates. Legacy
pre-cutover repository coordinates are rejected after the clean cutover.

Manifests that claim the `client-signing-identity-recovery` contract include a
`contracts` entry at version 1 and architecture commit
`6cf37091846920e238bef631ef8951d395c084a1`. The schema and compatibility gate
then require:

- a checksummed v2 pin of
  `github.com/endless-net/client-api/clientapi/v2`;
- exact component commits for Client API, Coordinator, Gateway, Client, Client
  UI, and System Tests.

This declaration is added only to the immutable candidate that runs the
recovery acceptance suite. Older candidates that do not claim the contract do
not acquire a false recovery guarantee.

The `gateway-browser-login` version 2 contract is bound to architecture commit
`62175d5e97e3a5a57dcb0f2ab2c377c5eb7cd4ac`. Its candidate pins the exact
Gateway, Admin, Identity, supporting service, client, and OIDC fixture revisions
and artifacts accepted by the System Tests run-manifest validator. The matching
document under `release/evidence/` records the producer CI, publication,
artifact, archive, and provenance identifiers without adding producer-owned
fields to the compatibility manifest schema.

Schema v1 keeps `contracts` optional. The gateway-browser-login candidate omits
that extension so the strict System Tests consumer at
`f4ff29a13d973c4067c0a3787355bc197dd8c40a` can decode it; the release name,
schema conditional, and evidence document still enforce the architecture and
exact component bindings.

After CI succeeds for a push to `main` that adds exactly one candidate, the
publication workflow revalidates the candidate/provenance binding and uploads a
commit-addressed Actions artifact preserving the `release/` paths. The bundle
includes the candidate, provenance, versioned schemas, validation resolution,
`publication.json`, and `SHA256SUMS`. Publication does not promote a candidate;
the separate passing System Tests evidence remains mandatory.

Infrastructure owns the environment-specific schema-v3 run manifest. It must
reuse these component pins and add the deployed origins, account, exact client
artifact URL/hash, and browser-login discovery assertions before invoking the
live acceptance suite.

Clean cutover uses one released manifest. It does not export, import, dual-write,
CDC, or reconcile state from the retired stack.
