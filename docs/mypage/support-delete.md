# 現在storeのPrivacy削除CLI

この手順は所有者がprivate端末から実施する。コードのmergeやローカル試験は、実データ削除の承認・実環境受入の証拠ではない。受付から7暦日の期限は、OAuthや再試行で延長しない。

## 実装と確認の分担

`cmd/mypage-support-delete`は`--execute`の場合だけ、所有者の既存ADC/GCP IAMで固定projectへ接続する。実行用の新しい権限・サービスアカウント・Operator OIDCは作らない。

| 対象 | 処理と証拠 |
| --- | --- |
| verified SupportRequest | 初回Acquireでpurpose/環境/channel/fresh OAuth証跡を照合し、proof claim・実行checkpoint・privacyDeletionを同じtransactionで確保 |
| Firestore | 現在の固定collectionを直接操作。対象queryの先頭ページを繰り返し削除し、各ページでowner/generationを照合。完了前に全storeを再読 |
| Firebase Auth | channel IDと同じUIDのみ。revoke前にcreation timestampを保存し、別incarnationを採用しない。delete後の不存在を再読 |
| BigQuery | 固定`firestore_export`の3本表と`tmp`のみ。metadataで存在/所有fieldを確認、parameterized DELETE、安定job IDでACK消失を照合、COUNTで対象0を確認 |
| writer / BFF cache | 所有者が全実行系の停止・drain、cache破棄、古いqueue/import/restore排除を実確認しprivate Gateへ記録 |
| legacy mapping / backup・export / vendor・派生物 / log | 自動削除済みと扱わない。実inventoryの照合と処置を所有者が行い、scope別Gateにprivate証跡参照を記録。unknownは停止 |

Registry/SyntheticCatalog/PreparedDeletion、合成Auth/BQ/vendor世界とそれらのsnapshot状態機械は撤去した。固定29 checkpointは、storeごとのdelete→inspect、停止→drain→resume、途中失敗の再開位置として維持する。`supportdelete.Engine`はこの固定順序を実行する短いループであり、任意のworkflow登録機構を持たない。

## 初回実行前

1. [人的窓口](support-operator.md)で受付を記録し、同じ目的・channelのfresh YouTube OAuthを確認する。通常のFirebase sessionだけで代用しない。OAuth証跡のTTL消去前に初回checkpointを確保する。証跡が消失済みなら推測で作らず、新たな本人確認・受付の整合を解決するまでGateとする。
2. 対象環境・credential project・channel・request/proof/execution参照をprivateに照合する。DEVは`test-youtube-study-space`、Productionは`youtube-study-space`。credential file、emulator、矛盾するproject aliasはCLIで拒否する。
3. [対象一覧](support-delete-inventory.md)から、稼働中revision/instance、配信PCのBot、AWS batch/SFN、手動runner、export/restore、vendorコピーを調べる。旧合成checkpointが実環境に存在しないことも配備前に確認する。旧snapshot schemaから実削除へ自動移行しない。
4. manifestを0600の通常ファイルへ保存する。既存schemaの`schemaVersion:1`、`operation:delete`、`target`、`requestRef`、`executionRef`、`proofRef`、同じ値の`confirmation`を使用する。UIDや参照をCLI引数・公開log・PRへ貼らない。

```sh
# system/から。既定planはADC探索・store読み取り・変更をしない。
MYPAGE_ENVIRONMENT=development GOOGLE_CLOUD_PROJECT=test-youtube-study-space \
  go run ./cmd/mypage-support-delete --manifest /private/path/request.json

# 初回はprivacyDeletion/checkpointを確保し、GateなしならMANUAL_GATE_REQUIREDで停止する。
MYPAGE_ENVIRONMENT=development GOOGLE_CLOUD_PROJECT=test-youtube-study-space \
  go run ./cmd/mypage-support-delete --manifest /private/path/request.json \
  --execute --checkpoint /private/path/checkpoint.json
```

