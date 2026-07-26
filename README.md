# EndlessNet Client API

This public repository contains only the producer-owned Go contract consumed by
EndlessNet clients:

- module: `github.com/unng-lab/endlessnet/clientapi`;
- source: [`clientapi/`](clientapi/);
- release tags: `clientapi/vMAJOR.MINOR.PATCH`.

The private EndlessNet control plane is maintained separately and is not part
of this repository. Backend-internal packages, deployment configuration, and
production inventory are intentionally excluded.

See [`clientapi/README.md`](clientapi/README.md) for the contract boundary and
versioning rules.
