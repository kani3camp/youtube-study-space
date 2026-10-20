#!/usr/bin/env bash
# Credentialless only: neither S3 backend nor Google API authentication.
set -euo pipefail

cd "$(dirname "$0")/../../.."
terraform fmt -check -diff -recursive infra/mypage-platform

cmp infra/gcp/environments/dev/.terraform.lock.hcl infra/mypage-platform/environments/dev/.terraform.lock.hcl
cmp infra/mypage-platform/environments/dev/.terraform.lock.hcl infra/mypage-platform/environments/prod/.terraform.lock.hcl

for environment in dev prod; do
  root="infra/mypage-platform/environments/$environment"
  terraform -chdir="$root" init -backend=false -input=false -lockfile=readonly -no-color
  terraform -chdir="$root" validate -no-color
  terraform -chdir="$root" test -no-color
done

echo "MyPage platform: credentialless validation and mocks passed."
