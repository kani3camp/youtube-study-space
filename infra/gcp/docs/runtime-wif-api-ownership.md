# GCP Terraform 定常Plan/Applyとruntime WIF / API ownership

> この文書は `migration-plan` / `migration-apply` の既存import専用経路を扱います。通常CRUDは[README](../README.md#通常のdev-terraform開発)、共通の保護は[認証・実行境界](terraform-authentication-boundary.md)を参照してください。

## 定常Plan/Applyへの移行（2026-10-16）

Current Canon §10と本人の最新指示により、4 EnvironmentはRequired reviewersなし・本人GitHub ID `54093651` / `kani3camp` の初回 `workflow_dispatch`だけで運用する。旧節の毎回の人間承認、失われたhistory receipt、新issuer/catalogの期限、source activation windowは履歴・旧protocolであり、新しい `migration-plan` / `migration-apply` の実行条件には使用しない。`historical_delta_attested=false`は維持する。

新経路は既存 `ci.yml` → 同一SHAのreusable workflowと同じ4 Environment/OIDC/WIF/plan・apply別identityを使用する。`dev`への直接push・merge・auto-mergeは禁止のまま。Productionの `MIGRATION_PRODUCTION_READY` は、Development Exitと実backend/CI foundationの独立review後にtrueとなった。独自証跡を使う旧入口は撤廃し、現行のmigration経路だけを使う。ProductionのState/ownership完了は[現在の実行境界](production-readiness.md#current-execution-boundary--2026-10-17)で別に確認する。

- 各Environmentの `GCP_TERRAFORM_MANAGED_INPUTS_JSON` に、review済み固定tfvars、address/type/import IDの完全manifest、比較専用HMAC鍵を保存する。同じtype/import IDを複数addressへ採用する重複ownershipは拒否する。平文と圧縮wrapperのどちらもSecretのwireは1行JSONとする。複数行では構造文字がrunner maskへ登録されjob出力を抑止するため、parsed入力・全比較条件を変えずに1行へ保存する。これはdesired baselineであり、実行ごとにlive metadataへ追従して書き換えない。旧Secret/receipt binding/nonceは読み出し・置換しない。
- 固定入力がGitHub Secretの[48KB上限](https://docs.github.com/en/actions/reference/security/secrets#limits-for-secrets)を超える場合、`terraform_migration.encode_input_transport` でreview済み元JSON bytesをlosslessに圧縮する。wireはUTF-8で48KiB以下・改行なしのJSONとし、exact keysは `schema_version: "gcp-migration-inputs-packed-v1"`、`codec: "zlib+base64"`、`payload`。canonical Base64の単一zlib streamだけを受理し、復元4MiB上限、EOF、末尾・連結streamなし、JSON重複key/nonfiniteなし、元4field schemaを認証前に検査する。入れ子wrapper・未知key/codec・切れたstream・展開bombはSTOPする。平文JSONもwire上限内で引き続き受理する。圧縮は秘匿化ではないため既存private経路に留め、元candidate fileは変更しない。plan/apply両Environmentへ同じwireを保存し、HMACと全Terraform検証には復元した元4fieldだけを使う。
- 選択した `owned_api_keys` はmanifest内の `google_project_service` のservice集合と完全一致させ、分類を `Own` / `ENABLED`・理由ありとする。選択APIの `dependency_addresses` は重複のない非空listとし、すべて同じmanifest内の非API resourceへ解決することを認証前に検証する。未採用consumer、API自身・API間循環を依存根拠にせず、後段のfull-root planとState照合で実際のownershipを確認する。未選択APIの将来consumerはこの入力検証では拒否しない。
- 本人初回・固定repo/ref/callerを認証前に検査。実OIDC claimsとAWS account/RoleId、GCP SAを照合し、同じEnvironmentの短期認証だけを使う。
- plan/apply両identityについて、project GETとproject-scope mutation権限なし・他環境への権限なしをfresh `testIamPermissions` で確認する。Runtime SAのpolicy/token/actAsとRuntime Poolのpolicy変更権限はresource-scopeでも確認し、project probeだけで条件付き権限まで否定したと扱わない。
- fresh metadata、4MiB以下のHEAD→GET→HEAD同一State、lineage/serial、native lockとworkspace不在、source closureとprovider lockを確認する。通常full-root planで完全graph・既存全件no-op・候補1件以下のexact import・unknown/move/replace/予期しないaction0を必須にする。
- BigQuery Tableの `etag` / `last_modified_time` はprovider schemaのcomputed-only性を実検査した場合だけ、**設定変更と区別した観測差分**として許す。raw drift件数は公開summaryに保持する。他field・sensitivity・未知差分を無視しない。通常Terraformにもresource変更を提案させない。
- `migration-plan` はStateを書かない。`migration-apply` は同runの独立plan job成功後、apply Environmentで同SHAを再planする。source、固定private入力、全State/HEAD、安定metadata、private plan values/masks/import IDs/config/outputs/checksをHMACで完全比較し、保存binary sealと直前再検証後に一度だけ通常saved-plan applyする。
- import/計算観測のState永続化後は、保存planとの全State差分、lineage/serial/version、live metadata不変、lock解放と別の通常full-root no-opを検証する。未知のapply結果では再送・force unlock・古いStateの上書きを行わない。S3 Versioningを調査・限定復旧の根拠として保持する。
- raw State/plan/API/error/credentialsはowned0700/0600のrunner一時領域に閉じ、always cleanupする。公開artifactは作らず、出力は固定stage、件数、purpose-bound HMACのみ。履歴証跡を現在の実行成功へ読み替えない。

Core/providerが返す `before_identity` / `after_identity` は独立したprovider identity schemaで型・required属性・versionを検査し、resource属性・planned/prior/raw State・保存plan後の全Stateと結合する。未知keyや識別子の変更/削除は拒否する。native回帰は実Core/providerとloopback GET専用ダミーAPIを使い、実認証・実Cloud・共有Stateなしでimportとno-op、計算観測保存、sensitivity-only update拒否を検証する。

Google 8.5の既存API importは、設定が `false` でも `disable_on_destroy` / `disable_dependent_services` をJSON keyありの `null` としてStateへ保存する。値はpinned schemaのoptional boolに対する `null` / bool `false` だけを受理し、選択APIの各manifest instanceを通常planのstatic configurationへ結合して、review済みlocal API module・両expressionのexact `constant_value: false` を必須にする。欠落、dynamic参照、`true`、数値、未知source/treeはSTOPする。APIを含むStateDeltaにはこのconfigurationを持つ保存planを必須とし、null/falseを値・mask・identity・re-plan比較で同一視しない。実Core/provider＋GET専用loopback APIの回帰がimport、保存planのlocal apply、native null保存、通常post no-opを検証する。実Cloud/credential/backendやAPI consumerの実使用はこの回帰の対象外。

失敗時は固定action名・既知field名の件数だけを診断し、address/import ID/値/未知key名を公開しない。before観測が存在する境界だけを一度再取得し、Stateと安定metadataを `SAME` / `CHANGED` / `UNOBSERVED` で報告する。取得失敗を不変の証明にせず、apply試行済みかも明記する。結果にかかわらず元の失敗で停止し、再applyしない。

再plan完全比較の原因分類には、source/closure/inputs/state/metadata/planの固定6区分と鍵確認を、別domainのkeyed HMACでjob間へ渡す。詳細もplanの既知top-level field、metadataの既知resource type、Stateの固定HEAD項目/other/raw/parsedだけを別domainで比較する。公開診断は不一致区分・固定項目名と鍵一致boolのみ。unkeyed private hashや値は出さず、全6区分・詳細と従来のexecution全体HMACの一致をすべて必須にする。区分を除外したり、比較失敗をapply許可へ変えたりしない。

設定担当はfull-rootで候補を1件ずつ追加し、完全manifestとbaselineをplan/apply両Environmentへ同一設定する。各adoption前に実値とrequired callers/dependenciesをreviewし、採用後の固定baselineに対して通常planでdriftを検知する。State writerは既存reusable workflowの環境別concurrencyで直列化し、operatorは別writerを走らせない。非協調writer不存在を完全に証明したとは扱わず、前後State/Versionとnative lock/stale-plan検査で異常時停止する。

この節は運用契約であり、コード/CI準備だけでは移行完了を主張しない。実行結果はIssue #1191・最終PR #1251とCurrent Canon/Inventoryへ記録する。


## 起動と運用上の判定

Devの実行branchはPlan/Apply Environmentの選択branch制限で認可し、Prodは`main`固定です。AWS/GCP/SA IAMの実設定との整合はreadbackが必要です。

認証設定と対象branchのreview済みsource/CIを確認し、`ci.yml` を本人accountから初回dispatchします。Productionではrefを `main`、targetを `prod` にします。

```sh
gh workflow run ci.yml --ref dev \
  -f terraform_authenticated=true -f terraform_target=dev -f terraform_mode=migration-plan
```

この経路のapplyは、既存ownershipのimport/no-op・許容されたState同期に限ります。
適用時は `terraform_mode=migration-apply` を選ぶ。同じrun内の独立plan成功後、apply identityが同SHA・同baselineで再planし、全HMAC一致を確認して一度だけsaved applyする。
過去のplan runをそのまま適用したり、失敗runをrerunしたりしない。未知結果ではState/metadataを読み取りで照合し、元のUNKNOWN記録を保持する。
`migration-plan`もnative `.tflock`の一時作成・削除を行うが、State本体を書かない。State versionの調査・復旧はoperatorの別scopeで、CIへversion削除権限を追加しない。

各Environmentはbackend role ARN/RoleId・State bucket/account、GCP WIF provider・専用SAの6参照と固定managed入力の計7種だけを使う。
認証はOIDC/WIFの短期credentialで、参照値やHMAC鍵を長期credentialと扱わない。値、mailbox、IAM本文を公開しない。
初期State例外はProductionの正規不在・manifest0・固定outputs2件だけで、既存Stateや通常resource createへ拡張しない。

実証の判定は[ReadyGate](phase2-approval-and-readiness.md#development--production--mypage-readygate)へ集約します。MyPage provisioning/releaseはHOLDで、後続PRにtype/schema・create/update専用workflow/検証契約・exact IAM/API・費用/rollbackが必要です。現在の13type・exact import ID・resource no-op契約へcreate/update権限を追加するだけでは実行できません。

## 移行記録

旧current-state発行、Ownership Receipt chain、private packet、issuer closure、期限付きwindowは現行の実行条件ではありません。
過去の判断・失敗・検証結果は[整理前の固定Source](https://github.com/kani3camp/youtube-study-space/blob/781a1a98d038793f6eba4b02f640210a9473dffd/infra/gcp/docs/runtime-wif-api-ownership.md)と[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)から参照できます。
履歴のツールや期限切れ入力を現在の実行承認で再利用しません。
