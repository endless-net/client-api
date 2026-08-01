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

Clean cutover uses one released manifest. It does not export, import, dual-write,
CDC, or reconcile state from the retired stack.
