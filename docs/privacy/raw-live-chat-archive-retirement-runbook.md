# 生ライブチャット履歴アーカイブの廃止手順書（Runbook）

YouTube の生データ `live-chat-history` の永続保存を廃止しつつ、メモリ上のチャットコマンド処理・モデレーションと Study Space の業務データを維持するための手順書です（生チャットから新しい作業履歴は生成しません。`users`・座席・work segment・作業時間・RP・注文・activity・membership はこの廃止の削除対象外です）。

---

## 対象と実行境界

| 環境 | GCP プロジェクト | BigQuery データセット / 生テーブル | Firestore エクスポートバケット | AWS プロファイル |
| --- | --- | --- | --- | --- |
| `development` | `test-youtube-study-space` | `firestore_export` / `live-chat-history` | `firestore-backup-test-youtube-study-space` | `soraride-dev` |
| `production` | `youtube-study-space` | `firestore_export` / `live-chat-history` | `firestore-backup-youtube-study-space` | `soraride-prod` |

- プロジェクトの一致確認: 各操作では認証情報が解決したプロジェクトと明示したターゲットを照合します（GCS は Firestore 設定のバケットも照合）。`system/.env` の値や過去の記録だけで環境を判断しないでください。
- 秘密・生情報の保護: 認証情報 JSON とそのパス、confirmation token、生棚卸し一覧はコミットや公開ログに出しません。
- 実環境変更操作の制限: Bot は OBS / モニターと同じ配信 PC、日次バッチは AWS 上で動作します。Bot 起動・CDK デプロイ・データ削除（purge）は実環境を変更するため通常のローカル検証には使わず、Bot 起動時は表示された `Project ID` が対象と完全一致する場合のみ `yes` で続行します。
- 記録管理: 実施記録・Day 0・対象ソースと成果物・承認・結果は対応する Issue / PR の非公開証跡参照で管理します（日付経過やマージだけで完了とみなしません）。

### 変更してはいけない条件

- 必ず **development** の検証完了後に **production** へ進み、環境ごとに Day 0 を記録する。
- 混合エクスポートの `kind_live-chat-history/**` のみを個別削除しない（削除単位は最上位スナップショット全体とし、[GCS の追加条件](gcs-raw-chat-cleanup.md) を満たすまで apply しない）。
- 混合バケット、`firestore_export` データセット、`user-activity-history`、`order-history` とそれらの共有基盤は削除せず保持する。
- `preview` の `ready_to_apply` やトークンを人間の承認代わりにせず、purge は個別承認済みの手動操作として実行する（デプロイや CI に組み込まない）。
- リソース名だけで廃止専用と決めつけず、共有リソースや帰属不明の IAM は削除しない。

---

## 1. Producer（書き込み元）の切り替え

1. Bot からの生 Firestore 書き込み除去と、日次バッチからの生 GCS → BigQuery インポート除外をソース・成果物で確認します（古い Bot・手動ランナー・再試行経路も確認）。
2. development の配信 PC と AWS の両方へ変更を配備します（CDK は `aws-cdk/` の正規手順で対象プロファイルの差分を確認してからデプロイ）。
3. 実際の development チャットで通常メッセージ、`!info`、テストユーザーの `!in` / `!out`、安全なモデレーション操作を行い、生保存停止がメモリ上の処理を壊していないことを確認します。
4. 両ランタイムの切り替えと動作確認が成功した時点を **Day 0** とします（新規チャットで生レコードが増えた場合は直ちに停止し、残存する書き込み元を調査）。
5. development でドレイン・クリーンなスナップショット取得・承認済み削除・保持データ検証を終えたあと、通常の `dev` → `main` リリースを経て production の配信 PC / AWS を更新します（`dev` から本番へ直接配備しません）。

---

## 2. ドレイン確認とクリーンなスナップショット取得

`system/` ディレクトリから以下の読み取り専用コマンドを実行し、毎回環境・プロジェクト・バケットを明示します。

```sh
go run ./cmd/raw-chat-drain-audit <environment> <project-id>
go run ./cmd/privacy-gcs-admin preview <environment> <project-id> <bucket>
go run ./cmd/privacy-retention-admin preview-bigquery <environment> <project-id> <cutoff-rfc3339>
```

Firestore の `total_rows=0` / `drained=true`、新規生書き込みがないこと、最後の生スナップショットより新しいクリーンなスナップショットが 7 世代以上あること、最新エクスポートが 72 時間以内であること、バケット/プロジェクト一致、versioning 無効を確認します（5 日保持と日次エクスポートの環境では約 12〜14 日かかる場合がありますが、日数の経過ではなく実カウントと `preview` 結果で判定します）。production でも独立して同じ条件を満たす必要があります。

---

## 3. 手動 purge と完了確認

BigQuery `preview` の対象が上表の dataset / raw table のみであることを確認し、直前の `preview` と同じ cutoff・件数・トークンを使って適用します（production では `--allow-production` も必須）。

```sh
go run ./cmd/privacy-retention-admin apply-bigquery \
  <environment> <project-id> <same-cutoff-rfc3339> \
  --expected-rows <preview-count> --confirm '<exact-preview-token>'
```

GCS については [GCS の追加条件](gcs-raw-chat-cleanup.md) で retention・hold・費用・復元カバー範囲を確認したうえで実施してください（一部のみ削除可能な状態で一括 apply は使いません）。

purge 後は、Firestore raw が 0 件、指定 cutoff の BigQuery 候補が 0 件、GCS 本番名前空間の raw プレフィックスが 0 件であること、および共有データセット・バケット・保持テーブルと通常サービスが正常であることを確認します（GCS は名前空間からの削除と soft-delete 期間後の完全削除を分けて記録し、失敗や `unknown` を完了扱いしません）。

---

## 4. 専用資源と一時機能の撤去

両環境のデータ削除完了後、認証済みの棚卸し環境で読み取り専用の調査スクリプト（またはコンソール調査）を実行します（通常の Bot / Go 管理コマンドはローカルの `gcloud` 設定に依存しません）。

```sh
scripts/gcp/raw-chat-resource-inventory.sh <environment> <project-id> > /path/to/private-inventory.txt
```

- Cloud Asset、Scheduler、Functions、Cloud Run、Pub/Sub、Eventarc、SA / IAM、Firestore インデックス、GCS、BigQuery を調べ、`dedicated` / `shared` / `uncertain` に分類します。
- BigQuery の `live-chat-history` テーブル自体の削除は、両環境の移行完了と保持データ参照元の不在を確認したうえでこの段階で行います（共有エクスポート・スケジューラー・SA / IAM・混合バケット・データセットと保持テーブルは残します）。
- 最終整理 PR で BigQuery / GCS の一回限りの `DELETE` / `apply` 機能・一時テスト・役目を終えた移行文書を削除し、再出現を検知する `drain-audit` と読み取り専用棚卸しは維持します。

---

## 完了条件

両環境について、producer / import 停止、Firestore raw 0 件、GCS 本番削除および完全削除確認、生 BigQuery テーブル撤去、共有データと基盤の保持、専用資源・IAM・インデックス撤去、一時 `DELETE` 機能の除去、読み取り専用監査の継続を根拠資料で確認して完了とします（ソースコード上の実装だけでは移行完了とみなしません）。
