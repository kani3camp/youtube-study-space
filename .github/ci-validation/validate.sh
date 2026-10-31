#!/usr/bin/env bash
set -euo pipefail
mkdir -p "$HOME" "$RUNNER_TEMP" /work/repository
cp -R /source/. /work/repository/
cd /work/repository
[[ "$(git rev-parse HEAD)" == "$CI_SOURCE_SHA" ]] || { echo "Source SHA mismatch" >&2; exit 1; }
case "$1" in
  terraform)
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
    # Check formatting
    terraform fmt -check -recursive infra/gcp
    # Initialize and validate roots
    set -euo pipefail
    for env in dev prod; do
      terraform -chdir="infra/gcp/environments/$env" init -backend=false -input=false -lockfile=readonly
      terraform -chdir="infra/gcp/environments/$env" validate
    done
    # Test Pool import fidelity with credentialless mocks
    python3 -B infra/gcp/tests/test_runtime_pool_import_fidelity.py
    # Test quota alert mock plans and import-only boundary
    python3 -B .github/scripts/run_terraform_native_tests.py --workers 2
    # Test Terraform create/update/delete/replace
    python3 -B infra/gcp/tests/test_terraform_normal_ops_native.py
    # Verify multi-platform provider locks are complete
    set -euo pipefail
    for env in dev prod; do
      terraform -chdir="infra/gcp/environments/$env" providers lock \
        -platform=linux_amd64 \
        -platform=darwin_arm64
    done

    git diff --exit-code -- \
      infra/gcp/environments/dev/.terraform.lock.hcl \
      infra/gcp/environments/prod/.terraform.lock.hcl
    # Verify provider locks match
    cmp infra/gcp/environments/dev/.terraform.lock.hcl infra/gcp/environments/prod/.terraform.lock.hcl
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
    /opt/ci-tools/actionlint -shellcheck= -pyflakes= .github/workflows/ci.yml .github/workflows/ci-*.yml .github/workflows/gcp-terraform-authenticated.yml
    ;;
  *) echo "Unknown validation kind" >&2; exit 2 ;;
esac
