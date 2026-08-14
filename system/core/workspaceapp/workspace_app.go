package workspaceapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"

	"app.modules/core/i18n"
	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	"app.modules/core/youtubebot"
)

type WorkspaceApp struct {
	Configs            *Configs
	Repository         repository.Repository
	LiveChatBot        youtubebot.LiveChatBot
	alertOwnerBot      moderatorbot.MessageBot
	alertModeratorsBot moderatorbot.MessageBot
	logModeratorsBot   moderatorbot.MessageBot

	ProcessedUserID                 string
	ProcessedUserDisplayName        string
	ProcessedUserProfileImageURL    string
	ProcessedUserIsModeratorOrOwner bool
	ProcessedUserIsMember           bool

	SortedMenuItems []repository.MenuDoc // メニューコードで昇順ソートして格納

	nowFunc func() time.Time // テストの時刻注入用
}

// Configs WorkspaceApp生成時に初期化すべきフィールド値
type Configs struct {
	Constants repository.ConstantsConfigDoc

	LiveChatBotChannelID string
}

func NewWorkspaceApp(ctx context.Context, interactive bool, clientOption option.ClientOption) (*WorkspaceApp, error) {
	if err := i18n.LoadLocaleFolderFS(); err != nil {
		return nil, fmt.Errorf("in LoadLocaleFolderFS(): %w", err)
	}

	slog.InfoContext(ctx, "initializing firestore client...")
	firestoreController, err := repository.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return nil, fmt.Errorf("in NewFirestoreController(): %w", err)
	}

	// credentials
	slog.InfoContext(ctx, "reading credentials config...")
	credentialsDoc, err := firestoreController.ReadCredentialsConfig(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("in ReadCredentialsConfig(): %w", err)
	}

	// YouTube live chatbot
	slog.InfoContext(ctx, "initializing youtube live chat bot...")
	liveChatBot, err := youtubebot.NewYoutubeLiveChatBot(credentialsDoc.YoutubeLiveChatID, firestoreController, ctx)
	if err != nil {
		return nil, fmt.Errorf("in NewYoutubeLiveChatBot(): %w", err)
	}

	discordOwnerBot, err := moderatorbot.NewDiscordBot(credentialsDoc.DiscordOwnerBotToken, credentialsDoc.DiscordOwnerBotTextChannelID)
	if err != nil {
		return nil, fmt.Errorf("in NewDiscordBot(): %w", err)
	}

	discordSharedBot, err := moderatorbot.NewDiscordBot(credentialsDoc.DiscordSharedBotToken, credentialsDoc.DiscordSharedBotTextChannelID)
	if err != nil {
		return nil, fmt.Errorf("in NewDiscordBot(): %w", err)
	}

	// discord bot for logging
	discordSharedLogBot, err := moderatorbot.NewDiscordBot(credentialsDoc.DiscordSharedBotToken, credentialsDoc.DiscordSharedBotLogChannelID)
	if err != nil {
		return nil, fmt.Errorf("in NewDiscordBot(): %w", err)
	}

	// core constant values
	slog.InfoContext(ctx, "reading system constants config...")
	constantsConfig, err := firestoreController.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("in ReadSystemConstantsConfig(): %w", err)
	}

	configs := Configs{
		Constants:            constantsConfig,
		LiveChatBotChannelID: credentialsDoc.YoutubeBotChannelID,
	}

	// 全ての項目が初期化できているか確認
	v := reflect.ValueOf(configs.Constants)
	var uninitializedFields []string
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).IsZero() {
			fieldName := v.Type().Field(i).Name
			fieldValue := fmt.Sprintf("%v", v.Field(i))
			uninitializedFields = append(uninitializedFields, fieldName+" = "+fieldValue)
		}
	}

	if interactive && len(uninitializedFields) > 0 {
		fmt.Println("The following fields may not be initialized:")
		for _, field := range uninitializedFields {
			fmt.Println("- " + field)
		}
		fmt.Println("Continue? (yes / no)")
		var s string
		_, err := fmt.Scanln(&s)
		if err != nil {
			return nil, fmt.Errorf("in fmt.Scanln(): %w", err)
		}
		if s != "yes" {
			return nil, errors.New("aborted")
		}
	}

	slog.InfoContext(ctx, "reading menu docs...")
	sortedMenuItems, err := firestoreController.ReadAllMenuDocsOrderByCode(ctx)
	if err != nil {
		return nil, fmt.Errorf("in ReadAllMenuDocsOrderByCode(): %w", err)
	}

	return &WorkspaceApp{
		Configs:            &configs,
		Repository:         firestoreController,
		LiveChatBot:        liveChatBot,
		alertOwnerBot:      discordOwnerBot,
		alertModeratorsBot: discordSharedBot,
		logModeratorsBot:   discordSharedLogBot,
		SortedMenuItems:    sortedMenuItems,
		nowFunc:            nil,
	}, nil
}

func (app *WorkspaceApp) currentTime() time.Time {
	if app.nowFunc != nil {
		return app.nowFunc()
	}
	return timeutil.JstNow()
}

