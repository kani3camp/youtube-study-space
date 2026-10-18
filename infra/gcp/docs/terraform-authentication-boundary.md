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

## 通常変更（create/update）への段階移行案（第1段階：offline-only実装）

2026-10-18：`infra/gcp/scripts/terraform_normal_change_contract.py` と純粋なnegative-firstテストを追加しました。これらは **認証情報なし・クラウド呼出しなし・State操作なし** の専用プロトタイプです。既存`migration-*` 経路へは接続せず、実行許可を増やしません。

現時点の検証器は、Terraform 1.16.4の`terraform show -json`形式、明示的にレビューした最大3件の`google_cloud_run_v2_service`作成・更新と、それ以外の既存no-opを照合します。確認できないdelete/replace/move/import、想定外のcreate/update、data read、drift、未承認output変更、未知のupdate値や不完全planは拒否します。公開出力は固定コードと件数のみで、値・address・Plan JSONを表示しません。

初期review manifestの形（**以下はsynthetic例であり、実際の実行承認ではない**）：

```json
{
  "schema_version": "gcp-normal-change-review-v1",
  "environment": "dev",
  "changes": [
    {
      "address": "module.mypage.google_cloud_run_v2_service.api",
      "type": "google_cloud_run_v2_service",
      "action": "create"
    }
  ]
}
```

検証は`python3 -B infra/gcp/tests/test_terraform_normal_change_contract.py` で実行し、`ci.yml`のcredentialless GCP Terraform Validateにも登録しています。実Plan JSONとmanifestを外部ファイルに置いた場合は`python3 infra/gcp/scripts/terraform_normal_change_contract.py --plan <private-plan.json> --review <private-review.json>`で読み取りだけ行います。生Planとmanifestには実ID・秘密値が含まれ得るためpublic artifactへ公開しません。

**未実装の安全条件**：review manifestとGitHubの承認済みSHAの結合、HCL/metadata/Stateと全resource graphの厳密検証、create/updateの値・変更属性・未知値のreview、Dev専用のcredentialed Plan/ApplyとGCP最小IAM、アカウント分離・復旧・費用評価。このoffline検証成功だけでクラウド実行は許可しません。最終的なPlan/Apply提供には、これらの条件を別レビューで実装する必要があります。

### 次の実装境界

現在のGitHub Actionsジョブ表示名は `Terraform Existing Resources Plan/Apply` とします。旧表示名の `Import-only migration ...` は利用者に古い移行工程と誤解させるため更新しますが、**`migration-plan` / `migration-apply` は当面、認証済みコントローラーの内部モードとして残す**方針です。表示変更のみで動作対象やGCP IAMが拡張されたと解釈しません。

既存の `terraform_migration_contract.py` は全resourceの既存no-opと限定import、観測値だけのState同期を検証する専用契約です。ここから `create/update` 禁止条件を削って一般用に転用すると、検証済みの安全境界を壊すため行いません。以下は次期実装の受入条件であり、現時点では**実行不可**です。

1. **設計とPR**：MyPage等で必要なresource type、Terraform root／module、依存関係、dev/prodの差分、create/update許容対象、想定費用、rollbackを[MyPage基盤 #864](https://github.com/kani3camp/youtube-study-space/issues/864)と[Terraform移行 #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)に照らして別PRでレビューする。既存31/30 resourceを無関係な更新に巻き込まない。
2. **独立実行経路**：通常変更用のPlanとApplyの入口・実行契約を新設し、当初はDevの非認証offline fixture／credentialless Terraform validateのみ通す。現在のimport専用ルート、`migration-*` selectors、State・Secretsを変更しない。切替を示す明示的なdispatch inputとdefault-off gateを設け、PR/forkへのcredential配布を禁止する。
3. **認証と権限**：本人の初回dispatch、同一SHA caller/reusable、trusted branchとEnvironment制限、AWS backend Plan/Apply別role、GCP WIF/Service Account別identityを保持する。create/updateに必要なGCP IAM permissionは**管理するresource typeと個別操作を列挙して最小化**し、既存のread-only/import-only identityへ一括で権限を追加しない。最初は非破壊的な小さなresourceに限定する。
4. **差分とApplyの保護**：Planでcreate/updateとresource address・属性変化・依存graphをレビューし、初期契約でdelete/replace・IAM policy全体の上書き・秘密値のログ出力・未承認resource typeを拒否する。PlanとApplyは独立Environmentと権限、同一review済みcommit・inputs・State/metadata・planの整合を確認する。Apply直前の再plan、S3 versioning/native lock、失敗時の復旧とpost-apply再検証を必須とする。
5. **実証と段階拡張**：credentialless negative/contract/native試験、CodeQL/CIを通し、**明示的な本人承認後に限り**GitHub Environment／AWS IAM／GCP WIF／SA IAMの実値readbackと限定Dev認証Plan、必要時のApply実証を行う。本番はDevの実証後に別途通常リリースと最小権限審査で開放する。
6. **最終的な名称統一**：一般用Plan/Applyの実行成功と旧consumerの参照廃止が確認できてから、内部`migration-plan` / `migration-apply` を退役・統一する。既存import専用契約を運用中に無条件で緩和しない。

この段階では表示名と設計文書の変更だけで、Terraform HCL、内部job ID、mode値、OIDC claims、IAM／Environment／State、workflow実行は変更しません。

## リリース前の残件

本人によるdev向けPRの最終review/merge、通常のdev→main昇格、実際のIAM/WIF/SA IAM/Environmentとの整合readbackが必要です。
認証付きのnegative/read-only Plan、必要なApplyとState/no-op検証は、別途許可された運用作業として実施します。
このPRではcredentialed dispatch/Plan/Apply、State/クラウド/GitHub保護設定の変更を行いません。
MyPage create/updateの実行契約と最小権限、recovery #1165、durable audit #1166、privacy #983は引き続き別課題です。
