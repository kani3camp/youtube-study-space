#!/usr/bin/env bash
# check-image-path-absent.sh の回帰検査。実行権限のないファイルも「存在する」と判定することを、
# 検査対象imageへファイルを1つ足した使い捨てimageで確認する。cloudへは接続しない。
set -euo pipefail

image="${1:?Usage: test-check-image-path-absent.sh IMAGE ABSENT_PATH}"
path="${2:?Usage: test-check-image-path-absent.sh IMAGE ABSENT_PATH}"
script="$(dirname "$0")/check-image-path-absent.sh"
fixture="check-image-path-absent-fixture:$$"
container=''
stub=''
cleanup() {
	status=$?
	if [[ -n "$container" ]]; then
		docker rm -f "$container" >/dev/null 2>&1 || true
	fi
	docker image rm -f "$fixture" >/dev/null 2>&1 || true
	if [[ -n "$stub" ]]; then
		rm -f "$stub"
	fi
	exit "$status"
}
trap cleanup EXIT

bash "$script" "$image" "$path"

stub="$(mktemp)"
printf 'not an executable\n' >"$stub"
chmod 0644 "$stub"
container="$(docker create --network=none "$image")"
docker cp "$stub" "$container:/$path"
docker commit "$container" "$fixture" >/dev/null
if bash "$script" "$fixture" "$path" 2>/dev/null; then
	echo "check-image-path-absent.sh accepted an image that contains a non-executable /$path." >&2
	exit 1
fi
echo "check-image-path-absent.sh rejects a non-executable /$path and accepts the real image."
