#!/usr/bin/env bash
# Execute the actual deployment image offline, without ADC or a cloud target.
set -euo pipefail

image="${1:?Usage: check-mypage-image.sh IMAGE}"
test "$(docker image inspect --format '{{.Config.User}}' "$image")" = 'nonroot:nonroot'

# The same immutable image is used by the API, three fixed Jobs and the fixed
# readback service. Each Go binary rejects --help with exit code 1 before any
# ADC lookup; a missing/non-executable entrypoint exits differently.
for maintenance_binary in mypage-maintenance-operator mypage-maintenance-breakglass mypage-maintenance-bootstrap-check mypage-maintenance-inspect; do
	maintenance_status=0
	docker run --rm --network=none --read-only --entrypoint "/app/$maintenance_binary" "$image" --help >/dev/null 2>&1 || maintenance_status=$?
	test "$maintenance_status" = 1
done

# Missing configuration must fail before any credential discovery or listen.
if docker run --rm --network=none --read-only "$image" --check-config; then
	echo 'MyPage image unexpectedly accepted missing configuration.' >&2
	exit 1
fi

# Reuse the server's synthetic configuration contract. These are not credentials.
result="$(docker run --rm --network=none --read-only \
	-e MYPAGE_ENVIRONMENT=development -e MYPAGE_REGION=asia-southeast2 \
	-e GOOGLE_CLOUD_PROJECT=demo-mypage -e MYPAGE_PROJECT_NUMBER=123456789 \
	-e MYPAGE_WEB_APP_ID=1:123456789:web:synthetic \
	-e MYPAGE_PUBLIC_ORIGIN=https://example.invalid \
	-e MYPAGE_SIGNER_EMAIL=mypage-signer@demo-mypage.iam.gserviceaccount.com \
	-e MYPAGE_OAUTH_CLIENT_ID=synthetic-client \
	-e MYPAGE_OAUTH_CLIENT_SECRET=synthetic-secret \
	-e MYPAGE_PRIVACY_VERSION=synthetic-p -e MYPAGE_TERMS_VERSION=synthetic-t \
	"$image" --check-config)"
python3 - "$result" <<'PY'
import json
import sys

assert json.loads(sys.argv[1]) == {
    "schemaVersion": 1,
    "mode": "check-config",
    "offline": True,
    "configuration": "valid",
    "startupReadinessDeclaration": "pending",
    "infrastructureReadiness": "pending",
    "liveE2E": "pending",
}
print("MyPage deployment image: nonroot, offline configuration accepted; live readiness remains pending.")
PY
