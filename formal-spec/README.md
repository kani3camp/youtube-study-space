# Formal Specification

このディレクトリは、状態遷移や業務上の不変条件を**実行可能な仕様**として管理するための、ツール非依存の境界です。

現在の実装には FSL (`formal-spec/fsl/`) を採用していますが、Formal Specification 自体をアーキテクチャ上の概念とし、FSL は交換可能な実装として扱います。

## 方針

- 本番コードから formal-spec のツールやランタイムを import / invoke しない。
- Firestore のスキーマやデプロイ成果物に formal-spec 固有情報を持ち込まない。
- 仕様は実装のコピーではなく、守りたい状態遷移・不変条件を表現する。
- 生成物、検証用バイナリ、HTML レポートなどはコミットしない。
- 将来別の形式手法へ移行するときは、このディレクトリ配下の実装と専用 CI を交換できる構造を保つ。

## 仕様と実装が食い違ったとき

CI を緑にすること自体を目的に仕様を弱めてはいけません。まず差分を次のいずれかに分類します。

1. 実装の不具合
2. 仕様・モデルの不具合
3. 仕様と実装を結ぶアダプター／検証基盤の不具合

理由なく invariant / action / requires を削除・緩和したり、検証 depth を下げたり、失敗ケースを skip する変更は禁止します。振る舞いの契約を変える場合は、PR に変更理由を明記します。

## 現在の構成

- `fsl/`: FSL による形式仕様、検証、language-neutral conformance vector の生成
- `system/core/repository/seat_formalspec_test.go`: `formalspec` build tag のときだけ有効になる Go conformance adapter
- 最初の対象: `SeatSession`（座席の Work / Break 状態遷移）

CI では FSL が生成した conformance vector を oracle として `SeatDoc` の実装を照合します。vector は一時生成物でコミットせず、通常の `go test ./...` と本番ランタイムは FSL 非依存のまま維持します。

将来 FSL を置き換える場合は、`formal-spec/fsl/` と build tag 付き adapter / 専用 CI の入力部分だけを交換し、本番コードや Firestore schema には波及させません。
