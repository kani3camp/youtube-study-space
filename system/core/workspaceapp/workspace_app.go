package workspaceapp

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"app.modules/core/i18n"
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	"app.modules/core/wordsreader"
	"app.modules/core/youtubebot"
	"cloud.google.com/go/firestore"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
)

type WorkspaceApp struct {
	Configs            *Configs
	Repository         repository.Repository
	WordsReader        wordsreader.WordsReader
	LiveChatBot        youtubebot.LiveChatBot
	alertOwnerBot      moderatorbot.MessageBot
	alertModeratorsBot moderatorbot.MessageBot
	logModeratorsBot   moderatorbot.MessageBot

	ProcessedUserID                 string
	ProcessedUserDisplayName        string
	ProcessedUserProfileImageURL    string
	ProcessedUserIsModeratorOrOwner bool
	ProcessedUserIsMember           bool

	blockRegexesForChatMessage        []string
	blockRegexesForChannelName        []string
	notificationRegexesForChatMessage []string
	notificationRegexesForChannelName []string

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

	slog.InfoContext(ctx, "initializing spreadsheet reader...")
	wordsReader, err := wordsreader.NewSpreadsheetReader(ctx, clientOption, configs.Constants.BotConfigSpreadsheetID, "01", "02")
	if err != nil {
		return nil, fmt.Errorf("in NewSpreadsheetReader(): %w", err)
	}
	slog.InfoContext(ctx, "reading block regexes...")
	blockRegexesForChatMessage, blockRegexesForChannelName, err := wordsReader.ReadBlockRegexes(ctx)
	if err != nil {
		return nil, fmt.Errorf("in ReadBlockRegexes(): %w", err)
	}
	slog.InfoContext(ctx, "reading notification regexes...")
	notificationRegexesForChatMessage, notificationRegexesForChannelName, err := wordsReader.ReadNotificationRegexes(ctx)
	if err != nil {
		return nil, fmt.Errorf("in ReadNotificationRegexes(): %w", err)
	}

	slog.InfoContext(ctx, "reading menu docs...")
	sortedMenuItems, err := firestoreController.ReadAllMenuDocsOrderByCode(ctx)
	if err != nil {
		return nil, fmt.Errorf("in ReadAllMenuDocsOrderByCode(): %w", err)
	}

	return &WorkspaceApp{
		Configs:                           &configs,
		Repository:                        firestoreController,
		WordsReader:                       wordsReader,
		LiveChatBot:                       liveChatBot,
		alertOwnerBot:                     discordOwnerBot,
		alertModeratorsBot:                discordSharedBot,
		logModeratorsBot:                  discordSharedLogBot,
		blockRegexesForChannelName:        blockRegexesForChannelName,
		blockRegexesForChatMessage:        blockRegexesForChatMessage,
		notificationRegexesForChatMessage: notificationRegexesForChatMessage,
		notificationRegexesForChannelName: notificationRegexesForChannelName,
		SortedMenuItems:                   sortedMenuItems,
		nowFunc:                           nil,
	}, nil
}

func (app *WorkspaceApp) currentTime() time.Time {
	if app.nowFunc != nil {
		return app.nowFunc()
	}
	return timeutil.JstNow()
}

