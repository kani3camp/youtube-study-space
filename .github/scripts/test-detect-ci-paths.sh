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
	for group in system room_image_prompt menu_image_generator youtube_monitor docs_site aws_cdk node_projects all; do
		expected=false
		case " $expected_groups " in
			*" $group "*) expected=true ;;
		esac
		if ! printf '%s\n' "$output" | grep -Fxq "$group=$expected"; then
			echo "Expected $group=$expected for paths: $*" >&2
			exit 1
		fi
	done
}

assert_exact_groups "" README.md
assert_exact_groups youtube_monitor youtube-monitor/src/app.ts
assert_exact_groups youtube_monitor biome.json
assert_exact_groups system system/core/workspaceapp/app.go
assert_exact_groups "system aws_cdk" system/Dockerfile.lambda
assert_exact_groups "system aws_cdk" system/.dockerignore
assert_exact_groups aws_cdk aws-cdk/lib/aws-cdk-stack.ts
assert_exact_groups docs_site docs-site/docs/intro.md
assert_exact_groups room_image_prompt tools/room-image-prompt/cmd/room-image-prompt/main.go
assert_exact_groups menu_image_generator tools/menu-image-generator/src/index.ts
assert_exact_groups node_projects .node-version
assert_exact_groups node_projects .nvmrc
assert_exact_groups "system room_image_prompt menu_image_generator youtube_monitor docs_site aws_cdk node_projects all" .github/workflows/ci.yml
assert_exact_groups "system room_image_prompt menu_image_generator youtube_monitor docs_site aws_cdk node_projects all" .github/workflows/deploy-docs.yml
assert_exact_groups "system room_image_prompt menu_image_generator youtube_monitor docs_site aws_cdk node_projects all" .github/scripts/detect-ci-paths.sh
assert_exact_groups "system room_image_prompt menu_image_generator youtube_monitor docs_site aws_cdk node_projects all" .github/scripts/test-detect-ci-paths.sh
assert_exact_groups "system youtube_monitor" system/core/app.go youtube-monitor/src/app.ts

manual_output="$(GITHUB_EVENT_NAME=workflow_dispatch "$detector")"
for group in system room_image_prompt menu_image_generator youtube_monitor docs_site aws_cdk node_projects all; do
	if ! printf '%s\n' "$manual_output" | grep -Fxq "$group=true"; then
		echo "Expected workflow_dispatch to select $group" >&2
		exit 1
	fi
done

fallback_output="$(GITHUB_EVENT_NAME=pull_request "$detector")"
if ! printf '%s\n' "$fallback_output" | grep -Fxq 'all=true'; then
	echo "Expected missing pull_request SHAs to select all groups" >&2
	exit 1
fi

if GITHUB_EVENT_NAME=pull_request BASE_SHA=missing HEAD_SHA=missing "$detector" >/dev/null 2>&1; then
	echo "Expected a failed git diff to fail the detector" >&2
	exit 1
fi

echo "detect-ci-paths.sh tests passed"
