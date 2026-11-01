#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
detector="$script_dir/detect-ci-paths.sh"

# テストケースの判定結果を、呼び出し元jobのSummaryやoutputsへ書き込まない。
test_dir="$(mktemp -d)"
trap 'rm -rf "$test_dir"' EXIT
export GITHUB_STEP_SUMMARY="$test_dir/summary"
export GITHUB_OUTPUT="$test_dir/output"

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
assert_exact_groups "" .github/scripts/check-doc-references.py
assert_exact_groups "" .github/scripts/test_check_doc_references.py
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
assert_exact_groups gcp_terraform infra/gcp/modules/monitor-auth/main.tf
assert_exact_groups gcp_terraform infra/gcp/scripts/monitor_auth.py
assert_exact_groups gcp_terraform infra/gcp/environments/prod/main.tf
assert_exact_groups "gcp_terraform mypage" infra/mypage-platform/environments/dev/main.tf
assert_exact_groups "gcp_terraform mypage" infra/mypage-platform/environments/dev/desired-stage.txt
assert_exact_groups "gcp_terraform mypage" infra/mypage-platform/scripts/foundation_contract.py
assert_exact_groups "gcp_terraform mypage" infra/mypage-oauth-ttl/environments/dev/main.tf
assert_exact_groups "gcp_terraform mypage" infra/mypage-oauth-ttl/scripts/check-local.sh
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
assert_exact_groups gcp_terraform .github/scripts/test_user_activity_history_preparation.py
assert_exact_groups gcp_terraform .github/scripts/test_terraform_observation_native.py
assert_exact_groups gcp_terraform infra/gcp/scripts/terraform_runtime.py
assert_exact_groups gcp_terraform infra/gcp/tests/test_terraform_runtime.py
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
assert_exact_groups "$all_groups" .github/scripts/terraform_source_pin.py
assert_exact_groups "$all_groups" .github/scripts/test_ci_terraform_source.py
assert_exact_groups "$all_groups" .github/ci-validation/Dockerfile
assert_exact_groups "$all_groups" .github/scripts/base-image-pin-check.mjs
assert_exact_groups "$all_groups" .github/ci-validation/validate.sh
assert_exact_groups "$all_groups" .github/actions/prepare-ci-validation/action.yml
# Every repository script that the GCP Terraform CI executes must route to it,
# so a change to that script alone still runs its own test.
# Only the Terraform kinds of the isolated validation; the repository kinds run on every CI.
terraform_ci_scripts="$(awk '/^  terraform-(static|tests)\)$/{p=1; next} /^  [a-z-]+\)$/{p=0} p' "$script_dir/../ci-validation/validate.sh" | grep -oE '\.github/scripts/[A-Za-z0-9_.-]+\.(py|sh|mjs)' | sort -u)"
if [[ "$(printf '%s\n' "$terraform_ci_scripts" | grep -c .)" -lt 10 ]]; then
	echo "Expected to read the scripts executed by the isolated Terraform validation" >&2
	exit 1
fi
while IFS= read -r script; do
	routed="$("$detector" --paths "$script")"
	if ! grep -Fxq 'gcp_terraform=true' <<< "$routed"; then
		echo "Expected gcp_terraform=true for a script run by the isolated Terraform validation: $script" >&2
		exit 1
	fi
done <<< "$terraform_ci_scripts"

# A manual CI run always selects every domain; the retired Terraform dispatch
# marker can no longer narrow it.
for marker in "" true false; do
	manual_output="$(GITHUB_EVENT_NAME=workflow_dispatch CI_AUTHENTICATED_TERRAFORM_DISPATCH="$marker" "$detector")"
	assert_output_groups "$all_groups" "$manual_output" "manual dispatch with marker: $marker"
done
for event_name in pull_request push schedule unsupported; do
	fallback_output="$(GITHUB_EVENT_NAME="$event_name" BASE_SHA= HEAD_SHA= "$detector")"
	assert_output_groups "$all_groups" "$fallback_output" "non-dispatch event without SHAs: $event_name"
