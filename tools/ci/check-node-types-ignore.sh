#!/usr/bin/env bash
set -euo pipefail

node_version="$(tr -d '[:space:]' < .node-version)"
nvm_version="$(tr -d '[:space:]' < .nvmrc)"

if [[ "$node_version" != "$nvm_version" ]]; then
  echo "::error::.node-version and .nvmrc must match."
  exit 1
fi

if [[ ! "$node_version" =~ ^([0-9]+)\. ]]; then
  echo "::error::Unsupported Node version format: $node_version"
  exit 1
fi

node_major="${BASH_REMATCH[1]}"
expected_ignore=">=$((node_major + 1)).0.0"

python3 - "$expected_ignore" <<'PY'
from pathlib import Path
import re
import sys

expected = sys.argv[1]
text = Path(".github/dependabot.yml").read_text(encoding="utf-8")
values = re.findall(
    r'dependency-name:\s*"@types/node"\s*\n\s*versions:\s*\n\s*-\s*"([^"]+)"',
    text,
)
if not values:
    raise SystemExit("::error::No @types/node Dependabot ignore rule found.")
wrong = [value for value in values if value != expected]
if wrong:
    raise SystemExit(
        "::error::@types/node ignore rules must start at "
        + expected
        + "; found "
        + ", ".join(wrong)
    )
print(f"@types/node ignore policy matches Node major: {expected}")
PY