func (app *WorkspaceApp) RunTransaction(ctx context.Context, f func(ctx context.Context, tx *firestore.Transaction) error) error {
	return app.Repository.FirestoreClient().RunTransaction(ctx, f)
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

func (app *WorkspaceApp) GetInfoString() string {
	numAllFilteredRegex := len(app.blockRegexesForChatMessage) + len(app.blockRegexesForChannelName) + len(app.notificationRegexesForChatMessage) + len(app.notificationRegexesForChannelName)
	return fmt.Sprintf("全規制ワード数: %d", numAllFilteredRegex)
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

func (app *WorkspaceApp) CheckIfUnwantedWordIncluded(ctx context.Context, userID, message, channelName string) (bool, error) {
	// ブロック対象チェック
	found, index, err := utils.ContainsRegexWithIndex(app.blockRegexesForChatMessage, message)
	if err != nil {
		return false, err
	}
	if found {
		if err := app.BanUser(ctx, userID); err != nil {
			return false, fmt.Errorf("in BanUser(): %w", err)
		}
		return true, app.LogToModerators(ctx, "発言から禁止ワードを検出、ユーザーをブロックしました。"+
			"\n禁止ワード: `"+app.blockRegexesForChatMessage[index]+"`"+
			"\nチャンネル名: `"+channelName+"`"+
			"\nチャンネルURL: https://youtube.com/channel/"+userID+
			"\nチャット内容: `"+message+"`"+
			"\n日時: "+app.currentTime().String())
	}
	found, index, err = utils.ContainsRegexWithIndex(app.blockRegexesForChannelName, channelName)
	if err != nil {
		return false, fmt.Errorf("in ContainsRegexWithIndex(): %w", err)
	}
	if found {
		if err := app.BanUser(ctx, userID); err != nil {
			return false, fmt.Errorf("in BanUser(): %w", err)
		}
		return true, app.LogToModerators(ctx, "チャンネル名から禁止ワードを検出、ユーザーをブロックしました。"+
			"\n禁止ワード: `"+app.blockRegexesForChannelName[index]+"`"+
			"\nチャンネル名: `"+channelName+"`"+
			"\nチャンネルURL: https://youtube.com/channel/"+userID+
			"\nチャット内容: `"+message+"`"+
			"\n日時: "+app.currentTime().String())
	}

	// 通知対象チェック
	found, index, err = utils.ContainsRegexWithIndex(app.notificationRegexesForChatMessage, message)
	if err != nil {
		return false, fmt.Errorf("in ContainsRegexWithIndex(): %w", err)
	}
	if found {
		return false, app.MessageToModerators(ctx, "発言から禁止ワードを検出しました。（通知のみ）"+
			"\n禁止ワード: `"+app.notificationRegexesForChatMessage[index]+"`"+
			"\nチャンネル名: `"+channelName+"`"+
			"\nチャンネルURL: https://youtube.com/channel/"+userID+
			"\nチャット内容: `"+message+"`"+
			"\n日時: "+app.currentTime().String())
	}
	found, index, err = utils.ContainsRegexWithIndex(app.notificationRegexesForChannelName, channelName)
	if err != nil {
		return false, fmt.Errorf("in ContainsRegexWithIndex(): %w", err)
	}
	if found {
		return false, app.MessageToModerators(ctx, "チャンネルから禁止ワードを検出しました。（通知のみ）"+
			"\n禁止ワード: `"+app.notificationRegexesForChannelName[index]+"`"+
			"\nチャンネル名: `"+channelName+"`"+
			"\nチャンネルURL: https://youtube.com/channel/"+userID+
			"\nチャット内容: `"+message+"`"+
			"\n日時: "+app.currentTime().String())
	}
	return false, nil
}

// ProcessMessage 入力コマンドを解析して実行
func (app *WorkspaceApp) ProcessMessage(
	ctx context.Context,
	commandString string,
	userID string,
	userDisplayName string,
	userProfileImageURL string,
	isChatModerator bool,
	isChatOwner bool,
	isChatMember bool,
) error {
	if userID == app.Configs.LiveChatBotChannelID {
		return nil
	}
	if !app.Configs.Constants.YoutubeMembershipEnabled {
		isChatMember = false
	}
	app.SetProcessedUser(userID, userDisplayName, userProfileImageURL, isChatModerator, isChatOwner, isChatMember)

	// check if an unwanted word included
	if !isChatModerator && !isChatOwner {
		blocked, err := app.CheckIfUnwantedWordIncluded(ctx, userID, commandString, userDisplayName)
		if err != nil {
			app.MessageToOwnerWithError(ctx, "in CheckIfUnwantedWordIncluded", err)
			// continue
		}
		if blocked {
			return nil
		}
	}

	// 初回の利用の場合はユーザーデータを初期化
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		isRegistered, err := app.IfUserRegistered(ctx, tx)
		if err != nil {
			return fmt.Errorf("in IfUserRegistered(): %w", err)
		}
		if !isRegistered {
			if err := app.CreateUser(ctx, tx); err != nil {
				return fmt.Errorf("in CreateUser(): %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		app.MessageToLiveChat(ctx, i18nmsg.CommandError(app.ProcessedUserDisplayName))
		return fmt.Errorf("in RunTransaction(): %w", txErr)
	}

	// コマンドの解析
	commandDetails, message := utils.ParseCommand(commandString, isChatMember)
	if message != "" { // これはシステム内部のエラーではなく、入力コマンドが不正ということなので、return nil
		app.MessageToLiveChat(ctx, i18nmsg.CommonSir(app.ProcessedUserDisplayName)+message)
		return nil
	}

	if message = app.ValidateCommand(*commandDetails); message != "" {
		app.MessageToLiveChat(ctx, i18nmsg.CommonSir(app.ProcessedUserDisplayName)+message)
		return nil
	}

	// コマンドの実行
	return app.executeCommand(ctx, commandDetails, commandString)
}

// executeCommand 解析済みのコマンドを実行する
func (app *WorkspaceApp) executeCommand(ctx context.Context, commandDetails *utils.CommandDetails, commandString string) error {
	// commandDetailsに基づいて命令処理
	switch commandDetails.CommandType {
	case utils.NotCommand:
		return nil
	case utils.InvalidCommand:
		return nil
	case utils.In:
		return app.In(ctx, &commandDetails.InOption)
	case utils.Out:
		return app.Out(ctx)
	case utils.Info:
		return app.ShowUserInfo(ctx, &commandDetails.InfoOption)
	case utils.My:
		return app.My(ctx, commandDetails.MyOptions)
	case utils.Change:
		return app.Change(ctx, &commandDetails.ChangeOption)
	case utils.Seat:
		return app.ShowSeatInfo(ctx, &commandDetails.SeatOption)
	case utils.Report:
		return app.Report(ctx, &commandDetails.ReportOption)
	case utils.Kick:
		return app.Kick(ctx, &commandDetails.KickOption)
	case utils.Check:
		return app.Check(ctx, &commandDetails.CheckOption)
	case utils.Block:
		return app.Block(ctx, &commandDetails.BlockOption)
	case utils.More:
		return app.More(ctx, &commandDetails.MoreOption)
	case utils.Break:
		return app.Break(ctx, &commandDetails.BreakOption)
	case utils.Resume:
		return app.Resume(ctx, &commandDetails.ResumeOption)
	case utils.Rank:
		return app.Rank(ctx, commandDetails)
	case utils.Order:
		return app.Order(ctx, &commandDetails.OrderOption)
	case utils.Clear:
		return app.Clear(ctx)
	default:
		return errors.New("Unknown command: " + commandString)
	}
}
