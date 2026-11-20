#!/usr/bin/env bash
# Regression tests for scripts/cleanup-local-git.sh using throwaway repositories.

set -euo pipefail

SCRIPT="$(cd "$(dirname "$0")" && pwd -P)/cleanup-local-git.sh"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/test-cleanup-local-git.XXXXXX")"
WORK="$(cd "$WORK" && pwd -P)"
trap 'rm -rf -- "$WORK"' EXIT

failures=0

pass() { echo "ok   - $1"; }
fail() {
	echo "FAIL - $1" >&2
	failures=$((failures + 1))
}

g() { git -C "$REPO" "$@"; }

branch_exists() { g show-ref --verify --quiet "refs/heads/$1"; }

expect_branch_kept() {
	if branch_exists "$1"; then pass "$2"; else fail "$2"; fi
}

expect_branch_deleted() {
	if branch_exists "$1"; then fail "$2"; else pass "$2"; fi
}

expect_path_exists() {
	if [[ -e "$1" ]]; then pass "$2"; else fail "$2"; fi
}

expect_path_absent() {
	if [[ -e "$1" ]]; then fail "$2"; else pass "$2"; fi
}

commit_file() {
	local dir="$1" file="$2" content="$3"
	mkdir -p "$(dirname "${dir}/${file}")"
	printf '%s\n' "$content" >"${dir}/${file}"
	git -C "$dir" add -- "$file"
	git -C "$dir" commit -q -m "update ${file}"
}

# Pushes the branch, then deletes it on the remote so the upstream becomes [gone].
publish_then_delete_remote() {
	g push -q -u origin "$1"
	g push -q origin --delete "$1"
}

setup_repo() {
	export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.com
	export GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.com
	export GIT_CONFIG_NOSYSTEM=1
	export HOME="${WORK}/home"
	mkdir -p "$HOME"

	git init -q --bare "${WORK}/origin.git"
	REPO="${WORK}/repo"
	git init -q -b dev "$REPO"
	g remote add origin "${WORK}/origin.git"
	printf '%s\n' '.worktrees/' 'node_modules/' '.env' >"${REPO}/.gitignore"
	g add .gitignore
	g commit -q -m init
	commit_file "$REPO" README.md base
	g push -q -u origin dev

	# 1. Pushed, remote deleted, then one more local-only commit.
	g switch -q -c topic-unpushed
	commit_file "$REPO" unpushed.txt v1
	publish_then_delete_remote topic-unpushed
	commit_file "$REPO" unpushed.txt v2-local-only
	g switch -q dev

	# 2. Fast-forward merged into dev.
	g switch -q -c topic-merged
	commit_file "$REPO" merged.txt v1
	publish_then_delete_remote topic-merged
	g switch -q dev
	g merge -q --ff-only topic-merged

	# 3. Squash merged into dev, and dev moved on afterwards.
	g switch -q -c topic-squashed
	commit_file "$REPO" squashed.txt v1
	commit_file "$REPO" squashed.txt v2
	publish_then_delete_remote topic-squashed
	g switch -q dev
	g merge -q --squash topic-squashed >/dev/null
	g commit -q -m "squash topic-squashed"
	commit_file "$REPO" README.md after-squash

	# 3b. Remote deleted, then local-only commits whose net effect is empty.
	g switch -q -c topic-reverted
	commit_file "$REPO" reverted-base.txt v1
	publish_then_delete_remote topic-reverted
	g switch -q dev
	g merge -q --ff-only topic-reverted
	g switch -q topic-reverted
	commit_file "$REPO" draft.txt draft
	g revert --no-edit HEAD >/dev/null
	g switch -q dev

	# 4. Merged worktrees: one with an ignored local file, one with only node_modules.
	for name in wt-env wt-deps; do
		g switch -q -c "$name"
		commit_file "$REPO" "${name}.txt" v1
		publish_then_delete_remote "$name"
		g switch -q dev
		g merge -q --ff-only "$name"
	done
	g push -q origin dev
	g fetch -q --prune

	g worktree add -q "${REPO}/.worktrees/wt-env" wt-env
	printf 'SECRET=1\n' >"${REPO}/.worktrees/wt-env/.env"
	g worktree add -q "${REPO}/.worktrees/wt-deps" wt-deps
	mkdir -p "${REPO}/.worktrees/wt-deps/node_modules/pkg"
	printf 'x\n' >"${REPO}/.worktrees/wt-deps/node_modules/pkg/index.js"

	# 5. Worktree for an unpushed branch.
	g switch -q -c wt-unpushed
	commit_file "$REPO" wt-unpushed.txt v1
	publish_then_delete_remote wt-unpushed
	commit_file "$REPO" wt-unpushed.txt v2-local-only
	g switch -q dev
	g worktree add -q "${REPO}/.worktrees/wt-unpushed" wt-unpushed

	# 6. Artifacts: one regular, one reached through a symlinked parent.
	mkdir -p "${REPO}/youtube-monitor/storybook-static" "${WORK}/outside/storybook-static"
	printf 'x\n' >"${REPO}/youtube-monitor/storybook-static/index.html"
	printf 'keep\n' >"${WORK}/outside/storybook-static/index.html"
	ln -s "${WORK}/outside" "${REPO}/docs-site"
	mkdir -p "${WORK}/outside/build"
	printf 'keep\n' >"${WORK}/outside/build/index.html"
}

