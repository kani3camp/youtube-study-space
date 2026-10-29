# Specifications

## `seat_session.fsl`

`system/core/repository/seat.go` の `SeatDoc` が担う Work / Break 状態遷移のうち、業務上重要な部分を有限モデルとして表現します。

### v1 でモデル化するもの

- Work と Break の状態
- Work からの休憩開始
- Break からの作業再開
- 作業終了時刻の変更・延長
- 休憩終了時刻の延長と、必要な自動退室時刻の延長
- Break の終了が自動退室時刻を超えないこと
- Work 中は状態の終了時刻と自動退室時刻が一致すること
- 状態／セグメント開始時刻が現在時刻より未来にならないこと
- 累積作業時間が負にならないこと

`tick` は時間経過を表現する検証ハーネス用 action で、Go のドメインメソッドには直接対応しません。

### v1 で意図的にモデル化しないもの

- `WorkName` 等の文字列
- Firestore のフィールド名や serialization
- i18n / エラーメッセージ文言
- JST の日付跨ぎを含む `DailyCumulativeWorkSec`
- `GenerateWorkSegment`
- 各メソッドの表示用戻り値の細部

実装を丸ごと転記するのではなく、壊したくない状態機械の契約に絞るためです。未モデル化部分は既存 Go テストで引き続き保証します。

## `seat_allocation.fsl`

`WorkspaceApp.In` / `IsUserInRoom` / `IfSeatVacant` / `moveSeat` / `enterRoom` / `exitRoom` にまたがる座席割当の安全条件を、小さな有限モデルとして表現します。

モデル上は 2つの一般席と 1つのメンバー席、1人のメンバー利用者と1人の非メンバー利用者を置きます。これは実際の席数を再現するためではなく、衝突・権限・入退室・席移動の不変条件を最小の状態空間で検証するためです。

### v1 でモデル化するもの

- 入室先・移動先は空席であること
- 非メンバー利用者はメンバー席へ入れないこと
- 入室は退室中の利用者だけが行えること
- 席移動は入室中かつ現在と異なる座席だけを対象とすること
- 退室は入室中の利用者だけが行えること
- 1つの座席を2人が同時に占有しないこと
- メンバー利用者がメンバー席を利用している状態が到達可能であること
- 非メンバー利用者が第2一般席を利用している状態が到達可能であること
- 2人が異なる座席を同時に利用している状態が到達可能であること

`member_move` / `general_move` action は移動時の許可条件と遷移結果を定義します。一方、`reachable` は特定の座席状態へ到達できることだけを主張し、どの action 列で到達したかまでは主張しません。

1ユーザーが同時に複数座席を持てないことは、ユーザーごとに現在位置を1つの scalar state として持つモデル構造自体で表現しています。実装側でも `IsUserInRoom` は一般席・メンバー席への二重在席を異常として扱います。

### v1 で意図的にモデル化しないもの

- 実際の `MaxSeats` / `MemberMaxSeats` の可変値
- ランダム・最小空席選択アルゴリズム
- 長時間着席制限の white / black list
- 作業時間・RP・作業名・メニューなど、席移動に付随して引き継ぐ属性
- Firestore transaction / retry の実装詳細
- activity / work segment ログ

この仕様では「どの席を選ぶか」ではなく、「選んだ席へ遷移してよい条件」と「遷移後も壊れてはいけない安全条件」に焦点を当てます。


## MyPage の安全モデル

[MyPage 実装契約](../../../docs/mypage/README.md) と [ServiceAccessControl 契約](../../../docs/mypage/service-access-control.md) のうち、認証・本人切替・制限の境界を3つの独立モデルに分けます。全仕様は既存の `specs/*.fsl` 検証/reportループに入り、`check` / depth 8・`--vacuity error` の bounded verification / HTML report を実行します。既存の座席仕様・検証深度は変更しません。

**MyPage の3仕様は実装 conformance 未接続です。** 下表の実装/テストは追跡先であり、FSL vectorを入力するadapterではありません。モデル同士のcompositionや実際のOAuth/Firestore/ブラウザの動作を証明したとも扱いません。

| モデル / requirement | 対応実装 | 既存の実装検証 |
| --- | --- | --- |
| `MyPageOAuth` / `REQ-MYPAGE-AUTH-*` | [AuthService](../../../system/core/mypage/auth.go)、[AuthStore](../../../system/core/mypage/auth_store.go)、[support auth store](../../../system/core/mypage/support_auth_store.go) | [auth integration](../../../system/core/mypage/auth_integration_test.go)、[support auth integration](../../../system/core/mypage/support_auth_integration_test.go)、[access integration](../../../system/core/mypage/access_integration_test.go) |
| `MyPageSession` / `REQ-MYPAGE-SESSION-*` | [MyPageMemory](../../../mypage/src/features/mypage/memory.ts) | [memory tests](../../../mypage/src/features/mypage/memory.test.ts)、[runtime tests](../../../mypage/src/runtime.test.ts) |
| `MyPageAccess` / `REQ-MYPAGE-ACCESS-*` | [SAC Transition / Allowed](../../../system/core/serviceaccess/control.go)、[accessVerdict](../../../system/core/mypage/access.go) | [SAC tests](../../../system/core/serviceaccess/control_test.go)、[SAC integration](../../../system/core/serviceaccess/firestore_integration_test.go)、[access tests](../../../system/core/mypage/access_test.go) |

