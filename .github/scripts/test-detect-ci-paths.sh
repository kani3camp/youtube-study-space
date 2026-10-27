#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
detector="$script_dir/detect-ci-paths.sh"

assert_output_groups() {
	local expected_groups="$1"
	local output="$2"
	local context="$3"
	local group
	local expected

	for group in system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk gcp_terraform node_projects firestore_integration formal_spec all; do
		expected=false
		case " $expected_groups " in
			*" $group "*) expected=true ;;
		esac
		if ! grep -Fxq "$group=$expected" <<< "$output"; then
			echo "Expected $group=$expected for $context" >&2
			exit 1
		fi
	done
}

assert_exact_groups() {
	local expected_groups="$1"
	shift
	local output
	output="$($detector --paths "$@")"
	assert_output_groups "$expected_groups" "$output" "paths: $*"
}

assert_exact_groups "" README.md
assert_exact_groups formal_spec formal-spec/README.md
assert_exact_groups formal_spec formal-spec/fsl/specs/seat_session.fsl
assert_exact_groups formal_spec .github/scripts/run-formal-spec-tests.sh
assert_exact_groups firestore_integration firebase/firebase.json
assert_exact_groups firestore_integration firebase/firestore.rules
assert_exact_groups firestore_integration firebase/firestore.indexes.json
assert_exact_groups youtube_monitor youtube-monitor/src/app.ts
assert_exact_groups mypage mypage/src/main.tsx
assert_exact_groups mypage mypage/package.json
assert_exact_groups mypage docs/mypage/openapi.yaml
assert_exact_groups mypage infra/mypage-oauth-ttl/environments/dev/main.tf
assert_exact_groups mypage .github/scripts/check-mypage-contract.py
assert_exact_groups "youtube_monitor mypage" biome.json
assert_exact_groups "system firestore_integration" system/core/mypage/store.go
assert_exact_groups "system firestore_integration" system/cmd/mypage-server/main.go
assert_exact_groups "system firestore_integration" system/core/workspaceapp/app.go
assert_exact_groups "system firestore_integration" system/core/repository/firestore_controller.go
assert_exact_groups "system firestore_integration" system/core/workspaceapp/workspace_app_private_test.go
assert_exact_groups "system firestore_integration formal_spec" system/core/repository/seat.go
assert_exact_groups "system firestore_integration formal_spec" system/core/repository/models.go
assert_exact_groups "system firestore_integration formal_spec" system/core/repository/seat_formalspec_test.go
assert_exact_groups "system firestore_integration formal_spec" system/core/timeutil/time.go
assert_exact_groups "system firestore_integration aws_cdk" system/Dockerfile.lambda
assert_exact_groups "system firestore_integration aws_cdk" system/.dockerignore
assert_exact_groups aws_cdk aws-cdk/lib/aws-cdk-stack.ts
assert_exact_groups "mypage gcp_terraform" infra/mypage-platform/environments/dev/main.tf
assert_exact_groups "mypage gcp_terraform" infra/mypage-platform/modules/runtime/main.tf
assert_exact_groups gcp_terraform infra/gcp/README.md
assert_exact_groups gcp_terraform infra/gcp/environments/dev/main.tf
assert_exact_groups gcp_terraform infra/gcp/environments/prod/main.tf
assert_exact_groups gcp_terraform infra/gcp/tests/test_runtime_pool_import_fidelity.py
assert_exact_groups gcp_terraform .github/scripts/terraform_plan_summary.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_plan_summary.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_authenticated_workflow.py
assert_exact_groups gcp_terraform .github/scripts/run_terraform_native_tests.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_native_runner.py
assert_exact_groups gcp_terraform .github/scripts/terraform_identity_diagnostics.py
assert_exact_groups gcp_terraform .github/scripts/terraform_identity_smoke.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_identity_smoke.py
assert_exact_groups gcp_terraform .github/scripts/test_youtube_quota_alerts.py
assert_exact_groups gcp_terraform .github/scripts/test_backup_bucket.py
assert_exact_groups gcp_terraform .github/scripts/test_export_topic.py
assert_exact_groups gcp_terraform .github/scripts/test_export_scheduler.py
assert_exact_groups gcp_terraform .github/scripts/test_export_function.py
assert_exact_groups gcp_terraform .github/scripts/test_order_history.py
assert_exact_groups gcp_terraform .github/scripts/terraform_migration.py
assert_exact_groups gcp_terraform .github/scripts/terraform_migration_contract.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_migration.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_migration_contract.py
assert_exact_groups gcp_terraform .github/scripts/production_initializer.py
assert_exact_groups gcp_terraform .github/scripts/test_production_initializer.py
assert_exact_groups gcp_terraform .github/scripts/test_production_initializer_controller.py
assert_exact_groups gcp_terraform .github/scripts/test_production_initializer_s3.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_migration_native_fidelity.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_api_import_fidelity.py
assert_exact_groups gcp_terraform infra/gcp/modules/retained-order-history/tests/contract.tftest.hcl
assert_exact_groups gcp_terraform infra/gcp/environments/dev/export-function.tf
assert_exact_groups gcp_terraform infra/gcp/modules/firestore-export-function/main.tf
assert_exact_groups gcp_terraform infra/gcp/environments/dev/export-scheduler.tf
assert_exact_groups gcp_terraform infra/gcp/modules/firestore-export-scheduler/main.tf
assert_exact_groups gcp_terraform .github/scripts/terraform_export_topic_gate.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_export_topic_gate.py
assert_exact_groups gcp_terraform .github/scripts/test_notification_channels.py
assert_exact_groups gcp_terraform .github/scripts/terraform_quota_create_gate.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_quota_create_gate.py
assert_exact_groups gcp_terraform .github/scripts/terraform_protected_plan.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_protected_plan.py
assert_exact_groups gcp_terraform .github/scripts/terraform_plan_diagnostic.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_plan_diagnostic.py
assert_exact_groups gcp_terraform .github/scripts/terraform_baseline_drift_fields.py
assert_exact_groups gcp_terraform .github/scripts/terraform_baseline_drift_diagnostic.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_baseline_drift_diagnostic.py
assert_exact_groups gcp_terraform .github/scripts/terraform_email_adoption_gate.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_email_adoption_gate.py
assert_exact_groups docs_site docs-site/docs/intro.md
assert_exact_groups room_image_prompt tools/room-image-prompt/cmd/room-image-prompt/main.go
assert_exact_groups room_image_prompt .agents/skills/room-art-direction/references/direction-a-clean-vivid-digital.md
assert_exact_groups menu_image_generator tools/menu-image-generator/src/index.ts
assert_exact_groups video_maker_simulator tools/video-maker/1000-minutes-simulator/src/pages/index.tsx
assert_exact_groups figma_plugin tools/figma-plugin/room-layout-analyzer/code.ts
assert_exact_groups node_projects .node-version
assert_exact_groups node_projects .nvmrc
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk gcp_terraform node_projects firestore_integration formal_spec all" .github/workflows/ci.yml
for codeql_path in .github/codeql/.gitignore .github/codeql/README.md .github/codeql/analysis-config.yml .github/codeql/build-go.sh .github/codeql/install-actionlint.sh .github/codeql/lint-workflows.py .github/codeql/requirements.txt .github/codeql/test_source_contracts.py .github/codeql/tooling.json .github/codeql/validate-source.py .github/scripts/detect-ci-paths.sh .github/scripts/test-detect-ci-paths.sh .github/workflows/codeql-advanced.yml .github/workflows/codeql-analyzer.yml .github/workflows/codeql-source-contracts.yml; do
	assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk gcp_terraform node_projects firestore_integration formal_spec all" "$codeql_path"
