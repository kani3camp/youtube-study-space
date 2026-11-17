# Development Foundation の構築記録と境界

開発環境（Development）における MyPage 基盤の初回構築手順と、各段階の実作業記録です（対象は開発環境の Foundation のみで、Cloud Run・Firestore 書き込み権限・公開 Invoker・Hosting・Secret 値・本番は含みません）。

> [!NOTE]
> 最新の適用・配備状況は [統合 PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236) を参照し、過去の未実施記録を現在の状態と混同しないでください。

---

## 構築準備の記録（2026-10-20）

- AWS 側に State 用ロール `youtube-study-space-mypage-dev-plan` / `youtube-study-space-mypage-dev-apply` を新規作成し、`youtube-study-space/mypage-platform/dev/terraform.tfstate` と `.tflock` のみに権限を限定しました（Plan は State 読み取りのみ、Apply は読み書き、State 削除は禁止で、ロック作成・削除のみ両方に許可。18 組の IAM 模擬判定で他 State 拒否・削除拒否を確認しましたが、GitHub OIDC からの実 AssumeRole 証拠ではありません）。
- S3 State バケットは versioning 有効・AES256 暗号化・TLS 強制を確認し、開発用 prefix に既存 State がないことを確認しました（空 State JSON やロックを手書き投入せず初回 Apply で作成）。
- GCP Dev の既存 SA 12 件・WIF pool 4 件・Cloud Run 一覧に MyPage 専用資源がないことを確認しました（この時点では Secret / Firebase / 他環境の棚卸しは未完了）。

---

## Development 実構築の記録（2026-10-21 JST）

実行ソースは `df69028252c5e7f5f31e10da14bef5b619c15d8e`（#1405 / #1406 統合後、通常 CI Gate と CodeQL 成功・最新 `dev` に対し behind 0）です。