snapshot_state() {
	{
		g for-each-ref --format='%(refname) %(objectname)'
		g worktree list --porcelain
		find "$(g rev-parse --path-format=absolute --git-path objects)" -type f | LC_ALL=C sort
		find "$REPO" -path "${REPO}/.git" -prune -o -print | LC_ALL=C sort
	} >"$1"
}

setup_repo

# Dry-run must not change refs, worktrees, objects, or files.
snapshot_state "${WORK}/before"
(cd "$REPO" && bash "$SCRIPT" --include-artifacts) >"${WORK}/dry-run.log"
snapshot_state "${WORK}/after"
if cmp -s "${WORK}/before" "${WORK}/after"; then
	pass "dry-run leaves repository state unchanged"
else
	fail "dry-run leaves repository state unchanged"
	diff "${WORK}/before" "${WORK}/after" >&2 || true
fi

(cd "$REPO" && bash "$SCRIPT" --apply --include-artifacts) >"${WORK}/apply.log"

expect_branch_kept topic-unpushed "keeps [gone] branch with commits not in retained refs"
expect_branch_deleted topic-merged "deletes [gone] branch merged into origin/dev"
expect_branch_kept topic-squashed "keeps squash-merged branch whose commits are not in retained refs"
if grep -Fq "content matches origin/dev (likely squash-merged, review and delete manually): topic-squashed" "${WORK}/apply.log"; then
	pass "reports squash-merged branch as a manual-review candidate"
else
	fail "reports squash-merged branch as a manual-review candidate"
fi
expect_branch_kept topic-reverted "keeps branch whose local-only commits revert each other"

expect_path_exists "${REPO}/.worktrees/wt-env/.env" "keeps worktree with non-regenerable ignored file"
expect_branch_kept wt-env "keeps branch of worktree with non-regenerable ignored file"
expect_path_absent "${REPO}/.worktrees/wt-deps" "removes merged worktree with only regenerable ignored files"
expect_branch_deleted wt-deps "deletes branch of removed merged worktree"
expect_path_exists "${REPO}/.worktrees/wt-unpushed" "keeps worktree whose branch is not integrated"
expect_branch_kept wt-unpushed "keeps branch of worktree that is not integrated"

expect_path_absent "${REPO}/youtube-monitor/storybook-static" "removes regular artifact directory"
expect_path_exists "${WORK}/outside/build/index.html" "does not remove artifact reached through symlinked parent"

if ((failures > 0)); then
	echo
	echo "--- dry-run output ---" >&2
	cat "${WORK}/dry-run.log" >&2
	echo "--- apply output ---" >&2
	cat "${WORK}/apply.log" >&2
	echo "${failures} test(s) failed" >&2
	exit 1
fi
echo "all tests passed"
