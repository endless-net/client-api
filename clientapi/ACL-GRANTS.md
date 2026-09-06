# Destination-scoped ACL grants

Status: public model and validation foundation; Coordinator production and Client
enforcement are not implemented by this contract change.

`Peer.acl_grants` contains correlated `ACLGrant` entries: each entry binds
`destination_cidrs` to `allowed_ports`. Grants are unioned; never union all CIDRs
and all ports separately. Empty ports permit all protocols only for that grant's
destinations. Traffic not covered by a grant is denied.

A nonempty grant list requires `acl_restricted=true` and empty peer-wide
`allowed_ports`. Each destination must be a canonical prefix contained in one
of the peer's `allowed_ips` routes. IPv4-mapped IPv6 destinations are rejected.
Ports use TCP/UDP 1..65535 or ICMP zero. Limits are 1000 grants, 1000 prefixes per
grant and 65535 ports per grant. The complete map validator invokes this check.

The canonical signing payload serializes the full Peer, including these grants.
Tests mutate destination, protocol, port and grant presence independently and
verify that each mutation changes the signed payload. This does not replace
end-to-end signature verification and enforcement acceptance in consumers.

The map-stream regression test additionally signs a real delta, verifies its
reconstructed result, rejects a changed grant port with the original signature,
and confirms that accepted grants no longer alias event buffers. Snapshot and
delta copies detach nested peer grants, ports, route lists, node slices and
timestamps before consumers retain them. Runtime firewall acceptance is still
separate from these signature and memory-isolation checks.

Do not emit grants in production until the pinned Coordinator, Signing and Client
artifacts all support the exact model and traffic tests prove the intended
decisions. No production cutover or signature/schema generation increase occurs
here. Old peer-wide fields are not an alternative representation for a correlated
grant set: mixed representations are rejected.

Required consumers: Coordinator must commit grants with map revisions and its
outbox; Client must apply their union while preserving CIDR/port correlation;
System Tests must prove both positive cases and prohibited cross-grant traffic,
including overlap, IPv6, deletion, tampering and existing connections.
