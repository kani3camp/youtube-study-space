# CodeQL advanced setup: staged-disabled source

caller/callee を `.github/workflows` に配置した準備段階です。**両方の解析 job は `if: ${{ false }}` のままです。**
手動・push・PR・schedule、または別 caller の `workflow_call` のどの経路でも解析 job は起動しません。
実行可能な新規 CI はソース契約の静的検証だけです。これは解析成功、OIDC 修復、Environment 作成、default setup の切替を示しません。

受領 source は PR #1272 を取り込んだ `fedecbf621c7021c64f94c3e8ca604f073c529ff` です。
このソース契約は dev/main/Terraform/MyPage の各 tree で同じ両 false gate を維持する同期案です。
親作業では secretless `codeql-analysis` の明示作成と指定 5 branch policies の検証が完了したと報告されています。
本ローカル同期準備では設定 API の再取得・変更、remote push、PR 作成、merge、解析、default setup 切替を行いません。
各 branch への実反映と後日の有効化は別承認です。

## 2026-10-09 の履歴 snapshot（現値ではない）

2026-10-09 19:04 UTC、既存の認証済み `gh api --method GET` で取得しました。permission error はありませんでした。
CodeQL の既知の失敗原因（repository OIDC subject が要求する `environment` を managed default job が持たない）は再調査していません。

| 項目 | API の実値 |
| --- | --- |
| default setup | `state: configured` |
| languages（返値をそのまま記録） | `actions`, `go`, `javascript`, `javascript-typescript`, `typescript` |
| query suite / threat model | `default` / `remote` |
| runner | `runner_type: standard`, `runner_label: ""` |
| schedule / updated_at | `weekly` / `2026-07-23T07:21:59Z`。実行時刻は未提供 |
| `codeql-analysis` | 全 10 Environment を case-insensitive に比較して absent |
| 対象 Environment の protection / branch policies / secrets / variables | absent のため N/A。secret/variable listing は行っていない |
| OIDC `include_claim_keys`（順序保持） | `repository_id`, `repository_owner_id`, `environment`, `ref`, `workflow_ref`, `job_workflow_ref`, `event_name` |
| OIDC その他 | `use_default: false`, `use_immutable_subject: false`, `sub_claim_prefix: repo:kani3camp/youtube-study-space` |

使用した repository 相対 GET routes は `/code-scanning/default-setup`、`/environments?per_page=100`（pagination）、
`/actions/oidc/customization/sub` のみです。無関係な Environment の secrets は取得していません。
この absent 記録は Environment 作成前の履歴です。現行の専用 Environment は親作業で作成・検証済みと報告されています。
activation 直前に read-only で現値と承認済み設定を照合します。この履歴は有効化の許可ではありません。

## 無効状態のソース契約

| 項目 | 現在の契約 |
| --- | --- |
| caller → callee | [`codeql-advanced.yml`](../workflows/codeql-advanced.yml) → same-tree local [`codeql-analyzer.yml`](../workflows/codeql-analyzer.yml) |
| caller gate | literal `if: ${{ false }}`。手動入力・vars・secrets による解除経路なし |
| callee gate | 同じ literal false を matrix job 自体に指定。別 caller から呼ばれても全解析が無効 |
| callee Environment | literal `codeql-analysis`。job がスキップされるので runner/deployment/Environment 作成経路は到達不能 |
| workflow-wide permissions | `{}`。予定の job permissions は `contents: read`, `security-events: write`, `id-token: none` のみ。スキップ中は解析 token を発行しない |
| secrets / variables / cloud | input・secret 継承・secret/vars 参照・AWS/GCP auth なし。既存 dev Environment を使わない |
| runner | standard GitHub-hosted `ubuntu-latest`。custom runner label なし |
| languages / build mode | `actions: none`, `go: manual`, `javascript-typescript: none`。JS と TS は同じ canonical language で両方を解析する案 |
| analysis scope | [`analysis-config.yml`](analysis-config.yml) で default queries と remote sources を固定。追加 queries/packs、query/path filters、local model なし |
| Go | [`build-go.sh`](build-go.sh) は全 tracked `go.mod` を NUL 区切りで列挙し `GOWORK=off GOFLAGS=-mod=readonly go build -a ./...` のみ。Bot/tests/generators は実行しない |
| checkout/cache/upload | checkout credential 保存なし、Go/dependency cache 無効、debug/database artifact 無効、language category と processing wait を維持 |
| 静的 CI | [`codeql-source-contracts.yml`](../workflows/codeql-source-contracts.yml) は Environment・解析権限・解析実行を持たない |

