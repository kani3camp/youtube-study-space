# Codex の専用 identity による dev metadata 取得

[`collect_codex_runtime_metadata.py`](../scripts/collect_codex_runtime_metadata.py) は、
元の AWS IAM user の環境変数の長期鍵から、private 正本で確認した専用 dev WIF と
metadata SA を使う任意の read-only collector です。既存の operator 用 CLI は変更しません。
この source の追加では実認証・Cloud API・権限追加を実行していません。
実行に必要な専用 SA の全 read 権限も、この source や CI の dummy test からは証明できません。

デフォルトは offline です。private 入力・source・依存・出力先だけを検証し、
repository companion / SDK import、AWS 環境変数の取得、認証、network、capture 作成を行いません。
`--execute` とレビュー済み private manifest の `execute_approved=true` の両方が必要です。
このフラグは権限変更・protected dispatch・migration の承認にはなりません。

## private 入力と pins

入力 JSON は checkout 外の owner 所有 0700 directory に置く regular 0600 file です。
symlink、hardlink、重複 JSON key、未知 key、placeholder hash を拒否します。
actual manifest の作成と最終 source/hash/window の確定は主担当が行います。
identity の値を public source、Issue、ログへコピーしません。

CLI は次の path と manifest hash を受け取ります。実値を含む実行コマンドは別途の private handoff です。

| 引数 | 契約 |
| --- | --- |
| `--manifest-file` / `--manifest-sha256` | `private-codex-dev-runtime-metadata-read` schema 1、全 exact keys は source の `MANIFEST_KEYS` |
| `--auth-selector-file` | canonical 専用 identity selector。source page/section・dev project number・`codex-cloud/aws-state-reader`・native metadata SA を exact hash で固定 |
| `--runtime-selector-file` | 既存 collector の `SELECTOR_KEYS`。固定 dev project、runtime Pool/Provider、appspot SA、source-match 入力を固定 |
| `--dependency-selector-file` | isolated install の absolute root・全 file tree hash・Python/platform を固定する schema 1 |
| `--output` | 同じ安全条件の directory 内にある未作成の absolute file。排他的 0600 作成 |

manifest は exact source commit、collector 2 本、依存 lock、3 selector の SHA-256、
元 IAM user の Account/Arn/UserId、発行・失効 UTC を固定します。
source は HEAD と tracked file の一致も検査します。既存 collector と validator 2 本は、
検証済み HEAD bytes だけを実行時に読み込み、未検証の import / ambient module / bytecode cache を使いません。
read window は最大 15 分、実行時間は最大 10 分です。
各 request の前後と保存前後に window と private 入力 hash を再検査し、
保存前には source と dependency tree も再検査します。失敗時は collector が予約した出力を削除します。
出力先が置換された場合は成功扱いせず、その別 file を消しません。

## 認証と取得範囲

実行時だけ `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` を各一回取得します。
AKIA 長期鍵に限定し、session token、profile、role/web identity、container credentials の指定を拒否します。
Assume 後の session、AWS profile/credential file、IMDS、ADC、gcloud、token cache は使いません。
元 IAM user の Account/Arn/UserId を固定 region の STS `GetCallerIdentity` 一回で照合し、
3 点一致後だけ Google STS と exact metadata SA の `generateAccessToken` へ進みます。
公式 `AwsSecurityCredentialsSupplier` と `RequestSigner` を用い、鍵・token はメモリ内だけです。
runtime workload SA や Terraform CI identity を認証主体として選ぶことはできません。

HTTP は標準ライブラリの HTTPS transport です。host/path/method は固定 allowlist、
redirect、再試行、別認証経路への fallback を許可しません。
HTTP error body は読み取りも表示も保存もしません。例外・URI・identity・鍵・token・argv を public 出力しません。
stdout は固定 alias の結果・mode・complete・migration-ready の 4 行だけです。

取得は [`collect_runtime_metadata.collect`](../scripts/collect_runtime_metadata.py) の callback を再利用します。
fresh project ID/number/ACTIVE 一致を最初に確認し、Pool、Provider、空 attestation、
SA policy v3（空 body POST）、全 enabled services page、history table metadata を検証します。
サービスの重複・pagination 不完了、未知の trust shape、canonical history 8 fields 不一致は STOP です。
IAM 変更、State、Terraform、business rows、artifact upload、receipt/nonce 発行はありません。

出力は `private-codex-runtime-metadata-capture` schema 1 です。
全 metadata、source/manifest pins、観測開始終了、明示 request 数と source identity 照合結果を保存します。
operator capture、CI receipt、full7 join、caller scope/ownership の証明への自動昇格は行いません。
`ci_identity_read_verified` / `caller_scope_verified` / `root_admission_verified` /
`migration_ready` は常に false です。AWS の typed historical projection を fresh 証拠へ変更しません。

## optional SDK の検証

依存の正本は [`codex-runtime-metadata.txt`](../requirements/codex-runtime-metadata.txt) です。
CPython 3.12 / Linux x86_64 の別 directory へ、wheel の hash を必須として install します。
既存 gcloud vendor の path を恒久的に固定したり、別 version へ fallback したりしません。

```sh
python3 -m pip install --no-cache-dir --no-compile --no-deps --require-hashes --only-binary=:all: \
  --target "${CODEX_METADATA_TEST_DEPENDENCIES:?absolute public dependency directory required}" -r infra/gcp/requirements/codex-runtime-metadata.txt
python3 -B infra/gcp/tests/test_collect_codex_runtime_metadata.py
```

`CODEX_METADATA_TEST_DEPENDENCIES` は public package だけを置く任意の absolute directory です。
private selector は install 後の exact tree hash を固定し、SDK/dependency の ambient import を拒否します。
CI はこの依存を isolated install し、実 SDK import・公式 signer/supplier を含む dummy-only test を実行します。
実鍵、実認証、Cloud API は使いません。
