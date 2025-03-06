package workspaceapp

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"app.modules/core/i18n"
	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
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

	ProcessedUserId                 string
	ProcessedUserDisplayName        string
	ProcessedUserProfileImageUrl    string
	ProcessedUserIsModeratorOrOwner bool
	ProcessedUserIsMember           bool

	blockRegexesForChatMessage        []string
	blockRegexesForChannelName        []string
	notificationRegexesForChatMessage []string
	notificationRegexesForChannelName []string

	SortedMenuItems []repository.MenuDoc // メニューコードで昇順ソートして格納
}

// Configs WorkspaceApp生成時に初期化すべきフィールド値
type Configs struct {
	Constants repository.ConstantsConfigDoc

	LiveChatBotChannelId string
}

func NewWorkspaceApp(ctx context.Context, interactive bool, clientOption option.ClientOption) (*WorkspaceApp, error) {
	if err := i18n.LoadLocaleFolderFS(); err != nil {
		return nil, fmt.Errorf("in LoadLocaleFolderFS(): %w", err)
	}

	firestoreController, err := repository.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return nil, fmt.Errorf("in NewFirestoreController(): %w", err)
	}

	// credentials
	credentialsDoc, err := firestoreController.ReadCredentialsConfig(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("in ReadCredentialsConfig(): %w", err)
	}

	// YouTube live chatbot
	liveChatBot, err := youtubebot.NewYoutubeLiveChatBot(credentialsDoc.YoutubeLiveChatId, firestoreController, ctx)
	if err != nil {
		return nil, fmt.Errorf("in NewYoutubeLiveChatBot(): %w", err)
	}

	discordOwnerBot, err := moderatorbot.NewDiscordBot(credentialsDoc.DiscordOwnerBotToken, credentialsDoc.DiscordOwnerBotTextChannelId)
	if err != nil {
		return nil, fmt.Errorf("in NewDiscordBot(): %w", err)
	}

	discordSharedBot, err := moderatorbot.NewDiscordBot(credentialsDoc.DiscordSharedBotToken, credentialsDoc.DiscordSharedBotTextChannelId)
	if err != nil {
		return nil, fmt.Errorf("in NewDiscordBot(): %w", err)
	}

	// discord bot for logging
	discordSharedLogBot, err := moderatorbot.NewDiscordBot(credentialsDoc.DiscordSharedBotToken, credentialsDoc.DiscordSharedBotLogChannelId)
	if err != nil {
		return nil, fmt.Errorf("in NewDiscordBot(): %w", err)
	}

	// core constant values
	constantsConfig, err := firestoreController.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("in ReadSystemConstantsConfig(): %w", err)
	}

	configs := Configs{
		Constants:            constantsConfig,
		LiveChatBotChannelId: credentialsDoc.YoutubeBotChannelId,
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

	wordsReader, err := wordsreader.NewSpreadsheetReader(ctx, clientOption, configs.Constants.BotConfigSpreadsheetId, "01", "02")
	if err != nil {
		return nil, fmt.Errorf("in NewSpreadsheetReader(): %w", err)
	}
	blockRegexesForChatMessage, blockRegexesForChannelName, err := wordsReader.ReadBlockRegexes()
	if err != nil {
		return nil, fmt.Errorf("in ReadBlockRegexes(): %w", err)
	}
	notificationRegexesForChatMessage, notificationRegexesForChannelName, err := wordsReader.ReadNotificationRegexes()
	if err != nil {
		return nil, fmt.Errorf("in ReadNotificationRegexes(): %w", err)
	}

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
	}, nil
}

func (s *WorkspaceApp) RunTransaction(ctx context.Context, f func(ctx context.Context, tx *firestore.Transaction) error) error {
	return s.Repository.FirestoreClient().RunTransaction(ctx, f)
}

func (s *WorkspaceApp) SetProcessedUser(userId string, userDisplayName string, userProfileImageUrl string, isChatModerator bool, isChatOwner bool, isChatMember bool) {
	s.ProcessedUserId = userId
	s.ProcessedUserDisplayName = userDisplayName
	s.ProcessedUserProfileImageUrl = userProfileImageUrl
	s.ProcessedUserIsModeratorOrOwner = isChatModerator || isChatOwner
	s.ProcessedUserIsMember = isChatMember
}

