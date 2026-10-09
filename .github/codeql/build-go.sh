#!/usr/bin/env bash
# Compile only: do not run applications, tests, generators, or cloud commands.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
manifests=()
while IFS= read -r -d '' manifest; do
  manifests+=("$manifest")
done < <(git ls-files -z -- go.mod '**/go.mod')
if ((${#manifests[@]} == 0)); then
  echo 'No tracked Go module found' >&2
  exit 1
fi
for manifest in "${manifests[@]}"; do
  module_dir=$(dirname "$manifest")
  (
    cd "$module_dir"
    # Force fresh compilation for extraction; do not reuse a Go build cache.
    GOWORK=off GOFLAGS=-mod=readonly go build -a ./...
  )
done
