#!/usr/bin/env bash
set -euo pipefail

# Compare with the newest stable tag for the same public Go module.
# A prerelease of a different module major is not a released baseline.
report="${PROTOBUF_REPORT:-protobuf-contract-report.txt}"
exec > >(tee "$report") 2>&1
module_path="$(sed -n 's/^module //p' clientapi/go.mod | tr -d '\r')"
baseline=
while IFS= read -r tag; do
  [[ "$tag" =~ ^clientapi/v[0-9]+\.[0-9]+\.[0-9]+$ ]] || continue
  released_module="$(git show "$tag:clientapi/go.mod" | sed -n 's/^module //p' | tr -d '\r')"
  [[ "$released_module" == "$module_path" ]] || continue
  baseline="$tag"
  break
done < <(git tag --list 'clientapi/v*' --sort=-version:refname)

if [[ -z "$baseline" ]]; then
  echo 'SKIPPED: no stable released tag exists for this Go module.'
  exit 0
fi
printf 'Baseline: %s (%s)\n' "$baseline" "$(git rev-parse "$baseline^{commit}")"
if [[ -z "$(git ls-tree -r --name-only "$baseline" -- clientapi/proto)" ]]; then
  echo 'SKIPPED: the latest stable release has no Protobuf contract.'
  exit 0
fi
scratch="$(mktemp -d)"
trap 'rm -rf -- "$scratch"' EXIT
git archive "$baseline" clientapi/proto | tar -x -C "$scratch"
buf breaking clientapi --against "$scratch/clientapi/proto"
echo 'PASS: no breaking Protobuf changes against the released contract.'
