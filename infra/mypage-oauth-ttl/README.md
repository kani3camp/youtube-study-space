# MyPage OAuth トランザクション TTL（専用 Terraform）

[Data03](https://app.notion.com/p/3ec357a8d0ce81ac94eef9e6ac0c9964) に基づき、`oauth-transactions` コレクションの `expiresAt` フィールドに対する Firestore TTL ポリシーを管理する専用 Terraform ルートです。

- [dev](environments/dev/main.tf) と [prod](environments/prod/main.tf) で独立したルートと State を持ち、従来の GCP Terraform 移行経路（`import-only` 検査・State・ワークフロー）には含めません。
- `expiresAt` はアプリ側が作成時刻の 10 分後の Timestamp として保存します。[アプリ側の期限判定](../../system/core/mypage/auth.go) はそのまま維持します（Firestore TTL 削除は非同期であり、期限到達時の即時削除や認証失効を保証しません。参考: [Firestore TTL](https://docs.cloud.google.com/firestore/native/docs/ttl)）。

---

## 対象範囲と停止条件

- 管理対象: [共有モジュール](modules/oauth-transaction-ttl/main.tf) の `google_firestore_field`（`ttl_config {}`、`offset` 指定なし）1 つのみです。DB・API・IAM・ルール・認証は含めません。
- インデックスの保護: `index_config` は省略して継承インデックスを維持します（空の `index_config {}` はインデックスを無効化するため追加しません。明示的な上書きがある場合は Plan 検査で停止して別途レビューします。参考: [provider v8.5.0 仕様](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/website/docs/r/firestore_field.html.markdown)）。
- 削除防止と公開前の確認事項: `prevent_destroy` と `deletion_policy = "PREVENT"` を設定しています（撤去・置換・TTL 無効化は対象外）。なおサポート 3 コレクションの保持・削除、新 D01 方式の実装、実サポート受付の完了、実環境のインデックスと運用は別の公開条件として残っており、CI の成功だけでは公開承認や実環境での TTL `ACTIVE` 状態は証明されません。

---

## 認証情報を使わないローカル検証

Terraform バージョンは [.terraform-version](.terraform-version)、プロバイダーは各ルートのロックファイルを使用します（リポジトリルートから実行）。

```sh
bash infra/mypage-oauth-ttl/scripts/check-local.sh
```

バックエンドは初期化せず、プロバイダー取得・`validate`・`mock_provider` による Plan・Plan 仕様の回帰テストを実行します（apply や実 Google API 呼び出しは行いません）。

---

## Plan の差分を限定する検査

[plan_contract.py](scripts/plan_contract.py) は `terraform show -json` の結果とオーナー承認済みのターゲットマニフェストを照合し、対象フィールド・TTL 有効・offset なし・インデックス不変を確認します（`create`、TTL 有効化のみの `update`、`no-op` のみを許可し、削除・置換・import・drift・追加リソース等を拒否）。

```sh
python3 infra/mypage-oauth-ttl/scripts/plan_contract.py \
  --plan /private/mypage-dev.plan.json \
  --target /private/mypage-dev-target.json
```

出力は固定の `status` / `code`、Plan・対象の SHA-256、`action` のみとし、成功時も `executionAuthorized=false` / `releaseReady=false` を明示します（生 Plan/State はシークレットを含み得るため公開成果物や PR に残さないでください）。

---

## リリース手順（実行は別途承認）

1. 従来の GCP Terraform（MyPage 抜き）を完了したあと、対象コミットをレビューして本ルートを適用対象にします（`dev` / `prod` へ直接マージしません）。
2. MyPage Ready Gate・対象プロジェクト・既存 DB・State 所有権を確認します（既存 State で同フィールドを管理している場合は停止。IAM/認証は変更しません）。
3. オーナー承認後に承認済み担当者が非公開バックエンド設定と `-lockfile=readonly` を指定して `dev` を初期化し、既存のフィールド・TTL・インデックスの所有者・期限切れ文書の削除影響を事前に読み取り専用で照合します（既存の管理ポリシーがある場合は独立した移行計画を別途レビュー）。

```sh
terraform -chdir=infra/mypage-oauth-ttl/environments/dev init -backend-config=/private/dev-backend.hcl -lockfile=readonly
```

4. 非公開の変数ファイルと保存先を指定して `plan` と JSON 出力を実行したうえで、上記の契約検査（`plan_contract.py`）を行い、生の Plan と固定ダイジェストを非公開記録に残して独立レビュー・承認を受けます（確定コミット・ロック済みプロバイダー・バックエンドと対象・Plan ハッシュ・インデックス不変を確認。契約検査の成功だけで `apply` はしません）。

```sh
terraform -chdir=infra/mypage-oauth-ttl/environments/dev plan -var-file=/private/dev.tfvars -out=/private/dev.tfplan
terraform -chdir=infra/mypage-oauth-ttl/environments/dev show -json /private/dev.tfplan > /private/dev.plan.json
```

5. `apply` の別途承認後に、同じ保存済み Plan を指定して適用します（Plan・State・対象に変化がある場合は適用せず、再 Plan・再レビュー・再承認を行います）。適用後は API 完了と TTL `ACTIVE` を読み取り専用で確認したうえで、承認済みの合成 OAuth トランザクションで 10 分境界の拒否と自然削除を実測し、時刻・対象・フィールド・状態・結果を非公開のリリース記録へ残します（実ユーザーデータは使わず、`mock` 成功や `ACTIVE` 確認だけで自然削除を完了とみなさないでください）。

```sh
terraform -chdir=infra/mypage-oauth-ttl/environments/dev apply /private/dev.tfplan
```

6. `dev` 観測後に同じ手順を `prod` で実施します（自然削除の遅延・失敗時は `pending` / `STOP` とし、アプリ側の期限判定は緩めません）。

> [!CAUTION]
> TTL で削除済みのドキュメントは、TTL 無効化やコードのロールバックでは復元できません。異常時は後続のリリースと観測を止め、オーナー承認のもとで forward-fix または復旧を判断してください（記録先は [MyPage 公開条件](../../docs/mypage/release/README.md) と [リリース記録例](../../docs/mypage/release/release-record.example.json)）。
