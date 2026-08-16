#!/usr/bin/env bash
set -uo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/gcp/raw-chat-resource-inventory.sh <development|production> <gcp-project-id>

Read-only inventory for resources that may participate in raw live-chat archival.
The script only uses describe/get/list/search/show operations. It does not deploy,
update, delete, or change the active gcloud project.
EOF
}

if [[ $# -ne 2 ]]; then
  usage >&2
  exit 2
fi

TARGET_ENVIRONMENT="$1"
PROJECT_ID="$2"

case "$TARGET_ENVIRONMENT" in
  development|production) ;;
  *)
    echo "environment must be development or production: $TARGET_ENVIRONMENT" >&2
    exit 2
    ;;
esac

if [[ -z "$PROJECT_ID" ]]; then
  echo "gcp-project-id must not be empty" >&2
  exit 2
fi

if ! command -v gcloud >/dev/null 2>&1; then
  echo "gcloud is required" >&2
  exit 1
fi

section() {
  printf '\n\n===== %s =====\n' "$1"
}

run_read_only() {
  printf '\n+ '
  printf '%q ' "$@"
  printf '\n'
  if ! "$@"; then
    printf 'WARN: command failed; continuing read-only inventory\n' >&2
  fi
}

section "TARGET"
printf 'environment: %s\n' "$TARGET_ENVIRONMENT"
printf 'project_id: %s\n' "$PROJECT_ID"
run_read_only gcloud auth list --filter=status:ACTIVE --format='value(account)'
run_read_only gcloud config get-value project
run_read_only gcloud projects describe "$PROJECT_ID" --format=json

PROJECT_NUMBER="$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)' 2>/dev/null || true)"
if [[ -z "$PROJECT_NUMBER" ]]; then
  echo "Unable to resolve project number; Cloud Asset Inventory section will be skipped." >&2
else
  section "CLOUD ASSET INVENTORY - RAW CHAT NAME SEARCH"
  for term in live-chat-history raw-chat live-chat; do
    run_read_only gcloud asset search-all-resources \
      --scope="projects/${PROJECT_NUMBER}" \
      --query="$term" \
      --format=json
  done
fi

section "CLOUD SCHEDULER"
run_read_only gcloud scheduler jobs list --project="$PROJECT_ID" --format=json

section "CLOUD FUNCTIONS"
run_read_only gcloud functions list --project="$PROJECT_ID" --format=json

section "CLOUD RUN SERVICES"
run_read_only gcloud run services list --project="$PROJECT_ID" --format=json

section "CLOUD RUN JOBS"
run_read_only gcloud run jobs list --project="$PROJECT_ID" --format=json

section "PUB/SUB TOPICS"
run_read_only gcloud pubsub topics list --project="$PROJECT_ID" --format=json

section "PUB/SUB SUBSCRIPTIONS"
run_read_only gcloud pubsub subscriptions list --project="$PROJECT_ID" --format=json

section "EVENTARC TRIGGERS"
run_read_only gcloud eventarc triggers list --project="$PROJECT_ID" --format=json

section "SERVICE ACCOUNTS"
run_read_only gcloud iam service-accounts list --project="$PROJECT_ID" --format=json

section "PROJECT IAM POLICY"
run_read_only gcloud projects get-iam-policy "$PROJECT_ID" --format=json

section "FIRESTORE COMPOSITE INDEXES"
run_read_only gcloud firestore indexes composite list \
  --project="$PROJECT_ID" \
  --database='(default)' \
  --format=json

section "FIRESTORE SINGLE-FIELD INDEX OVERRIDES"
run_read_only gcloud firestore indexes fields list \
  --project="$PROJECT_ID" \
  --database='(default)' \
  --format=json

section "GCS BUCKETS"
run_read_only gcloud storage buckets list --project="$PROJECT_ID" --format=json

section "BIGQUERY"
if command -v bq >/dev/null 2>&1; then
  run_read_only bq --project_id="$PROJECT_ID" ls --format=prettyjson
  run_read_only bq --project_id="$PROJECT_ID" show --format=prettyjson "${PROJECT_ID}:firestore_export"
  run_read_only bq --project_id="$PROJECT_ID" show --format=prettyjson "${PROJECT_ID}:firestore_export.live-chat-history"
  run_read_only bq --project_id="$PROJECT_ID" show --format=prettyjson "${PROJECT_ID}:firestore_export.user-activity-history"
  run_read_only bq --project_id="$PROJECT_ID" show --format=prettyjson "${PROJECT_ID}:firestore_export.order-history"
else
  echo "WARN: bq is not installed; BigQuery details were not collected." >&2
fi

section "INVENTORY REVIEW"
cat <<EOF
Target environment: ${TARGET_ENVIRONMENT}
Target project:     ${PROJECT_ID}

Review the output for resources dedicated only to raw live-chat archival. Do not
classify a resource as removable merely because its name contains 'chat'. Confirm
its targets, triggers, service account permissions, and whether Study Space domain
data also depends on it.

Potential retirement candidates after the migration completes:
- BigQuery firestore_export.live-chat-history table
- raw-chat-only Scheduler / Function / Cloud Run Job / Pub/Sub / Eventarc resources
- raw-chat-only service accounts or IAM grants
- raw-chat-only Firestore indexes

Resources that must remain when shared by other data flows include:
- BigQuery firestore_export dataset
- user-activity-history and order-history tables
- mixed Firestore export GCS bucket
- shared Firestore export infrastructure
EOF
