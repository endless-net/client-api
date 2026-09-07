# Unified Client API v1 cutover

Status on 2026-09-07: module `v1.12.0` published from
[`307d5c4`](https://github.com/endless-net/client-api/commit/307d5c4c48ab79063a74564d182c254fe091278a).
The client pins the published module in
[`ea736d7`](https://github.com/endless-net/client/commit/ea736d7).
Client `go vet`, lint and `go test -short ./...` passed with `GOWORK=off`, and
`git diff --check` passed. Server adapters and component checks are now present
at the revisions below. Same-artifact release acceptance and production
activation are not established.

The authorized hard cutover keeps one Go module,
`github.com/endless-net/client-api/clientapi`, with HTTP/recovery types in `v1`.
The nested v2 module and its aliases are removed. Existing registration/error
`schema_version` 2 and v1 proof domain `endlessnet-register-identity-v3` are retained;
only the separately approved Go module minor version is increased. Registration
replaces `idempotency_key` with
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

## Client implementation and required follow-up

- [client, main](https://github.com/endless-net/client/tree/main): completed.
  The client consumes only this module, removes administrative route commands
  and uses Client API logout. The approved `v1.12.0` pin was validated without
  a local module replacement or workspace; temporary verification files were
  removed. These component checks do not establish cross-service acceptance.
- [coordinator, main](https://github.com/endless-net/coordinator/tree/main):
  adapters and strict registration/enrollment bindings implemented in
  [`9813940`](https://github.com/endless-net/coordinator/commit/9813940c86c27fffd990b1d0a180a669b3e70938).
  The obsolete public Connector/Flow services were removed from coordinatorapi;
  its published minor is `v1.22.0`. Root vet, lint and short tests passed.
  The live YDB sharing run found an obsolete enrollment fixture; its correction
  is [`bbae79c`](https://github.com/endless-net/coordinator/commit/bbae79c).
  [Run 34161022236](https://github.com/endless-net/coordinator/actions/runs/34161022236)
  must finish successfully before claiming that live check passed.
- [management, main](https://github.com/endless-net/management/tree/main):
  all eight UserService projections implemented in
  [`b116710`](https://github.com/endless-net/management/commit/b116710c3544fedd5ef6b7297d51c91d6aeb97ff).
  Component tests cover producer actor/account/cursor forwarding, denied access,
  public-plan filtering and the exact Gateway workload boundary. Vet, lint and
  short tests passed; live producer authorization remains an acceptance gate.
- [identity, main](https://github.com/endless-net/identity/tree/main) and
  [gateway, main](https://github.com/endless-net/gateway/tree/main): expose
  `/auth/logout` and RPC routing implemented in Gateway
  [`aa0cff3`](https://github.com/endless-net/gateway/commit/aa0cff3e1809eefc2bdfb0ed4027dd60bb6eec2f).
  Identity acknowledgement and typed logout errors implemented in
  [`98679d3`](https://github.com/endless-net/identity/commit/98679d32b93ad55161988a812fddbac0f3a1f6de).
  Both passed vet, lint and short tests, including host/workload isolation.
  These checks do not prove persistent revocation through the deployed Gateway.
- [system-tests, main](https://github.com/endless-net/system-tests/tree/main):
  verify affected edges against the same pinned artifacts: denied operations,
  direct/browser enrollment, recovery, pagination and immutable flow retries.
  [`282a132`](https://github.com/endless-net/system-tests/commit/282a132)
  updates recovery manifests to the unified module and rejects a second Client
  API module; local tests and vet passed. This is verifier coverage, not a live
  acceptance run. Existing D-025 HTTP checks do not exercise the new UserService,
  application discovery or flow-window RPCs. Those scenario additions and an
  immutable complete artifact set remain required; Windows recovery additionally
  requires the pinned Client UI MSI, schema artifact and signed recovery driver.
- [releases, main](https://github.com/endless-net/releases/tree/main): record
  affected-edge evidence after module publication and consumer implementation.
  Deployment needs separate approval in
  [infrastructure, main](https://github.com/endless-net/infrastructure/tree/main).

Historical `release/` records describe their original v2 artifacts and remain
unchanged. They are not evidence for this cutover. Component tests do not prove
server support, release acceptance or deployment.
