#!/usr/bin/env bash

set -euo pipefail

APPLY=false
INCLUDE_ARTIFACTS=false

readonly ARTIFACT_TARGETS=(
	youtube-monitor/storybook-static
	youtube-monitor/.next
	docs-site/build
)

usage() {
	cat <<'EOF'
Usage:
  scripts/cleanup-local-git.sh [--dry-run] [--apply] [--include-artifacts]

Safely cleans up local git state in this repository.
Default mode is dry-run (no files, worktrees, or branches are modified).

Options:
  --dry-run            Show what would be cleaned up without making changes (default)
  --apply              Execute the cleanup operations
  --include-artifacts  Also inspect/remove regenerable local build artifacts
                       (e.g. youtube-monitor/storybook-static). Only deletes when
                       combined with --apply; prints size/status in dry-run.
  -h, --help           Show this help message

Cleanup scope:
  1. Prune stale/prunable git worktree metadata (`git worktree prune`)
  2. Remove unlocked worktrees under `.worktrees/` whose checked-out branch has
     upstream tracking status `[gone]`, only when
       - the branch is integrated into a retained remote-tracking ref, and
       - the worktree has no modified, untracked, or ignored files other than
         regenerable ones (node_modules/, .next/, storybook-static/, ...)
  3. Delete local branches not checked out in any worktree whose upstream tracking
     status is `[gone]` and that are integrated into a retained remote-tracking ref
     (never deletes `main`, `dev`, `integration/*`, or current HEAD)
  4. (With `--include-artifacts`) Remove regenerable local build artifacts whose
     real path is the expected path inside the main worktree

Retained remote-tracking refs are `<remote>/main`, `<remote>/dev`, and
`<remote>/integration/*`. A branch counts as integrated only when its tip is an
ancestor of one of them, so every commit on it is still reachable after deletion.
Other branches and worktrees are listed as `[keep]` and left untouched. When
merging such a branch into a retained ref would change nothing (e.g. after a
squash merge), the `[keep]` line says so; delete it manually after review.

Tip: Run `git fetch --prune` beforehand to refresh `[gone]` upstream tracking status.
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		--apply)
			APPLY=true
			;;
		--dry-run)
			APPLY=false
			;;
		--include-artifacts)
			INCLUDE_ARTIFACTS=true
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			echo "Unknown option: $1" >&2
			usage >&2
			exit 2
			;;
	esac
	shift
done

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
	echo "Error: git リポジトリ内で実行してください。" >&2
	exit 1
fi

canonical_dir() {
	(cd "$1" && pwd -P)
}

is_protected_branch() {
	local branch="$1"
	case "$branch" in
		main|dev|integration/*)
			return 0
			;;
	esac
	if [[ -n "$CURRENT_HEAD_BRANCH" && "$branch" == "$CURRENT_HEAD_BRANCH" ]]; then
		return 0
	fi
	return 1
}

array_contains() {
	local needle="$1"
	shift
	local item
	for item in "$@"; do
		if [[ "$item" == "$needle" ]]; then
			return 0
		fi
	done
	return 1
}

# Ignored files that are safe to delete together with a worktree because a
# build or package install regenerates them. Anything else (e.g. `.env`,
# `public/audio/*`) keeps the worktree.
is_regenerable_ignored_path() {
	local path="$1"
	if [[ "$path" == */ ]]; then
		path="${path%/}"
		case "$path" in
			docs-site/build)
				return 0
				;;
		esac
		case "${path##*/}" in
			node_modules|.next|storybook-static|.docusaurus|.turbo|coverage|__pycache__|cdk.out)
				return 0
				;;
		esac
		return 1
	fi
	case "${path##*/}" in
		.DS_Store|*.tsbuildinfo|next-env.d.ts)
			return 0
			;;
	esac
	return 1
}

CURRENT_TOPLEVEL_RAW="$(git rev-parse --show-toplevel)"
CURRENT_TOPLEVEL_REAL="$(canonical_dir "$CURRENT_TOPLEVEL_RAW")"
CURRENT_HEAD_BRANCH="$(git symbolic-ref --quiet --short HEAD || true)"

MAIN_ROOT_RAW=""
while IFS= read -r line; do
	case "$line" in
		"worktree "*)
			MAIN_ROOT_RAW="${line#worktree }"
			break
			;;
	esac
done < <(git worktree list --porcelain)

if [[ -z "$MAIN_ROOT_RAW" || ! -d "$MAIN_ROOT_RAW" ]]; then
	echo "Error: メインの worktree ルートを特定できませんでした。" >&2
	exit 1
fi

MAIN_ROOT_REAL="$(canonical_dir "$MAIN_ROOT_RAW")"
WORKTREES_DIR_PREFIX="${MAIN_ROOT_REAL}/.worktrees/"

