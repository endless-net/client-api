# Contract agent guidance

- This directory owns browser-auth semantics and the public-site runtime schema.
  Management owns the browser API as a Buf-validated Protobuf/Connect contract;
  do not create or restore an OpenAPI/REST duplicate here.
- Do not preserve legacy behavior, obsolete interfaces, or backward
  compatibility. Evolve `x-endlessnet-contract-version` and contract fields
  for the current design, including breaking changes when required.
- Update API messages in the owning producer repository and consume its
  generated SDK. Never patch a generated consumer artifact from here.
- Run `go test ./contracts` and the PR verification tier after a contract change.

## Version increases

- Never increase any version or generation number, including schema, configuration,
  API, protocol, contract, manifest, migration, artifact, or rollout versions,
  without the user's direct explicit permission for that exact increase.
- A request to implement, refactor, fix, remove compatibility, or make a breaking
  change does not authorize a version increase. Without explicit permission, keep
  the current version number.
