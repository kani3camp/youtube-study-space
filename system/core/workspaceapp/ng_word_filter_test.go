package workspaceapp

import (
	"context"
	"strings"
	"testing"
	"time"

	"app.modules/core/timeutil"
	mock_youtubebot "app.modules/core/youtubebot/mocks"
	"go.uber.org/mock/gomock"
)

type spyMessageBot struct {
	messages          []string
	messagesWithError []string
}

func (b *spyMessageBot) SendMessage(_ context.Context, message string) error {
	b.messages = append(b.messages, message)
	return nil
}

func (b *spyMessageBot) SendMessageWithError(_ context.Context, message string, _ error) error {
	b.messagesWithError = append(b.messagesWithError, message)
	return nil
}

func TestCheckIfUnwantedWordIncluded_BlocksByChatMessageRegex(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)
	mockLiveChatBot.EXPECT().
		BanUser(gomock.Any(), "test_user_id").
		Return(nil).
		Times(1)

	logBot := &spyMessageBot{}
	alertBot := &spyMessageBot{}
	app := newTestNGWordFilterApp(mockLiveChatBot, logBot, alertBot)
	ngWordConfig := NewNGWordConfig(
		[]string{"荒らし"},
		nil,
		nil,
		nil,
	)

	blocked, err := app.CheckIfUnwantedWordIncluded(
		ctx,
		ngWordConfig,
		"test_user_id",
		"これは荒らしです",
		"テストユーザー",
	)

	if err != nil {
		t.Fatalf("CheckIfUnwantedWordIncluded() error = %v", err)
	}
	if !blocked {
		t.Fatal("blocked = false, want true")
	}
	if got, want := len(logBot.messages), 1; got != want {
		t.Fatalf("log messages len = %d, want %d", got, want)
	}
	if !strings.Contains(logBot.messages[0], "禁止ワード: `荒らし`") {
		t.Fatalf("log message does not contain matched regex: %q", logBot.messages[0])
	}
	if got, want := len(alertBot.messages), 0; got != want {
		t.Fatalf("alert messages len = %d, want %d", got, want)
	}
}

func TestCheckIfUnwantedWordIncluded_NotifiesByChatMessageRegex(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)

	logBot := &spyMessageBot{}
	alertBot := &spyMessageBot{}
	app := newTestNGWordFilterApp(mockLiveChatBot, logBot, alertBot)
	ngWordConfig := NewNGWordConfig(
		nil,
		nil,
		[]string{"要確認"},
		nil,
	)

	blocked, err := app.CheckIfUnwantedWordIncluded(
		ctx,
		ngWordConfig,
		"test_user_id",
		"これは要確認です",
		"テストユーザー",
	)

	if err != nil {
		t.Fatalf("CheckIfUnwantedWordIncluded() error = %v", err)
	}
	if blocked {
		t.Fatal("blocked = true, want false")
	}
	if got, want := len(alertBot.messages), 1; got != want {
		t.Fatalf("alert messages len = %d, want %d", got, want)
	}
	if !strings.Contains(alertBot.messages[0], "禁止ワード: `要確認`") {
		t.Fatalf("alert message does not contain matched regex: %q", alertBot.messages[0])
	}
	if got, want := len(logBot.messages), 0; got != want {
		t.Fatalf("log messages len = %d, want %d", got, want)
	}
}

func TestCheckIfUnwantedWordIncluded_NoMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)

	logBot := &spyMessageBot{}
	alertBot := &spyMessageBot{}
	app := newTestNGWordFilterApp(mockLiveChatBot, logBot, alertBot)
	ngWordConfig := NewNGWordConfig(
		[]string{"荒らし"},
		[]string{"スパム"},
		[]string{"要確認"},
		[]string{"注意"},
	)

	blocked, err := app.CheckIfUnwantedWordIncluded(
		ctx,
		ngWordConfig,
		"test_user_id",
		"通常のメッセージです",
		"テストユーザー",
	)

	if err != nil {
		t.Fatalf("CheckIfUnwantedWordIncluded() error = %v", err)
	}
	if blocked {
		t.Fatal("blocked = true, want false")
	}
	if got, want := len(logBot.messages), 0; got != want {
		t.Fatalf("log messages len = %d, want %d", got, want)
	}
	if got, want := len(alertBot.messages), 0; got != want {
		t.Fatalf("alert messages len = %d, want %d", got, want)
	}
}

func newTestNGWordFilterApp(
	liveChatBot *mock_youtubebot.MockLiveChatBot,
	logBot *spyMessageBot,
	alertBot *spyMessageBot,
) WorkspaceApp {
	fixedNow := time.Date(2026, time.January, 1, 10, 0, 0, 0, timeutil.JapanLocation())

	return WorkspaceApp{
		LiveChatBot:        liveChatBot,
		alertModeratorsBot: alertBot,
		logModeratorsBot:   logBot,
		nowFunc:            func() time.Time { return fixedNow },
	}
}
