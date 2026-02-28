package moderatorbot

import "context"

// DummyMessageBot は何も送信しない MessageBot の実装。
// テストや通知が不要な場合に利用する。
type DummyMessageBot struct{}

// SendMessage implements MessageBot.
func (DummyMessageBot) SendMessage(context.Context, string) error {
	return nil
}

// SendMessageWithError implements MessageBot.
func (DummyMessageBot) SendMessageWithError(context.Context, string, error) error {
	return nil
}
