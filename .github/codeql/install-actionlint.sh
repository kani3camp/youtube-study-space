#!/usr/bin/env bash
# CI-only public tool download, with the reviewed checksum from tooling.json.
set -euo pipefail
tool_dir=$1
mkdir -p "$tool_dir"
read -r version checksum < <(python3 - <<'PY'
import json
from pathlib import Path
tool = json.loads(Path('.github/codeql/tooling.json').read_text())['actionlint']
print(tool['version'], tool['linux_amd64_sha256'])
PY
)
archive="actionlint_${version}_linux_amd64.tar.gz"
curl --fail --location --silent --show-error \
  "https://github.com/rhysd/actionlint/releases/download/v${version}/${archive}" \
  -o "$tool_dir/$archive"
printf '%s  %s\n' "$checksum" "$tool_dir/$archive" | sha256sum --check --status
tar -xzf "$tool_dir/$archive" -C "$tool_dir" actionlint
"$tool_dir/actionlint" -version
