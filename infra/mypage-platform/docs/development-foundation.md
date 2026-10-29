# Development Foundation構築

構築手順と各段階の履歴を保持する文書。最新の適用・配備状態は[統合PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)を参照し、過去のinventoryや未実施の記録を現在状態へ読み替えない。

2026-10-21 JSTの適用後no-op [run 39522369969](https://github.com/kani3camp/youtube-study-space/actions/runs/39522369969)は、IAM memberの`secret_id`がprovider refresh後に短名から完全resource nameとなるため、Plan guardで停止した（Applyなし）。独立したState readbackで3件とも同じDEV project/Secretの完全名と確認した。IAM memberの当該3 fieldだけ、固定短名または`projects/test-youtube-study-space/secrets/<固定名>`の完全一致を受理する。別project・project番号・別Secret・version path・任意prefixは拒否し、Secret metadataの短名契約、予定操作、unknown、drift、全Plan HMACを維持する。修正後の正規no-opは別の実行証跡で確認する。

MyPage統合PR [#1236](https://github.com/kani3camp/youtube-study-space/pull/1236) 内の最初の構築段階。既存 `infra/gcp` のimport-only経路とは独立し、**DevelopmentのFoundationだけ**を扱う。Cloud Run、Firestore書込み権限、public invoker、Hosting、Secret値、Productionはこの経路に含めない。

## 構築準備の記録（2026-10-20）

- AWS State専用アカウントに `youtube-study-space-mypage-dev-plan` / `youtube-study-space-mypage-dev-apply` を新規作成し、trust/inline policyのreadback一致、追加managed policyなしを確認した。既存ロールとbucket policyは変更していない。
- 両ロールの許可対象は `youtube-study-space/mypage-platform/dev/terraform.tfstate` と `.tflock`。PlanはState読取りのみ、ApplyはState読書き。State削除は禁止、ロックの作成・削除は両者に許可する。18組のIAM模擬判定で別StateへのアクセスとState削除の拒否を確認した。これはGitHub OIDCからの実AssumeRoleの証拠ではない。
- State bucketはversioning有効、AES256暗号化、TLS強制。MyPage Development prefixにはState objectを確認していない。空のState JSONやロックを手書きで投入しない。最初のTerraform ApplyでStateを作成する。
- GCP DevのSA全12件、WIF pool全4件の一覧にMyPage専用資源なし、Cloud Runサービス一覧は空をブラウザで確認した。Secret/Firebase/他環境のinventoryは未完了。GCP WIF/SAのbootstrap、GitHub Environment設定・secret投入、実Plan/Applyも未実施。実施前にその時点のinventoryを再確認する。

## Development実構築（2026-10-21 JST）

実行ソースは `df69028252c5e7f5f31e10da14bef5b619c15d8e`（#1405 / #1406統合後）。このソースの通常CI Gate・CodeQLは成功し、最新devに対してbehind 0を確認した。以下は模擬検証ではなく、Developmentへの実操作である。

- 開始前に専用SA・WIF pool・Secretが存在しないことを再確認し、Secret Manager APIを有効化した。既存GCP rootのowned APIとは分離したbootstrap管理で、同じAPIを重複してTerraformへ追加していない。
- 専用WIF pool `mypage-foundation` / provider `github-actions` と、`mypage-foundation-plan` / `mypage-foundation-apply` の2 SAを作成・readbackした。repository/owner/actorの固定ID、統合ref、caller/reusable、event、attempt 1、workflow SHA一致、Environmentと完全一致するsubjectをprovider条件に含める。`google.subject` はrepository ID・Environment・actor IDの短い組合せへmappingし、各SAのimpersonationは自分のEnvironmentだけに限定する。user-managed SA keyは0件。
- GitHub `mypage-dev-plan` / `mypage-dev-apply` は各7 secretを設定した。Plan/ApplyのAWS RoleIdとSAは別、Plan commitment用HMAC keyだけを両Environmentで共有する。統合branch 1件・tag 0件の許可を維持し、既存Terraform用のsecret/Environment/subject templateは変更していない。
- [実metadata Plan run 39452570203](https://github.com/kani3camp/youtube-study-space/actions/runs/39452570203) は専用OIDC、AWS STS、GCP WIF/SA、Terraform native plan/guardを通過し、**4 resources / 4 creates / no apply**。
- [実metadata Apply run 39452784387](https://github.com/kani3camp/youtube-study-space/actions/runs/39452784387) はPlan/Applyの独立identity、保存PlanのHMAC照合、適用後のState 4資源readbackに成功した。専用runtime SAと3 Secret metadataだけを作成し、Secret version/value・Cloud Run・Firestore writer・Hosting・Productionは作成していない。
- bootstrap operatorは作成済みruntime SAと3 SecretのIAMをreadbackし、Secret version 0件を確認してから、それぞれの資源だけに専用policy editorを付与した。Apply SAをmetadata read roleへ移し、一時的なproject create role bindingを解除・readbackした。project全域のsetIamPolicyやSecret payload/version操作はCIへ付与していない。
- 専用Stateの実objectをAWS側から独立確認し、version IDあり・AES256暗号化・metadata Apply後の`.tflock`解放を確認した。State本文・bucket/account/role ID・WIF番号・HMAC keyを公開記録へ載せていない。
- 8資源へ進む最初の[run 39453504555](https://github.com/kani3camp/youtube-study-space/actions/runs/39453504555)はPlan guardで停止し、Applyはskipした。#1407の安全な検査labelを用いた[run 39454711022](https://github.com/kani3camp/youtube-study-space/actions/runs/39454711022)で`resource-drift`と特定した。
- S3 Stateのmetadata 4資源だけを一時的なlocal Stateへ複製し、同一ソース/固定providerで**read-only Planのみ**を実行した。3 Secretの`annotations` / `labels` / `version_aliases`が`null`→`{}`へrefreshされることを、値を出さずに属性名と形だけで確認した。一時State/Planは終了時に削除し、S3 StateへのwriteやApplyは行っていない。
- #1408統合後の[run 39456968440](https://github.com/kani3camp/youtube-study-space/actions/runs/39456968440)では両identityのnative Plan guardが**8 resources / 4 creates**に成功したが、独立PlanのHMAC不一致でApplyをskipした。同一Stateを使った複数のread-only native Planを値なしで比較し、`relevant_attributes`の順序だけが変わることを再現した。

### Bootstrap権限の所有範囲

専用WIF、構築用2 SA、4 custom roleとbootstrap IAM bindingはoperator bootstrap管理で、runtimeのTerraform Stateには含めない。project IAMをauthoritative管理せず、以下のmemberだけを加減する。

| Custom role | Permission | 付与範囲 |
| --- | --- | --- |
| `mypageFoundationRead` | `resourcemanager.projects.get`、`iam.serviceAccounts.get/getIamPolicy`、`secretmanager.secrets.get/getIamPolicy` | Development projectのmetadata読取り |
| `mypageFoundationCreate` | 上記read + `iam.serviceAccounts.create`、`secretmanager.secrets.create` | 初回metadata作成時だけApply SAに一時付与 |
| `mypageFoundationSAPolicy` | `iam.serviceAccounts.getIamPolicy/setIamPolicy` | 作成済みruntime SAだけ |
| `mypageFoundationSecretPolicy` | `secretmanager.secrets.get/getIamPolicy/setIamPolicy` | 作成済みの3 Secretそれぞれだけ |

read roleはSecret payloadを読めない。資源単位のpolicy editorは当該資源の権限を変更できるため、単なるread権限とは扱わず、専用WIF・Environment・実行ソース・plan guardの制限を維持する。create roleの定義自体はbootstrap記録として残し、器の作成後はproject bindingを外す。将来の再作成は自動的に権限を復活させず、対象とownershipの再確認を必要とする。

## 実行条件

以下は初回Foundation構築時に使用した**履歴上の操作**であり、現在の通常UIでは選択しない。当時は既存 `CI` の `mypage_foundation=true` と段階別modeを使い、`foundation_metadata` の4資源から `foundation` の8資源へ段階導入した。現行の通常運用は `operation=gcp-plan|gcp-apply` と `environment` に集約し、これらのstageはbootstrap内部契約としてのみ保持する。

`integration/mypage-canon-20261006`、repository/owner ID、owner actor、run attempt 1、caller `ci.yml` と専用reusable workflowの同一SHAを検査する。PR、fork、他branch、main、再実行、古いSHAはcredential取得前またはOIDC検査で拒否する。AWS trustは同じcustom subjectへ完全一致する。repositoryの既存OIDC subject templateは変更しない。

GitHub Environment `mypage-dev-plan` / `mypage-dev-apply` を先に作り、Selected branchesはこの統合branchだけに限定する。Apply側は環境の実行制御を設定する。従来の統合branch protectionを追加しないオーナー方針を維持する。統合branchをdevへマージした後の当該branch削除と、この暫定trustの失効・移行は別の明示作業とする。現在のコードはbranchが違えば拒否する。

両Environmentに次の**専用**secretを設定する。既存GCP移行用secretを流用しない。role ID・bucket・account番号・WIF番号・HMAC keyの実値は公開repoへ置かない。

| Secret | 内容 |
| --- | --- |
| `MYPAGE_STATE_BUCKET` / `MYPAGE_STATE_ACCOUNT` | 確認済みState bucket / 所有account |
| `MYPAGE_BACKEND_ROLE_ARN` / `MYPAGE_BACKEND_ROLE_ID` | 当該Environmentの新専用PlanまたはApply roleの実ARN / immutable RoleId |
| `MYPAGE_GCP_WIF_PROVIDER` | Dev project番号の `mypage-foundation/providers/github-actions` |
| `MYPAGE_GCP_SERVICE_ACCOUNT` | `mypage-foundation-plan@test-youtube-study-space.iam.gserviceaccount.com` または `mypage-foundation-apply@…` |
| `MYPAGE_PLAN_HMAC_KEY` | 両Environmentで同一の安全な乱数key（32文字以上）。他用途へ流用しない |

GCP bootstrapでは新専用pool/provider/Plan・Apply SAを使用する。provider条件は上記repository/owner/actor/event/ref/workflowと一致させ、各SAの `roles/iam.workloadIdentityUser` は自分のEnvironment principalだけへ限定する。長期SA keyを作成しない。

GCP権限は実inventoryと公式role catalogで確認してから設定する。Planに必要なのは対象project、SA、3 Secretと各IAM policyのmetadata読取り。最初のApplyにはmetadata読取りと `iam.serviceAccounts.create` / `secretmanager.secrets.create` だけを追加し、`foundation_metadata` の4資源を作る。create権限は親project上の権限であり、名前制限をIAMだけで保証したと扱わず、exact workflow/source/Plan guardと合わせて制限する。

その後、bootstrap operatorが**作成済みruntime SAだけ**にApply SAのget/setIamPolicy、**3つのSecretだけ**にApply SAのmetadata/get/setIamPolicyをresource単位で付与・readbackしてから、full Foundationへ進む。このbootstrap管理者bindingはTerraformのruntime memberとは所有責務を分ける。Secret payloadの読取り・version追加、project IAM変更、Cloud Run操作、他SAへのimpersonationはCIへ付与しない。`iam.serviceAccounts.setIamPolicy` をproject全域で付与せず、未作成SAをresource.name条件で安全に絞れるとも仮定しない。

## Plan / Apply境界

`foundation_metadata` は専用runtime SAとSecret metadata 3件だけの4資源、`foundation` はさらにSecret accessor member 3件とruntime signer member 1件を加えた8資源。guardは選んだ段階の正確なresource集合、project、member、role、replication、stageを検証する。create/no-op以外、import/削除・置換、provider action、想定外resource、未知の認可field、条件付きIAMを拒否する。create対象名が既にGCPに存在すれば、State未所有として停止する。

refresh差分は原則拒否する。唯一、上記3 Secretの`annotations` / `labels` / `version_aliases`だけの`null`→空mapへの表現正規化を、同じ資源の予定操作が**no-op**の場合だけ許可する。before/afterのfield集合は同一、他fieldの差分なし、unknown/import/replaceなしを要求する。非空mapの追加・消去、逆方向、別資源、IAM/replication/project等の差分、Terraform writeを伴う場合は停止する。この差分をPlanから取り除かず、独立Plan比較のHMACにも含め、許可件数だけを公開ログへ出す。

Plan/Apply jobは別identityでそれぞれnative Planを保存し、全Plan・source SHA・S3 State versionのHMAC一致と30分以内を条件に、Apply job自身が保存したPlanだけを適用する。生成timestampだけを除外し、順序に意味を持たないdependency source一覧`relevant_attributes`だけは、全entryの内容・件数・重複を保持したまま整列する（[Terraform JSON format](https://developer.hashicorp.com/terraform/internals/json-format)）。配列を一律にsortせず、action sequence、attribute path、before/afterの値、drift、unknown、その他のPlan fieldは引き続き照合する。HMACはGitHub expression内で比較し、生Plan/State/targetはlog/artifactへ出さない。raw evidenceはrunnerのprivate directoryへ置き、成功・失敗の両方で削除する。

Apply後はStateに選択段階の4件または8件だけが存在することをreadbackする。GCP実物のIAM/Secret metadataも別途readbackし、その証跡を残す。これだけではRuntime起動、公開Ready、実OAuthの成立を示さない。次はSecretの安全な投入と固定version、digest固定image、App Check/Auth/Hosting inventoryを確認し、別のRuntime段階へ進む。