func (app *WorkspaceApp) RunTransaction(ctx context.Context, f func(ctx context.Context, tx *firestore.Transaction) error) error {
	if err := app.Repository.FirestoreClient().RunTransaction(ctx, f); err != nil {
		return fmt.Errorf("run Firestore transaction: %w", err)
	}
	return nil
}

func (app *WorkspaceApp) SetProcessedUser(userID string, userDisplayName string, userProfileImageURL string, isChatModerator bool, isChatOwner bool, isChatMember bool) {
	app.ProcessedUserID = userID
	app.ProcessedUserDisplayName = userDisplayName
	app.ProcessedUserProfileImageURL = userProfileImageURL
	app.ProcessedUserIsModeratorOrOwner = isChatModerator || isChatOwner
	app.ProcessedUserIsMember = isChatMember
}

func (app *WorkspaceApp) CloseFirestoreClient() {
	if err := app.Repository.FirestoreClient().Close(); err != nil {
		slog.Error("failed close firestore client.")
	} else {
		slog.Info("successfully closed firestore client.")
	}
}

// GoroutineCheckLongTimeSitting 長時間座席占有検出ループ
func (app *WorkspaceApp) GoroutineCheckLongTimeSitting(ctx context.Context) {
	minimumInterval := time.Duration(app.Configs.Constants.MinimumCheckLongTimeSittingIntervalMinutes) * time.Minute
	slog.Info("", "居座りチェックの最小間隔", minimumInterval)

	for {
		slog.Info("checking long time sitting.")
		start := app.currentTime()

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

		end := app.currentTime()
		duration := end.Sub(start)
		if duration < minimumInterval {
			time.Sleep(timeutil.NoNegativeDuration(minimumInterval - duration))
		}
	}
}

func (app *WorkspaceApp) CheckIfUnwantedWordIncluded(ctx context.Context, ngWordConfig NGWordConfig, userID, message, channelName string) (bool, error) {
	// ブロック対象チェック
	found, index, err := utils.ContainsRegexWithIndex(ngWordConfig.blockRegexesForChatMessage, message)
	if err != nil {
		return false, fmt.Errorf("check chat message against block regexes: %w", err)
	}
	if found {
		if err := app.BanUser(ctx, userID); err != nil {
			return false, fmt.Errorf("in BanUser(): %w", err)
		}
		return true, app.LogToModerators(ctx, "発言から禁止ワードを検出、ユーザーをブロックしました。"+
			"\n禁止ワード: `"+ngWordConfig.blockRegexesForChatMessage[index]+"`"+
			"\nチャンネル名: `"+channelName+"`"+
			"\nチャンネルURL: https://youtube.com/channel/"+userID+
			"\nチャット内容: `"+message+"`"+
			"\n日時: "+app.currentTime().String())
	}
	found, index, err = utils.ContainsRegexWithIndex(ngWordConfig.blockRegexesForChannelName, channelName)
	if err != nil {
		return false, fmt.Errorf("in ContainsRegexWithIndex(): %w", err)
	}
	if found {
		if err := app.BanUser(ctx, userID); err != nil {
			return false, fmt.Errorf("in BanUser(): %w", err)
		}
		return true, app.LogToModerators(ctx, "チャンネル名から禁止ワードを検出、ユーザーをブロックしました。"+
			"\n禁止ワード: `"+ngWordConfig.blockRegexesForChannelName[index]+"`"+
			"\nチャンネル名: `"+channelName+"`"+
			"\nチャンネルURL: https://youtube.com/channel/"+userID+
			"\nチャット内容: `"+message+"`"+
			"\n日時: "+app.currentTime().String())
	}

	// 通知対象チェック
	found, index, err = utils.ContainsRegexWithIndex(ngWordConfig.notificationRegexesForChatMessage, message)
	if err != nil {
		return false, fmt.Errorf("in ContainsRegexWithIndex(): %w", err)
	}
	if found {
		return false, app.MessageToModerators(ctx, "発言から禁止ワードを検出しました。（通知のみ）"+
			"\n禁止ワード: `"+ngWordConfig.notificationRegexesForChatMessage[index]+"`"+
			"\nチャンネル名: `"+channelName+"`"+
			"\nチャンネルURL: https://youtube.com/channel/"+userID+
			"\nチャット内容: `"+message+"`"+
			"\n日時: "+app.currentTime().String())
	}
	found, index, err = utils.ContainsRegexWithIndex(ngWordConfig.notificationRegexesForChannelName, channelName)
	if err != nil {
		return false, fmt.Errorf("in ContainsRegexWithIndex(): %w", err)
	}
	if found {
		return false, app.MessageToModerators(ctx, "チャンネルから禁止ワードを検出しました。（通知のみ）"+
			"\n禁止ワード: `"+ngWordConfig.notificationRegexesForChannelName[index]+"`"+
			"\nチャンネル名: `"+channelName+"`"+
			"\nチャンネルURL: https://youtube.com/channel/"+userID+
			"\nチャット内容: `"+message+"`"+
			"\n日時: "+app.currentTime().String())
	}
	return false, nil
}
