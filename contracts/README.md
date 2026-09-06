# EndlessNet frontend contracts

This directory owns browser authentication semantics and the public-site runtime
configuration schema. The browser-facing Management API is Protobuf/Connect and
is generated from `endless-net/management`; it is not duplicated here.

Contracts:

- `frontend-runtime-config.schema.json` — runtime configuration consumed by the
  public site in `endless-net/front`. Admin bootstrap is not a consumer.
- `browser-auth.md` — normative redirect, cookie and CORS behavior that cannot
  be expressed completely by OpenAPI.

Management owns and publishes its Buf-validated generated SDKs. Consumers pin
an exact published module/package revision and must not recreate REST routes or
handwritten DTOs for that service.
