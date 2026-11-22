# MyPage メンテナンスモード v1

計画メンテナンス中に MyPage の通常利用を停止する機能です。

> [!NOTE]
> これは計画メンテナンス用の停止機能であり、セキュリティ事故向けの即時遮断装置ではありません。切り替え時に実行中だった通常読み取りが完了する場合があります。なお、プライバシー請求・サポート用の専用経路はメンテナンス中も維持します。

---

## 制御データと動作仕様

各環境の Firestore 文書 `config/mypage` を制御状態の正規データとします（ブラウザからの直接読み書きは不可）。

- フィールド: `maintenance-enabled`（真偽値）、`updated-at`（Firestore Timestamp）
- Fail-closed（安全側停止）: 制御文書が存在しない・不正・読み取り不能の場合は、安全のため通常利用を停止します。
- バックエンドの判定: 保護対象の通常処理の開始時に最新状態を読み取り、有効なら `503 MAINTENANCE_ACTIVE` を返します。書き込みは Firestore トランザクション内でも再確認し、Custom Token は発行直前に再確認します（外部トークン発行と切替操作を完全にアトミックにはできないため、直前確認で保護します）。
- 公開 API の応答: `{"maintenanceEnabled": true}` または `{"maintenanceEnabled": false}` のみとし、更新日時は公開しません。
- フロントエンドの画面遷移: 保護ルートへ入る際に状態を確認し、有効または `MAINTENANCE_ACTIVE` を受け取るとメンテナンス画面を表示します（「お知らせを見る」「状態を再確認」を表示し、定期取得やタブ復帰時の自動再確認は行いません。解除後は再読み込みまたは「状態を再確認」で復帰します）。

---

## 初期化と切り替え手順

### 1. 初回構築・有効化

1. イメージの準備確認: 対象環境の Artifact Registry に、GitHub Environment の `MYPAGE_RUNTIME_SETTINGS_JSON` が指すレビュー済み API イメージがあることを確認します。Job を初めて作るときだけ、この API イメージ（`runtime_settings.image_digest`）で作成します（API イメージには Operator のバイナリが無いため、手順 3 の Operator 配備までは Job を実行できません）。
2. Terraform の Plan / Apply: `Terraform`（`terraform.yml`）を `operation=plan` と対象 `environment` で実行し、Core GCP と MyPage の desired state をまとめて確認したうえで、差分レビュー後に `operation=apply` を実行します（段階の選択は無く、runtime settings がそろった環境では API と同じ Apply で Job と専用 IAM/WIF を宣言します。本番の初回構築手順は [MyPage Platform Terraform](../../infra/mypage-platform/README.md#本番の初回構築main-から) を参照）。Job 作成後のコンテナイメージは Backend CI/CD が所有します。
3. Operator イメージ配備と制御文書初期化
   - 初回構築後は `Deploy Maintenance Operator`（`mode=build-and-deploy`）で Job を専用 operator イメージへ更新し、generation の更新（前進）と設定が変わっていないことを読み戻しで確認します（API Service とは同じ digest に同期しません）。
   - 続いて GitHub Actions の `Set Maintenance Mode`（`mypage-maintenance.yml`）を `workflow_dispatch` で実行し、対象 `environment` と `enabled=false` を指定して制御文書を初期化します（文書が欠落している場合も同じ Job が作成します）。

### 2. 日常の開始・解除

- 開始時は `enabled=true`、解除時は `enabled=false` を指定して `Set Maintenance Mode` を手動実行します。
- Job は入力検証のうえ、トランザクションで現在値を読み取って更新し、コミット後に読み戻して結果をログ出力します（同じ値への更新は正常終了の no-op）。
- 読み取り・更新・再読み取りに失敗した場合は成功とみなさず、対象環境・権限・Firestore 文書を調査して復旧後に新しい手動実行として起動してください。

---

## 権限境界と配備時の注意

- WIF と Job の権限: GitHub の WIF 本人情報は固定 Job を起動できますが、Firestore への直接権限は持ちません。Job 自体は専用 Service Account で動作します（Firestore IAM はコレクション単位に絞れないため、他文書にも権限が及びます）。
- Invoker のカスタムロール: `mypage-maintenance-invoker` には、固定 Job 上の `roles/run.jobsExecutorWithOverrides` に加えて、`run.jobs.get` と `run.executions.get` のみを含むカスタムロール `mypageMaintenanceExecutionRead` を付与します（`gcloud` が引数検証と `--wait` の完了確認に必要とするため。参考: [Cloud Run のロールと権限](https://docs.cloud.google.com/iam/docs/roles-permissions/run)）。
- `PERMISSION_DENIED` 発生時の復旧: `run.jobs.get` で権限不足により停止した場合は、修正後の Terraform を先に対象環境へ Plan/Apply し（追加差分はこのカスタムロールと固定 Job への IAM メンバーの 2 資源のみであることを確認）、適用後に `Set Maintenance Mode` を新しい手動実行として起動します（GitHub の「Re-run jobs」は `run_attempt=1` 制限により拒否されます）。
- 配備順序の注意: 配備時は Job と `enabled=false` を準備したうえで、バックエンドとフロントエンドを続けて配備してください（旧形式と新形式は非互換のため、切替中に古い画面を開いている利用者は再読み込みが必要になる場合があります）。
