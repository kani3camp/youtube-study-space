#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
fsl_root="$(cd -- "$script_dir/.." && pwd)"
fslc="${FSLC:-$("$script_dir/install.sh")}"
depth="${FSL_CONFORMANCE_DEPTH:-4}"
output="${1:-$fsl_root/generated/seat_session.conformance.json}"

mkdir -p "$(dirname -- "$output")"
tmp_output="${output}.tmp.$$"
trap 'rm -f "$tmp_output"' EXIT

"$fslc" conformance "$fsl_root/specs/seat_session.fsl" --depth "$depth" >"$tmp_output"
mv "$tmp_output" "$output"
trap - EXIT

printf '%s\n' "$output"
