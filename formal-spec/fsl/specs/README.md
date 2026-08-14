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