done
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk gcp_terraform node_projects firestore_integration formal_spec all" .github/workflows/deploy-docs.yml
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk gcp_terraform node_projects firestore_integration formal_spec all" .github/scripts/detect-ci-paths.sh
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk gcp_terraform node_projects firestore_integration formal_spec all" .github/scripts/test-detect-ci-paths.sh
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk gcp_terraform node_projects firestore_integration formal_spec all" .github/scripts/run-firestore-integration-tests.sh
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk gcp_terraform node_projects firestore_integration formal_spec all" .github/scripts/base-image-update-report.mjs
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk gcp_terraform node_projects firestore_integration formal_spec all" .github/scripts/base-image-update-report.test.mjs
assert_exact_groups "system firestore_integration youtube_monitor" system/core/app.go youtube-monitor/src/app.ts

all_groups="system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk gcp_terraform node_projects firestore_integration formal_spec all"
manual_output="$(env -u CI_AUTHENTICATED_TERRAFORM_DISPATCH GITHUB_EVENT_NAME=workflow_dispatch "$detector")"
assert_output_groups "$all_groups" "$manual_output" "manual dispatch with absent Terraform marker"
for marker in "" false FALSE 1 yes malformed; do
	manual_output="$(GITHUB_EVENT_NAME=workflow_dispatch CI_AUTHENTICATED_TERRAFORM_DISPATCH="$marker" "$detector")"
	assert_output_groups "$all_groups" "$manual_output" "manual dispatch with non-true Terraform marker: $marker"
