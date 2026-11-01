# CodeQL advanced setup

このtreeのCodeQLはGitHubのAdvanced setupを使用し、Environmentを要求しません。
解析結果は `GITHUB_TOKEN` でアップロードします。専用Environmentの作成やOIDC token発行は不要です。
これは共有OIDC subjectに `environment` claimが含まれていても、CodeQLのためにその設定を変更する必要がない構成です。

## 3つのworkflowの役割

| workflow | 用途 |
| --- | --- |
| [`codeql-advanced.yml`](../workflows/codeql-advanced.yml) | 実行契機と同じtreeのreusable analyzer呼出しを管理するcaller。全PR、対象4ブランチへのpush、週次schedule、手動実行を受け持つ |
| [`codeql-analyzer.yml`](../workflows/codeql-analyzer.yml) | Actions・Go・JavaScript/TypeScriptをmatrixで解析し、言語別categoryで結果をアップロードするcallee |
| [`codeql-source-contracts.yml`](../workflows/codeql-source-contracts.yml) | workflow・解析範囲・権限・Go helperなどの契約を静的に検証する。CodeQL解析そのものは実行しない |

push対象は `dev`、`main`、`feature/gcp-terraform-iac`、`integration/mypage-canon-20261006` です。
scheduleはdefault branchで実行されます。PRにはbase・pathによる絞り込みを設けません。

## 維持する契約

- caller/calleeのworkflow-wide permissionsは `{}`。解析jobには `contents: read`、`security-events: write`、`id-token: none` のみを指定する。
- caller・analyzer・静的CIはEnvironmentを指定せず、inputs・secrets継承・secret/vars参照・AWS/GCP認証を持たない。
- 3言語を条件でskipしない。ActionsとJavaScript/TypeScriptはbuild mode `none`、Goは `manual` とする。
- [`analysis-config.yml`](analysis-config.yml) でdefault queriesとremote sourcesを維持する。追加query/packs、path filter、local threat modelは追加しない。
- [`build-go.sh`](build-go.sh) は全tracked `go.mod` をNUL区切りで列挙し、`GOWORK=off GOFLAGS=-mod=readonly go build -a ./...` でcompileする。Bot・tests・generatorsは実行しない。
- Actionはcommit SHAで固定する。checkoutのcredential保存、Go/dependency cache、debug/database artifactを無効にする。
- 言語別category、SARIF upload、`wait-for-processing: true` を維持し、失敗をskipや `continue-on-error` に置き換えない。

[`validate-source.py`](validate-source.py) は上記の契約、追加のCodeQL経路、重複YAML key、追加job/step/env、静的CIの必須commandとbypassを検査します。
Environmentの再追加は文字列・mapping・nullのいずれでも拒否します。
Go helper/config/parser/toolingとcaller/calleeは同じtreeでtracked・実在・非symlinkである必要があります。
helperのreviewed SHA-256は [`tooling.json`](tooling.json) に固定し、合成fixtureでroot・入れ子・空白入りmoduleのcompile-only列挙を確認します。

## Environment廃止の反映順

sourceの変更とGitHub設定の削除は別操作です。このtreeにEnvironment参照がなくても、未同期ブランチや実行中の古いworkflowには参照が残り得ます。

1. この変更をdevへ反映し、Environmentなしのpush解析と3言語の結果処理完了を確認する。dev向けPRのマージはrepositoryのAGENTS.mdに従いユーザーが行う。
2. main・Terraform・MyPageなど継続運用するブランチへ同じCodeQL変更を同期する。古いPRはbase/headを更新する。残る作業ブランチを再利用・手動実行する場合も先に同期する。
3. 実行可能なCodeQL workflowからEnvironment参照がなく、古いEnvironment付きrunが実行中・待機中でないことを確認する。停止済みの旧準備ブランチを有効化する場合は現行sourceに更新する。
4. Settings → EnvironmentsでCodeQL専用Environmentを列挙し、参照・secrets・variablesを再確認して削除する。Terraform等のEnvironmentは変更しない。
5. Environment一覧にCodeQL専用Environmentがなく、後続解析でも再作成されないことを読み戻す。

[存在しないEnvironmentのworkflow参照は暗黙作成を起こし得ます](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments)。
参照が残ったまま先に `codeql-analysis` を削除すると、再作成後には元のprotection ruleが存在しない可能性があります。
そのため、GitHub設定の削除完了とsource変更の準備完了を区別します。
共有OIDC subject、AWS/GCP trust、Terraform workflow、既存alerts/analyses、required checksはこの変更の対象外です。

## 実解析の証拠と履歴

2026-11-01、[PR #1537](https://github.com/kani3camp/youtube-study-space/pull/1537) のEnvironmentなしの同一リポジトリPRで、
[CodeQL Advanced run 41276390860](https://github.com/kani3camp/youtube-study-space/actions/runs/41276390860) が成功しました。
Actions・Go・JavaScript/TypeScriptの全jobでinit・解析・SARIF upload・GitHub側processing完了を確認しています。
Goは全tracked moduleのcompileも成功しました。source headは `98bc4b9b44ac8001e4dda25cd73a3250c19b574d`、
解析対象のPR merge snapshotは `78bee31ce903d8b1ea7f5801a0ff839163b75016` です。これはdevへのマージではありません。
詳細なjob・権限・checkout SHA・静的検証・CI結果はPRへ記録しています。
この結果はDefault setupの内部OIDC処理やpush/schedule/fork/Dependabotの実行成功を証明するものではありません。

導入時にはDefault setupの起動前に `environment` claim欠損エラーがあり、専用Environment付きAdvanced setupを採用しました。
当時の切替・実解析の記録は [PR #1362](https://github.com/kani3camp/youtube-study-space/pull/1362) と
[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191) に残っています。
その履歴を、このEnvironmentなしAdvanced setupの必須条件として扱いません。

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

actionlintは実際の3 workflowとlocal reusable referenceを検証し、workflow/actionを実行しません。
公式releaseのchecksumとtool versionはtooling.jsonに固定します。shellcheck/pyflakesは含めず、shellは `bash -n` と合成fixtureで確認します。
解析scopeの根拠は [公式configuration options](https://docs.github.com/en/code-security/reference/code-scanning/workflow-configuration-options)、
設定inputは [固定CodeQL Action source](https://github.com/github/codeql-action/tree/2892aa5e19bbd11bc0cff5427e3b750a04d9e3c2) です。
