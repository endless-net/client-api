# Contract agent guidance

- This directory is the source of truth for browser-facing producer contracts.
  Do not expose node-control, map-streaming, relay, signing, or service APIs.
- Do not preserve legacy behavior, obsolete interfaces, or backward
  compatibility. Evolve `x-endlessnet-contract-version` and contract fields
  for the current design, including breaking changes when required.
- Update owning Go DTOs and the OpenAPI contract together. Never patch a
  generated consumer artifact in another repository from here.
- Run `go test ./contracts` and the PR verification tier after a contract change.