Productionでは環境・projectを両方切り替え、`--allow-production`を追加する。これは依頼された操作意図の確認であり、対象データやbackupの削除承認そのものではない。

## 所有者Gate

checkpointの`selector/generation/cutoff`を完全一致で転記した0600 JSONを作る。`OwnerDeletionGates`のschemaを使い、次を実測・確認して記録する。ファイルの宣言だけを第三者検証や自動削除の証拠として公表しない。

- `expiresAt`: 現在から1時間以内の期限。期限・世代の変更時は実状態を再確認する。
- `writersStoppedAt`: MyPageのmint/metadata/flight、Bot、batch、手動、import、Auth作成を含む全writerを停止し、開始済み処理の終了を確認した時刻。
- `cachePurgedAt`: 全BFF instance/revisionのcacheを破棄した時刻。保護開始後のinstance再起動とinflight終了の確認を使う。旧revisionを再稼働させない。
- `restoreExcludedAt`: 古いqueue、snapshot、copy/import/restoreが対象を再生成しないことを確認した時刻。停止確認3時刻はcutoff以降にする。
- `manual`: `legacy-mappings`、`backups-exports`、`derived-vendor`、`platform-logs`それぞれについて、`status`（`completed`または実在しないことを調査した`absent`）、64桁hexのprivate `evidenceRef`、cutoff以降の`observedAt`を記録する。未確認なら`unknown`のまま停止させる。
- `writersResumedAt`: 初回は未設定/zero。全inspect後にCLIがresume Gateで停止したことをcheckpointで確認し、旧job/queueを破棄したままwriterを再開・照合した時刻を、checkpointの更新時刻以降に記録する。

`legacy-mappings`の`absent`は、channel UID以外の旧UID/tenant/shared accountがないことまで含む。存在する場合はこのCLIで推測して消さず、同じ依頼・期限の下で別途帰属確認と処置を行う。

```sh
MYPAGE_ENVIRONMENT=development GOOGLE_CLOUD_PROJECT=test-youtube-study-space \
  go run ./cmd/mypage-support-delete --manifest /private/path/request.json \
  --execute --checkpoint /private/path/checkpoint.json --gates /private/path/gates.json
```

部分失敗はprivacyDeletionをactiveのまま残す。同じmanifest/executionで再実行し、Firestore上のcheckpointを正とする。local checkpoint fileは対象確認の写しであり、cursorを編集して実行位置を飛ばせない。現在のOAuth証跡・proof claimは最終transactionまで保持し、初回確保後はTTL消失で取得済み対象を別channelへ切り替えない。

## 完了・復旧

全storeのinspectとwriter再開確認が揃った後だけ、現在の申請・実行・SAC世代をtransactionで再照合し、Auth incarnation記録・OAuth/関連参照を消去し、最小完了記録へ置換してprivacyDeletionを解除する。moderationとSAC世代は独立して保持する。完了ACK消失後は完了bindingを読み、再削除へ戻らない。

Productionでは最初にDEVの専用データで失敗/再開/対象隔離/rollbackを実受入する。CLIの中断・不明な結果で保護を手動解除しない。削除済みデータを復元するrollbackを前提にせず、サービス停止・保護維持・同じcheckpointからの前進復旧を用いる。バックエンドimage/Hostingは配備Runbookの既知revisionへ戻せる状態を維持し、旧writerからの再生成を防ぐ。

backup/PITR/soft-delete/export/vendorの個別削除ができない場合、live namespace不在とhard deletionを区別する。7暦日要件と整合しない保持・復元可能コピーが残るなら、そのscopeを`completed`にしない。実設定・処置・期限の解決と公開可否判断は未解決Gateである。

検証責任: transaction原子性/対象隔離/世代/CASはEmulator、Auth再作成とresume Gateはcurrent-store integration、BQ job/parameter/readbackはローカルSDK HTTP試験、ACK消失/再開位置/完了後の新規利用保護はcheckpoint unit。実環境Gateの完了はこれらと別に確認する。