func (s *WorkspaceApp) CloseFirestoreClient() {
	if err := s.Repository.FirestoreClient().Close(); err != nil {
		slog.Error("failed close firestore client.")
	} else {
		slog.Info("successfully closed firestore client.")
	}
}

func (s *WorkspaceApp) GetInfoString() string {
	numAllFilteredRegex := len(s.blockRegexesForChatMessage) + len(s.blockRegexesForChannelName) + len(s.notificationRegexesForChatMessage) + len(s.notificationRegexesForChannelName)
	return fmt.Sprintf("全規制ワード数: %d", numAllFilteredRegex)
}

// GoroutineCheckLongTimeSitting 長時間座席占有検出ループ
func (s *WorkspaceApp) GoroutineCheckLongTimeSitting(ctx context.Context) {
	minimumInterval := time.Duration(s.Configs.Constants.MinimumCheckLongTimeSittingIntervalMinutes) * time.Minute
	slog.Info("", "居座りチェックの最小間隔", minimumInterval)

	for {
		slog.Info("checking long time sitting.")
		start := utils.JstNow()

		{
			if err := s.CheckLongTimeSitting(ctx, true); err != nil {
				s.MessageToOwnerWithError(ctx, "in CheckLongTimeSitting", err)
			}
		}
		{
			if err := s.CheckLongTimeSitting(ctx, false); err != nil {
				s.MessageToOwnerWithError(ctx, "in CheckLongTimeSitting", err)
			}
		}

		end := utils.JstNow()
		duration := end.Sub(start)
		if duration < minimumInterval {
			time.Sleep(utils.NoNegativeDuration(minimumInterval - duration))
		}
	}
}

func (s *WorkspaceApp) CheckIfUnwantedWordIncluded(ctx context.Context, userId, message, channelName string) (bool, error) {
	// ブロック対象チェック
	found, index, err := utils.ContainsRegexWithIndex(s.blockRegexesForChatMessage, message)
	if err != nil {
		return false, err
	}
	if found {
		if err := s.BanUser(ctx, userId); err != nil {
			return false, fmt.Errorf("in BanUser(): %w", err)
		}
		return true, s.LogToModerators(ctx, "発言から禁止ワードを検出、ユーザーをブロックしました。"+
			"\n禁止ワード: `"+s.blockRegexesForChatMessage[index]+"`"+
			"\nチャンネル名: `"+channelName+"`"+
			"\nチャンネルURL: https://youtube.com/channel/"+userId+
			"\nチャット内容: `"+message+"`"+
			"\n日時: "+utils.JstNow().String())
	}
	found, index, err = utils.ContainsRegexWithIndex(s.blockRegexesForChannelName, channelName)
	if err != nil {
		return false, fmt.Errorf("in ContainsRegexWithIndex(): %w", err)
	}
	if found {
		if err := s.BanUser(ctx, userId); err != nil {
			return false, fmt.Errorf("in BanUser(): %w", err)
		}
		return true, s.LogToModerators(ctx, "チャンネル名から禁止ワードを検出、ユーザーをブロックしました。"+
			"\n禁止ワード: `"+s.blockRegexesForChannelName[index]+"`"+
			"\nチャンネル名: `"+channelName+"`"+
			"\nチャンネルURL: https://youtube.com/channel/"+userId+
			"\nチャット内容: `"+message+"`"+
			"\n日時: "+utils.JstNow().String())
	}

	// 通知対象チェック
	found, index, err = utils.ContainsRegexWithIndex(s.notificationRegexesForChatMessage, message)
	if err != nil {
		return false, fmt.Errorf("in ContainsRegexWithIndex(): %w", err)
	}
	if found {
		return false, s.MessageToModerators(ctx, "発言から禁止ワードを検出しました。（通知のみ）"+
			"\n禁止ワード: `"+s.notificationRegexesForChatMessage[index]+"`"+
			"\nチャンネル名: `"+channelName+"`"+
			"\nチャンネルURL: https://youtube.com/channel/"+userId+
			"\nチャット内容: `"+message+"`"+
			"\n日時: "+utils.JstNow().String())
	}
	found, index, err = utils.ContainsRegexWithIndex(s.notificationRegexesForChannelName, channelName)
	if err != nil {
		return false, fmt.Errorf("in ContainsRegexWithIndex(): %w", err)
	}
	if found {
		return false, s.MessageToModerators(ctx, "チャンネルから禁止ワードを検出しました。（通知のみ）"+
			"\n禁止ワード: `"+s.notificationRegexesForChannelName[index]+"`"+
			"\nチャンネル名: `"+channelName+"`"+
			"\nチャンネルURL: https://youtube.com/channel/"+userId+
			"\nチャット内容: `"+message+"`"+
			"\n日時: "+utils.JstNow().String())
	}
	return false, nil
}

