#!/usr/bin/env bash

set -euo pipefail

readonly project_id="demo-youtube-study-space-rules"
readonly script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly repo_root="$(cd -- "$script_dir/../.." && pwd)"

if ! command -v firebase >/dev/null 2>&1; then
	echo "firebase CLI is required; install firebase-tools@15.25.1 first" >&2
	exit 1
fi

cd "$repo_root"

echo "Running Firestore Security Rules tests for project $project_id"
firebase emulators:exec \
	--config firebase/firebase.json \
	--project "$project_id" \
	--only firestore \
	--log-verbosity INFO \
	"pnpm --dir firebase test:rules"
