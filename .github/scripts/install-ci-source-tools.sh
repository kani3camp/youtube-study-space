#!/usr/bin/env bash
# Share the canonical pinned parser/actionlint tooling without running CodeQL.
set -euo pipefail
python -m pip install --disable-pip-version-check --only-binary=:all: -r .github/codeql/requirements.txt
bash .github/codeql/install-actionlint.sh "$1"
