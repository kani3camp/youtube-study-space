# DEV Plan の構成参照権限

Core と MyPage の DEV Plan では、[静的マニフェスト](../policies/plan-metadata-read.json) を正本とする共通のプロジェクトカスタムロールを使用します（対象は `test-youtube-study-space` の `terraform-dev-normal-plan` と `mypage-foundation-plan` のみ。SA・WIF・State は別々のまま維持します）。同じ SA を使う手動 DEV Plan にもこの参照範囲が適用されます。

---

## 権限設計の基本ルール

- マニフェストのサービス（family）ごとに、明示的な permission・API 返却内容・一次資料・平文秘密値を置かないための前提を記録しています（API や定義済みロールの変更に自動追従せず、未分類 permission は拒否）。
- 未使用サービスの参照権限も含みますが、API 自体の有効化は行いません。
- 課金情報・Secret の実値（payload）・業務データ本文・認証情報発行・署名/復号・書き込み権限は一切付与しません（Secret メタデータと AWS State 読み取りは別扱いとし、State/lock ポリシーはこのツールでは変更しません）。
- **個別の除外ルール**:
  - Cloud SQL instance の `get`/`list` は、外部接続設定の `password` / `clientKey` が返却から確実に除外される保証が未確定のため初期対象外としています。
  - GKE はクラスタ構成のみを対象とし、`getCredentials` や Kubernetes API の Secret / Pod / ConfigMap 参照は許可しません。
  - Firestore は `datastore.databases.getMetadata` を用い、トランザクション操作を含む `datastore.databases.get` は含めません。
- **平文秘密値の禁止**: VM メタデータ、スタートアップスクリプト、Run/Functions の環境変数、Scheduler の body/header、Pub/Sub push URL、BigQuery の view/routine 定義、監視設定には平文の秘密値を埋め込まないでください（既知の秘密値埋め込みが見つかった場合は修正まで権限拡張を止めます）。

---

## 生成・検証（ローカル / 固定 CI）

[plan_read_access.py](../scripts/plan_read_access.py) が Core / MyPage 共通の定義と差分生成器です（既定ではマニフェスト検証のみでクラウドには接続しません）。

```bash
python3 infra/gcp/scripts/plan_read_access.py
```

固定 CI の `terraform-static` は、信頼済みコントローラー側の [plan_policy.py](../../../.github/ci-validation/plan_policy.py) を使って対象コミットの JSON だけを実行せずに検証します（マニフェストがない過去コミットは停止します）。

Core の `bootstrap_normal_ops.py` は Plan 権限をこのマニフェストから生成し、Apply 権限は従来どおり独立して生成します（共有の旧 read ロールは変更しません）。

---

## 読み取り棚卸しと dry-run

管理端末の既存 `gcloud` 認証で実行します（Plan SA へ棚卸し権限を足したり、Plan SA になりすましたりしないでください。権限不足時は停止します）。

```bash
# 保存先は既存の非公開ディレクトリ、出力先は新規ファイルパスを指定
python3 infra/gcp/scripts/plan_read_access.py --live --export /private/plan-before.json

# 取得済みスナップショットを使ったオフライン差分確認（クラウド接続・反映なし）
python3 infra/gcp/scripts/plan_read_access.py --snapshot /private/plan-before.json
```

- プロジェクト/SA の同一性・有効状態、user-managed key がないこと、プロジェクトポリシーの `etag`、新旧ロールの範囲、GA / カスタムロール対応を確認します。Cloud Asset Analyzer で条件付き・間接・子リソースの追加付与や禁止 permission、解析不足（`fullyExplored != true`）が見つかった場合は停止します。
- 出力は追加/削除 permission・Plan 対象名・確認 digest・review 待ち状態のみとし、生 IAM ポリシーは公開ログに出しません。`export` は `0600` で新規作成し、スナップショットや復旧記録・レビュー記録は Git や公開 Issue/artifact に保存しないでください。

---

## 反映前の管理者レビュー

プロジェクトスコープの Analyzer だけでは対象外リソースや他プロジェクトからの間接付与・設定内の平文秘密値を網羅できないため、管理者が残る範囲を確認し、非公開の根拠記録を作成します（旧 PR 専用 identity が残っていても拡張対象に加えず、整理方針のみ記録します）。

