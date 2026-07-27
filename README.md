# EndlessNet contracts and integration

This public repository is the compatibility and release hub for independently
released EndlessNet components. It intentionally contains no runtime binary.

- `clientapi/` — `github.com/unng-lab/endlessnet/clientapi/v2`;
- `contracts/` — browser OpenAPI and runtime configuration contracts;
- `architecture/` — cross-service boundaries and interaction rules;
- `release/` — immutable compatibility-manifest schema and candidates;
- `systemtests/` — cross-repository contract and system gates.

Service implementations live in their producer-owned repositories. The release
manifest is the only supported way to select a mutually tested set of service
images, API modules, client, Relay, and STUN artifacts.
