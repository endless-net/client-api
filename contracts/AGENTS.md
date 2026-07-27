# Contract agent guidance

- This directory is the source of truth for browser-facing producer contracts.
  Do not expose node-control, map-streaming, relay, signing, or service APIs.
- Preserve `x-endlessnet-contract-version` semantic compatibility. Removing or
  reinterpreting fields, or adding required request fields, needs an approved
  major-version migration.
- Update owning Go DTOs and the OpenAPI contract together. Never patch a
  generated consumer artifact in another repository from here.
- Run `go test ./contracts` and the PR verification tier after a contract change.
