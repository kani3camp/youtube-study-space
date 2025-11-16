package workspaceapp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"app.modules/core/i18n"
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/repository"
	"app.modules/core/utils"
	"app.modules/core/youtubebot"
	"github.com/pkg/errors"
)

type WorkspaceApp struct {
	Configs    *Configs
	Repository repository.Repository
	LiveChatBot youtubebot.LiveChatBot

	ProcessedUserId          string
	ProcessedUserDisplayName string
	ProcessedUserProfileImageUrl string
}

// Configs WorkspaceApp生成時に初期化すべきフィールド値
type Configs struct {
	Constants repository.ConstantsConfigDoc

	LiveChatBotChannelId string
}

func NewWorkspaceApp(ctx context.Context) (*WorkspaceApp, error) {
	if err := i18n.LoadLocaleFolderFS(); err != nil {
		return nil, fmt.Errorf("in LoadLocaleFolderFS(): %w", err)
	}

	userRepository := repository.NewUserRepository()
	studySessionRepository := repository.NewStudySessionRepository()

	// YouTube live chatbot
	liveChatBot, err := youtubebot.NewYoutubeLiveChatBot("", nil, ctx)
	if err != nil {
		return nil, fmt.Errorf("in NewYoutubeLiveChatBot(): %w", err)
	}

	// core constant values
	constantsConfig, err := mongoController.ReadSystemConstantsConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("in ReadSystemConstantsConfig(): %w", err)
	}

	configs := Configs{
		Constants:            constantsConfig,
		LiveChatBotChannelId: "",
	}

	return &WorkspaceApp{
		Configs:    &configs,
		Repository: nil, // This will be handled by the individual repositories
		LiveChatBot: liveChatBot,
	}, nil
}

func (app *WorkspaceApp) RunTransaction(ctx context.Context, f func(ctx context.Context) error) error {
	return app.Repository.RunTransaction(ctx, f)
}

func (app *WorkspaceApp) SetProcessedUser(userId string, userDisplayName string, userProfileImageUrl string) {
	app.ProcessedUserId = userId
	app.ProcessedUserDisplayName = userDisplayName
	app.ProcessedUserProfileImageUrl = userProfileImageUrl
}

func (app *WorkspaceApp) GetInfoString() string {
	return ""
}

// GoroutineCheckLongTimeSitting 長時間座席占有検出ループ
func (app *WorkspaceApp) GoroutineCheckLongTimeSitting(ctx context.Context) {
	minimumInterval := time.Duration(app.Configs.Constants.MinimumCheckLongTimeSittingIntervalMinutes) * time.Minute
	slog.Info("", "居座りチェックの最小間隔", minimumInterval)

	for {
		slog.Info("checking long time sitting.")
		start := utils.JstNow()

		{
			if err := app.CheckLongTimeSitting(ctx, true); err != nil {
				app.MessageToOwnerWithError(ctx, "in CheckLongTimeSitting", err)
			}
		}
		{
			if err := app.CheckLongTimeSitting(ctx, false); err != nil {
				app.MessageToOwnerWithError(ctx, "in CheckLongTimeSitting", err)
			}
		}

		end := utils.JstNow()
		duration := end.Sub(start)
		if duration < minimumInterval {
			time.Sleep(utils.NoNegativeDuration(minimumInterval - duration))
		}
	}
}

func (app *WorkspaceApp) CheckIfUnwantedWordIncluded(ctx context.Context, userId, message, channelName string) (bool, error) {
	return false, nil
}

// ProcessMessage 入力コマンドを解析して実行
func (app *WorkspaceApp) ProcessMessage(
	ctx context.Context,
	commandString string,
	userId string,
	userDisplayName string,
	userProfileImageUrl string,
) error {
	if userId == app.Configs.LiveChatBotChannelId {
		return nil
	}
	app.SetProcessedUser(userId, userDisplayName, userProfileImageUrl)

	// 初回の利用の場合はユーザーデータを初期化
	txErr := app.RunTransaction(ctx, func(ctx context.Context) error {
		isRegistered, err := app.IfUserRegistered(ctx)
		if err != nil {
			return fmt.Errorf("in IfUserRegistered(): %w", err)
		}
		if !isRegistered {
			if err := app.CreateUser(ctx); err != nil {
				return fmt.Errorf("in CreateUser(): %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		app.MessageToLiveChat(ctx, i18nmsg.CommandError(app.ProcessedUserDisplayName))
		return fmt.Errorf("in RunTransaction(): %w", txErr)
	}

	return nil
}

