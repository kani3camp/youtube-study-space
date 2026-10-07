# MyPage runtime drain の実装境界

このsliceはMyPageの実呼び出し経路をprocess内の同じ`RuntimeRegistry`で追跡する。baseは`integration/mypage-canon-20261006`、Auth/関連record adapterとは独立したPR。既存の29 step、stable operation ID、SAC、privacy retention、CLIの`--execute`拒否、live infrastructure gateは変更しない。

`NewMyPageServer`は必ずregistryを作成または注入し、HTTP、Auth、BFF、標準metadata refreshへ同じinstanceを渡す。Cloud Run entrypointも明示的に注入する。libraryのnil registryは既存caller互換のためであり、runtime evidenceを発行する構成として使わない。別途起動する`MetadataCleanupJob`は同じprocessのregistryを明示注入する必要がある。cleanupの実scheduler/bootstrapはこのsliceに追加しない。

## 追跡する仕事

- callbackはchannel未確定のprovider Resolve中も追跡し、どの対象のdrainにも未帰属件数を含める。channel判明後に対象へbindする。別channelへbindした仕事を削除対象の件数へ残さない。
- callbackのClaim/Verify、login ConfirmのConsume、外部mint、support Confirm、session completion、metadata更新/clear、cleanup Clearを実呼び出しの前から追跡する。atomic Firestore操作の外側でSDKを呼ぶ既存順序を維持する。
- HTTP requestはlibraryからtoken/snapshotが返った後も、JSON serialization、ResponseWriterのWrite、標準net/httpのbufferを出すFlushまでticketを保持する。失敗したwrite/flushは配達不明としてpendingにする。
- BFFの共有flightはcallerのキャンセルから独立したticketを持ち、実dependencyの帰還まで残す。cleanupはキャンセルを切り離した匿名observationの記録終了まで追跡する。

pending external SDK、lost commit acknowledgment、dependencyが返す`ErrRuntimeOutcomeUnknown`、不確定なキャンセル結果は、functionのreturnやbudget経過だけでcompleteにしない。registryはこれらをsticky pendingとして保持し、drain/resumeを拒否する。実Google providerと公開metadata providerは内部timeout、transport failure、channel body-read failureも生の依存errorを保持しないunknown markerで伝える。完全に読み取ったHTTP/JSON/不存在の判定は通常のerrorとして扱う。確定した`invalid_grant`等のOAuth拒否・atomic storeのvalidation/access拒否は従来の失敗記録を残す。公開response/observationへchannel、opaque内部参照、SDK raw errorを出さない。

## cutoff と再開

pauseと結果publicationには同じcancellable gateを使う。すでに許可したstore/writeが帰るまではpauseのcutoffをacknowledgeできず、その間も件数を観測できる。pauseのacknowledgment後は、古いticketからmetadata/cache/HTTP結果をpublishしない。caller contextから切り離した子仕事も、元のadmission世代を引き継ぐ。

BFFのcacheとflightはregistry世代も照合し、resume後の古いcacheや結果を再利用しない。OAuth callbackはClaim前にserver-only transactionの作成時刻を読み、channel判明後にcutoffへ照合する。Channel/Confirmも永続化された作成時刻を照合する。resume後に初めて届く旧transactionも拒否し、cutoff後に明示的に開始した新しいOAuthは許可する。callback provenance readerや作成時刻が欠ける場合はfail closed。

同じcaseのresume retryは新しい仕事をpauseしない。guardで確認された新ownerは厳密に新しいrevisionで受け付け、cutoff/barrier/件数を維持する。resume replayでもowner/revision/generationのhigh-waterを更新して旧owner/旧generationを拒否する。同channelの後日の別caseは、前caseのresume、より新しいcutoff/generation、独立guardを必要とし、新barrierを作る。旧caseのpause/drain/resumeは新caseへ流用できない。

## evidence と未完了gate

`RuntimeDrainEffects`は既存`supportdelete.Effects`を実装する。selector、manifest、stable operation ID、step、generation、cutoff、時刻を照合した独立注入の`FleetGuard`が必須。local件数0だけから`AllInstances`を作らず、missing guard、不完全fleet/manifestではconsumable evidenceを返さない。pauseは追跡中の仕事を残して停止barrierを設定し、drain/resumeは未帰属callbackやactive/pending件数があれば拒否する。guardの確認後にだけlocal controlを変更する。

このregistryはprocess-memoryであり、再起動した空registryを旧instance停止の証明として扱えない。旧revision/全worker/外部SDK/queue/import/restoreのinventoryと停止保証は独立fleet/manifest guardの責務。sticky unknownのlive reconciliation、実scheduler全体の接続、実SDKとfleetの停止保証は未実装・未確認のまま。待ち時間、別processのlocal件数、Firestore不存在でそのgateを置き換えない。live CLI/bootstrapや実userdataへの操作は追加していない。

## 検証

`runtime_drain_test.go`はchannel同期のfake依存でcallback、mint、HTTP配達、metadata、cleanup、detached BFF、cache/replay、owner recovery、複数mock workerと二度目の削除を駆動する。`runtime_drain_integration_test.go`はdemo Firestore Emulatorで実OAuth/Consume/metadata/cleanup commitとfake provider/mintの境界を確認する。

```sh
cd system
go test -shuffle=on ./core/mypage ./cmd/mypage-server
go test -race -shuffle=on ./core/mypage ./cmd/mypage-server
# repository root: demo emulatorのみ
bash .github/scripts/run-firestore-integration-tests.sh
```

合成試験の成功は、実環境のfleet drain・Auth削除・全copy消去完了の証跡ではない。
