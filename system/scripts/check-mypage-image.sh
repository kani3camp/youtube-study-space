#!/usr/bin/env bash
# 実際のAPI配備イメージを、ADCやクラウド対象なしのオフラインで実行する。
set -euo pipefail

image="${1:?Usage: check-mypage-image.sh API_IMAGE}"
test "$(docker image inspect --format '{{.Config.User}}' "$image")" = 'nonroot:nonroot'

# API imageはmaintenance operatorを含めない。Jobは専用imageを独立して配備する。
# 実行の可否ではなく、image filesystemにそのpathが無いことを確認する。
bash "$(dirname "$0")/check-image-path-absent.sh" "$image" app/mypage-maintenance-operator

# 設定不足は、認証情報探索や待ち受け開始より前に失敗する必要がある。
if docker run --rm --network=none --read-only "$image" --check-config; then
	echo 'MyPage image unexpectedly accepted missing configuration.' >&2
	exit 1
fi

# サーバーの合成設定仕様を再利用する。認証情報ではない。
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
