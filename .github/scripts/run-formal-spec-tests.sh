#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/../.." && pwd)"
fsl_root="$repo_root/formal-spec/fsl"

export FSL_TOOL_DIR="${FSL_TOOL_DIR:-$fsl_root/.tools}"
FSLC="$($fsl_root/scripts/install.sh)"
export FSLC

bash "$fsl_root/scripts/verify.sh"
