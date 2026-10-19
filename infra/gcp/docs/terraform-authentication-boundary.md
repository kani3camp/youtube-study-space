# Terraform認証境界と独自証跡の撤廃

この文書はPR #1373の調査・変更契約です。現在のコード・GitHub設定を正本とし、過去の成功runを新しい実行の承認根拠にしません。

## 調査結果

`ci.yml` の `workflow_dispatch` → 同SHAの `gcp-terraform-authenticated.yml` が入口です。
旧 `plan` と現行 `migration-plan` は別の実行経路でした。名前だけで未使用とは判定していません。

| 機構 | 変更前の実呼出し・gate | 通常運用の依存と撤廃判断 |
| --- | --- | --- |
| `terraform_current_state_attestation.py` | Dev `plan` + `history_post_noop=true` + current-state selector。emitter=true、独立no-op gate=true | 旧12件baselineを再発行する経路は実行可能だった。Dev31/Prod30件の現行controllerは利用しないため発行・検証を撤廃 |
| `terraform_ownership_receipt.py` | 旧history importとruntime applyの発行、private binding、GitHub run/job/Environment/artifact照合。emitter/publication=true、旧apply/runtime gate=false | 過去の移行chain専用。通常の現在State/plan検査と分離して撤廃 |
| `terraform_history_post_noop_receipt.py` | Dev旧 `plan` の独立12件no-op。`refs/heads/dev`固定で選択統合branchを認証後に拒否 | 入口ごと撤廃。既存controllerのfull-root post-planを維持し、ブランチ互換処理は追加しない |
| `runtime_ownership_provenance.py` | runtime wave指定 + root-ready/runtime gates。現在false。認証前とplan後に証跡chainを再認証 | 現行migration経路から呼ばれない。未来用の再有効化を残さない |
| `runtime_ownership_packet.py` | 旧runtime gate配下のprivate staging、v2/v3 chain、root basis、期限・split入力 | 証跡入力を撤廃。native import fidelityに必要な純粋な選択・graph/value検査は別のoffline責務として維持 |
| `.github/terraform/*sources.json` | current-state issuer 2件、runtime issuer 0件。旧consumerの固定issuer許可 | 通常の認可に不要。期限切れ履歴を再承認するcatalogは撤廃 |
| 専用artifact fixture workflows | PRでdummy証跡を発行しartifact transportを検証 | 独自証跡廃止に伴い撤廃。過去のActions artifact/runは削除しない |
| 現行migration controller/contract | Dev/Prodの `migration-plan` / `migration-apply` | 履歴証跡を利用せず、現在のState・metadata・planを検査する。継続 |

旧経路を削除すると、旧mode/selector/packetでのdispatch、独自証跡発行とconsumer、履歴API再認証は使えなくなります。
通常運用のmanaged入力schemaとStateは変換しません。
`terraform_migration.py` / `terraform_migration_contract.py` / `production_initializer.py`と現行Plan/Applyジョブ本体は保持し、現行permission catalogも同一です。
`runtime_plan_selection.py`と`validate_runtime_ownership_plan.py`はnative/offline検査専用です。旧`terraform_protected_plan.py`も共有fixtureのadapterであり、認証付きworkflowの入口ではありません。
GCPの実Pool属性`attestation_rules`が空であることの検査は、独自Attestation証跡とは別のクラウド設定検査なので維持します。旧Secretを読み出したり削除したりせず、workflowのSecret interfaceから参照を外します。
過去の独自証跡やissuerの失効日を延長する互換処理、SLSA/Sigstoreへの置換は追加しません。

## 維持する契約

- public PRはcredentialless。owner名/immutable ID、repo/owner ID、初回dispatch、target/ref/caller/SHAを認証前にfail-closedで検査する。
- DevはEnvironmentで選択されたbranch、Prodは`main`のみ。plan/applyそれぞれのEnvironment、AWS role/GCP SA、root/State keyを分離する。
- 実OIDC claimsのEnvironment・caller/reusable ref・同SHAと、実AWS account/RoleId、GCP project/SA、許可/禁止権限を検査する。
- 現在のS3 Stateはbounded HEAD→version固定GET→HEAD、lineage/serial、bucket controls/versioning、native `.tflock`、workspace不在を検査する。環境別concurrencyで協調writerを直列化する。
- 固定private tfvars/manifestからfull-root planを検査し、予期しないcreate/update/delete/replace、unknown、graph欠落やidentity変更を拒否する。既存import/no-op経路を一般の変更用へ拡大しない。
- applyは同runのplan成功後、別identityで同SHAを再planし、source/input/State/metadata/plan全体のHMACと保存plan sealを照合する。これらはjob内・同runの整合性検査であり、履歴承認証跡ではない。
- post-applyのState全差分・クラウドmetadata・full-root no-op・lock解放を確認する。raw State/plan/API/credentialを公開せず、private runner filesをalways cleanupする。

