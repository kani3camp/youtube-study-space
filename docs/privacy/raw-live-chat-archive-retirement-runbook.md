# Raw live-chat archive retirement runbook

YouTube の raw `live-chat-history` の永続保存を廃止し、メモリ上の command / moderation と Study Space の domain data を維持する。raw chat から新しい作業履歴を生成しない。`users`、座席、work segment、作業時間、RP、注文、activity、membership はこの廃止の削除対象外。

## 対象と実行境界

| 環境 | GCP project | BigQuery dataset / raw table | Firestore export bucket | AWS profile |
| --- | --- | --- | --- | --- |
| development | `test-youtube-study-space` | `firestore_export` / `live-chat-history` | `firestore-backup-test-youtube-study-space` | `soraride-dev` |
| production | `youtube-study-space` | `firestore_export` / `live-chat-history` | `firestore-backup-youtube-study-space` | `soraride-prod` |

各操作は credential が解決した project と明示 target を照合する。GCS は Firestore 設定の bucket も照合する。`system/.env` の選択や過去の観測だけで環境を判断しない。credential JSON とその path、confirmation token、raw inventory は commit / public log に出さない。

Bot は OBS / monitor と同じ配信 PC、daily batch は AWS 上で動く。Bot 起動時は実 credential から表示された `Project ID` が対象と完全一致する場合だけ `yes` で続行する。Bot 起動・CDK deploy・purge は実環境を変更する運用操作であり、通常のローカル検証には使わない。

実施記録・Day 0・対象 source / artifact・承認・結果は対応する Issue / PR の private 証跡参照で管理する。日付経過や merge を配備・削除完了の証明にしない。

## 変更してはいけない条件

- development の一連の検証後に production へ進み、環境別に Day 0 を記録する。
- mixed export の `kind_live-chat-history/**` だけを削除しない。削除単位は top-level snapshot 全体。[GCS の追加条件](gcs-raw-chat-cleanup.md)を満たすまで apply しない。
- mixed bucket、`firestore_export` dataset、`user-activity-history`、`order-history` とそれらの共有基盤を保持する。
- preview の `ready_to_apply` や token は人間の削除承認ではない。purge は個別承認済みの手動操作とし、deploy / CI に組み込まない。
- 資源名だけで廃止専用と判定しない。共有・帰属不明な resource / IAM は削除しない。

## 1. Producer 切替

1. Bot の raw Firestore write 除去と、daily batch の raw GCS → BigQuery import 除外を source / artifact で確認する。古い Bot・手動 runner・再試行経路も調べる。
2. development の配信 PC と AWS の両方へ対象変更を配備する。CDK は `aws-cdk/` の正規手順で対象 profile の diff を確認してから deploy する。
3. 実 development chat の通常 message、`!info`、test user の `!in` / `!out`、安全に実施できる moderation を確認する。raw 保存停止がメモリ上の処理を壊していないことを確認する。
4. 両 runtime の切替と smoke 成功後を Day 0 とする。新規 chat によって raw row が増えたら停止して残存 writer を探す。
5. development の drain・clean snapshot・承認済み cleanup・保持データ検証後、通常の `dev` → `main` release を経て production の配信 PC / AWS を更新する。production を `dev` から直接配備しない。

## 2. Drain と clean snapshot

以下は `system/` から実行する read-only command。表の environment / project / bucket を毎回明示する。

```sh
go run ./cmd/raw-chat-drain-audit <environment> <project-id>
go run ./cmd/privacy-gcs-admin preview <environment> <project-id> <bucket>
go run ./cmd/privacy-retention-admin preview-bigquery <environment> <project-id> <cutoff-rfc3339>
```

Firestore の `total_rows=0` / `drained=true`、新しい raw 書込なし、最後の raw snapshot より新しい clean snapshot が7世代以上、最新 export が72時間以内、bucket/project 一致、versioning 無効を確認する。5日保持と日次 export では約12〜14日かかり得るが、実 count と preview を判定に使う。production も独立して同じ条件を満たす。

## 3. 手動 purge と確認

BigQuery preview の対象は表の dataset / raw table だけであることを確認し、直前 preview と同じ cutoff・件数・token を使う。

```sh
go run ./cmd/privacy-retention-admin apply-bigquery   <environment> <project-id> <same-cutoff-rfc3339>   --expected-rows <preview-count> --confirm '<exact-preview-token>'
```

production は `--allow-production` も必要。GCS は [専用手順](gcs-raw-chat-cleanup.md)で retention / hold / 費用 / restore coverage まで確認する。**現在の GCS all-prefix apply は、一部だけ eligible な inventory に使用しない。**

purge 後は Firestore raw 0、選択 cutoff の BigQuery candidate 0、GCS live raw prefix 0、共有 dataset / bucket / retained table の存続、通常サービスの動作を確認する。GCS live namespace からの除去と soft-delete 期間後の hard deletion を分けて記録する。失敗・unknown を完了へ置き換えない。

## 4. 専用資源と一時機能の撤去

両環境の data cleanup 後、認証済みの inventory 環境で read-only helper または同等の Console 調査を行う。通常の Bot / Go admin command は local gcloud 設定に依存しない。

```sh
scripts/gcp/raw-chat-resource-inventory.sh <environment> <project-id>   > /path/to/private-inventory.txt
```

Cloud Asset、Scheduler、Functions、Run service/job、Pub/Sub、Eventarc、SA/IAM、Firestore index、GCS、BigQuery の trigger・code・consumer を調べ、`dedicated` / `shared` / `uncertain` に分類する。BigQuery raw table 自体の削除はこの段階で、両環境の移行完了と retained consumer 不在を確認してから行う。共有 export / scheduler / SA / IAM、mixed bucket、dataset と retained tables は残す。

最終 cleanup PR で BigQuery / GCS の一度限りの DELETE/apply、purge helper と専用 test、役目を終えた移行文書を削除する。raw の再出現を検出する drain audit と有用な read-only inventory は維持する。

## 完了条件

両環境について、producer/import 停止、Firestore raw 0、GCS live raw 0 と hard-delete 確認、raw BigQuery table 撤去、共有データ・基盤の保持、専用 resource/IAM/index 撤去、一時 DELETE 機能の除去、read-only audit の継続を証跡で確認する。現行 source だけで移行済みとは判断しない。
