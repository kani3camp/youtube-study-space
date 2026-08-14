#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
fsl_root="$(cd -- "$script_dir/.." && pwd)"
fslc="${FSLC:-$("$script_dir/install.sh")}"
depth="${FSL_REPORT_DEPTH:-8}"
output="${1:-$fsl_root/generated/seat_session.html}"

mkdir -p "$(dirname -- "$output")"
tmp_output="${output}.tmp.$$"
trap 'rm -f "$tmp_output"' EXIT

"$fslc" html "$fsl_root/specs/seat_session.fsl" --depth "$depth" -o "$tmp_output"
mv "$tmp_output" "$output"
trap - EXIT

printf '%s\n' "$output"
