# Unified Client API v1 cutover

Status: source implementation under component verification. Module publication,
server support, release acceptance and production activation are not established.

The authorized hard cutover keeps one Go module,
`github.com/endless-net/client-api/clientapi`, with HTTP/recovery types in `v1`.
The nested v2 module and its aliases are removed. Existing registration/error
`schema_version` 2 and v1 proof domain `endlessnet-register-identity-v3` are retained;
no version number is increased. Registration replaces `idempotency_key` with
required `idempotency_id` and `schema_version`. Renewal requires its prior
registration binding. The SDK validates identity proofs and response binding.
Browser enrollment must persist and echo the registration operation ID; its
approval/poll authorization is separate from direct enrollment authorization.

## Client RPC ownership

[client.proto](proto/client/v1/client.proto) owns the client projections, with Go
messages in `v1/clientrpc` and Connect bindings in `v1/clientrpc/clientrpcconnect`.
These are independent public projections, not aliases or imports of server DTOs.
Procedure prefixes are rooted at the Client API origin:

- `/endlessnet.client.v1.UserService/`: caller-scoped accounts, networks, route
  reads, plans, subscription, usage, checkout and invoices. Implementations must
  authorize each account/network and billing operation; IDs never grant access.
- `/endlessnet.client.v1.ConnectorService/`: discovery, bound to the authenticated
  node, current connector policy and a validated, bounded discovery lease.
- `/endlessnet.client.v1.FlowLogService/`: policy and flow windows, bound to the
  authenticated node, current consent generation and collection interval.
  Window IDs are immutable across retries; acknowledge only durable acceptance.

There are no route approval, policy mutation, account administration or billing
administration RPCs. Generate from repository root with installed protoc plugins:

```sh
protoc -I clientapi/proto --go_out=clientapi --go_opt=module=github.com/endless-net/client-api/clientapi --connect-go_out=clientapi --connect-go_opt=module=github.com/endless-net/client-api/clientapi clientapi/proto/client/v1/client.proto
```

## Required follow-up

- [client, main](https://github.com/endless-net/client/tree/main): consume only
  this module, remove administrative route commands and use Client API logout.
  A separately approved module release and new pin are required before committing
  the consumer cutover. A temporary Go workspace checks source, not the old pin.
- [coordinator, main](https://github.com/endless-net/coordinator/tree/main):
  implement discovery/flow handlers, node authorization, consent and durable
  acknowledgement; consume strict registration proofs and echo operation IDs.
- [management, main](https://github.com/endless-net/management/tree/main):
  implement UserService projections with account/network/billing permissions.
- [identity, main](https://github.com/endless-net/identity/tree/main) and
  [gateway, main](https://github.com/endless-net/gateway/tree/main): expose
  `/auth/logout` at the Client API origin, wire the RPC procedures and strict
  public recovery errors, and verify host/auth isolation.
- [system-tests, main](https://github.com/endless-net/system-tests/tree/main):
  verify affected edges against the same pinned artifacts: denied operations,
  direct/browser enrollment, recovery, pagination and immutable flow retries.
- [releases, main](https://github.com/endless-net/releases/tree/main): record
  affected-edge evidence after module publication and consumer implementation.
  Deployment needs separate approval in
  [infrastructure, main](https://github.com/endless-net/infrastructure/tree/main).

Historical `release/` records describe their original v2 artifacts and remain
unchanged. They are not evidence for this cutover. Component tests do not prove
server support, release acceptance or deployment.
