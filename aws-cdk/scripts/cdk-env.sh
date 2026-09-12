#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: bash scripts/cdk-env.sh <dev|prod> <diff|deploy>" >&2
  exit 2
}

ENVIRONMENT="${1:-}"
ACTION="${2:-}"

case "$ENVIRONMENT" in
  dev)
    PROFILE="soraride-dev"
    EXPECTED_AWS_ACCOUNT="657533259235"
    GCP_PROJECT_ID="test-youtube-study-space"
    GCP_PROJECT_NUMBER="48101442817"
    GCP_SERVICE_ACCOUNT="test-youtube-study-space@appspot.gserviceaccount.com"
    ;;
  prod)
    PROFILE="soraride-prod"
    EXPECTED_AWS_ACCOUNT="652333062396"
    GCP_PROJECT_ID="youtube-study-space"
    GCP_PROJECT_NUMBER="906336399194"
    GCP_SERVICE_ACCOUNT="youtube-study-space@appspot.gserviceaccount.com"
    ;;
  *)
    usage
    ;;
esac

case "$ACTION" in
  diff|deploy) ;;
  *) usage ;;
esac

WIF_AUDIENCE="//iam.googleapis.com/projects/${GCP_PROJECT_NUMBER}/locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider"

ACTUAL_AWS_ACCOUNT=$(aws sts get-caller-identity \
  --profile "$PROFILE" \
  --query Account \
  --output text)

if [[ "$ACTUAL_AWS_ACCOUNT" != "$EXPECTED_AWS_ACCOUNT" ]]; then
  echo "Refusing to continue: profile ${PROFILE} resolved to AWS account ${ACTUAL_AWS_ACCOUNT}, expected ${EXPECTED_AWS_ACCOUNT}." >&2
  exit 1
fi

CDK_ARGS=(
  AwsCdkStack
  --profile "$PROFILE"
  --parameters "AwsCdkStack:GoogleCloudProject=${GCP_PROJECT_ID}"
  --parameters "AwsCdkStack:GcpWifAudience=${WIF_AUDIENCE}"
  --parameters "AwsCdkStack:GcpWifServiceAccountEmail=${GCP_SERVICE_ACCOUNT}"
)

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
cd "$SCRIPT_DIR/.."

if [[ "$ACTION" == "diff" ]]; then
  corepack pnpm cdk:diff "${CDK_ARGS[@]}"
else
  corepack pnpm cdk:deploy "${CDK_ARGS[@]}" --require-approval never
fi
