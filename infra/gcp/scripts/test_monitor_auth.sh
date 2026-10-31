#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)"
function_dir="$repo_root/infra/gcp/modules/monitor-auth-registration/function"
export GCLOUD_PROJECT=demo-monitor-auth-registration
export GOOGLE_CLOUD_PROJECT="$GCLOUD_PROJECT"
export FIREBASE_AUTH_EMULATOR_HOST=127.0.0.1:9099
cd "$function_dir"
npm ci --ignore-scripts
npm test
firebase emulators:exec --config emulator.json --project "$GCLOUD_PROJECT" --only auth \
  'node --test --test-timeout=30000 auth-integration.test.cjs'
