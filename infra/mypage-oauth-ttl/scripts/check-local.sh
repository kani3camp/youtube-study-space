#!/usr/bin/env bash
set -euo pipefail

layer_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
repo_dir="$(cd -- "$layer_dir/../.." && pwd)"
check_tmp="$(mktemp -d)"
trap 'rm -rf -- "$check_tmp"' EXIT
unset TF_LOG TF_LOG_PATH
export TF_IN_AUTOMATION=true TF_INPUT=false

terraform fmt -check -recursive "$layer_dir"
python3 -m unittest discover -s "$layer_dir/scripts" -p 'test_*.py' -v
for environment in dev prod; do
  root_dir="$layer_dir/environments/$environment"
  export TF_DATA_DIR="$check_tmp/$environment-data"
  terraform -chdir="$root_dir" init -backend=false -input=false -lockfile=readonly -no-color
  terraform -chdir="$root_dir" validate -no-color
  # Mocks need provider schema but no credentials/API. Block accidental HTTP calls.
  HTTP_PROXY=http://127.0.0.1:9 HTTPS_PROXY=http://127.0.0.1:9 \
    ALL_PROXY=http://127.0.0.1:9 NO_PROXY= GCE_METADATA_HOST=127.0.0.1:9 \
    GOOGLE_APPLICATION_CREDENTIALS=/nonexistent-mypage-mock-credentials \
    terraform -chdir="$root_dir" test -filter=tests/ttl.tftest.hcl -no-color -json -verbose \
    > "$check_tmp/$environment-tests.jsonl"
  python3 "$layer_dir/scripts/check_mock_plans.py" "$check_tmp/$environment-tests.jsonl" \
    "$root_dir/target.example.json"
done
cd "$repo_dir"
bash .github/scripts/test-detect-ci-paths.sh
python3 .github/scripts/check-doc-references.py
python3 .github/scripts/check-mypage-contract.py
