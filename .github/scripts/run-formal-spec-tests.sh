#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/../.." && pwd)"
fsl_root="$repo_root/formal-spec/fsl"

export FSL_TOOL_DIR="${FSL_TOOL_DIR:-$fsl_root/.tools}"
FSLC="$("$fsl_root/scripts/install.sh")"
export FSLC

bash "$fsl_root/scripts/verify.sh"

generated_dir="${FSL_GENERATED_DIR:-$fsl_root/generated}"
vectors="$generated_dir/seat_session.conformance.json"
report="$generated_dir/seat_session.html"
bash "$fsl_root/scripts/conformance.sh" "$vectors" >/dev/null

(
	cd "$repo_root/system"
	FSL_SEAT_CONFORMANCE_FILE="$vectors" go test -count=1 -tags=formalspec -run '^TestSeatFSLConformance$' ./core/repository
)

bash "$fsl_root/scripts/report.sh" "$report" >/dev/null
