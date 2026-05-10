## 背景

`youtube_organize_database` の ERROR ログ通知で、YouTube Live Chat 投稿失敗時のログが Discord に通知された。

実際のログでは、1回目の投稿失敗後にリトライしており、`first post failed` は最終失敗ではなく中間失敗である。

```json
{
  "level": "ERROR",
  "msg": "first post failed",
  "err": "googleapi: Error 403: The specified live chat is no longer live., liveChatEnded"
}
```

現在の運用では ERROR ログが Discord 通知のトリガーになるため、リトライ途中の失敗まで ERROR にすると通知ノイズになりやすい。

## 現状の `postMessage` の流れ（実装に合わせた整理）

`system/core/youtubebot/live_chat.go` の `postMessage` は次の順で試行する。

1. `tryPostMessage`（1回目）
2. 失敗時: `tryPostMessage`（2回目・同一 `LiveChatID`）
3. まだ失敗時: `refreshLiveChatID` で ID 更新のうえ `tryPostMessage`（3回目）

したがって **「2回目失敗＝最終」ではない**。最終的な投稿失敗は **3回目の `tryPostMessage` が失敗したとき**（または `refreshLiveChatID` が失敗して 3 回目に到達できないとき）として扱う。

## 方針

`postMessage`（および必要なら同一ファイル内で同様のパターン）のログレベルを以下のように整理する。

| 状況 | ログレベル | 理由 |
| --- | --- | --- |
| 1回目の投稿失敗、2回目試行前 | WARN | まだ復旧可能な中間失敗のため |
| 2回目の投稿失敗、`refreshLiveChatID` 前 | WARN | まだ Live Chat ID 更新と 3 回目試行が残る中間失敗のため |
| `refreshLiveChatID` の失敗 | ERROR（現状維持でよい） | 以降の投稿試行に進めないため |
| 3回目の投稿失敗、かつ `liveChatEnded` 以外 | ERROR | 試行を尽くしたうえでの予期しない最終失敗のため |
| 3回目の投稿失敗、かつ `liveChatEnded` | WARN | 投稿先の Live Chat が終了している状態であり、アプリ障害として扱う必要性が低いため |

## 実装案

`googleapi.Error` を `errors.As` で取り出し、**`Errors` 配列の `Reason` が `liveChatEnded` かどうかを主に判定する** helper を追加する（**HTTP ステータスが 403 であることだけ**で `liveChatEnded` とみなさない）。

```go
package youtubebot

import (
	"errors"
	"strings"

	"google.golang.org/api/googleapi"
)

func isLiveChatEndedError(err error) bool {
	if err == nil {
		return false
	}

	var googleErr *googleapi.Error
	if !errors.As(err, &googleErr) {
		return false
	}

	if googleErr.Code != 403 {
		return false
	}

	for _, item := range googleErr.Errors {
		if item.Reason == "liveChatEnded" {
			return true
		}
	}

	// googleapi.Error.Errors が空のケースへの保険。
	// 原則は Reason 判定を優先する。
	return strings.Contains(googleErr.Message, "liveChatEnded") ||
		strings.Contains(googleErr.Body, "liveChatEnded")
}
```

呼び出し側のイメージ（実際の 1回目→2回目→refresh→3回目 に対応）:

```go
err := b.tryPostMessage(message, b.LiveChatID)
if err == nil {
	return nil
}
slog.Warn("first post failed; retrying", "err", err)

err = b.tryPostMessage(message, b.LiveChatID)
if err == nil {
	return nil
}
slog.Warn("second post failed; refreshing live chat id", "err", err)

if err := b.refreshLiveChatID(ctx); err != nil {
	return err
}

err = b.tryPostMessage(message, b.LiveChatID)
if err != nil {
	if isLiveChatEndedError(err) {
		slog.Warn("third post failed; live chat ended", "err", err)
		return nil // 呼び出し元で「終了により投稿不要」と扱うなら nil。エラーとして伝播させるなら専用エラーなど要設計。
	}
	slog.Error("third post failed", "err", err)
	return err
}
```

既存コードが `slog.Error`（Context なし）を使っている箇所が多い場合は、`WarnContext` / `ErrorContext` への統一は別タスクでもよい（本 Issue ではレベルとメッセージの整理を優先する）。

## 受け入れ条件

- `first post failed` が `ERROR` ではなく `WARN` で出力される
- `second post failed` が、まだ 3 回目試行が残る前提で `ERROR` ではなく `WARN` で出力される（メッセージ文言は実装で調整可）
- **3回目の `tryPostMessage` が失敗したとき**を「投稿としての最終失敗」とし、`liveChatEnded` の場合は `WARN`、それ以外は `ERROR` とする
- `liveChatEnded` 以外の最終失敗は `ERROR` のままにする
- `403` だけで `liveChatEnded` 判定しない（`Reason` またはフォールバック文字列で裏付ける）
- `isLiveChatEndedError` の単体テストを追加する
- ERROR ログ通知が不要に増えないことを確認する

## 補足

24時間配信として `liveChatEnded` 自体を検知したい場合は、`postMessage` の ERROR ではなく、`check_live_stream_status` 系の監視で拾う方がよい。
