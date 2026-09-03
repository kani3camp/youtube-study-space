#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
fsl_root="$(cd -- "$script_dir/.." && pwd)"
fslc="${FSLC:-$("$script_dir/install.sh")}"
depth="${FSL_REPORT_DEPTH:-8}"
spec_name="${1:-seat_session}"
spec="$fsl_root/specs/$spec_name.fsl"
output="${2:-$fsl_root/generated/reports/$spec_name.html}"

if [[ ! -f "$spec" ]]; then
	echo "FSL spec not found: $spec" >&2
	exit 2
fi

mkdir -p "$(dirname -- "$output")"
tmp_output="${output}.tmp.$$"
trap 'rm -f "$tmp_output"' EXIT

"$fslc" html "$spec" --depth "$depth" -o "$tmp_output"
mv "$tmp_output" "$output"
trap - EXIT

printf '%s\n' "$output"