done
unchanged_pr_output="$(GITHUB_EVENT_NAME=pull_request BASE_SHA=HEAD HEAD_SHA=HEAD "$detector")"
assert_output_groups "" "$unchanged_pr_output" "unchanged PR"
explicit_paths_output="$(GITHUB_EVENT_NAME=workflow_dispatch "$detector" --paths system/core/app.go)"
assert_output_groups "system firestore_integration" "$explicit_paths_output" "explicit paths override dispatch scope"
ci_paths_output="$(GITHUB_EVENT_NAME=workflow_dispatch "$detector" --paths .github/workflows/ci.yml)"
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
for ci_path in .github/dependabot.yml .github/scripts/test_ci_security.py .github/workflows/ci-{system,frontend,tools,infra,repository,gcp-terraform}.yml .github/scripts/check-ci-results.py .github/scripts/install-ci-source-tools.sh .github/scripts/test_ci_workflows.py .github/scripts/test_ci_results.py .github/scripts/test_ci_path_summary.py .github/scripts/test_ci_gcp_terraform.py; do
	assert_exact_groups "$all_groups" "$ci_path"
done

# Known omissions and future convention-matching additions each select Terraform.
for path in terraform_export_function_gate.py test_terraform_export_function_gate.py test_user_activity_history.py terraform_history_table_metadata.py terraform_future_gate.py test_terraform_future_gate.py test_export_future.py; do
	assert_exact_groups gcp_terraform ".github/scripts/$path"
done
for path in new_helper.py renamed_helper.py nested/helper.py new_helper.sh; do
	assert_exact_groups "$all_groups" ".github/scripts/$path"
done
assert_exact_groups "" docs/development/unrelated.md
assert_exact_groups youtube_monitor youtube-monitor/src/unrelated.tsx

# Exercise the actual PR three-dot diff, including deletion and both sides of
# renames. No-renames preserves the old domain when moving to unrelated paths.
(
	cd "$test_dir"
	git init -q
	git config user.name 'CI routing test'
	git config user.email 'ci-test@example.invalid'
	mkdir -p .github/scripts system docs
	touch .github/scripts/terraform_deleted.py .github/scripts/terraform_renamed.py system/removed.go
	git add .
	git commit -qm base
	base_sha="$(git rev-parse HEAD)"
	rm .github/scripts/terraform_deleted.py system/removed.go
	git mv .github/scripts/terraform_renamed.py docs/moved.md
	git add .
	git commit -qm 'delete and rename'
	output="$(GITHUB_EVENT_NAME=pull_request BASE_SHA="$base_sha" HEAD_SHA=HEAD "$detector")"
	assert_output_groups "system firestore_integration gcp_terraform" "$output" "PR deletion and rename out of Terraform"

	git mv docs/moved.md .github/scripts/unknown_renamed.py
	touch .github/scripts/unknown_added.py
	git add .
	git commit -qm 'unknown CI addition and rename'
	output="$(GITHUB_EVENT_NAME=pull_request BASE_SHA=HEAD^ HEAD_SHA=HEAD "$detector")"
	assert_output_groups "$all_groups" "$output" "PR unknown CI addition and rename"

	git rm -q .github/scripts/unknown_renamed.py .github/scripts/unknown_added.py
	git commit -qm 'unknown CI deletion'
	output="$(GITHUB_EVENT_NAME=pull_request BASE_SHA=HEAD^ HEAD_SHA=HEAD "$detector")"
	assert_output_groups "$all_groups" "$output" "PR unknown CI deletion"

	# Shared configuration is full-suite even when discovered via a PR diff.
	mkdir -p .github/actions/new-action
	touch .github/actions/new-action/action.yml
	git add .
	git commit -qm 'shared CI addition'
	output="$(GITHUB_EVENT_NAME=pull_request BASE_SHA=HEAD^ HEAD_SHA=HEAD "$detector")"
	assert_output_groups "$all_groups" "$output" "PR shared CI addition"

	# Quoted/newline filenames must not evade the CI-script fallback.
	mkdir -p .github/scripts
	touch $'.github/scripts/odd\nname.py'
	git add .
	git commit -qm 'unusual CI filename'
	output="$(GITHUB_EVENT_NAME=pull_request BASE_SHA=HEAD^ HEAD_SHA=HEAD "$detector")"
	assert_output_groups "$all_groups" "$output" "NUL-delimited PR paths"
)

echo "detect-ci-paths.sh tests passed"