# merge-tree writes tree objects. Keep them in a scratch object directory so
# that dry-run leaves the repository's object store untouched.
REPO_OBJECTS_DIR="$(git -C "$MAIN_ROOT_REAL" rev-parse --path-format=absolute --git-path objects)"
SCRATCH_DIR="$(mktemp -d "${TMPDIR:-/tmp}/cleanup-local-git.XXXXXX")"
trap 'rm -rf -- "$SCRATCH_DIR"' EXIT
mkdir -p "${SCRATCH_DIR}/objects"

RETAINED_REFS=()
while IFS= read -r ref; do
	remote_branch="${ref#refs/remotes/}"
	remote_branch="${remote_branch#*/}"
	case "$remote_branch" in
		main|dev|integration/*)
			RETAINED_REFS+=("$ref")
			;;
	esac
done < <(git -C "$MAIN_ROOT_REAL" for-each-ref --format='%(refname)' refs/remotes)

branch_tip() {
	git -C "$MAIN_ROOT_REAL" rev-parse --verify --quiet "refs/heads/${1}^{commit}"
}

# Prints the retained ref whose history contains the branch tip and returns 0.
# Only this check authorizes deletion: every commit on the branch stays reachable.
find_integrating_ref() {
	local tip ref

	if ((${#RETAINED_REFS[@]} == 0)) || ! tip="$(branch_tip "$1")"; then
		return 1
	fi
	for ref in "${RETAINED_REFS[@]}"; do
		if git -C "$MAIN_ROOT_REAL" merge-base --is-ancestor "$tip" "$ref" 2>/dev/null; then
			echo "${ref#refs/remotes/}"
			return 0
		fi
	done
	return 1
}

# Prints the retained ref into which merging the branch changes no files and
# returns 0. This is only a hint for manual review (e.g. squash-merged
# branches); matching content does not mean the branch's commits are retained.
find_content_merged_ref() {
	local tip ref merged_tree ref_tree

	if ((${#RETAINED_REFS[@]} == 0)) || ! tip="$(branch_tip "$1")"; then
		return 1
	fi
	for ref in "${RETAINED_REFS[@]}"; do
		if ! merged_tree="$(
			GIT_OBJECT_DIRECTORY="${SCRATCH_DIR}/objects" \
				GIT_ALTERNATE_OBJECT_DIRECTORIES="$REPO_OBJECTS_DIR" \
				git -C "$MAIN_ROOT_REAL" merge-tree --write-tree --no-messages "$ref" "$tip" 2>/dev/null
		)"; then
			continue
		fi
		merged_tree="${merged_tree%%$'\n'*}"
		ref_tree="$(git -C "$MAIN_ROOT_REAL" rev-parse "${ref}^{tree}")"
		if [[ "$merged_tree" == "$ref_tree" ]]; then
			echo "${ref#refs/remotes/}"
			return 0
		fi
	done
	return 1
}

# Prints the [keep] line for a branch whose tip is not in any retained ref.
print_not_integrated() {
	local label="$1" branch="$2" content_ref
	if content_ref="$(find_content_merged_ref "$branch")"; then
		echo "  [keep] not an ancestor of retained refs; content matches ${content_ref} (likely squash-merged, review and delete manually): ${label}"
	else
		echo "  [keep] not integrated into retained refs (review manually): ${label}"
	fi
}

if [[ "$APPLY" == true ]]; then
	echo "Mode: APPLY (cleanup will be executed)"
else
	echo "Mode: DRY-RUN (no changes will be made; pass --apply to execute)"
fi
echo "Repository root: ${MAIN_ROOT_REAL}"
echo

# -----------------------------------------------------------------------------
# 1. Prune stale/prunable worktree metadata
# -----------------------------------------------------------------------------
echo "== 1. Stale worktree metadata (git worktree prune) =="
if [[ "$APPLY" == true ]]; then
	prune_output="$(git -C "$MAIN_ROOT_REAL" worktree prune --verbose 2>&1)"
	if [[ -n "$prune_output" ]]; then
		printf '%s\n' "$prune_output"
	else
		echo "  (none)"
	fi
else
	prune_output="$(git -C "$MAIN_ROOT_REAL" worktree prune --dry-run --verbose 2>&1)"
	if [[ -n "$prune_output" ]]; then
		while IFS= read -r line; do
			echo "  [dry-run] $line"
		done <<< "$prune_output"
	else
		echo "  (none)"
	fi
fi
echo

# -----------------------------------------------------------------------------
# 2. Clean, unlocked [gone] worktrees under .worktrees/
# -----------------------------------------------------------------------------
echo "== 2. Integrated, clean [gone] worktrees under .worktrees/ =="

PLANNED_REMOVED_WORKTREES=()
wt_candidates_found=false

process_worktree_entry() {
	local wt_path="$1"
	local wt_branch="$2"
	local wt_locked="$3"
	local wt_prunable="$4"
	local wt_bare="$5"
	local wt_index="$6"

	# Skip main worktree (index 0) and bare entries.
	if [[ "$wt_index" -eq 0 || "$wt_bare" == true ]]; then
		return 0
	fi

	# Prunable entries are handled in Step 1.
	if [[ "$wt_prunable" == true || ! -d "$wt_path" ]]; then
		return 0
	fi

	local wt_real
	wt_real="$(canonical_dir "$wt_path")"

	# Only manage worktrees strictly inside <main_root>/.worktrees/.
	if [[ "$wt_real" != "${WORKTREES_DIR_PREFIX}"* ]]; then
		return 0
	fi

	wt_candidates_found=true

	if [[ "$wt_real" == "$CURRENT_TOPLEVEL_REAL" ]]; then
		echo "  [skip] current worktree: ${wt_real}"
		return 0
	fi

	if [[ "$wt_locked" == true ]]; then
		echo "  [skip] locked worktree: ${wt_real} (branch: ${wt_branch:-detached})"
		return 0
	fi

	if [[ -z "$wt_branch" ]]; then
		echo "  [skip] detached HEAD: ${wt_real}"
		return 0
	fi

	if is_protected_branch "$wt_branch"; then
		echo "  [skip] protected branch: ${wt_real} (branch: ${wt_branch})"
		return 0
	fi

	local track
	track="$(git -C "$MAIN_ROOT_REAL" for-each-ref --format='%(upstream:track)' "refs/heads/${wt_branch}")"
	if [[ "$track" != "[gone]" ]]; then
		echo "  [skip] upstream is not [gone]: ${wt_real} (branch: ${wt_branch}, track: ${track:-none})"
		return 0
	fi

	local integrated_ref
	if ! integrated_ref="$(find_integrating_ref "$wt_branch")"; then
		print_not_integrated "${wt_real} (branch: ${wt_branch})" "$wt_branch"
		return 0
	fi

	local status_file="${SCRATCH_DIR}/status"
	if ! git -C "$wt_real" status --porcelain=v1 -z --ignored=traditional --untracked-files=normal >"$status_file" 2>/dev/null; then
		echo "  [skip] failed to inspect git status: ${wt_real}"
		return 0
	fi

	local entry has_changes=false
	local kept_ignored=()
	while IFS= read -r -d '' entry; do
		if [[ "${entry:0:3}" == "!! " ]]; then
			if ! is_regenerable_ignored_path "${entry:3}"; then
				kept_ignored+=("${entry:3}")
			fi
		else
			has_changes=true
		fi
	done <"$status_file"

	if [[ "$has_changes" == true ]]; then
		echo "  [skip] dirty worktree: ${wt_real} (branch: ${wt_branch})"
		return 0
	fi
	if ((${#kept_ignored[@]} > 0)); then
		echo "  [keep] ignored local files present: ${wt_real} (branch: ${wt_branch})"
		for entry in "${kept_ignored[@]}"; do
			echo "         - ${entry}"
		done
		return 0
	fi

	if [[ "$APPLY" == true ]]; then
		if git -C "$MAIN_ROOT_REAL" worktree remove "$wt_real"; then
			echo "  [removed] worktree: ${wt_real} (branch: ${wt_branch}, integrated: ${integrated_ref})"
		else
			echo "  [warn] failed to remove worktree (kept): ${wt_real} (branch: ${wt_branch})" >&2
		fi
	else
		echo "  [dry-run] remove worktree: ${wt_real} (branch: ${wt_branch}, integrated: ${integrated_ref})"
		PLANNED_REMOVED_WORKTREES+=("$wt_real")
	fi
}

wt_path=""
wt_branch=""
wt_locked=false
wt_prunable=false
wt_bare=false
wt_index=0

while IFS= read -r line || [[ -n "$line" ]]; do
	if [[ -z "$line" ]]; then
		if [[ -n "$wt_path" ]]; then
			process_worktree_entry "$wt_path" "$wt_branch" "$wt_locked" "$wt_prunable" "$wt_bare" "$wt_index"
			wt_index=$((wt_index + 1))
		fi
		wt_path=""
		wt_branch=""
		wt_locked=false
		wt_prunable=false
		wt_bare=false
		continue
	fi
	case "$line" in
		"worktree "*)
			wt_path="${line#worktree }"
			;;
		"branch refs/heads/"*)
			wt_branch="${line#branch refs/heads/}"
			;;
		"locked"|"locked "*)
			wt_locked=true
			;;
		"prunable"|"prunable "*)
			wt_prunable=true
			;;
		"bare")
			wt_bare=true
			;;
	esac
done < <(git -C "$MAIN_ROOT_REAL" worktree list --porcelain; printf '\n')

if [[ "$wt_candidates_found" == false ]]; then
	echo "  (no worktrees under .worktrees/)"
fi
echo

# -----------------------------------------------------------------------------
# 3. Local [gone] branches not checked out in any remaining worktree
# -----------------------------------------------------------------------------
echo "== 3. Integrated local [gone] branches (not checked out in any worktree) =="

ACTIVE_WT_BRANCHES=()

record_active_wt_branch() {
	local wt_path="$1"
	local wt_branch="$2"
	local wt_prunable="$3"

	if [[ -z "$wt_branch" ]]; then
		return 0
	fi
	# In dry-run mode, exclude prunable worktrees (Step 1) and worktrees planned
	# for removal (Step 2) so dry-run matches what --apply will do.
	if [[ "$wt_prunable" == true ]]; then
		return 0
	fi
	if [[ -d "$wt_path" ]]; then
		local wt_real
		wt_real="$(canonical_dir "$wt_path")"
		if ((${#PLANNED_REMOVED_WORKTREES[@]} > 0)) && array_contains "$wt_real" "${PLANNED_REMOVED_WORKTREES[@]}"; then
			return 0
		fi
	fi
	ACTIVE_WT_BRANCHES+=("$wt_branch")
}

wt_path=""
wt_branch=""
wt_prunable=false

while IFS= read -r line || [[ -n "$line" ]]; do
	if [[ -z "$line" ]]; then
		if [[ -n "$wt_path" ]]; then
			record_active_wt_branch "$wt_path" "$wt_branch" "$wt_prunable"
		fi
		wt_path=""
		wt_branch=""
		wt_prunable=false
		continue
	fi
	case "$line" in
		"worktree "*)
			wt_path="${line#worktree }"
			;;
		"branch refs/heads/"*)
			wt_branch="${line#branch refs/heads/}"
			;;
		"prunable"|"prunable "*)
			wt_prunable=true
			;;
	esac
done < <(git -C "$MAIN_ROOT_REAL" worktree list --porcelain; printf '\n')

branch_candidates_found=false
while IFS=$'\t' read -r branch track; do
	if [[ "$track" != "[gone]" ]]; then
		continue
	fi
	branch_candidates_found=true

	if is_protected_branch "$branch"; then
		echo "  [skip] protected branch: ${branch}"
		continue
	fi

	if ((${#ACTIVE_WT_BRANCHES[@]} > 0)) && array_contains "$branch" "${ACTIVE_WT_BRANCHES[@]}"; then
		echo "  [skip] checked out in worktree: ${branch}"
		continue
	fi

	if ! integrated_ref="$(find_integrating_ref "$branch")"; then
		print_not_integrated "$branch" "$branch"
		continue
	fi

	if [[ "$APPLY" == true ]]; then
		git -C "$MAIN_ROOT_REAL" branch -D -- "$branch" >/dev/null
		echo "  [deleted] branch: ${branch} (integrated: ${integrated_ref})"
	else
		echo "  [dry-run] delete branch: ${branch} (integrated: ${integrated_ref})"
	fi
done < <(git -C "$MAIN_ROOT_REAL" for-each-ref --format='%(refname:short)%09%(upstream:track)' refs/heads)

if [[ "$branch_candidates_found" == false ]]; then
	echo "  (no [gone] local branches)"
fi
echo

# -----------------------------------------------------------------------------
# 4. Optional regenerable local build artifacts (--include-artifacts)
# -----------------------------------------------------------------------------
echo "== 4. Local build artifacts =="
if [[ "$INCLUDE_ARTIFACTS" == false ]]; then
	echo "  (skipped; pass --include-artifacts to inspect or remove build artifacts)"
else
	for rel_path in "${ARTIFACT_TARGETS[@]}"; do
		artifact_path="${MAIN_ROOT_REAL}/${rel_path}"
		if [[ -L "$artifact_path" ]]; then
			echo "  [skip] symlink (not removing): ${rel_path}"
			continue
		fi
		if [[ ! -d "$artifact_path" ]]; then
			echo "  [absent] ${rel_path}"
			continue
		fi
		artifact_real="$(canonical_dir "$artifact_path")"
		if [[ "$artifact_real" != "$artifact_path" ]]; then
			echo "  [skip] resolves to a different real path (not removing): ${rel_path} -> ${artifact_real}"
			continue
		fi
		if [[ -n "$(git -C "$MAIN_ROOT_REAL" ls-files -- "$rel_path")" ]]; then
			echo "  [skip] contains tracked files: ${rel_path}"
			continue
		fi
		size="$(du -sh "$artifact_real" | awk '{print $1}')"
		if [[ "$APPLY" == true ]]; then
			rm -rf -- "$artifact_real"
			echo "  [removed] artifact: ${rel_path} (${size})"
		else
			echo "  [dry-run] remove artifact: ${rel_path} (${size})"
		fi
	done
fi
