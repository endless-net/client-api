# Signed machine sharing

`Network.SharePeerGrants` carries effective permissions, never pending invitations
or user/group selectors. Each `SharePeerGrant` binds both endpoint network IDs,
node IDs, WireGuard public keys and exact host prefixes. `Peer.NetworkID` is
mandatory for a peer referenced by a grant; a peer from another network without
a matching grant is rejected. An ordinary local-network peer may omit it.

The source is the shared machine. Only the recipient may initiate new flows;
`Rights` permits explicit TCP/UDP destination port ranges or ICMP. An empty list
denies access. Both clients must enforce the direction, including stateful
replies on the source; symmetric peer reachability is insufficient. A grant does
not authorize account membership, neighboring machines, routes, or discovery.

Each lease is at most two minutes. Producers renew only after checking current
consent, active membership, node identities and addresses. Key/address changes
require a new grant revision and invalidate bindings to previous revisions.
Revocation removes grants; lease expiry bounds access when refresh is unavailable.

`ValidateNetworkMapSnapshot` checks structural binding, explicit host-only
addresses, overlapping peers/local-network ranges, direction, rights and lease
length. `ValidateSharePeerGrantsAt` additionally checks the current instant;
signature verification calls it even if the enclosing map signature remains
valid longer. All grant fields and peer network IDs are covered by the map
signature. Stream reconstruction clones the mutable grant fields.

Consumers must schedule removal at the earliest grant expiry independently of
stream activity, including established flows. Heartbeats and Relay session
traffic cannot extend a grant. Relay must authorize both network identities via
its upstream contract and permit reverse traffic only through an existing
recipient-created binding; a signed map alone is not Relay authorization.

This producer contract has unit, signature-tampering, lease-boundary and map
binding tests. Publishing it does not prove Coordinator reconciliation, Client
packet filtering, Relay enforcement, or production acceptance. Those outcomes
belong to the consuming repositories.