全項目の確認が完了したら、レビュー JSON の各 boolean を `true` に設定します（これは管理者の確認記録であり、技術的な完全証明ではありません）。

| フィールド | 確認内容 |
| --- | --- |
| `confirmation` | 最新 dry-run の digest（マニフェストと全スナップショットに紐付け） |
| `reviewed_at` | タイムゾーン付き ISO 日時（未来日時と 24 時間超は拒否） |
| `evidence_reference` | 非公開の確認根拠資料の参照先 |
| `resource_iam_review_complete` | Analyzer 対象外を含むリソース単位 IAM を確認済み |
| `cross_project_grants_review_complete` | 他プロジェクト・継承・間接経路を確認済み |
| `plaintext_secrets_absent` | 許可 API で読める設定に平文秘密値がないことを確認済み |
| `no_other_members_of_plan_role` | 新共通ロールが子リソースを含め他 identity へ付与されていないことを確認済み |
| `legacy_paths_reviewed` | 旧経路の残存理由・利用状況と整理方針を確認済み |

※`--live` 成功やコードマージだけでは反映可能とみなさず、レビュー記録がない dry-run では `blocked_pending_operator_review: true` を出力します。

---

## 明示的な反映指示を受けた場合

コード・CI・棚卸しを完了し、実行中の DEV Plan を止めて非公開のバックアップ先を用意します（既存 State/lock・AWS trust・PROD・Apply SA・共有ロール本文は変更しません。PR 再実行用 WIF の変更は [workflow 設計](../../../docs/development/ci-cd-workflows.md#45-pr-plan-の再実行を有効にするowner) を参照）。

```bash
# 同じマニフェストと最新の IAM 状態に対するレビュー作成後のみ実行
# digest は実際の dry-run 出力を指定し、backup には未作成のディレクトリを指定
python3 infra/gcp/scripts/plan_read_access.py --live --apply \
  --confirm ACTUAL_DRY_RUN_DIGEST --review /private/review.json \
  --backup /private/plan-access-backup
```

- 再取得した IAM とレビューの digest を照合後、`0700` の backup ディレクトリへ更新前スナップショットと提案内容を `0600` で保存し、新共通ロールの作成/更新と 2 つの Plan SA のバインディング差し替えを `etag` 付きで実行します（その他のバインディングや監査設定は保持）。
- permission 非対応・解析不足・レビュー不一致・`etag` 競合・読み戻し不一致時は停止します（ロール作成後にポリシー更新が失敗した場合は、実状態を読み戻して復旧か再 dry-run を選択）。
- 反映後はリソース IAM を再確認し、変更範囲が想定どおりであることを照合したうえで、Core / MyPage の native Plan と再実行を確認して結果を Issue / PR に記録します（具体的な整理記録・未確認事項は [Issue #1562](https://github.com/kani3camp/youtube-study-space/issues/1562) で管理）。

---

## 復旧手順

1. 実行中の DEV Plan を停止します（復旧も環境更新のため、明示的な指示の範囲内で実施）。
2. backup の `before.json` と **最新のプロジェクトポリシー** を取得し、最新ポリシーの `etag` と他メンバー・監査設定を維持したまま、対象 2 つの Plan SA のバインディングだけを `before.json` と同じ状態へ戻したポリシーを作成します（**古いプロジェクトポリシー全体でそのまま上書きしないでください**）。
3. `gcloud projects set-iam-policy test-youtube-study-space /private/restored-policy.json` で `etag` 付き更新を行い、読み戻して対象以外が変わっていないことを確認します。
4. 共通ロールが更新前から存在していた場合は、他メンバーへの新規付与がないことを確認のうえ現 `etag` を含めて元定義へ戻して読み戻します（新規作成した場合はバインディング解除のみで構いません。共有 legacy ロール・旧 SA/WIF・State には触れません）。
5. 新規トークン発行の停止と発行済みトークンの有効期限を考慮した確認を別々に行います（バインディング解除が既存トークンの即時失効を意味するわけではありません）。
