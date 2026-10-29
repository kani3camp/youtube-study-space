#!/usr/bin/env bash
# 認証情報は使わず、S3バックエンドにもGoogle APIにも認証しない。
set -euo pipefail

cd "$(dirname "$0")/../../.."
python3 -m unittest discover -s infra/mypage-platform/tests -p 'test_*.py' -v
python3 -m unittest discover -s infra/mypage-platform/scripts -p 'test_*_release.py' -v
terraform fmt -check -diff -recursive infra/mypage-platform

cmp infra/gcp/environments/dev/.terraform.lock.hcl infra/mypage-platform/environments/dev/.terraform.lock.hcl
cmp infra/mypage-platform/environments/dev/.terraform.lock.hcl infra/mypage-platform/environments/prod/.terraform.lock.hcl

for environment in dev prod; do
  root="infra/mypage-platform/environments/$environment"
  terraform -chdir="$root" init -backend=false -input=false -lockfile=readonly -no-color
  terraform -chdir="$root" validate -no-color
  terraform -chdir="$root" test -no-color
done

python3 infra/mypage-platform/scripts/check-foundation-native.py
python3 infra/mypage-platform/scripts/check-read-metrics-native.py
echo "MyPage platform: credentialless validation and mocks passed."
