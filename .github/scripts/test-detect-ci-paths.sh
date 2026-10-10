#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
detector="$script_dir/detect-ci-paths.sh"

assert_exact_groups() {
	local expected_groups="$1"
	shift
	local output
	local group
	local expected

	output="$($detector --paths "$@")"
	for group in system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk node_projects firestore_integration formal_spec all; do
		expected=false
		case " $expected_groups " in
			*" $group "*) expected=true ;;
		esac
		if ! grep -Fxq "$group=$expected" <<< "$output"; then
			echo "Expected $group=$expected for paths: $*" >&2
			exit 1
		fi
	done
}

# A here-string avoids a printf producer receiving SIGPIPE when grep -q exits.
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
assert_exact_groups mypage mypage/pnpm-lock.yaml
assert_exact_groups mypage docs/mypage/openapi.yaml
assert_exact_groups mypage infra/mypage-oauth-ttl/environments/dev/main.tf
assert_exact_groups mypage infra/mypage-oauth-ttl/environments/prod/tests/ttl.tftest.hcl
assert_exact_groups mypage infra/mypage-oauth-ttl/scripts/plan_contract.py
assert_exact_groups mypage .github/scripts/check-mypage-contract.py
assert_exact_groups "system firestore_integration" system/core/mypage/store.go
assert_exact_groups "system firestore_integration" system/cmd/mypage-server/main.go
assert_exact_groups "system firestore_integration" system/cmd/mypage-operator-dryrun/main.go
assert_exact_groups "mypage system firestore_integration" mypage/src/main.tsx system/core/mypage/store.go
assert_exact_groups "youtube_monitor mypage" biome.json
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
assert_exact_groups docs_site docs-site/docs/intro.md
assert_exact_groups room_image_prompt tools/room-image-prompt/cmd/room-image-prompt/main.go
assert_exact_groups room_image_prompt .agents/skills/room-art-direction/references/direction-a-clean-vivid-digital.md
assert_exact_groups menu_image_generator tools/menu-image-generator/src/index.ts
assert_exact_groups video_maker_simulator tools/video-maker/1000-minutes-simulator/src/pages/index.tsx
assert_exact_groups figma_plugin tools/figma-plugin/room-layout-analyzer/code.ts
assert_exact_groups node_projects .node-version
assert_exact_groups node_projects .nvmrc
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk node_projects firestore_integration formal_spec all" .github/workflows/ci.yml
for codeql_path in .github/codeql/.gitignore .github/codeql/README.md .github/codeql/analysis-config.yml .github/codeql/build-go.sh .github/codeql/install-actionlint.sh .github/codeql/lint-workflows.py .github/codeql/requirements.txt .github/codeql/test_source_contracts.py .github/codeql/tooling.json .github/codeql/validate-source.py .github/scripts/detect-ci-paths.sh .github/scripts/test-detect-ci-paths.sh .github/workflows/codeql-advanced.yml .github/workflows/codeql-analyzer.yml .github/workflows/codeql-source-contracts.yml; do
	assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk node_projects firestore_integration formal_spec all" "$codeql_path"
done
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk node_projects firestore_integration formal_spec all" .github/workflows/mypage-oauth-ttl.yml
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk node_projects firestore_integration formal_spec all" .github/workflows/deploy-docs.yml
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk node_projects firestore_integration formal_spec all" .github/scripts/detect-ci-paths.sh
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk node_projects firestore_integration formal_spec all" .github/scripts/test-detect-ci-paths.sh
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk node_projects firestore_integration formal_spec all" .github/scripts/run-firestore-integration-tests.sh
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk node_projects firestore_integration formal_spec all" .github/scripts/base-image-update-report.mjs
assert_exact_groups "system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk node_projects firestore_integration formal_spec all" .github/scripts/base-image-update-report.test.mjs
assert_exact_groups "system firestore_integration youtube_monitor" system/core/app.go youtube-monitor/src/app.ts

manual_output="$(GITHUB_EVENT_NAME=workflow_dispatch "$detector")"
for group in system room_image_prompt menu_image_generator video_maker_simulator figma_plugin youtube_monitor mypage docs_site aws_cdk node_projects firestore_integration formal_spec all; do
	if ! grep -Fxq "$group=true" <<< "$manual_output"; then
		echo "Expected workflow_dispatch to select $group" >&2
		exit 1
	fi
done

fallback_output="$(GITHUB_EVENT_NAME=pull_request "$detector")"
if ! grep -Fxq 'all=true' <<< "$fallback_output"; then
	echo "Expected missing pull_request SHAs to select all groups" >&2
	exit 1
fi

if GITHUB_EVENT_NAME=pull_request BASE_SHA=missing HEAD_SHA=missing "$detector" >/dev/null 2>&1; then
	echo "Expected a failed git diff to fail the detector" >&2
	exit 1
fi

echo "detect-ci-paths.sh tests passed"
