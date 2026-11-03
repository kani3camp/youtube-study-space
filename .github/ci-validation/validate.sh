#!/usr/bin/env bash
set -euo pipefail
mkdir -p "$HOME" "$RUNNER_TEMP" /work/repository
cp -R /source/. /work/repository/
cd /work/repository
[[ "$(git rev-parse HEAD)" == "$CI_SOURCE_SHA" ]] || { echo "Source SHA mismatch" >&2; exit 1; }
case "$1" in
  terraform-static)
    # Fixed Terraform checks of the pinned commit. This kind never executes target
    # scripts or tests, and runs in its own container before any target test.
    # Providers come only from the public HashiCorp namespace (no implied local
    # mirror such as a committed terraform.d/plugins), and the only step that
    # starts a provider (init/validate) reads the read-only /source and runs last,
    # so neither a target test nor a provider can rewrite what is checked.
    printf 'disable_checkpoint = true\nprovider_installation {\n  direct {\n    include = ["registry.terraform.io/hashicorp/*"]\n  }\n}\n' > /work/terraformrc
    export TF_CLI_CONFIG_FILE=/work/terraformrc
    # Check formatting
    terraform -chdir=/source fmt -check -recursive infra/gcp
    # Verify multi-platform provider locks are complete (downloads and hashes
    # providers without executing them)
    for env in dev prod; do
      TF_DATA_DIR="/work/terraform-lock-$env" terraform -chdir="infra/gcp/environments/$env" get
      TF_DATA_DIR="/work/terraform-lock-$env" terraform -chdir="infra/gcp/environments/$env" providers lock \
        -platform=linux_amd64 \
        -platform=darwin_arm64
    done
    git diff --exit-code -- \
      infra/gcp/environments/dev/.terraform.lock.hcl \
      infra/gcp/environments/prod/.terraform.lock.hcl
    # Verify provider locks match
    cmp /source/infra/gcp/environments/dev/.terraform.lock.hcl /source/infra/gcp/environments/prod/.terraform.lock.hcl
    # Initialize and validate roots on the read-only source
    for env in dev prod; do
      TF_DATA_DIR="/work/terraform-data-$env" terraform -chdir="/source/infra/gcp/environments/$env" init -backend=false -input=false -lockfile=readonly
      TF_DATA_DIR="/work/terraform-data-$env" terraform -chdir="/source/infra/gcp/environments/$env" validate
    done
    # The same fixed fmt/init/validate for the MyPage platform and OAuth TTL roots
    for layer in mypage-platform mypage-oauth-ttl; do
      terraform -chdir=/source fmt -check -recursive "infra/$layer"
      for env in dev prod; do
        TF_DATA_DIR="/work/terraform-data-$layer-$env" terraform -chdir="/source/infra/$layer/environments/$env" init -backend=false -input=false -lockfile=readonly
        TF_DATA_DIR="/work/terraform-data-$layer-$env" terraform -chdir="/source/infra/$layer/environments/$env" validate
      done
    done
    ;;
  terraform-tests)
    # The native suites use the provider mirror of an initialized DEV/PROD root.
    for env in dev prod; do
      terraform -chdir="infra/gcp/environments/$env" init -backend=false -input=false -lockfile=readonly
    done
    # Test authenticated Terraform CI guardrails
    python3 .github/scripts/test_terraform_native_runner.py
    python3 -B infra/gcp/tests/test_terraform_runtime.py
    python3 .github/scripts/test_terraform_plan_summary.py
    python3 .github/scripts/test_terraform_authenticated_workflow.py
    python3 .github/scripts/test_terraform_identity_smoke.py
    python3 .github/scripts/test_terraform_email_adoption_gate.py
    python3 .github/scripts/test_terraform_protected_plan.py
    python3 .github/scripts/test_terraform_plan_diagnostic.py
    python3 .github/scripts/test_terraform_baseline_drift_diagnostic.py
    python3 infra/gcp/tests/test_runtime_plan_contract.py
    python3 infra/gcp/tests/test_runtime_ownership.py
    python3 infra/gcp/tests/test_production_readiness.py
    python3 infra/gcp/tests/test_production_backend_packet.py
    python3 infra/gcp/tests/test_production_execution_contract.py
    python3 -B infra/gcp/tests/test_terraform_normal_ops.py
    python3 -B infra/gcp/tests/test_terraform_source.py
    python3 -B infra/gcp/tests/test_enable_normal_integration.py
    python3 -B infra/gcp/tests/test_production_normal_ops.py
    python3 -B infra/gcp/tests/test_bootstrap_production_normal_ops.py
    python3 .github/scripts/test_terraform_quota_create_gate.py
    python3 .github/scripts/test_terraform_quota_refresh_gate.py
    python3 .github/scripts/test_terraform_export_topic_gate.py
    python3 .github/scripts/test_terraform_export_scheduler_gate.py
    python3 .github/scripts/test_terraform_export_function_gate.py
    python3 .github/scripts/test_user_activity_history_preparation.py
    # Test email/password Auth ownership and Terraform contract
    python3 -B infra/gcp/tests/test_monitor_auth.py
    python3 -B infra/gcp/scripts/test_monitor_auth_terraform.py
    # Test Pool import fidelity with credentialless mocks
    python3 -B infra/gcp/tests/test_runtime_pool_import_fidelity.py
    # Test quota alert mock plans and import-only boundary
    python3 -B .github/scripts/run_terraform_native_tests.py --workers 2
    # Test Terraform create/update/delete/replace
    python3 -B infra/gcp/tests/test_terraform_normal_ops_native.py
    # Validate MyPage platform desired state
    bash infra/mypage-platform/scripts/check-local.sh
    # Validate MyPage OAuth TTL Terraform
    bash infra/mypage-oauth-ttl/scripts/check-local.sh
    ;;
  base-image)
    # Run tests
    node --test .github/scripts/base-image-update-report.test.mjs
    ;;
  node-policy)
    # Check Node types ignore policy
    bash tools/ci/check-node-types-ignore.sh
    ;;
  docs)
    # Test document link guardrails
    python3 .github/scripts/test_check_doc_references.py
    # Check repository-relative documentation links
    python3 .github/scripts/check-doc-references.py
    ;;
  contracts)
    # Test CI selection, required checks and authentication boundary
    python -m unittest discover -s .github/scripts -p 'test_ci_*.py' -v
    # Lint CI and operational workflow calls
    /opt/ci-tools/actionlint -shellcheck= -pyflakes= .github/workflows/ci.yml .github/workflows/ci-*.yml .github/workflows/gcp-*.yml .github/workflows/terraform*.yml .github/workflows/mypage-*.yml .github/workflows/deploy-mypage-*.yml
    ;;
  *) echo "Unknown validation kind" >&2; exit 2 ;;
esac
