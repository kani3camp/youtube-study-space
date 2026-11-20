#!/usr/bin/env bash
# imageのfilesystemに指定pathが存在しないことを確認する。コンテナのprocessは起動しない。
# 実行権限のない・壊れたファイルも「存在する」として拒否する。
set -euo pipefail

image="${1:?Usage: check-image-path-absent.sh IMAGE PATH...}"
shift
test "$#" -gt 0

container=''
listing=''
cleanup() {
	status=$?
	if [[ -n "$container" ]]; then
		docker rm -f "$container" >/dev/null 2>&1 || true
	fi
	if [[ -n "$listing" ]]; then
		rm -f "$listing"
	fi
	exit "$status"
}
trap cleanup EXIT

listing="$(mktemp)"
container="$(docker create --network=none "$image")"
docker export "$container" | tar -t >"$listing"
# 一覧を取得できたことを、必ず存在するpathで確認する（空の一覧で素通りしない）。
grep -Fxq -e app/ -e ./app/ -e app "$listing"
for path in "$@"; do
	if grep -Fxq -e "$path" -e "./$path" -e "$path/" -e "./$path/" "$listing"; then
		echo "Image $image unexpectedly contains /$path." >&2
		exit 1
	fi
done
