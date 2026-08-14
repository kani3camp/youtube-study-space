#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
fsl_root="$(cd -- "$script_dir/.." && pwd)"
fslc="${FSLC:-$("$script_dir/install.sh")}"
depth="${FSL_VERIFY_DEPTH:-8}"
specs=("$fsl_root"/specs/*.fsl)

if [[ ! -e "${specs[0]}" ]]; then
	echo "No FSL specs found under $fsl_root/specs" >&2
	exit 2
fi

for spec in "${specs[@]}"; do
	echo "Checking $spec" >&2
	"$fslc" check "$spec"
	echo "Verifying $spec (depth=$depth, vacuity=error)" >&2
	"$fslc" verify "$spec" --depth "$depth" --vacuity error
done
