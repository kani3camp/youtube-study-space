package moderatorbot

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 送信リクエストには呼び出し元の context が渡り、キャンセル済みなら送信せずにそのエラーを返す。
func TestDiscordBotSendMessageHonorsContext(t *testing.T) {
	bot, err := NewDiscordBot("synthetic-token", "synthetic-channel")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = bot.SendMessage(ctx, "message")

	require.ErrorIs(t, err, context.Canceled)
}
