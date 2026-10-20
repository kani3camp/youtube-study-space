# Terraform認証・実行境界

認証付き実行の入口は `ci.yml` の本人による初回 `workflow_dispatch` と、同SHAの
`gcp-terraform-authenticated.yml` です。運用手順は[README](../README.md#通常のdev-terraform開発)、
実際の認証・State・Plan検査はworkflowとcontrollerを正本とします。

## 共通の信頼境界

- PR/forkのCIはcredentialless。認証前にrepository/owner/actorのimmutable ID、event、ref、caller、SHAを検査する。
- Plan/ApplyごとにGitHub Environment・AWS backend role・GCP SAを分離する。
- OIDC/WIFはEnvironment・caller/reusable ref・同SHAへ制限し、実AWS account/RoleIdとGCP project/SAも照合する。
- DEV/ProductionはrootとState keyを分離する。S3 native locking、Versioning、環境別concurrencyでStateを保護し、workspaceでは環境を切り替えない。
- raw State/Plan/API応答/credentialをpublicログやartifactへ出さず、private runner filesをalways cleanupする。
- 過去の成功run、廃止済みAttestation/Ownership Receiptや履歴chainを、新しい実行の認可に使わない。

## DEVの通常Plan/Apply

`normal-plan` / `normal-apply` / `normal-e2e` はDEVの通常操作に使います。
controllerは `infra/gcp/scripts/terraform_normal_ops.py`、運用入口と設定手順は[README](../README.md#通常のdev-terraform開発)です。
リソース型・変更件数の独自allowlistはありません。provider schemaと通常変更の検証契約を使用します。

| 境界 | 実装 |
| --- | --- |
| PR/fork/actor/branch/SHA | owner immutable ID・初回本人dispatch・有効な非`main` branch ref・正確なexpected SHA・same-ref/same-SHA caller/reusableをcredentials前に検査。どのbranchを認証付き実行へ許可するかはGitHub Environmentが決定 |
| Environment/OIDC | 既存DEV Plan/Apply EnvironmentとAWS backend roleを再利用。AWSは既存7-claim subjectとネイティブactor_idの完全一致で本人に固定。通常専用GCP WIF/SAはrepository・owner・actor・Environment・workflow path・branch-shaped same-ref・same-SHAを検証するが**branch名をallowlistしない**。通常Plan/Applyを許可するbranch名はGitHub EnvironmentのSelected branchesだけで管理 |
| 入力 | 既存private baselineのtfvarsへcommit-bound `normal.tfvars.json`を重ねる。manifestは通常変更の型/件数制限に使わない。projectはDEV固定 |
| Plan | refresh付きfull-root native saved plan。Coreのcomplete/errored/checksとdriftを検査。create/updateのcomputed unknownはTerraformに委ねる |
| delete/replace | commit内`normal-operations.json`のaddress/actions完全一致が必須。未使用指定も拒否。忘却操作・importは通常経路では拒否 |
| Apply binding | 同runの独立Apply identityで同SHA/入力を再Plan。timestampと順序のない配列のみ正規化し、全Plan JSON、SHA、run ID、mode、入力、破壊的review、State本文/HEADをprivate keyのHMACで照合。GitHub Actionsのstep `if`でHMAC一致だけを評価し値をstep環境・ログに展開しない。30分の期限、未来時刻・欠落も拒否 |
| saved plan | 新しく生成した一致Planだけを適用。公開artifactへの生Plan転送はしない。Apply直前にState/保存Planを再確認し、native S3 lockとCoreのstale-plan検査を使う |
| State | DEV固定key、account制限、AES256、VersionId、lineage/serial、lock/workspace不在。State欠損時は停止し、自動初期化しない |
| 事後確認 | lineage維持・serial単調増加、refresh付きfull-root no-op、State前後照合とlock解放。エラーを無視せず停止 |
| 出力/cleanup | stdout/stderr/Plan/State/API例外はprivate runnerへ。公開は固定phase・件数・provider schemaで照合した変更リソース種別/操作別件数・プラン時刻・HMAC不一致セクション名のみ。address/for_eachキー/属性名/値は出さず、ログとjob summaryへ同じ変更一覧を表示。alwaysでprivate filesを削除。強制unlock/自動State巻戻しなし |

HMACは同runのPlan/Apply比較に限った小さな整合検査です。独自の発行サービス、durable receipt、
承認台帳、履歴chain、per-resource attestationは作りません。
Plan用SAにはリソースの読取りとmetadataだけ、Applyには限定したDEVリソース管理権限を付与します。
新規リソース型を追加しても通常実行器の改修は不要です。未知の権限領域は同bootstrapの権限審査で扱います。
公式の[S3 locking](https://developer.hashicorp.com/terraform/language/backend/s3)と
[saved-plan apply](https://developer.hashicorp.com/terraform/cli/commands/apply)を基本にします。

## PRODの通常Plan/Apply（main固定）

Productionは `normal-plan` / `normal-apply` を `refs/heads/main` と完全一致する同SHA caller/reusableだけに許可します。`normal-e2e` はPRODでは拒否します。GCP Projectは `youtube-study-space`、State keyは `youtube-study-space/prod/terraform.tfstate`、Terraform root/private inputsは `infra/gcp/environments/prod/` 専用です。DEV Root/Stateを使い回しません。

通常Planのrefresh差分は、移行経路と共通のprovider schemaでcomputed専用と確認したBigQuery tableの`etag` / `last_modified_time`だけを許容します。保存済みnative State→refresh前値→refresh後値→Planのbefore、identity・sensitivity maskを照合し、未知値・他フィールド・他リソース種別のdriftは拒否します。drift全体はHMACに含め、初回30資源・outputsのno-opとState不変の条件は維持します。検証失敗時の公開診断は固定理由コードだけです。

GitHubの `terraform-prod-plan` / `terraform-prod-apply` はmain-only Selected branches、別GCP normal WIF/SA、既存のPROD AWS backend role、同runの独立再Plan/HMAC一致・saved plan seal、明示的な破壊的操作review、適用後のfull-root no-opとnative S3 lockを要求します。source+inputs+target+State/HEAD+全planのHMACは値をstep envやshell commandへ渡さずGitHub式で比較します。既存migration controller/identityの権限は変えません。

本番用の通常Applyカスタムロールは資源単位の `*.setIamPolicy` を除外し、コード変更だけで公開grantや他identityへの権限昇格を認めません。必要なgrantは独立した権限審査を前提に追加します。

管理端末で既存PROD AWS Roleのnative owner actor_id/main trust、GitHub Environmentのmain-only branch rule、GCPの専用identity/permissionのreadbackを先に行ってください。コードの合格だけでは実PROD認証成功ではありません。実mainリリース後に本番full-root no-op Planを確認し、必要な変更について別途Applyを検証します。初回bootstrapはPlan identityだけを作り、現在のmainで既存30資源の通常full-root no-op実績をAPI/ログで検証する別操作までApply identityを作りません。AWS backendの全inline/attached policyをPROD State/lock/prefixへ完全照合し、Plan内のproject-bearing ID/SA参照もPRODへ制限します。詳細は[README](../README.md#通常のprod-terraform開発main固定)。

## 既存import専用経路

`migration-plan` / `migration-apply` は既存baselineの限定import/no-opとState観測値の同期に使います。
固定private tfvars/manifest、クラウドmetadata、full-root Planを照合し、一般のcreate/update/delete/replaceは拒否します。
通常CRUDは上記の `normal-*` 経路で行い、import-only契約を緩めて転用しません。

Applyでは同runのPlan成功後、独立identityで同SHAを再Planし、source/input/State/metadata/Planを比較します。
保存Planのseal、事後State差分・クラウドmetadata・full-root no-op・lock解放を検証します。
詳細は[既存import専用の運用契約](runtime-wif-api-ownership.md#定常planapplyへの移行2026-10-16)を参照してください。

## 実行refとEnvironmentの保護

| 経路 | 許可するref |
| --- | --- |
| DEV通常操作 | GitHub Environmentで明示選択した任意の非`main` branch。AWS/GCPはbranch名allowlistを持たず、同一refの正規workflowであることだけを検証 |
| DEV既存import | 従来の`dev` / `integration/mypage-canon-20261006`のみ。共有Environmentへの通常用branch追加ではimportの認可を広げない |
| Production既存import | `main`固定 |
| Production通常操作 | **`main`固定**。GCP通常専用WIF/SA、PROD backend role・GitHub 2 Environmentの独立保護と実Plan/Apply認証が必要 |

Environmentのbranch policyはcredentialed jobの最終認可を行います。preflightだけで許可一覧を取得・代替しません。
Environmentの許可branch・required reviewers・admin bypass等の実設定は、管理APIで実行時に確認してください。
許可branchを追加しても、そのbranchのworkflow/controllerの安全性が自動的に保証されるわけではありません。一方、AWS/GCPの恒久trustはbranch追加のたびに変更しません。`enable_normal_integration.py --apply --branch <branch>`が変更するのはDEV Plan/Apply GitHub EnvironmentのSelected branchesだけです。AWS/GCPは必要時に`--verify-aws` / `--verify-gcp`でread-only監査できます。

Selected branches方式のEnvironmentはref名を検査します。許可branch内のコード改変経路には、PRレビュー・必須CI・force-push/deletion禁止を適用します。
同SHAのcaller/reusableであることは、内容が安全にレビュー済みであることまで保証しません。
Environment reviewは実行時の確認を加える別の制御です。必要なreview方式は運用条件に合わせて設定します。
[GitHub Environment](https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments)と
[OIDC claims](https://docs.github.com/en/actions/reference/security/oidc)の仕様も参照してください。

## 設定変更時の確認

1. 対象project/account/ref/Environmentと、Plan/Applyの実identityを確認する。
2. GCP WIFのissuer・attribute mapping/condition、SA trust、project roleをbootstrap定義と照合する。WIF conditionに個別branch名を入れず、same-ref workflow条件とbranch-shaped refだけを保持する。
   user-managed keyがないこと、runtime SAに不要なproject roleがないこと、actAsの付与先が限定されていることも確認する。
   権限数やpermission一覧は `bootstrap_normal_ops.py --check` とIAMの実値から確認する。
3. AWS backend roleの対象State/lock key、PlanのState書込禁止、Applyの環境限定書込、S3のVersioning・暗号化・公開アクセス遮断を確認する。
4. GitHub Environmentの許可ref・保護設定と必要なSecret名を確認する。Secret値は読み戻せないため、認証付き実行で動作を検証する。
5. 認証付きfull-root Plan、必要なApply、事後no-opとlock解放を確認する。
   初回導入や基盤変更の検証には[単一DEV E2E](../README.md#dev基盤のe2e検証)を使う。

設定readback、既存import identityのno-op成功、新しい通常identityの実Plan/Apply成功は区別して記録します。
基盤設定やソースのマージだけで、新identityの実証が完了したとは判定しません。

## 実行記録と変更履歴

実施日時、対象SHA、IAM readback、CI・レビュー結果、Plan/Apply run、未検証事項は、対応するPRとActionsへ記録します。
恒久ドキュメントには運用方法・制約・確認手順を残し、一時点の設定値や進捗を複製しません。

- [独自証跡の撤廃（PR #1373）](https://github.com/kani3camp/youtube-study-space/pull/1373)
- [DEV通常運用の導入と検証記録（PR #1382）](https://github.com/kani3camp/youtube-study-space/pull/1382)
