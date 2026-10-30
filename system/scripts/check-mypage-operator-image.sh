#!/usr/bin/env bash
# 実際のmaintenance operator配備イメージを、ADCやクラウド対象なしのオフラインで実行する。
set -euo pipefail

image="${1:?Usage: check-mypage-operator-image.sh OPERATOR_IMAGE}"
test "$(docker image inspect --format '{{.Config.User}}' "$image")" = 'nonroot:nonroot'
test "$(docker image inspect --format '{{json .Config.Entrypoint}}' "$image")" = '["/app/mypage-maintenance-operator"]'

# operatorは引数検査をADC探索より前に行い、--helpや引数なしを終了コード1で拒否する。
# 起動ファイルがない場合や実行不能の場合は、Dockerが126/127で終了する。
for arguments in --help ''; do
	status=0
	# shellcheck disable=SC2086 # 空文字は「引数なし」を表す。
	docker run --rm --network=none --read-only "$image" $arguments >/dev/null 2>&1 || status=$?
	test "$status" = 1
done

# operator imageはAPI serverを含めない（実行の可否ではなくpathの不存在で確認する）。
bash "$(dirname "$0")/check-image-path-absent.sh" "$image" app/mypage-server
echo 'MyPage maintenance operator image: nonroot, operator-only, rejects missing arguments offline.'
