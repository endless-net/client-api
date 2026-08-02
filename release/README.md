# Compatibility manifests

CI assembles a candidate from exact Git commits, module checksums, and immutable
OCI digests. A candidate becomes `released` only after the cross-repository
system suite passes. Released manifests never contain tags or mutable image
references.

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

After CI succeeds for a push to `main` that changes exactly one candidate, the
publication workflow revalidates the candidate/evidence binding and uploads a
commit-addressed Actions artifact preserving the `release/` paths for the
manifest, evidence, and both schemas, plus `publication.json` and `SHA256SUMS`.
It never promotes a candidate to `released`; that status still requires the
cross-repository acceptance gate.

Infrastructure owns the environment-specific schema-v3 run manifest. It must
reuse these component pins and add the deployed origins, account, exact client
artifact URL/hash, and browser-login discovery assertions before invoking the
live acceptance suite.

Clean cutover uses one released manifest. It does not export, import, dual-write,
CDC, or reconcile state from the retired stack.
