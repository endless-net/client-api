# D-025 release contract fixtures v1

These are exact cross-repository handoff fixtures, not deployable releases.

- `candidate.json` is the byte-addressed validation input.
- `candidate-provenance.json` covers its exact component and module set.
- `system-test-evidence.json` records a passing suite for that candidate digest.
- `promotion-request.json` pins all promotion inputs.
- `released-envelope.json` is the deterministic promotion output.
- `validation-resolution.json` is accepted only by validation environments.
- `production-resolution.json` is the normalized Infrastructure input derived
  only from the released envelope.

Infrastructure and System Tests should fetch this directory and `schemas/v1/`
from the same commit or published contract bundle and test their own decoders
against it. Do not copy a schema into a consumer repository: a copied schema can
drift independently and is not this versioned contract.
