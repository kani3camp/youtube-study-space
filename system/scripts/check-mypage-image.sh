#!/usr/bin/env bash
# 実際の配備イメージを、ADCやクラウド対象なしのオフラインで実行する。
set -euo pipefail

image="${1:?Usage: check-mypage-image.sh IMAGE}"
test "$(docker image inspect --format '{{.Config.User}}' "$image")" = 'nonroot:nonroot'

# APIと固定Jobは同じ不変イメージを使う。各GoバイナリはADC探索前に--helpを終了コード1で拒否する。
# 起動ファイルがない場合や実行不能の場合は、異なるコードで終了する。
for maintenance_binary in mypage-maintenance-operator; do
	maintenance_status=0
	docker run --rm --network=none --read-only --entrypoint "/app/$maintenance_binary" "$image" --help >/dev/null 2>&1 || maintenance_status=$?
	test "$maintenance_status" = 1
done

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
