# ServiceAccessControl — A

2026-10-07の[Canon 04](https://app.notion.com/p/3ec357a8d0ce812f8520f787fedcc6bd)に従い、`service-access-controls/{channelId}`をUser/WebAccountから独立したserver-only認可storeとして使う。MyPageが最初の利用先。既存Botの`!block`・YouTube Ban・通常writerは接続しない。

## Storage / response contract

channelは`UC`と22文字のYouTube ID。recordは`generation`（1以上の単調増加integer）、`updatedAt`、`moderation`、`privacyDeletion`の4fieldだけを持つ。各reasonは`active`を必須とし、active時のみ`since`、固定`reasonCode`、64文字lowercase hexのopaque参照を持つ。moderationは`actionRef`、privacyDeletionは`requestRef`。表示名・作業名・自由文・連絡先を保存しない。

moderation codeは`MODERATION`、`SECURITY`、`POLICY_VIOLATION`、`LEGACY_COMPATIBILITY`。privacyDeletion codeは`PRIVACY_DELETION`。各reasonは独立し、同時activeも許可する。同じstate/code/refの再適用はrecord revisionとgenerationを変えない。参照変更を含む実変更はgenerationを増やす。

record不存在と全inactiveは許可。取得/decode/schema障害は`503 TEMPORARY_UNAVAILABLE`、moderation activeは`403 SERVICE_ACCESS_RESTRICTED`、privacyDeletion activeは`403 DATA_DELETION_IN_PROGRESS`。両activeではdeletion表示を優先する。内部reason、channel、参照をAPIへ出さない。旧WebAccount `accessBlocked`はdecode互換のみ。

## MyPage boundaries

verified custom-provider uidを使い、WebAccount/cacheより前にfresh document readを行う。正常channel確認、login consume、Custom Token mint、session completion、metadata refreshにも適用する。support proofのchannel確認・confirmとpublic法的文面は利用できる。

OAuth consume/account upsert、firstWebLoginAt、metadata更新/clearは同じFirestore transactionでcontrolを読み、captured checkpointを照合してからwriteする。controlと競合したtransactionは再評価される。OAuth/YouTube/token mintはretryable transactionの外で実行し、mint前後にもfresh照合する。consume後のmint失敗や拒否でtransactionを再利用しない。trusted scheduled metadata cleanupは削除処理を継続できる。

checkpointはgenerationとFirestore update timeを含む。cache/flight keyにcheckpointを含め、cache consult・detached work・publish・response/fallbackでfresh照合する。inactive permissionをcacheしない。制限store障害は通常metadata provider障害と型を分け、partial/stale 200へ変換しない。

frontendは制限403でcurrent/stats/channel metadataを同期破棄し、epoch更新とabortで旧resultを拒否する。poll/manual/visibility retryを止め、専用message、logout、legal/support、明示fresh loginを残す。同じuidのbfcache復帰でも制限を維持する。通常503の既存backoff/stale動作は継続する。

## Trusted moderation CLI

entrypointは`system/cmd/service-access-control`。browser admin APIは追加しない。`--stdin`またはprivate regular fileの`--manifest`を使い、16KiBのstrict JSONを読む。defaultは通信なしplan、`--check-config`もoffline。`--execute`でSDKを使い、productionは`--allow-production`も必要。

全field必須のpayloadは`schemaVersion:1`、`target:{environment,projectID,channelID}`、`operation`（block/unblock/inspect）、`actionRef`（64lowercase hex）、`reasonCode`、`confirmation:{environment,projectID,channelID,operation,actionRef}`。confirmationは完全一致。blockだけ固定reasonCodeを要求し、unblock/inspectは空文字。未知・重複・null・大小文字違い・余分なJSONを拒否する。

実行targetはtrusted processの`MYPAGE_ENVIRONMENT`と`GOOGLE_CLOUD_PROJECT`へbindする。任意project aliasの不一致、credential file、emulator redirectを拒否し、selected projectのkeyless metadata identityだけを許可する。runtime権限と実環境結線は別途確認する。

blockはcontrol commit/検証後にFirebase session revokeを補助的に試みる。revoke失敗でguardを戻さず、再実行でrevokeを再試行する。unblockはmoderationだけをclearし、privacyDeletionを保持する。unblock actionRefは当該operationのbindingであり、既存blockへのcompare-and-swapではない。

reportは固定status/code/state、generationと検証済み固定reasonだけを出す。target、ref、path、timestamp、raw SDK errorを出さない。exitは0=成功/plan/config正常、2=拒否、1=取得不能/output失敗、3=control保存後のrevoke部分失敗。既存emulator-only `mypage-operator-dryrun`は維持する。

## Verification and remaining decisions

通常Go tests、race、Firestore Emulatorのatomicity/reason競合/旧token/metadata/token競合、anonymous・合成authenticated clientのcontrol read/write deny、frontend testsと実Chromium合成QAで検証する。実SDK・実userdata・deploy・migrationはこの受入に含めない。

Aはcontrol削除・TTL・保持期限を追加せず、unblock後のinactive checkpointを維持する。これは保持期限のpolicy決定ではない。不存在→block→外部delete→不存在のABAを防ぐ世代非再利用、全revision/instance/SDKのdrain、legacy再開後の新しい利用とのcutoff、実行時fresh proofの許容ageはBの判断事項。MyPage budgetの時間待ちだけでdrain完了を主張しない。

Bはsupport受付完了と関連OAuth cleanup、Bot/座席整理/定時・手動runner/BQ import、作業名派生・vendor/log/backupsを含む横断inventoryと閉路検証を要する。vendor/logの実保持・権限は未確認。D02の設計選択は2026-10-09確定済みで規約適合性確認が残る。D03の実値、CodeQL/securityと実環境release gateも残る。
