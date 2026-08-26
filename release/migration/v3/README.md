# D-025 component release handoff

This directory records the current compatibility handoff from the superseded
full-server-set model. It is not a candidate publication, released record,
deployment request, inventory, or source of production credentials.

Each candidate and released record concerns exactly one deployable producer
component. Its immutable digest binds the component name, exact commit,
GitHub Release archive digest, module/contract pins, provenance, and only the
consumer/provider gates affected by that component. An empty gate list means
the producer declared no cross-service edge; it does not waive any required
end-to-end test identified by the owning component.

Releases owns candidate and released records. A released record contains the
digest of the exact tested candidate; changing an artifact digest requires a
new candidate and fresh evidence. The sample documents are schema and validator
fixtures only and must never be treated as production inputs.

After a release, Releases signals Infrastructure with the component, released
record digest, manifest commit and event ID. Infrastructure owns production
desired state and the idempotency tuple `(environment, component, released
record digest, config generation)`; it neither accepts host/command/credential
input from the producer nor deploys a candidate.

`client-api` continues to own executable public client and browser contracts.
It does not own server release-control records, production inventory, secret
references, host access, or deployment apply.