- 開始前に専用 SA・WIF pool・Secret が未作成であることを再確認し、Secret Manager API を有効化しました（既存 GCP ルートの管理対象とは分離し重複管理しません）。
- 専用 WIF pool `mypage-foundation` / provider `github-actions` と、`mypage-foundation-plan` / `mypage-foundation-apply` の 2 SA を作成・読み戻ししました（repository/owner/actor の固定 ID、統合 ref、caller/reusable、event、attempt 1、workflow SHA 一致、Environment と一致する `google.subject` マッピングを設定し、user-managed SA key は 0 件）。
- GitHub Environment `mypage-dev-plan` / `mypage-dev-apply` にそれぞれ 7 つの専用 Secret を設定しました（Plan/Apply で AWS RoleId と SA を分離し HMAC キーのみ共有、統合ブランチ 1 件・タグ 0 件の許可を維持）。
- metadata 段階の Plan と Apply を実行しました。
  - [Plan run 39452570203](https://github.com/kani3camp/youtube-study-space/actions/runs/39452570203) は 4 resources / 4 creates で Apply なし。
  - [Apply run 39452784387](https://github.com/kani3camp/youtube-study-space/actions/runs/39452784387) で専用ランタイム SA と 3 つの Secret メタデータを作成し、State 4 資源の読み戻しに成功しました（Secret 値・Cloud Run・Firestore writer・Hosting・本番は未作成。AWS 側でも version ID あり・AES256・`.tflock` 解放を確認）。その後 Secret version 0 件を確認したうえで作成済み SA と 3 Secret にのみ専用ポリシー編集権限を付与し、一時的なプロジェクト作成権限は解除しました（プロジェクト全域の `setIamPolicy` や Secret payload 操作は CI に非付与）。
  - 適用後 no-op の [run 39522369969](https://github.com/kani3camp/youtube-study-space/actions/runs/39522369969) では IAM メンバーの `secret_id` が短名から完全リソース名に変わるため停止しましたが、独立再読み取りで 3 件とも同一 DEV プロジェクト・Secret と確認し、当該 3 フィールドだけ短名または `projects/test-youtube-study-space/secrets/<固定名>` の完全一致を許可するよう修正しました（別プロジェクト・番号・バージョンパス等は拒否）。
- 8 資源への拡張（`foundation`）を進めました。
  - 初回の [run 39453504555](https://github.com/kani3camp/youtube-study-space/actions/runs/39453504555) は停止し、#1407 の安全な検査ラベルを用いた [run 39454711022](https://github.com/kani3camp/youtube-study-space/actions/runs/39454711022) で `resource-drift` と特定しました。ローカル読み取り専用 Plan で 3 Secret の `annotations` / `labels` / `version_aliases` が `null` → `{}` に正規化されることを確認しました（S3 State への書き込みや Apply は非実施）。
  - #1408 統合後の [run 39456968440](https://github.com/kani3camp/youtube-study-space/actions/runs/39456968440) で 8 resources / 4 creates の Plan guard を通過しましたが、独立 Plan 間の `relevant_attributes` 配列順序差による HMAC 不一致で Apply はスキップされました（現在は順序非依存で整列して比較するよう修正済み）。

### Bootstrap 権限の所有範囲

専用 WIF、構築用 2 SA、4 つのカスタムロールと bootstrap IAM バインディングは管理者の bootstrap 管理とし、ランタイムの Terraform State には含めません（プロジェクト IAM を authoritative 管理しません）。

| カスタムロール | 権限 | 付与範囲 |
| --- | --- | --- |
| `mypageFoundationRead` | `resourcemanager.projects.get`、`iam.serviceAccounts.get/getIamPolicy`、`secretmanager.secrets.get/getIamPolicy` | Development プロジェクトのメタデータ読み取り |
| `mypageFoundationCreate` | 上記 read + `iam.serviceAccounts.create`、`secretmanager.secrets.create` | 初回メタデータ作成時のみ Apply SA へ一時付与（作成後に解除） |
| `mypageFoundationSAPolicy` | `iam.serviceAccounts.getIamPolicy/setIamPolicy` | 作成済みランタイム SA のみ |
| `mypageFoundationSecretPolicy` | `secretmanager.secrets.get/getIamPolicy/setIamPolicy` | 作成済みの 3 Secret それぞれのみ |
| `mypageRuntimeRead` | `artifactregistry.tags.get`、`resourcemanager.projects.getIamPolicy`、`run.operations.get`、`run.services.get/getIamPolicy`、`secretmanager.versions.get`、`run.jobs.get/getIamPolicy`、`iam.roles.get`、`iam.workloadIdentityPools.get/getAttestationRules`、`iam.workloadIdentityPoolProviders.get` | Plan / Apply / PR Plan SA（プロジェクト）。`run.jobs.*` 以降は 2026-10-31 追加（未作成 maintenance 資源の 404 確認で権限不足時の 403 停止を防ぐため。また `run.jobs.getIamPolicy` は State 入り後の refresh に、`iam.workloadIdentityPools.getAttestationRules` は provider 8.5.0 の `:listAttestationRules` 呼び出し後の 403 taint 防止に必要） |

※read ロールは Secret の実値（payload）を読めません。リソース単位のポリシー編集権限も権限変更を伴うため専用 WIF・Environment・Plan guard の制限を維持し、作成権限は IAM だけで名前制限を保証できるとは仮定せず exact workflow / source / guard で制限します（`create` ロール定義のみ記録として残しバインディングは解除済み）。

---

## 初回構築時の実行条件（履歴情報）

> [!NOTE]
> 以下は初回 Foundation 構築時に使用した履歴上の段階操作です（当時は `CI` の `mypage_foundation=true` で `foundation_metadata` 4 資源から `foundation` 8 資源へ段階導入）。現行の通常運用は `Terraform`（`terraform.yml`）の `environment` / `operation=plan|apply` に集約されており、通常 UI から段階を個別選択することはありません。

- 統合ブランチ・repository/owner ID・owner actor・`run_attempt == 1`・caller `ci.yml`（現在は `terraform.yml` に移行）・reusable の同一 SHA を検証し、PR / fork / 他ブランチ / `main` / 再実行 / 古い SHA は拒否していました。
- GitHub Environment `mypage-dev-plan` / `mypage-dev-apply` は branch 制限なし（`No restriction`）とし、資格情報境界はクラウド側 trust の統合 ref・owner・初回実行固定で担保します（#1500 以降、PR Plan は owner の PR も許可）。統合ブランチを `dev` へマージした後のブランチ削除と暫定 trust の移行は別途実施します。

| Secret 名 | 内容（実値は公開リポジトリへ置かない） |
| --- | --- |
| `MYPAGE_STATE_BUCKET` / `MYPAGE_STATE_ACCOUNT` | State バケット / 所有アカウント |
| `MYPAGE_BACKEND_ROLE_ARN` / `MYPAGE_BACKEND_ROLE_ID` | Plan または Apply 専用ロールの ARN / RoleId |
| `MYPAGE_GCP_WIF_PROVIDER` | Dev プロジェクト番号の `mypage-foundation/providers/github-actions` |
| `MYPAGE_GCP_SERVICE_ACCOUNT` | `mypage-foundation-plan@...` または `mypage-foundation-apply@...` |
| `MYPAGE_PLAN_HMAC_KEY` | Plan / Apply で共有する 32 文字以上の乱数キー（他用途へ流用しない） |

---

## Plan / Apply 境界のガード仕様

- 対象資源: `foundation_metadata` は専用ランタイム SA と 3 Secret メタデータの 4 資源、`foundation` はさらに Secret accessor 3 件と signer 1 件を加えた 8 資源です（`create` / `no-op` のみ許可し、import・削除・置換・条件付き IAM・既存未管理リソースの上書きは拒否）。`create` 後に作成済みランタイム SA のみに `get/setIamPolicy`、3 つの Secret のみに個別 `get/setIamPolicy` を付与したうえで 8 資源へ進みます。
- refresh 差分の例外: 原則拒否ですが、上記 3 Secret の `annotations` / `labels` / `version_aliases` が `null` → `{}` に変わる表現正規化だけは、予定操作が `no-op` かつ他差分・unknown・write なしの場合に限り許可します（HMAC 比較には含め、許可件数のみ公開ログに出します）。
- 独立 Plan の HMAC 照合: Plan ジョブと Apply ジョブが別 identity でそれぞれ Plan を生成し、ソース SHA・S3 State バージョン・Plan 内容の HMAC 一致と 30 分以内を条件に、Apply ジョブ自身の保存済み Plan のみを適用します（生成タイムスタンプのみ除外し、順序に意味がない `relevant_attributes` のみ全 entry の内容・件数・重複を保持したまま整列して比較。生 Plan/State はランナーの一時ディレクトリで処理し、成功・失敗の両方で削除します）。Apply 後は選択段階の 4 件または 8 件のみ State に存在することと実 GCP IAM/Secret メタデータを読み戻します。

---

## 現行 DEV Plan の共通構成参照ロール

Core / MyPage の Plan SA には共通の静的マニフェストを適用します（Apply が使う `mypageFoundationRead` / `mypageRegistryRead` / `mypageRuntimeRead` の本文やバインディングは拡張せず、Plan SA のプロジェクトバインディングのみを専用ツールで置き換え、旧 PR 専用 SA は対象に含めません）。詳細は [DEV Plan の構成参照権限](../../gcp/docs/plan-read-access.md) を参照してください。
