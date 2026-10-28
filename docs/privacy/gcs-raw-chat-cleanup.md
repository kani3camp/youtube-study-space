# GCS raw チャットを含む snapshot の cleanup

[retirement runbook](raw-live-chat-archive-retirement-runbook.md) の GCS 操作条件。mixed Firestore export は raw chat と `users` / `user-activities` / `order-history` を含み得るため、raw kind だけの部分削除や bucket 全体への短期 lifecycle を使わない。対象は raw を含む **top-level snapshot prefix 全体**であり、その中の古い domain-data backup も失う。新しい clean snapshot と共有 export 基盤を維持する。

## Apply 前の条件

1. 対象 environment / credential project / 設定 bucket を毎回照合する。DEV の観測を PROD へ流用しない。
2. 配備済み producer/export と自然実行の collection group を確認し、raw の再流入・予期しない再配備がないことを確認する。
3. Firestore raw 0、versioning 無効、最後の raw snapshot より新しい clean snapshot 7世代以上、最新 export 72時間以内を fresh preview で確認する。
4. bucket の location / storage class / soft delete / retention / lock と、全 candidate object の generation・retention expiration・temporary/event hold・object retention を private inventory に固定する。prefix 内の **全 object** が eligible であることを確認する。
5. complete clean snapshot の復元可能性、保持する recovery coverage、失う古い backup、復旧経路・hard-delete 時期、request / soft-delete storage / COLDLINE early-deletion 費をレビューし、別途削除承認を得る。

Production の COLDLINE / 93日 retention は維持する。個々の実 metadata から eligibility を求め、prefix 名や DEV の日付から推測しない。retention 経過だけで storage-class 料金がなくなると仮定しない。

**現在の `privacy-gcs-admin` は versioning / soft delete と object 名による prefix を確認するが、retention / hold・snapshot の完全性や復元可能性を検証しない。全 raw prefix を対象とし、eligible な wave だけを選べない。** `ready_to_apply=true` だけでは削除を決定できず、一部 eligible な inventory に all-prefix apply を使わない。retention/hold を外したり、部分削除を retry loop にしない。必要な wave 対応・レビューを先に行う。

## Guarded command

`system/` から、[対象表](raw-live-chat-archive-retirement-runbook.md#対象と実行境界)の値を明示する。preview は read-only。

```sh
go run ./cmd/privacy-gcs-admin preview <environment> <project-id> <bucket>
```

preview は raw row count、candidate prefix/object/bytes、最後の raw snapshot、clean 世代数、各 safety check、target-bound token を返す。前節の追加条件・人間の承認まで揃った場合だけ、直前 preview の値を使う。

```sh
go run ./cmd/privacy-gcs-admin apply   <environment> <project-id> <bucket>   --expected-prefixes <preview-count>   --expected-objects <preview-count>   --confirm '<exact-preview-token>'
```

production は `--allow-production` も必要。command は直前に再 scan し、target・raw count・clean 世代・件数・token の変化で停止する。各 prefix では non-raw object を先に削除し、最後まで raw marker を残すため、部分失敗後も対象を再発見できる。この再開特性は retention-aware wave 対応の代用にはならない。

## 結果と記録

live raw prefix 不在と共有基盤・clean recovery set の保持を確認する。soft delete 期間中は復元可能なので、live 削除成功と hard deletion 完了を分ける。soft-delete 期間は対象 bucket の実 metadata で確認する。

公開記録は集約 count/bytes、eligibility 範囲、保持 backup coverage、費用と private manifest 参照だけにする。raw object 名、設定全文、token、business data は public log/artifact へ出さない。fresh preview で raw prefix 0なら削除を繰り返さない。両環境の移行完了後、一時 DELETE 機能を撤去する。一般 backup retention は raw archive 廃止とは独立して扱う。
