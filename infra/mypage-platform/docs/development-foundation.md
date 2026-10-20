# Development Foundation構築

MyPage統合PR [#1236](https://github.com/kani3camp/youtube-study-space/pull/1236) 内の最初の構築段階。既存 `infra/gcp` のimport-only経路とは独立し、**DevelopmentのFoundationだけ**を扱う。Cloud Run、Firestore書込み権限、public invoker、Hosting、Secret値、Productionはこの経路に含めない。

## 実施記録（2026-10-20）

- AWS State専用アカウントに `youtube-study-space-mypage-dev-plan` / `youtube-study-space-mypage-dev-apply` を新規作成し、trust/inline policyのreadback一致、追加managed policyなしを確認した。既存ロールとbucket policyは変更していない。
- 両ロールの許可対象は `youtube-study-space/mypage-platform/dev/terraform.tfstate` と `.tflock`。PlanはState読取りのみ、ApplyはState読書き。State削除は禁止、ロックの作成・削除は両者に許可する。18組のIAM模擬判定で別StateへのアクセスとState削除の拒否を確認した。これはGitHub OIDCからの実AssumeRoleの証拠ではない。
- State bucketはversioning有効、AES256暗号化、TLS強制。MyPage Development prefixにはState objectを確認していない。空のState JSONやロックを手書きで投入しない。最初のTerraform ApplyでStateを作成する。
- GCP DevのSA全12件、WIF pool全4件の一覧にMyPage専用資源なし、Cloud Runサービス一覧は空をブラウザで確認した。Secret/Firebase/他環境のinventoryは未完了。GCP WIF/SAのbootstrap、GitHub Environment設定・secret投入、実Plan/Applyも未実施。実施前にその時点のinventoryを再確認する。

## 実行条件

入口は既存 `CI` のworkflow_dispatch。`mypage_foundation=true`、`mypage_foundation_expected_sha=<統合HEADの40桁SHA>` を指定する。最初は `mypage_foundation_mode=foundation-metadata-plan` / `foundation-metadata-apply` で4資源の器だけを作成する。次のresource単位のbootstrap後に `foundation-plan` / `foundation-apply` で8資源へ進める。既存Terraform操作と同時指定しない。

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

`foundation_metadata` は専用runtime SAとSecret metadata 3件だけの4資源、`foundation` はさらにSecret accessor member 3件とruntime signer member 1件を加えた8資源。guardは選んだ段階の正確なresource集合、project、member、role、replication、stageを検証する。create/no-op以外、import/drift/削除・置換、provider action、想定外resource、未知の認可field、条件付きIAMを拒否する。create対象名が既にGCPに存在すれば、State未所有として停止する。

Plan/Apply jobは別identityでそれぞれnative Planを保存し、全Plan（生成timestampのみ除外）・source SHA・S3 State versionのHMAC一致と30分以内を条件に、Apply job自身が保存したPlanだけを適用する。HMACはGitHub expression内で比較し、生Plan/State/targetはlog/artifactへ出さない。raw evidenceはrunnerのprivate directoryへ置き、成功・失敗の両方で削除する。

Apply後はStateに選択段階の4件または8件だけが存在することをreadbackする。GCP実物のIAM/Secret metadataも別途readbackし、その証跡を残す。これだけではRuntime起動、公開Ready、実OAuthの成立を示さない。次はSecretの安全な投入と固定version、digest固定image、App Check/Auth/Hosting inventoryを確認し、別のRuntime段階へ進む。
