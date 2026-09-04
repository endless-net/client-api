# EndlessNet frontend contracts

This directory is the source of truth for browser-facing producer interfaces.
Consumer repositories pin a released contract version and must pass their
consumer checks before updating it.

Contracts:

- `frontend-api.openapi.json` — browser-facing management HTTP API. It
  deliberately excludes node control, map streaming, relay and
  service-to-service endpoints.
- `frontend-runtime-config.schema.json` — runtime configuration consumed by the
  public site in `endless-net/front`. Admin bootstrap is not a consumer.
- `browser-auth.md` — normative redirect, cookie and CORS behavior that cannot
  be expressed completely by OpenAPI.

Compatibility follows semantic versioning through the
`x-endlessnet-contract-version` field. Removing an operation or response
field, changing field meaning, or adding a required request field requires a
new major version. Additive optional fields and operations require a minor
version.

The backend owns the producer contract. Consumer repositories use a versioned
copy or release artifact; they must not import backend source code.