[GitHub の job 条件](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#jobsjob_idif) は matrix 展開前に評価されます。
step 単位の skip だけには依存しません。[Environment 参照は暗黙作成を起こし得る](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments)
ため、実行可能な Environment job は現在の契約で拒否します。
`id-token: none` は cloud 用 token を要求する権限を与えません。GitHub/CodeQL 内部の claim 検証が解消するかは未検証です。

validator は templates を読みません。実際の workflow 全体、両 gate、全解析 job、予定権限、解析範囲、別経路の CodeQL 呼出し、
静的 CI の全必須 commands と bypass を検査します。重複 YAML key、追加 job/step/env、CI の `if`/`continue-on-error` も拒否します。
全 workflow・CodeQL source・routing scripts・全 `go.mod`/`go.sum` の変更が静的検証に入り、
静的 CI の push 対象は 4 対象 branch と専用 CodeQL integration を含み、CodeQL source の変更も通常 CI の全群に分類します。

Go helper/config/parser/tooling と caller/callee は同じ tree で tracked・実在・非 symlink である必要があります。
helper の reviewed SHA-256 は [`tooling.json`](tooling.json) に固定し、合成 fixture の command recorder で root/入れ子/空白入り module の compile-only 列挙を確認します。
現在の `system/go.mod` と `tools/room-image-prompt/go.mod` の全 require は同じ tree の `go.sum` に対応し、canonical `system/go.mod` の version が全 module を覆うことも確認します。
この検査は Go compiler や application/service を起動しません。CGO/build tags/generated source/CodeQL toolchain 対応と実ビルド結果は未検証です。

## 後日の有効化 packet（この PR の実行範囲外）

### 1. 直前の再確認と対象 tree の準備

上記 GET snapshot、default/protected refs、rulesets・required contexts/app IDs、対象 PR の head/merge tree を read-only で再確認します。
403/unavailable はそのまま報告し、別 account、credential 抽出、permission 拡張をしません。
専用 `codeql-analysis` の protection と secret/variable **名前・件数のみ**を read-only 照合します。
absent、secret/variable が非 0、または承認済み 5 branch policies と不一致なら停止し、作成・修正は別承認とします。
runner debug logging と query scope、全 tracked Go modules、Go/CodeQL supported toolchain も確認します。
OIDC 7 claims と順序は保持し、AWS/GCP trust、IAM/WIF、long-lived keys への変更は 0 とします。

| 将来の coverage | trigger 案 / Environment branch policy 案 | 必要な別作業 |
| --- | --- | --- |
| default `dev` | push `dev`、weekly schedule、dispatch / `dev` | owner による専用 integration → dev 反映 |
| protected `main` | push `main` / `main` | 別の exact-diff PR で同じ helper/config/dependencies を同期 |
| protected Terraform integration | push `feature/gcp-terraform-iac` / 同名 branch | 担当者と調整した独立 PR。既存 Terraform workflow は変更しない |
| MyPage integration | push `integration/mypage-canon-20261006` / 同名 branch | 担当者の独立 PR |
| PR（全 base/fork/Dependabot） | `pull_request.branches: ["**"]`、paths filter なし / `refs/pull/*/merge` | 各 merge tree の同一 tree 整合と check/upload 実互換性の検証 |

この trigger 案は workflow に既に記述されていますが、両 job gate は false です。
schedule は default branch でのみ動きます。古い PR は base 更新だけで run が保証されないため、別承認の synchronize/dispatch を計画します。
2026-10-07 の過去の ruleset 観測（Terraform の `CI Gate` / `GCP Terraform Validate`、dev/main に CodeQL gate なし）は現在値の保証ではありません。
activation 直前に required contexts/app IDs を照合し、緑 alias、required-check 削除、strict 無効化で置き換えません。

### 2. 作成済み専用 Environment の照合（以下は作成時の承認案）

| 設定 | Before snapshot | After proposal |
| --- | --- | --- |
| `codeql-analysis` | absent | 専用 Environment を 1 個明示作成 |
| secrets / variables | N/A | 0 / 0。継承なし |
| reviewers / wait timer / custom protection | N/A | none / 0 / none |
| admin bypass | N/A | false |
| deployment_branch_policy | N/A | `protected_branches: false`, `custom_branch_policies: true` |
| selected policies（type branch、tag 0） | N/A | `dev`, `main`, `feature/gcp-terraform-iac`, `integration/mypage-canon-20261006`, `refs/pull/*/merge` |
| OIDC / cloud trust / credentials | 現設定 | 変更 0 |

作成済み Environment は read-only で承認案と照合します。新規作成・修正は行いません。
Environment が存在しても両 false gate は変えず、自動作成には依存しません。

### 3. default → advanced 切替と source 有効化の別承認

1. 承認対象の exact base/head と **caller + callee 両方の false gate を変更する差分**、validator/negative tests を active 用契約へ変更する差分、各 branch の反映順をまとめて承認します。静的 CI 自体は維持します。
2. default setup の `configured → not-configured` を別承認の owner 操作で変更し read-back します。default enabled 中は advanced SARIF upload が拒否されるため、切替の gap と security/release hold を計画します。
3. 承認対象 tree の gate を有効にし、承認済み trigger で実行します。root permissions `{}` と予定 job の bounded permissions、default/remote/standard を維持します。
4. 全 3 言語の runner/init/Go compile/analyze/SARIF processing、head/merge SHA/ref/category、nested check context/app ID、tool status を確認します。実行成功を確認するまで修復完了とは扱いません。
5. fork/Dependabot の権限や Environment policy による pending/failure/skip も検証し、既存 security gate と release hold を維持します。JWT/raw SARIF/database/debug data を汎用 artifact に出しません。

snapshot の language aliases は API 返値として保持しています。設定復元 payload はその時点の supported enum を確認し、
Actions/Go/JS/TS の範囲を欠かさない exact payload を承認します。query_suite を extended にしたり threat_model を local に拡大したりしません。
実 check/category と過去 analysis/alerts の対応は read-back で照合し、既存 analysis/alerts を削除・一括 dismiss しません。

### 4. 戻し方

この同期案のローカル差分は破棄できます。後日反映する場合は各対象 branch の source-only 同期 commit を記録し、その差分だけを revert します。
同期案は security 設定を変更しないため、同期の取り消しに設定 rollback は不要です。専用 Environment は削除しません。

将来の有効化後は、別承認した範囲で caller/callee 両 gate と staged-disabled validator を復元し、in-flight advanced runs を停止します。
保存した default 設定を [supported default-setup API](https://docs.github.com/en/rest/code-scanning/code-scanning#update-a-code-scanning-default-setup-configuration) / owner UI で復元し read-back します。
default 再設定は managed validation run を起動し得るため、この実行も rollback 承認に含めます。
rollback は既知の default OIDC failure に戻る可能性があり、解析成功を保証しません。security/release hold は維持します。
専用 Environment の削除は無参照・secret/variable 0 の確認後に別の明示承認がある場合のみです。

## 静的検証

```sh
python3 -m pip install --only-binary=:all: -r .github/codeql/requirements.txt
python3 .github/codeql/validate-source.py
python3 -m unittest discover -s .github/codeql -p 'test_*.py' -v
python3 .github/codeql/lint-workflows.py --actionlint /path/to/verified/actionlint
bash -n .github/codeql/build-go.sh .github/codeql/install-actionlint.sh
bash .github/scripts/test-detect-ci-paths.sh
python3 .github/scripts/check-doc-references.py
```

actionlint は実際の 3 workflow と local reusable reference を検証し、workflow/action を実行しません。
両 gate を先に検証した上で、意図的な constant-false の exact diagnostic だけを除外します。shellcheck/pyflakes は含めません。
公式 release の checksum と tool version は tooling.json に固定し、shell は `bash -n` と合成 fixture で確認します。
解析 scope の根拠は [公式 configuration options](https://docs.github.com/en/code-security/reference/code-scanning/workflow-configuration-options)、
default queries の扱いと `config-file` input は [固定 CodeQL action source](https://github.com/github/codeql-action/tree/2892aa5e19bbd11bc0cff5427e3b750a04d9e3c2) です。