done

authenticated_output="$(GITHUB_EVENT_NAME=workflow_dispatch CI_AUTHENTICATED_TERRAFORM_DISPATCH=true "$detector")"
assert_output_groups gcp_terraform "$authenticated_output" "authenticated Terraform dispatch"

# A marker cannot narrow other events, incomplete PRs, or explicit path tests.
for event_name in pull_request push schedule unsupported; do
	fallback_output="$(GITHUB_EVENT_NAME="$event_name" CI_AUTHENTICATED_TERRAFORM_DISPATCH=true BASE_SHA= HEAD_SHA= "$detector")"
	assert_output_groups "$all_groups" "$fallback_output" "non-dispatch event: $event_name"
done
unchanged_pr_output="$(GITHUB_EVENT_NAME=pull_request CI_AUTHENTICATED_TERRAFORM_DISPATCH=true BASE_SHA=HEAD HEAD_SHA=HEAD "$detector")"
assert_output_groups "" "$unchanged_pr_output" "unchanged PR with ignored Terraform marker"
explicit_paths_output="$(GITHUB_EVENT_NAME=workflow_dispatch CI_AUTHENTICATED_TERRAFORM_DISPATCH=true "$detector" --paths system/core/app.go)"
assert_output_groups "system firestore_integration" "$explicit_paths_output" "explicit paths override dispatch scope"
ci_paths_output="$(GITHUB_EVENT_NAME=workflow_dispatch CI_AUTHENTICATED_TERRAFORM_DISPATCH=true "$detector" --paths .github/workflows/ci.yml)"
assert_output_groups "$all_groups" "$ci_paths_output" "CI source paths still select all groups"

fallback_output="$(GITHUB_EVENT_NAME=pull_request "$detector")"
if ! printf '%s\n' "$fallback_output" | grep -Fxq 'all=true'; then
	echo "Expected missing pull_request SHAs to select all groups" >&2
	exit 1
fi

if GITHUB_EVENT_NAME=pull_request BASE_SHA=missing HEAD_SHA=missing "$detector" >/dev/null 2>&1; then
	echo "Expected a failed git diff to fail the detector" >&2
	exit 1
fi

assert_exact_groups gcp_terraform .github/scripts/terraform_quota_refresh_gate.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_quota_refresh_gate.py

assert_exact_groups gcp_terraform .github/scripts/terraform_export_scheduler_gate.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_export_scheduler_gate.py

# Independent Terraform contracts must route to Terraform CI.



assert_exact_groups gcp_terraform infra/gcp/scripts/runtime_plan_selection.py
assert_exact_groups gcp_terraform infra/gcp/tests/test_runtime_plan_contract.py

# Every reusable CI module and its executable contracts select the full suite.
for ci_path in .github/dependabot.yml .github/scripts/test_ci_security.py .github/workflows/ci-{system,frontend,tools,infra,repository}.yml .github/scripts/check-ci-results.py .github/scripts/install-ci-source-tools.sh .github/scripts/test_ci_workflows.py .github/scripts/test_ci_results.py; do
	assert_exact_groups "$all_groups" "$ci_path"
done

echo "detect-ci-paths.sh tests passed"