// ProcessMessage 入力コマンドを解析して実行
func (s *WorkspaceApp) ProcessMessage(
	ctx context.Context,
	commandString string,
	userId string,
	userDisplayName string,
	userProfileImageUrl string,
	isChatModerator bool,
	isChatOwner bool,
	isChatMember bool,
) error {
	if userId == s.Configs.LiveChatBotChannelId {
		return nil
	}
	if !s.Configs.Constants.YoutubeMembershipEnabled {
		isChatMember = false
	}
	s.SetProcessedUser(userId, userDisplayName, userProfileImageUrl, isChatModerator, isChatOwner, isChatMember)

	// check if an unwanted word included
	if !isChatModerator && !isChatOwner {
		blocked, err := s.CheckIfUnwantedWordIncluded(ctx, userId, commandString, userDisplayName)
		if err != nil {
			s.MessageToOwnerWithError(ctx, "in CheckIfUnwantedWordIncluded", err)
			// continue
		}
		if blocked {
			return nil
		}
	}

	// 初回の利用の場合はユーザーデータを初期化
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		isRegistered, err := s.IfUserRegistered(ctx, tx)
		if err != nil {
			return fmt.Errorf("in IfUserRegistered(): %w", err)
		}
		if !isRegistered {
			if err := s.CreateUser(ctx, tx); err != nil {
				return fmt.Errorf("in CreateUser(): %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		s.MessageToLiveChat(ctx, i18n.T("command:error", s.ProcessedUserDisplayName))
		return fmt.Errorf("in RunTransaction(): %w", txErr)
	}

	// コマンドの解析
	commandDetails, message := utils.ParseCommand(commandString, isChatMember)
	if message != "" { // これはシステム内部のエラーではなく、入力コマンドが不正ということなので、return nil
		s.MessageToLiveChat(ctx, i18n.T("common:sir", s.ProcessedUserDisplayName)+message)
		return nil
	}

	if message = s.ValidateCommand(*commandDetails); message != "" {
		s.MessageToLiveChat(ctx, i18n.T("common:sir", s.ProcessedUserDisplayName)+message)
		return nil
	}

	// コマンドの実行
	return s.executeCommand(ctx, commandDetails, commandString)
}

// executeCommand 解析済みのコマンドを実行する
func (s *WorkspaceApp) executeCommand(ctx context.Context, commandDetails *utils.CommandDetails, commandString string) error {
	// commandDetailsに基づいて命令処理
	switch commandDetails.CommandType {
	case utils.NotCommand:
		return nil
	case utils.InvalidCommand:
		return nil
	case utils.In:
		return s.In(ctx, &commandDetails.InOption)
	case utils.Out:
		return s.Out(ctx)
	case utils.Info:
		return s.ShowUserInfo(ctx, &commandDetails.InfoOption)
	case utils.My:
		return s.My(ctx, commandDetails.MyOptions)
	case utils.Change:
		return s.Change(ctx, &commandDetails.ChangeOption)
	case utils.Seat:
		return s.ShowSeatInfo(ctx, &commandDetails.SeatOption)
	case utils.Report:
		return s.Report(ctx, &commandDetails.ReportOption)
	case utils.Kick:
		return s.Kick(ctx, &commandDetails.KickOption)
	case utils.Check:
		return s.Check(ctx, &commandDetails.CheckOption)
	case utils.Block:
		return s.Block(ctx, &commandDetails.BlockOption)
	case utils.More:
		return s.More(ctx, &commandDetails.MoreOption)
	case utils.Break:
		return s.Break(ctx, &commandDetails.BreakOption)
	case utils.Resume:
		return s.Resume(ctx, &commandDetails.ResumeOption)
	case utils.Rank:
		return s.Rank(ctx, commandDetails)
	case utils.Order:
		return s.Order(ctx, &commandDetails.OrderOption)
	case utils.Clear:
		return s.Clear(ctx)
	default:
		return errors.New("Unknown command: " + commandString)
	}
}
