# Runtime ownership

The contract hub has no runtime. Runtime ownership is:

| Repository | Runtime |
| --- | --- |
| `endlessnet-gateway` | public edge and stream proxy |
| `endlessnet-identity` | OIDC users and sessions |
| `endlessnet-coordinator` | networks, nodes, enrollment, YDB map projection and streams |
| `endlessnet-billing` | accounts, entitlements, reservations and payments |
| `endlessnet-management` | admin API, policy/DNS source state and event inbox |
| `endlessnet-signing` | map, node and relay signing via OpenBao Transit |
| `endlessnet-mcp` | public API to MCP/stdio adapter |
| `endlessnet-relay` | Relay and Relay Coordinator |
| `endlessnet-stun` | stateless STUN Binding |
| `endlessnet-client` | CLI/agent and local network state |

Services may import producer API modules and `endlessnet-servicekit`; importing
another service's implementation is forbidden. Decisions required for the
current response are synchronous. Notifications use transactional outbox and an
idempotent Management inbox.

Coordinator alone uses YDB. Identity, Billing, and Management own independent
PostgreSQL databases. Production Signing state is owned by OpenBao. No service
reads or writes another service's database.
