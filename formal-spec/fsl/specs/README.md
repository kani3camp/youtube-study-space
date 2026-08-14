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

- `WorkName` / `BreakWorkName` 等の文字列
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

## `chat_message_processing.fsl`

P1「YouTubeチャット処理を冪等化・耐障害化」で目標としている、1件の**副作用ありメッセージ**の durable ingest → Inbox processing → domain transaction → reply outbox の契約を有限モデルとして表現します。

これは現行 `youtube-bot` の挙動を写した conformance spec ではありません。現行は `nextPageToken` 保存後に履歴保存・`ProcessMessage` を行い、`live-chat-history` もランダム document ID を使っています。P1 の移行先を先に固定し、実装PRでこの契約へ近づけます。

### v1 でモデル化するもの

- checkpoint/cursor は、メッセージの command 処理完了ではなく Inbox + History へ安全に永続化できたことを意味する
- Inbox/History/checkpoint は1つの ingest transaction で同時に確定する
- Pending message を lease 付きで Processing に claim する
- lease 失効時は未完了 message を Pending に戻して再取得できる
- 同一 source message の domain effect は論理的に高々1回
- domain effect と reply outbox intent は処理成功 transaction で同時に確定する
- 処理失敗は再試行でき、上限到達後は dead-letter にする
- dead-letter は checkpoint を巻き戻さず、後続 ingest を止めない
- reply delivery は永続化済み outbox intent がある場合だけ許可する

`FailureCount = 0..2` は bounded model で retry / dead-letter の両経路を検証するための代表値です。本番の最大試行回数を2回に固定する仕様ではありません。

### v1 で意図的にモデル化しないもの

- YouTube API の page token 文字列や polling interval
- message payload / author / membership / NG word 判定の内容
- 非コマンドや副作用なしコマンドの細かな分岐
- Firestore collection 名・field 名・index
- worker lease の秒数、retry backoff、最大試行回数の具体値
- outbox の外部配送で起こり得る「送信成功後、Delivered記録前のクラッシュ」による重複配送
- 複数メッセージ間の順序・並列度

外部 YouTube/Discord 配送まで exactly-once と主張しません。P1 の狙いは、Firestore 内の message ledger / domain side effect / reply intent を冪等に管理し、外部配送は再試行可能な outbox として分離することです。