### `mypage_oauth.fsl`

1つのOAuth transactionを `Idle -> Pending -> Processing -> Verified -> Consumed` と追い、途中失敗・期限切れ・同意版変更・login guard失効を許します。目的は作成後に変わらず、`Login` と `Support` の2種類です。

- callbackは期限内・state一致で一度だけclaimし、唯一のchannelを確認した後だけconfirmへ進む。
- login confirmはconfirmation・同意版・login guardを確認し、consumeとaccount更新を同じatomic actionで行う。
- support confirmはrequest/channel/environment等のbinding一致を確認し、proofだけを作る。account更新・Custom Token mintは行わず、login側の制限時にも利用できる。
- consume後に外部mintを一度だけ試み、失敗/ACK不明でもconsumeを巻き戻さない。mint中にguardが失効したらtokenを返さない。
- `ensures` は操作時点の期限・入力・guardを検査する。consume後の期限切れや、tokenを返した後のblockを過去の違反にはしない。

`unexpired` は10分の期限判定、`policy_current` は同意版一致、`login_guard_current` は各login gate/checkpointの有効性を射影した値です。失効したcaptured guardは元に戻さず、再ログインは別transactionとして扱います。`unique_channel` / `binding_matches` / confirmation等は真偽値へ抽象化し、SDK・暗号・実channel選択・Firestore rollback/retryの詳細はモデル化しません。既存Emulatorテストの原子性検証を置き換えません。

`Failed` とconsume後の処理完了/guard失効による停止は意図した終端です。reachableはlogin成功、mint失敗、外部mint後のguard失効による拒否、login制限とsupport proofの併存を確認します。mint後の拒否は専用ghost flagで外部処理前の失敗と区別します。再認証や処理完了のlivenessは主張しません。

### `mypage_session.fsl`

2人の本人と2つのrequest slotを使い、古いrequestがabortを無視して完了する場合も、新セッションの成功応答と並行できるようにします。同じepochでの通常取得は1本に制限し、epochは循環させません。`data_uid` / `data_epoch` は表示値の帰属を検査するghost stateです。値は0/1へ射影し、`data_uid=0`の未取得と、本人に紐付いた正常な0を区別します。

- 本人切替・logout・pagehide・maintenance・制限403はprivate dataを同期破棄しepochを進める。
- 応答は本人とepochが一致する場合だけ適用し、古い成功・古い制限403が新セッションを汚さない。
- restricted/maintenanceでは新規取得を開始しない。同じ本人のrestrictedはpagehideだけでは解除せず、明示fresh loginを必要とする。
- 通常503は同じ本人の直前の成功値を保持し、値のない状態も値のある状態も取り得る。失敗actionはdataを新しい0で上書きしない。

logoutの同期破棄部分、pagehideの停止、maintenanceの停止/再開をモデル化します。maintenance終了時は現行実装どおり本人を再評価するため、maintenanceを跨ぐrestrictedのラッチ保持はこのモデルの保証対象ではありません。非同期signOut完了・signOut失敗・Auth listener抑止、5分polling・backoff/Retry-After・forced-token/App Check retry、section単位のstale/metadata期限切れ・履歴/描画はモデル外です。request slot数は本番上限ではなく、旧応答と新応答の衝突を検査するための検証範囲です。

reachableは成功値と正常な0、同一uidのepoch変更中の旧request、新成功値と旧requestの共存、一時障害時の値保持、制限状態が到達可能なことを確認します。

### `mypage_access.fsl`

1channelの `moderation` / `privacyDeletion` と1つのcaptured requestを追います。record不存在/全inactiveの許可を初期状態に射影し、理由の有効/無効の実変更ごとにcheckpointを増やします。同じ状態の再適用はcheckpointを変えません。

- 両理由は独立し、moderation解除でprivacyDeletionを消さず、privacy解除でmoderationを消さない。
- 処理開始時にfreshな許可checkpointをcaptureし、保護されたread/commit/publishの直前に再照合する。
- store取得/decode障害は拒否する。deletionをmoderationより優先し、両理由が解除されていても古いcheckpointの処理は成功させない。
- `ensures` はfinish時点の許可条件・優先順位を検査する。その後のblockを遡及適用することや、全writer/instanceのdrainを主張しない。

実checkpointのgeneration + Firestore revisionは単調な整数へ、取得/decode/schema障害は`readable=false`へ抽象化します。record削除/再作成・counter再利用は許しません。同じactive状態でのreason code/ref変更、CAS・実cache/flight key、[削除B](../../../docs/mypage/support-delete.md) の全29step・所有権recovery・SLA・実データ不存在・再生成禁止はモデル外です。モデルの成功をlive削除完了や配備承認とは扱いません。

reachableは通常成功、独立した両理由の併存、処理中の制限、両理由の表示優先、block→unblock後の古いcheckpoint拒否を確認します。reachableは到達状態の主張であり、特定action列の義務や実行完了の保証ではありません。