詳細は[定常運用契約](runtime-wif-api-ownership.md#定常planapplyへの移行2026-10-16)と実装を参照してください。

## GitHub実設定のread-only確認（2026-10-18 JST）

GitHub管理APIで確認した値です。AWS IAM/GCP WIF/SA IAMは今回readbackしておらず、過去の本人設定報告から実値を推定しません。

| Environment | Selected branches（type=branch） | required reviewers |
| --- | --- | --- |
| terraform-dev-plan | dev、integration/mypage-canon-20261006 | なし |
| terraform-dev-apply | dev、integration/mypage-canon-20261006 | なし |
| terraform-prod-plan | main | なし |
| terraform-prod-apply | main | なし |

4 Environmentは `custom_branch_policies=true` / `protected_branches=false` です。
統合branchは `protected=false`、観測HEADは `52b171dd518b0d4176c6fa1dfb2026451e66b30b` でした。
同HEADの認証workflowもAPIで取得し、preflightが旧`feature/gcp-terraform-iac`完全固定であることを確認しました。Environmentへの追加だけでは統合branchの現在のworkflowは実行できません。
これはその時点のread-only観測であり、将来の設定や実認証成功を証明しません。

## 統合branchの保護は何を防ぐか

Selected branches方式では、branch protectionはGitHub/OIDCがtokenを発行する技術上の必須条件ではありません。
Environmentのbranch制限は許可branch名を検査しますが、そのbranch内のworkflowがreview済みかまでは保証しません。
required reviewersを設定すればjob開始前の確認が加わります。現在の4 Environmentにはその承認待ちはありません。
[GitHub Environmentの公式仕様](https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments)を参照してください。

想定する攻撃経路は、許可branchにwriteできる主体がworkflow/scriptを改変し、ownerにdispatchさせることです。
改変workflowはpreflightやState/plan検査を削除でき、branch/ref/workflowパスを固定するOIDC条件にも合致し得ます。
`workflow_sha == job_workflow_sha` は同じcommitであることを保証しても、その内容の安全性は保証しません。
AWSのbranch wildcardを含むsubjectも独立したwildcard同士のbranch一致を証明しません。
[OIDC claimsの公式仕様](https://docs.github.com/en/actions/reference/security/oidc)を参照してください。

個人運用での必要十分な提案は次のとおりです。今回設定は変更しません。

1. 通常はreview済み`dev`で運用し、統合branchを追加する期間と目的を限定する。使い終わったbranchはPlan/Applyそれぞれのallowlistから外す。
2. 許可するbranchには最新workflowとcontrollerを同期し、必須CI、PR経由の変更、force-push/deletion禁止を適用する。共同作業者の変更にはreviewを要求する。個人運用で常時独立reviewerを要求すると運用不能になる場合があるため、機械的に増やさない。
3. 未保護branchを一時利用するなら、許可するSHA/workflow差分を本人が確認する運用と、必要に応じたApply Environmentの本人reviewを選ぶ。長期間の無承認Apply許可は避ける。Plan identityにもStateアクセスや短期credentialがあるため、Plan側も無条件に安全とは扱わない。
4. Dev/ProdとPlan/Applyの分離、厳密なrepo/owner/Environment/caller/reusable/event/actor/attempt条件を維持する。旧branchのtrustが意図せず残っていないかAWS/GCP/SA IAMをreadbackしてから切替を判定する。

branch protectionはコード改変経路を制限し、Environment reviewは実行時の確認を加える別の制御です。
owner自体の侵害や管理者による意図した設定変更を、これらだけで防げるとは主張しません。

## DEVの通常Plan/Apply

`normal-plan` / `normal-apply` / `normal-e2e` を追加します。既存migration jobs/controller/contractは保持します。
新controllerは `infra/gcp/scripts/terraform_normal_ops.py`、運用入口と設定手順は[README](../README.md#通常のdev-terraform開発)です。
Cloud Run限定offline prototypeは認証付き経路から呼びません。リソース型・変更件数のallowlistはありません。

| 境界 | 実装 |
| --- | --- |
| PR/fork/actor/branch/SHA | owner immutable ID・初回本人dispatch・`dev`固定・正確なexpected SHA・same-SHA caller/reusableをcredentials前に検査 |
| Environment/OIDC | 既存DEV Plan/Apply EnvironmentとAWS backend roleを再利用。新規のDEV固定WIF poolと通常専用GCP SAを分離。旧providerと同poolを共有すると旧integration claimsから通常SAへ到達し得るため、poolも分離 |
| 入力 | 既存private baselineのtfvarsへcommit-bound `normal.tfvars.json`を重ねる。manifestは通常変更の型/件数制限に使わない。projectはDEV固定 |
| Plan | refresh付きfull-root native saved plan。Coreのcomplete/errored/checksとdriftを検査。create/updateのcomputed unknownはTerraformに委ねる |
| delete/replace | commit内`normal-operations.json`のaddress/actions完全一致が必須。未使用指定も拒否。忘却操作・importは通常経路では拒否 |
| Apply binding | 同runの独立Apply identityで同SHA/入力を再Plan。timestampだけを除く全Plan JSON、SHA、run ID、mode、入力、破壊的review、State本文/HEADをprivate keyのHMACで照合。30分の期限、未来時刻・欠落も拒否 |
| saved plan | 新しく生成した一致Planだけを適用。公開artifactへの生Plan転送はしない。Apply直前にState/保存Planを再確認し、native S3 lockとCoreのstale-plan検査を使う |
| State | DEV固定key、account制限、AES256、VersionId、lineage/serial、lock/workspace不在。State欠損時は停止し、自動初期化しない |
| 事後確認 | lineage維持・serial単調増加、refresh付きfull-root no-op、State前後照合とlock解放。エラーを無視せず停止 |
| 出力/cleanup | stdout/stderr/Plan/State/API例外はprivate runnerへ。公開は固定phaseと件数、HMAC/timeだけ。alwaysでprivate filesを削除。強制unlock/自動State巻戻しなし |

HMACは同runのPlan/Apply比較に限った小さな整合検査です。独自の発行サービス、durable receipt、
承認台帳、履歴chain、per-resource attestationは作りません。
Plan用SAにはリソースの読取りとmetadataだけ、Applyには限定したDEVリソース管理権限を準備します。
新規リソース型を追加しても通常実行器の改修は不要です。未知の権限領域は同bootstrapの権限審査で扱います。
公式の[S3 locking](https://developer.hashicorp.com/terraform/language/backend/s3)と
[saved-plan apply](https://developer.hashicorp.com/terraform/cli/commands/apply)を基本にします。

### 実設定・検証状況（2026-10-19 JST）

- GitHubの最新dev `98fc225c` / main `68dfb342` と指定Issue/PRを直接確認。
- DEV2 Environmentのselected branchesはdev/既存MyPage統合branch。通常経路は独立preflightと新WIFでdevだけに限定する設計。
- AWS State管理アカウントへの接続、DEV backend roleのState/lock分離、Versioning、AES256、全Public Access Blockを実readback。
- GCP既存login、DEV project、旧WIFのimmutable ID/actor/event/ref/same-SHA条件、旧SAのread-only権限を確認。
- 当初、自動承認レビューはState本文のAWSコネクタ取得と、通常用IAM/WIF/Environment設定を拒否した。
  State本文はコネクタで取得せず、HEADだけを確認した。
- 2026-10-19、本人の具体的な再承認後、通常用IAM/WIF/Environment設定が成功。
  独立readbackでPlan88／Apply176権限、通常SA2個の正確なproject roleとWIF trust、SA3個のuser-managed key0、
  WIF ACTIVE・dev固定条件/mapping/issuer完全一致、runtimeのproject role0・通常ApplyだけのactAs、旧SA project role不変を確認。
  両DEV EnvironmentのSA/provider参照は同日08:46–08:47 JSTに更新済み。
  Secretの値は読み戻せないため名前と更新時刻を確認し、新identityの認証付きPlan/Apply実証とは区別する。
- PR #1382への`@codex review`投稿は、再承認後に成功。レビュー結果はPRへ記録する。
- 正規dev `98fc225c` の[実run39144216181](https://github.com/kani3camp/youtube-study-space/actions/runs/39144216181)は認証・Plan・cleanup成功。managed31 / no-op31 / import0 / mutation0 / raw drift0。新通常identityでの実証ではない。
- 最終PRのdevマージ前は通常経路を起動できない。本人マージ後の単一`normal-e2e`で
  既存no-op、create/update/replace/delete、事後no-op、lock解放をまとめて検証する。
- Productionの通常操作、MyPage provisioning、#1165/#1166/#983は今回の実行対象外。
