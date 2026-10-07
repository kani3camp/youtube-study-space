# CodeQL advanced setup: source preparation

この変更は実行されない設定案と静的 CI の準備です。解析成功、OIDC 修復、Environment 作成、default setup 切替を示すものではありません。
caller/callee は `templates/*.yml.template` に置き、Actions の workflow 探索対象から外しています。caller の `if: ${{ false }}` も維持します。
通常 CI、Terraform workflow、AWS/GCP trust、MyPage のソースは変更しません。

## 観測した失敗と未検証の仮説

[run 37618974902](https://github.com/kani3camp/youtube-study-space/actions/runs/37618974902)
は `dynamic/github-code-scanning/codeql` の managed default setup です。Actions / Go / JavaScript-TypeScript の全 job が
`environment` claim の欠落で失敗し、runner steps は空でした。前段のプラットフォーム処理で停止したと考えられます。
通常 `ci.yml` の変更でこの別 workflow が修復したとは判断できません。

annotation が列挙した required claims は次の 7 個です。設定 API の現値を再取得した証拠ではありません。

```text
repository_id, repository_owner_id, environment, ref,
workflow_ref, job_workflow_ref, event_name
```

[公式 OIDC reference](https://docs.github.com/en/actions/reference/security/oidc) は Environment claim と reusable job の
`job_workflow_ref` を説明しています。本案は fixed Environment を callee job に付け、通常の reusable workflow 構造を作ります。
`id-token: none` は cloud 用 OIDC を要求する権限を与えません。GitHub/CodeQL 内部の token 処理が無くなる、同じエラーが必ず解消する、とは断定できません。
内部処理のどこで claim が検証されるか、advanced job が runner 開始・init・analyze・upload まで到達するかは、別承認後の実行で確認します。
repo subject template の 7 claims を削除・順序変更せず、AWS/GCP trust に CodeQL の Environment/ref/audience を追加しません。

## ソース案の契約

| 項目 | 案 |
| --- | --- |
| caller → callee | `.github/workflows/codeql-advanced.yml` → local `codeql-analyzer.yml`（将来の配置先） |
| callee Environment | literal `codeql-analysis`。入力なし、`secrets: inherit` なし |
| caller/callee permissions | `contents: read`, `security-events: write`, `id-token: none`。他の権限は付与しない |
| languages | `actions: none`, `go: manual`, `javascript-typescript: none` |
| Go | `system/go.mod` の toolchain。全 tracked `go.mod` を `go build -a ./...`。Bot、tests、generate は起動しない |
| checkout/cache | checkout credentials を保存しない。Go/dependency cache を無効化し、fresh compile を抽出する |
| upload | 言語別 category `/language:<language>`。処理完了を待つ。`upload-database: false`, `debug: false` を明示 |
| 準備段階の実行 | 新しい `CodeQL Source Contracts` は parser / unit contract / actionlint / shell syntax / CI routing のみ。Environment と解析権限を持たない |

manual build は `init` と `analyze` の間に置き、root と入れ子の module を NUL 区切りで列挙します。
現 dev の `system` と `tools/room-image-prompt`、将来追加される tracked module を対象にします。
ビルド設定・生成ファイル・CGO・build tags による対象外ファイル、Go toolchain と CodeQL bundle の互換性は実解析前に再確認します。
fixture テストは偽の command recorder を使い、実 Go コンパイラや外部サービスを実行しません。
[compiled-language docs](https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/manage-your-configuration/codeql-for-compiled-languages)
に従い Go の `none` は採用しません。

## Repository 全体の coverage と checks

| ref / event | 将来の trigger / Environment branch policy | 有効化前に必要な準備 |
| --- | --- | --- |
| default `dev` | push `dev`、weekly schedule、dispatch / branch `dev` | owner が active caller/callee/helper を同じ tree に配置。dev merge は owner のみ |
| protected `main` | push `main` / branch `main` | main の workflow/helper を独立 PR で同期 |
| protected `feature/gcp-terraform-iac` | push exact branch / 同名 branch | infra 担当と調整した独立 PR で同期。既存 Terraform workflow は変更しない |
| MyPage integration | push `integration/mypage-canon-20261006` / 同名 branch | MyPage 担当が独立 PR で同期。本準備 PR はその枝に触れない |
| PR（全 base / slice / forks / Dependabot） | `pull_request.branches: ["**"]`、paths filter なし / branch `refs/pull/*/merge` | PR の merge tree に caller/callee/helper が揃う。fork/Dependabot の実 check/upload は承認後に検証 |

push は表の 4 枝を対象とし、その他の作業枝は PR で coverage を得ます。schedule は default branch でのみ動きます。
branch 名、default、保護対象が変わったら activation 前に trigger / Environment policy / この表 / static contract を一緒に見直します。
古い open PR は base に workflow を足しただけで新しい run が保証されません。対象 head と merge tree を一覧にし、別承認の synchronize/dispatch を計画します。
CI routing の更新で MyPage の共通 CI と競合する場合は担当に調整を依頼します。

2026-10-07 の read-only ruleset snapshot では Terraform integration の strict required contexts は `CI Gate` と `GCP Terraform Validate` です。
default `dev` と `main` の ruleset には required status-check / CodeQL rules はありませんでした。直前に再取得し、snapshot と異なれば停止します。
本案の reusable check 名は managed setup の `Analyze (go)` 等と同じとは限りません。実際の nested context、app ID、category、ref/SHA、
tool status を read-back して既存 gate と照合します。緑色の代替 alias、required-check 削除、strict 無効化は行いません。
`CodeQL Source Contracts` 成功を security analysis 成功の代わりにしません。ruleset 変更が必要なら別 packet / 別承認です。

## 将来の approval packet（現在は未承認）

対象は public `kani3camp/youtube-study-space` の GitHub security 設定です。cloud IAM/SA/credentials の作成・変更は含みません。
source PR は専用 stack `slice/codeql-advanced-source-prep-20261007 → feature/codeql-advanced-setup → dev` に置きます。
複数 wave のため最新 dev から独立 integration を作り、Terraform integration に混ぜません。各 wave の exact base/head/diff を承認時に提示します。

### 1. 直前の read-only packet

承認済みの正規 connector/owner 操作で次を取得します。admin read が unavailable / 403 なら owner に取得を依頼し、token 抜き出しや経路迂回はしません。

- `GET /repos/kani3camp/youtube-study-space/code-scanning/default-setup` の state / languages / query_suite / threat_model / runner_type / runner_label / schedule と取得時刻。未提供 field は推測で補わない。
- default/protected refs、ruleset と required contexts/app IDs、対象 PR の head/merge tree、最近の言語別 analysis/category/tool status。全言語成功の baseline が無ければそう記録する。
- `codeql-analysis` が case-insensitive に存在しないこと。既存なら設定を上書きせず衝突として停止。Environment policy / secret・variable の**名前と件数だけ**を owner が確認する。解析 run の debug logging が有効になっていないことも確認する。
- repository OIDC 7 claims と AWS/GCP trust の unchanged を owner の既存 read 権限で照合。JWT、credential、secret value、非公開設定の原文はログ/PR/artifact に出さない。

### 2. Environment 作成の action-time 承認

source を全対象 tree に gate false のまま配置する PR は、branch ごとの exact diff をレビューします。
その際、この準備 validator の「active workflow 不在」契約を staged-disabled 用の契約へ置き換える専用 diff が必要です。static CI を単に無効化しません。
まだ false caller からは runner/Environment/解析を起動しません。
[GitHub は job の参照だけで新しい Environment を暗黙作成し得る](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments)
ため、自動作成に依存せず次の exact settings diff を別承認します。

| 設定 | Before | After proposal |
| --- | --- | --- |
| `codeql-analysis` | owner が absent を確認 | 専用 Environment を 1 個作成 |
| secrets / variables | absent | 0 / 0。継承なし、新 credential なし |
| required reviewers / wait timer / custom protection rules | absent | none / 0 / none（通常 PR 解析を待ち状態にしない） |
| admin bypass | absent | false |
| deployment_branch_policy | absent | `protected_branches: false`, `custom_branch_policies: true` |
| selected branch policies（type `branch` のみ） | absent | `dev`, `main`, `feature/gcp-terraform-iac`, `integration/mypage-canon-20261006`, `refs/pull/*/merge`。tag policy 0 |
| IAM / WIF / OIDC template / long-lived keys | 現設定 | 変更 0 |

[Environment policies](https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments)
は workflow の ref に一致させます。`refs/pull/*/merge` を別 rule として指定し、protected-branches-only で PR を塞ぎません。
作成後の read-back が一致しない場合、解析は始めません。

### 3. default → advanced 切替と実解析の action-time 承認

[公式 advanced setup](https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/configure-code-scanning/configuring-advanced-setup-for-code-scanning)
は default を無効化してから切り替えます。
[default setup は advanced CodeQL upload をブロックする](https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/configure-code-scanning/configure-code-scanning)
ため、default を enabled のまま成功検証する手順や無停止の overlap は約束しません。

1. 全対象 tree の disabled caller/callee/helper と Environment read-back、保存済み rollback snapshot、対象 head/merge tree、実行上限を owner が確認する。MyPage/infra 担当の同期完了が前提。
2. bounded maintenance window を承認する。repository default setup の state を `configured → not-configured` とする（現 state の owner 確認が前提）。他の security settings を変更しない。
3. 各枝の exact source activation diff（caller の false gate 解除と activation-stage contract）を owner が承認・反映する。dev merge は owner のみ。default/protected/MyPage 各 push/head と対象 PR merge tree の実行を同じ承認範囲で指定する。
4. 各 run で全 3 言語の runner開始 / init / Go manual compile / analyze / SARIF processing を確認する。run URL、head/merge SHA、ref、category、check context/app ID、upload結果、tool status のみを記録する。JWT/debug database/実データ/raw SARIF を汎用 artifact に保存しない。
5. fork/Dependabot の permissions/Environment policy により失敗・pending・解析 skip があれば全体完了にしない。CI と解析 coverage が揃うまで release/ReadyGate を開けない。default 解除から coverage 確認までの gap と終了時刻を記録する。

Go build の成否、実行時間、CodeQL supported toolchain、upload/category の既存 analysis との対応はこの wave の未検証項目です。
既存 categories を削除したり alert を一括 dismiss したりしません。切替後の alerts/history を読み、差分が必要なら別レビューを行います。

### 4. rollback

未作成・未有効化の現在は source PR を閉じるだけで security 設定への rollback は不要です。
将来の rollback も設定変更なので activation packet に owner の実行権限と exact target を含めます。

1. advanced caller を全対象 tree で disabled に戻し、承認範囲の in-flight advanced runs を停止する。外部 cloud trust は変更しない。
2. 保存した default 設定を [default-setup API](https://docs.github.com/en/rest/code-scanning/code-scanning#update-a-code-scanning-default-setup-configuration) / owner UI で復元し read-back する。languages/query_suite/threat_model/runner fields は snapshot の supported fields に限り復元する。再設定は managed validation run を起動し得るので rollback 承認にも含める。
3. rollback は**以前の既知の OIDC failure 状態へ戻る可能性**があり、解析を緑にする保証ではない。security gate と release hold を維持する。
4. 今回作成した Environment は無参照・secret/variable 0 を確認し、削除は別の明示承認があるときだけ行う。既存 analysis/alerts は保持する。

## 静的検証と出典

```sh
python3 -m pip install --only-binary=:all: -r .github/codeql/requirements.txt
python3 .github/codeql/validate-source.py
python3 -m unittest discover -s .github/codeql -p 'test_*.py' -v
python3 .github/codeql/lint-templates.py --actionlint /path/to/verified/actionlint
bash -n .github/codeql/build-go.sh .github/codeql/install-actionlint.sh
bash .github/scripts/test-detect-ci-paths.sh
```

tool version / official release archive checksum と参照した starter commit は [`tooling.json`](tooling.json) に固定します。
caller/callee の action は commit SHA pin です。source contract は無効 gate / active analyzer 不在 / bounded permissions / fixed Environment / 全言語 / 全 PR routing を検査します。
actionlint は disposable directory に template を写して local reusable reference も検証し、action を実行しません。
意図的な constant-false gate の exact diagnostic だけを除外します。shellcheck/pyflakes は含めず、shell は `bash -n` と合成 fixture で確認します。
[公式 starter](https://github.com/actions/starter-workflows/blob/e3c451d60f119b71caebf13c98ac45da6e15b4b7/code-scanning/codeql.yml)
の最新版を確認した上で、public/secretless、reusable、複数 Go module、gate false を本 repo の契約として加えています。
