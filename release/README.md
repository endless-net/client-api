# Compatibility manifests

CI assembles a candidate from exact Git commits, module checksums, and immutable
OCI digests. A candidate becomes `released` only after the cross-repository
system suite passes. Released manifests never contain tags or mutable image
references.

Schema version 1 uses canonical `endless-net/*` repository coordinates. Legacy
pre-cutover repository coordinates are rejected after the clean cutover.

Clean cutover uses one released manifest. It does not export, import, dual-write,
CDC, or reconcile state from the retired stack.
