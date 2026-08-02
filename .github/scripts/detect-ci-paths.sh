#!/usr/bin/env bash

set -euo pipefail

readonly CI_GROUPS=(
	system
	room_image_prompt
	menu_image_generator
	youtube_monitor
	docs_site
	aws_cdk
	node_projects
	all
)

system=false
room_image_prompt=false
menu_image_generator=false
youtube_monitor=false
docs_site=false
aws_cdk=false
node_projects=false
all=false

changed_paths=()
unmatched_paths=()

usage() {
	cat <<'EOF'
Usage:
  detect-ci-paths.sh [--paths PATH ...]

With --paths, PATH arguments are classified directly for local testing.
Without --paths, the script resolves paths from the GitHub event:
  - pull_request: BASE_SHA...HEAD_SHA
  - workflow_dispatch or an unsupported/incomplete event: all groups
EOF
}

set_all_groups() {
	system=true
	room_image_prompt=true
	menu_image_generator=true
	youtube_monitor=true
	docs_site=true
	aws_cdk=true
	node_projects=true
	all=true
}

path_is_ci_config() {
	case "$1" in
		.github/workflows/ci.yml|.github/scripts/detect-ci-paths.sh|.github/actions/*)
			return 0
			;;
		*)
			return 1
			;;
	esac
}

classify_path() {
	local path="$1"
	local matched=false

	if path_is_ci_config "$path"; then
		set_all_groups
		matched=true
	fi

	case "$path" in
		system/*)
			system=true
			if [[ "$path" == system/Dockerfile* || "$path" == system/.dockerignore ]]; then
				aws_cdk=true
			fi
			matched=true
			;;
		tools/room-image-prompt/*)
			room_image_prompt=true
			matched=true
			;;
		tools/menu-image-generator/*)
			menu_image_generator=true
			matched=true
			;;
		youtube-monitor/*|biome.json)
			youtube_monitor=true
			matched=true
			;;
		docs-site/*)
			docs_site=true
			matched=true
			;;
		aws-cdk/*)
			aws_cdk=true
			matched=true
			;;
		.node-version|.nvmrc)
			node_projects=true
			matched=true
			;;
	esac

	if [[ "$matched" == false ]]; then
		unmatched_paths+=("$path")
	fi
}

resolve_changed_paths() {
	if [[ "${1:-}" == "--paths" ]]; then
		shift
		changed_paths=("$@")
		return
	fi

	if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
		usage
		exit 0
	fi

	if [[ $# -gt 0 ]]; then
		usage >&2
		exit 2
	fi

	local event_name="${GITHUB_EVENT_NAME:-}"
	if [[ "$event_name" == "workflow_dispatch" ]]; then
		set_all_groups
		return
	fi

	if [[ "$event_name" != "pull_request" ]]; then
		set_all_groups
		return
	fi

	local base_sha="${BASE_SHA:-}"
	local head_sha="${HEAD_SHA:-}"
	if [[ -z "$base_sha" || -z "$head_sha" ]]; then
		set_all_groups
		return
	fi

	local diff_file
	diff_file="$(mktemp)"
	if ! git diff --no-renames --name-only "$base_sha...$head_sha" >"$diff_file"; then
		rm -f "$diff_file"
		echo "::error::Failed to resolve changed paths from $base_sha...$head_sha" >&2
		exit 1
	fi
	changed_paths=()
	while IFS= read -r path; do
		changed_paths+=("$path")
	done <"$diff_file"
	rm -f "$diff_file"
}

group_value() {
	case "$1" in
		system) printf '%s' "$system" ;;
		room_image_prompt) printf '%s' "$room_image_prompt" ;;
		menu_image_generator) printf '%s' "$menu_image_generator" ;;
		youtube_monitor) printf '%s' "$youtube_monitor" ;;
		docs_site) printf '%s' "$docs_site" ;;
		aws_cdk) printf '%s' "$aws_cdk" ;;
		node_projects) printf '%s' "$node_projects" ;;
		all) printf '%s' "$all" ;;
		*)
			echo "Unknown CI group: $1" >&2
			exit 2
			;;
	esac
}

write_output() {
	local group="$1"
	local value
	value="$(group_value "$group")"

	if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
		printf '%s=%s\n' "$group" "$value" >>"$GITHUB_OUTPUT"
	fi
	printf '%s=%s\n' "$group" "$value"
}

write_summary() {
	local summary_file="${GITHUB_STEP_SUMMARY:-}"
	[[ -n "$summary_file" ]] || return 0

	{
		echo "### Detect CI Changes"
		echo
		echo "Changed paths:"
		if ((${#changed_paths[@]} == 0)); then
			echo "- (none)"
		else
			printf '%s\n' "${changed_paths[@]}" | sed 's/^/- /'
		fi
		echo
		echo "Selected groups:"
		for group in "${CI_GROUPS[@]}"; do
			printf -- '- %s: %s\n' "$group" "$(group_value "$group")"
		done
		echo
		echo "Unmatched paths:"
		if ((${#unmatched_paths[@]} == 0)); then
			echo "- (none)"
		else
			printf '%s\n' "${unmatched_paths[@]}" | sed 's/^/- /'
		fi
	} >>"$summary_file"
}

resolve_changed_paths "$@"

if [[ "$all" == false ]]; then
	if ((${#changed_paths[@]} > 0)); then
		for path in "${changed_paths[@]}"; do
			classify_path "$path"
		done
	fi
else
	# A CI configuration change selects every group and is still shown in the summary.
	if ((${#changed_paths[@]} > 0)); then
		for path in "${changed_paths[@]}"; do
			if ! path_is_ci_config "$path"; then
				unmatched_paths+=("$path")
			fi
		done
	fi
fi

for group in "${CI_GROUPS[@]}"; do
	write_output "$group"
done
write_summary
