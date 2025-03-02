package workspaceapp

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"reflect"
	"strconv"
	"time"

	"app.modules/core/i18n"
	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	"app.modules/core/studyspaceerror"
	"app.modules/core/utils"
	"app.modules/core/wordsreader"
	"app.modules/core/youtubebot"
	"cloud.google.com/go/firestore"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewSystem(ctx context.Context, interactive bool, clientOption option.ClientOption) (*WorkspaceApp, error) {
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

	configs := SystemConfigs{
		Constants:            constantsConfig,
		LiveChatBotChannelId: credentialsDoc.YoutubeBotChannelId,
	}

	// 全ての項目が初期化できているか確認
	v := reflect.ValueOf(configs.Constants)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).IsZero() {
			if interactive {
				fieldName := v.Type().Field(i).Name
				fieldValue := fmt.Sprintf("%v", v.Field(i))
				fmt.Println("The field \"" + fieldName + " = " + fieldValue + "\" may not be initialized. " +
					"Continue? (yes / no)")
				var s string
				_, err := fmt.Scanln(&s)
				if err != nil {
					return nil, fmt.Errorf("in fmt.Scanln(): %w", err)
				}
				if s != "yes" {
					return nil, errors.New("aborted")
				}
			}
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

// Command 入力コマンドを解析して実行
func (s *WorkspaceApp) Command(
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
		return s.In(ctx, commandDetails)
	case utils.Out:
		return s.Out(commandDetails, ctx)
	case utils.Info:
		return s.ShowUserInfo(commandDetails, ctx)
	case utils.My:
		return s.My(commandDetails, ctx)
	case utils.Change:
		return s.Change(commandDetails, ctx)
	case utils.Seat:
		return s.ShowSeatInfo(commandDetails, ctx)
	case utils.Report:
		return s.Report(commandDetails, ctx)
	case utils.Kick:
		return s.Kick(commandDetails, ctx)
	case utils.Check:
		return s.Check(commandDetails, ctx)
	case utils.Block:
		return s.Block(commandDetails, ctx)
	case utils.More:
		return s.More(commandDetails, ctx)
	case utils.Break:
		return s.Break(ctx, commandDetails)
	case utils.Resume:
		return s.Resume(ctx, commandDetails)
	case utils.Rank:
		return s.Rank(commandDetails, ctx)
	case utils.Order:
		return s.Order(ctx, commandDetails)
	default:
		return errors.New("Unknown command: " + commandString)
	}
}

func (s *WorkspaceApp) In(ctx context.Context, command *utils.CommandDetails) error {
	var replyMessage string
	t := i18n.GetTFunc("command-in")
	inOption := &command.InOption
	isTargetMemberSeat := inOption.IsMemberSeat

	if isTargetMemberSeat && !s.ProcessedUserIsMember {
		if s.Configs.Constants.YoutubeMembershipEnabled {
			s.MessageToLiveChat(ctx, t("member-seat-forbidden", s.ProcessedUserDisplayName))
		} else {
			s.MessageToLiveChat(ctx, t("membership-disabled", s.ProcessedUserDisplayName))
		}
		return nil
	}

	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 席が指定されているか？
		if inOption.IsSeatIdSet {
			// 0番席だったら最小番号の空席に決定
			if inOption.SeatId == 0 {
				seatId, err := s.MinAvailableSeatIdForUser(ctx, tx, s.ProcessedUserId, isTargetMemberSeat)
				if err != nil {
					return fmt.Errorf("in s.MinAvailableSeatIdForUser(): %w", err)
				}
				inOption.SeatId = seatId
			} else {
				// その席が空いているか？
				{
					isVacant, err := s.IfSeatVacant(ctx, tx, inOption.SeatId, isTargetMemberSeat)
					if err != nil {
						return fmt.Errorf("in s.IfSeatVacant(): %w", err)
					}
					if !isVacant {
						replyMessage = t("no-seat", s.ProcessedUserDisplayName, utils.InCommand)
						return nil
					}
				}
				// ユーザーはその席に対して入室制限を受けてないか？
				{
					isTooMuch, err := s.CheckIfUserSittingTooMuchForSeat(ctx, s.ProcessedUserId, inOption.SeatId, isTargetMemberSeat)
					if err != nil {
						return fmt.Errorf("in s.CheckIfUserSittingTooMuchForSeat(): %w", err)
					}
					if isTooMuch {
						replyMessage = t("no-availability", s.ProcessedUserDisplayName, utils.InCommand)
						return nil
					}
				}
			}
		} else { // 席の指定なし
			seatId, err := s.RandomAvailableSeatIdForUser(ctx, tx, s.ProcessedUserId, isTargetMemberSeat)
			if err != nil {
				if errors.Is(err, studyspaceerror.ErrNoSeatAvailable) {
					return fmt.Errorf("席数がmax seatに達していて、ユーザーが入室できない事象が発生: %w", err)
				}
				return err
			}
			inOption.SeatId = seatId
		}

		userDoc, err := s.Repository.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in ReadUser(): %w", err)
		}

		// 作業時間が指定されているか？
		if !inOption.MinutesAndWorkName.IsDurationMinSet {
			if userDoc.DefaultStudyMin == 0 {
				inOption.MinutesAndWorkName.DurationMin = s.Configs.Constants.DefaultWorkTimeMin
			} else {
				inOption.MinutesAndWorkName.DurationMin = userDoc.DefaultStudyMin
			}
		}

		// ランクから席の色を決定
		seatAppearance, err := s.GetUserRealtimeSeatAppearance(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in GetUserRealtimeSeatAppearance(): %w", err)
		}

		// 動作が決定

		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in IsUserInRoom(): %w", err)
		}
		isInRoom := isInGeneralRoom || isInMemberRoom
		var currentSeat repository.SeatDoc
		if isInRoom { // 現在座っている席を取得
			var err error
			currentSeat, err = s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in CurrentSeat(): %w", err)
			}
		}

		// =========== 以降は書き込み処理のみ ===========

		if isInRoom { // 退室させてから、入室させる
			// 席移動処理
			workedTimeSec, addedRP, untilExitMin, err := s.moveSeat(ctx, tx, inOption.SeatId, s.ProcessedUserProfileImageUrl, isInMemberRoom, isTargetMemberSeat, *inOption.MinutesAndWorkName, currentSeat, &userDoc)
			if err != nil {
				return fmt.Errorf("failed to moveSeat for %s (%s): %w", s.ProcessedUserDisplayName, s.ProcessedUserId, err)
			}

			var rpEarned string
			if userDoc.RankVisible {
				rpEarned = i18n.T("command:rp-earned", addedRP)
			}
			previousSeatIdStr := utils.SeatIdStr(currentSeat.SeatId, isInMemberRoom)
			newSeatIdStr := utils.SeatIdStr(inOption.SeatId, isTargetMemberSeat)

			replyMessage += t("seat-move", s.ProcessedUserDisplayName, previousSeatIdStr, newSeatIdStr, workedTimeSec/60, rpEarned, untilExitMin)

			return nil
		} else { // 入室のみ
			untilExitMin, err := s.enterRoom(
				ctx,
				tx,
				s.ProcessedUserId,
				s.ProcessedUserDisplayName,
				s.ProcessedUserProfileImageUrl,
				inOption.SeatId,
				isTargetMemberSeat,
				inOption.MinutesAndWorkName.WorkName,
				"",
				inOption.MinutesAndWorkName.DurationMin,
				seatAppearance,
				"",
				repository.WorkState,
				userDoc.IsContinuousActive,
				time.Time{},
				time.Time{},
				utils.JstNow())
			if err != nil {
				return fmt.Errorf("in enterRoom(): %w", err)
			}
			var newSeatId string
			if isTargetMemberSeat {
				newSeatId = i18n.T("common:vip-seat-id", inOption.SeatId)
			} else {
				newSeatId = strconv.Itoa(inOption.SeatId)
			}

			// 入室しましたのメッセージ
			replyMessage = t("start", s.ProcessedUserDisplayName, untilExitMin, newSeatId)
			return nil
		}
	})
	if txErr != nil {
		slog.Error("txErr in In()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

// GetUserRealtimeSeatAppearance リアルタイムの現在のランクを求める
func (s *WorkspaceApp) GetUserRealtimeSeatAppearance(ctx context.Context, tx *firestore.Transaction, userId string) (repository.SeatAppearance, error) {
	userDoc, err := s.Repository.ReadUser(ctx, tx, userId)
	if err != nil {
		return repository.SeatAppearance{}, fmt.Errorf("in ReadUser(): %w", err)
	}
	totalStudyDuration, _, err := s.GetUserRealtimeTotalStudyDurations(ctx, tx, userId)
	if err != nil {
		return repository.SeatAppearance{}, fmt.Errorf("in GetUserRealtimeTotalStudyDurations(): %w", err)
	}
	seatAppearance, err := utils.GetSeatAppearance(int(totalStudyDuration.Seconds()), userDoc.RankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
	if err != nil {
		return repository.SeatAppearance{}, fmt.Errorf("in GetSeatAppearance(): %w", err)
	}
	return seatAppearance, nil
}

func (s *WorkspaceApp) Out(_ *utils.CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-out")
	var replyMessage string
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := s.Repository.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in ReadUser(): %w", err)
		}

		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in IsUserInRoom(): %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			if userDoc.LastExited.IsZero() {
				replyMessage = t("already-exit", s.ProcessedUserDisplayName)
			} else {
				lastExited := userDoc.LastExited.In(utils.JapanLocation())
				replyMessage = t("already-exit-with-last-exit-time", s.ProcessedUserDisplayName, lastExited.Hour(), lastExited.Minute())
			}
			return nil
		}

		// 現在座っている席を特定
		seat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("in CurrentSeat(): %w", err)
		}

		// 退室処理
		workedTimeSec, addedRP, err := s.exitRoom(ctx, tx, isInMemberRoom, seat, &userDoc)
		if err != nil {
			return fmt.Errorf("in exitRoom(): %w", err)
		}
		var rpEarned string
		var seatIdStr string
		if userDoc.RankVisible {
			rpEarned = i18n.T("command:rp-earned", addedRP)
		}
		if isInMemberRoom {
			seatIdStr = i18n.T("common:vip-seat-id", seat.SeatId)
		} else {
			seatIdStr = strconv.Itoa(seat.SeatId)
		}
		replyMessage = i18n.T("command:exit", s.ProcessedUserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Out()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) ShowUserInfo(command *utils.CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-user-info")
	var replyMessage string
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		totalStudyDuration, dailyTotalStudyDuration, err := s.GetUserRealtimeTotalStudyDurations(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in s.GetUserRealtimeTotalStudyDurations(): %w", err)
		}
		dailyTotalTimeStr := utils.DurationToString(dailyTotalStudyDuration)
		totalTimeStr := utils.DurationToString(totalStudyDuration)
		replyMessage += t("base", s.ProcessedUserDisplayName, dailyTotalTimeStr, totalTimeStr)

		userDoc, err := s.Repository.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in s.Repository.ReadUser: %w", err)
		}

		if userDoc.RankVisible {
			replyMessage += t("rank", userDoc.RankPoint)
		}

		if command.InfoOption.ShowDetails {
			switch userDoc.RankVisible {
			case true:
				replyMessage += t("rank-on")
			case false:
				replyMessage += t("rank-off")
			}

			if userDoc.IsContinuousActive {
				continuousActiveDays := int(utils.JstNow().Sub(userDoc.CurrentActivityStateStarted).Hours() / 24)
				replyMessage += t("rank-on-continuous", continuousActiveDays+1, continuousActiveDays)
			}

			if userDoc.DefaultStudyMin == 0 {
				replyMessage += t("default-work-off")
			} else {
				replyMessage += t("default-work", userDoc.DefaultStudyMin)
			}

			if userDoc.FavoriteColor == "" {
				replyMessage += t("favorite-color-off")
			} else {
				replyMessage += t("favorite-color", utils.ColorCodeToColorName(userDoc.FavoriteColor))
			}

			replyMessage += t("register-date", userDoc.RegistrationDate.In(utils.JapanLocation()).Format("2006年01月02日"))
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in ShowUserInfo()", "txErr", txErr)
		replyMessage = i18n.T("command:error")
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) ShowSeatInfo(command *utils.CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-seat-info")
	showDetails := command.SeatOption.ShowDetails
	var replyMessage string
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// そのユーザーは入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in IsUserInRoom(): %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if isInRoom {
			currentSeat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in s.CurrentSeat(): %w", err)
			}

			realtimeSittingDurationMin := int(utils.NoNegativeDuration(utils.JstNow().Sub(currentSeat.EnteredAt)).Minutes())
			realtimeTotalStudyDurationOfSeat, err := utils.RealTimeTotalStudyDurationOfSeat(currentSeat, utils.JstNow())
			if err != nil {
				return fmt.Errorf("in RealTimeTotalStudyDurationOfSeat(): %w", err)
			}
			remainingMinutes := int(utils.NoNegativeDuration(currentSeat.Until.Sub(utils.JstNow())).Minutes())
			var stateStr string
			var breakUntilStr string
			switch currentSeat.State {
			case repository.WorkState:
				stateStr = i18n.T("common:work")
				breakUntilStr = ""
			case repository.BreakState:
				stateStr = i18n.T("common:break")
				breakUntilDuration := utils.NoNegativeDuration(currentSeat.CurrentStateUntil.Sub(utils.JstNow()))
				breakUntilStr = t("break-until", int(breakUntilDuration.Minutes()))
			}
			var seatIdStr string
			if isInMemberRoom {
				seatIdStr = i18n.T("common:vip-seat-id", currentSeat.SeatId)
			} else {
				seatIdStr = strconv.Itoa(currentSeat.SeatId)
			}
			replyMessage = t("base", s.ProcessedUserDisplayName, seatIdStr, stateStr, realtimeSittingDurationMin, int(realtimeTotalStudyDurationOfSeat.Minutes()), remainingMinutes, breakUntilStr)

			if showDetails {
				recentTotalEntryDuration, err := s.GetRecentUserSittingTimeForSeat(ctx, s.ProcessedUserId, currentSeat.SeatId, isInMemberRoom)
				if err != nil {
					return fmt.Errorf("in GetRecentUserSittingTimeForSeat(): %w", err)
				}
				replyMessage += t("details", s.Configs.Constants.RecentRangeMin, seatIdStr, int(recentTotalEntryDuration.Minutes()))
			}
		} else {
			replyMessage = i18n.T("command:not-enter", s.ProcessedUserDisplayName, utils.InCommand)
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in ShowSeatInfo()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Report(command *utils.CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-report")
	if command.ReportOption.Message == "" { // !reportのみは不可
		s.MessageToLiveChat(ctx, t("no-message", s.ProcessedUserDisplayName))
		return nil
	}

	ownerMessage := t("owner", utils.ReportCommand, s.ProcessedUserId, s.ProcessedUserDisplayName, command.ReportOption.Message)
	s.MessageToOwner(ctx, ownerMessage)

	messageForModerators := t("moderators", utils.ReportCommand, s.ProcessedUserDisplayName, command.ReportOption.Message)
	if err := s.MessageToModerators(ctx, messageForModerators); err != nil {
		s.MessageToOwnerWithError(ctx, "モデレーターへメッセージが送信できませんでした: \""+messageForModerators+"\"", err)
	}

	s.MessageToLiveChat(ctx, t("alert", s.ProcessedUserDisplayName))
	return nil
}

func (s *WorkspaceApp) Kick(command *utils.CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-kick")
	targetSeatId := command.KickOption.SeatId
	isTargetMemberSeat := command.KickOption.IsTargetMemberSeat
	var replyMessage string

	// commanderはモデレーターもしくはチャットオーナーか
	if !s.ProcessedUserIsModeratorOrOwner {
		s.MessageToLiveChat(ctx, i18n.T("command:permission", s.ProcessedUserDisplayName, utils.KickCommand))
		return nil
	}

	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// ターゲットの座席は誰か使っているか
		{
			isSeatAvailable, err := s.IfSeatVacant(ctx, tx, targetSeatId, isTargetMemberSeat)
			if err != nil {
				return fmt.Errorf("in IfSeatVacant(): %w", err)
			}
			if isSeatAvailable {
				replyMessage = i18n.T("command:unused", s.ProcessedUserDisplayName)
				return nil
			}
		}

		// ユーザーを強制退室させる
		targetSeat, err := s.Repository.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = i18n.T("command:unused", s.ProcessedUserDisplayName)
				return nil
			}
			return fmt.Errorf("in ReadSeat: %w", err)
		}

		seatIdStr := utils.SeatIdStr(targetSeatId, isTargetMemberSeat)
		replyMessage = t("kick", s.ProcessedUserDisplayName, seatIdStr, targetSeat.UserDisplayName)

		// s.ProcessedUserが処理の対象ではないことに注意。
		userDoc, err := s.Repository.ReadUser(ctx, tx, targetSeat.UserId)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		workedTimeSec, addedRP, exitErr := s.exitRoom(ctx, tx, isTargetMemberSeat, targetSeat, &userDoc)
		if exitErr != nil {
			return fmt.Errorf("%sさんのkick退室処理中にエラーが発生しました: %w", s.ProcessedUserDisplayName, exitErr)
		}
		var rpEarned string
		if userDoc.RankVisible {
			rpEarned = i18n.T("command:rp-earned", addedRP)
		}
		replyMessage += i18n.T("command:exit", targetSeat.UserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)

		{
			err := s.LogToModerators(ctx, s.ProcessedUserDisplayName+"さん、"+strconv.Itoa(targetSeat.
				SeatId)+"番席のユーザーをkickしました。\n"+
				"チャンネル名: "+targetSeat.UserDisplayName+"\n"+
				"作業名: "+targetSeat.WorkName+"\n休憩中の作業名: "+targetSeat.BreakWorkName+"\n"+
				"入室時間: "+strconv.Itoa(workedTimeSec/60)+"分\n"+
				"チャンネルURL: https://youtube.com/channel/"+targetSeat.UserId)
			if err != nil {
				return fmt.Errorf("failed LogToModerators(): %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Kick()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Check(command *utils.CommandDetails, ctx context.Context) error {
	targetSeatId := command.CheckOption.SeatId
	isTargetMemberSeat := command.CheckOption.IsTargetMemberSeat

	var replyMessage string
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// commanderはモデレーターかチャットオーナーか
		if !s.ProcessedUserIsModeratorOrOwner {
			replyMessage = i18n.T("command:permission", s.ProcessedUserDisplayName, utils.CheckCommand)
			return nil
		}

		// ターゲットの座席は誰か使っているか
		{
			isSeatVacant, err := s.IfSeatVacant(ctx, tx, targetSeatId, isTargetMemberSeat)
			if err != nil {
				return fmt.Errorf("in IfSeatVacant: %w", err)
			}
			if isSeatVacant {
				replyMessage = i18n.T("command:unused", s.ProcessedUserDisplayName)
				return nil
			}
		}
		// 座席情報を表示する
		seat, err := s.Repository.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = i18n.T("command:unused", s.ProcessedUserDisplayName)
				return nil
			}
			return fmt.Errorf("in ReadSeat: %w", err)
		}
		sinceMinutes := int(utils.NoNegativeDuration(utils.JstNow().Sub(seat.EnteredAt)).Minutes())
		untilMinutes := int(utils.NoNegativeDuration(seat.Until.Sub(utils.JstNow())).Minutes())
		var seatIdStr string
		if isTargetMemberSeat {
			seatIdStr = i18n.T("common:vip-seat-id", targetSeatId)
		} else {
			seatIdStr = strconv.Itoa(targetSeatId)
		}
		message := s.ProcessedUserDisplayName + "さん、" + seatIdStr + "番席のユーザー情報です。\n" +
			"チャンネル名: " + seat.UserDisplayName + "\n" + "入室時間: " + strconv.Itoa(sinceMinutes) + "分\n" +
			"作業名: " + seat.WorkName + "\n" + "休憩中の作業名: " + seat.BreakWorkName + "\n" +
			"自動退室まで" + strconv.Itoa(untilMinutes) + "分\n" +
			"チャンネルURL: https://youtube.com/channel/" + seat.UserId
		if err := s.LogToModerators(ctx, message); err != nil {
			return fmt.Errorf("failed LogToModerators(): %w", err)
		}
		replyMessage = i18n.T("command:sent", s.ProcessedUserDisplayName)
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Check()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Block(command *utils.CommandDetails, ctx context.Context) error {
	targetSeatId := command.BlockOption.SeatId
	isTargetMemberSeat := command.BlockOption.IsTargetMemberSeat

	var replyMessage string
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// commanderはモデレーターかチャットオーナーか
		if !s.ProcessedUserIsModeratorOrOwner {
			replyMessage = s.ProcessedUserDisplayName + "さんは" + utils.BlockCommand + "コマンドを使用できません"
			return nil
		}

		// ターゲットの座席は誰か使っているか
		{
			isSeatAvailable, err := s.IfSeatVacant(ctx, tx, targetSeatId, isTargetMemberSeat)
			if err != nil {
				return fmt.Errorf("in IfSeatVacant(): %w", err)
			}
			if isSeatAvailable {
				replyMessage = s.ProcessedUserDisplayName + "さん、その番号の座席は誰も使用していません"
				return nil
			}
		}

		// ユーザーを強制退室させる
		targetSeat, err := s.Repository.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = s.ProcessedUserDisplayName + "さん、その番号の座席は誰も使用していません"
				return nil
			}
			s.MessageToOwnerWithError(ctx, "in ReadSeat", err)
			return fmt.Errorf("in ReadSeat: %w", err)
		}
		replyMessage = s.ProcessedUserDisplayName + "さん、" + strconv.Itoa(targetSeat.SeatId) + "番席の" + targetSeat.UserDisplayName + "さんをブロックします。"

		// s.ProcessedUserが処理の対象ではないことに注意。
		userDoc, err := s.Repository.ReadUser(ctx, tx, targetSeat.UserId)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		workedTimeSec, addedRP, exitErr := s.exitRoom(ctx, tx, isTargetMemberSeat, targetSeat, &userDoc)
		if exitErr != nil {
			return fmt.Errorf("%sさんの強制退室処理中にエラーが発生しました: %w", s.ProcessedUserDisplayName, exitErr)
		}
		var rpEarned string
		var seatIdStr string
		if userDoc.RankVisible {
			rpEarned = "（+ " + strconv.Itoa(addedRP) + " RP）"
		}
		if isTargetMemberSeat {
			seatIdStr = i18n.T("common:vip-seat-id", targetSeatId)
		} else {
			seatIdStr = strconv.Itoa(targetSeatId)
		}
		replyMessage = i18n.T("command:exit", targetSeat.UserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)

		// ブロック
		if err := s.BanUser(ctx, targetSeat.UserId); err != nil {
			return fmt.Errorf("in BanUser: %w", err)
		}

		{
			err := s.LogToModerators(ctx, s.ProcessedUserDisplayName+"さん、"+strconv.Itoa(targetSeat.
				SeatId)+"番席のユーザーをblockしました。\n"+
				"チャンネル名: "+targetSeat.UserDisplayName+"\n"+
				"作業名: "+targetSeat.WorkName+"\n休憩中の作業名: "+targetSeat.BreakWorkName+"\n"+
				"入室時間: "+strconv.Itoa(workedTimeSec/60)+"分\n"+
				"チャンネルURL: https://youtube.com/channel/"+targetSeat.UserId)
			if err != nil {
				return fmt.Errorf("failed LogToModerators(): %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Block()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) My(command *utils.CommandDetails, ctx context.Context) error {
	// ユーザードキュメントはすでにあり、登録されていないプロパティだった場合、そのままプロパティを保存したら自動で作成される。
	// また、読み込みのときにそのプロパティがなくても大丈夫。自動で初期値が割り当てられる。
	// ただし、ユーザードキュメントがそもそもない場合は、書き込んでもエラーにはならないが、登録日が記録されないため、要登録。

	// オプションが1つ以上指定されているか？
	if len(command.MyOptions) == 0 {
		s.MessageToLiveChat(ctx, i18n.T("command:option-warn", s.ProcessedUserDisplayName))
		return nil
	}

	t := i18n.GetTFunc("command-my")

	replyMessage := ""
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 変更前のuserDocを読み込んでおく
		userDoc, err := s.Repository.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom

		var seats []repository.SeatDoc
		if isInMemberRoom {
			seats, err = s.Repository.ReadMemberSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadMemberSeats: %w", err)
			}
		}
		if isInGeneralRoom {
			seats, err = s.Repository.ReadGeneralSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadGeneralSeats: %w", err)
			}
		}
		realTimeTotalStudyDuration, _, err := s.GetUserRealtimeTotalStudyDurations(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in RetrieveRealtimeTotalStudyDuration: %w", err)
		}
		realTimeTotalStudySec := int(realTimeTotalStudyDuration.Seconds())

		// これ以降は書き込みのみ

		replyMessage = i18n.T("common:sir", s.ProcessedUserDisplayName)
		currenRankVisible := userDoc.RankVisible
		for _, myOption := range command.MyOptions {
			if myOption.Type == utils.RankVisible {
				newRankVisible := myOption.BoolValue
				// 現在の値と、設定したい値が同じなら、変更なし
				if userDoc.RankVisible == newRankVisible {
					var rankVisibleStr string
					if userDoc.RankVisible {
						rankVisibleStr = i18n.T("common:on")
					} else {
						rankVisibleStr = i18n.T("common:off")
					}
					replyMessage += t("already-rank", rankVisibleStr)
				} else { // 違うなら、切替
					if err := s.Repository.UpdateUserRankVisible(tx, s.ProcessedUserId, newRankVisible); err != nil {
						return fmt.Errorf("in UpdateUserRankVisible: %w", err)
					}
					var newValueStr string
					if newRankVisible {
						newValueStr = i18n.T("common:on")
					} else {
						newValueStr = i18n.T("common:off")
					}
					replyMessage += t("set-rank", newValueStr)

					// 入室中であれば、座席の色も変える
					if isInRoom {
						seatAppearance, err := utils.GetSeatAppearance(realTimeTotalStudySec, newRankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
						if err != nil {
							return fmt.Errorf("in GetSeatAppearance: %w", err)
						}

						// 席の色を更新
						newSeat, err := utils.GetSeatByUserId(seats, s.ProcessedUserId)
						if err != nil {
							return fmt.Errorf("in GetSeatByUserId: %w", err)
						}
						newSeat.Appearance = seatAppearance
						if err := s.Repository.UpdateSeat(ctx, tx, newSeat, isInMemberRoom); err != nil {
							return fmt.Errorf("in s.Repository.UpdateSeats: %w", err)
						}
					}
				}
				currenRankVisible = newRankVisible
			} else if myOption.Type == utils.DefaultStudyMin {
				if err := s.Repository.UpdateUserDefaultStudyMin(tx, s.ProcessedUserId, myOption.IntValue); err != nil {
					return fmt.Errorf("in UpdateUserDefaultStudyMin: %w", err)
				}
				// 値が0はリセットのこと。
				if myOption.IntValue == 0 {
					replyMessage += t("reset-default-work")
				} else {
					replyMessage += t("set-default-work", myOption.IntValue)
				}
			} else if myOption.Type == utils.FavoriteColor {
				// 値が""はリセットのこと。
				colorCode := utils.ColorNameToColorCode(myOption.StringValue)
				if err := s.Repository.UpdateUserFavoriteColor(tx, s.ProcessedUserId, colorCode); err != nil {
					return fmt.Errorf("in UpdateUserFavoriteColor: %w", err)
				}
				replyMessage += t("set-favorite-color")
				if !utils.CanUseFavoriteColor(realTimeTotalStudySec) {
					replyMessage += t("alert-favorite-color", utils.FavoriteColorAvailableThresholdHours)
				}

				// 入室中であれば、座席の色も変える
				if isInRoom {
					newSeat, err := utils.GetSeatByUserId(seats, s.ProcessedUserId)
					if err != nil {
						return fmt.Errorf("in GetSeatByUserId: %w", err)
					}
					seatAppearance, err := utils.GetSeatAppearance(realTimeTotalStudySec, currenRankVisible, userDoc.RankPoint, colorCode)
					if err != nil {
						return fmt.Errorf("in GetSeatAppearance: %w", err)
					}

					// 席の色を更新
					newSeat.Appearance = seatAppearance
					if err := s.Repository.UpdateSeat(ctx, tx, newSeat, isInMemberRoom); err != nil {
						return fmt.Errorf("in s.Repository.UpdateSeat(): %w", err)
					}
				}
			}
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in My()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Change(command *utils.CommandDetails, ctx context.Context) error {
	changeOption := &command.ChangeOption
	jstNow := utils.JstNow()
	replyMessage := ""
	t := i18n.GetTFunc("command-change")
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// そのユーザーは入室中か？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		currentSeat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}

		// validation
		if err := s.ValidateChange(*command, currentSeat.State); err != nil {
			replyMessage = fmt.Sprintf("%s%s", i18n.T("common:sir", s.ProcessedUserDisplayName), err) // TODO 動作確認
			return nil
		}

		// これ以降は書き込みのみ可。

		newSeat := &currentSeat
		replyMessage = i18n.T("common:sir", s.ProcessedUserDisplayName)
		if changeOption.IsWorkNameSet { // 作業名もしくは休憩作業名を書きかえ
			var seatIdStr string
			if isInMemberRoom {
				seatIdStr = i18n.T("common:vip-seat-id", currentSeat.SeatId)
			} else {
				seatIdStr = strconv.Itoa(currentSeat.SeatId)
			}

			switch currentSeat.State {
			case repository.WorkState:
				newSeat.WorkName = changeOption.WorkName
				replyMessage += t("update-work", seatIdStr)
			case repository.BreakState:
				newSeat.BreakWorkName = changeOption.WorkName
				replyMessage += t("update-break", seatIdStr)
			}
		}
		if changeOption.IsDurationMinSet {
			switch currentSeat.State {
			case repository.WorkState:
				// 作業時間（入室時間から自動退室までの時間）を変更
				realtimeEntryDurationMin := int(utils.NoNegativeDuration(jstNow.Sub(currentSeat.EnteredAt)).Minutes())
				requestedUntil := currentSeat.EnteredAt.Add(time.Duration(changeOption.DurationMin) * time.Minute)

				if requestedUntil.Before(jstNow) {
					// もし現在時刻が指定時間を経過していたら却下
					remainingWorkMin := int(currentSeat.Until.Sub(jstNow).Minutes())
					replyMessage += t("work-duration-before", changeOption.DurationMin, realtimeEntryDurationMin, remainingWorkMin)
				} else if requestedUntil.After(jstNow.Add(time.Duration(s.Configs.Constants.MaxWorkTimeMin) * time.Minute)) {
					// もし現在時刻より最大延長可能時間以上後なら却下
					remainingWorkMin := int(currentSeat.Until.Sub(jstNow).Minutes())
					replyMessage += t("work-duration-after", s.Configs.Constants.MaxWorkTimeMin, realtimeEntryDurationMin, remainingWorkMin)
				} else { // それ以外なら延長
					newSeat.Until = requestedUntil
					newSeat.CurrentStateUntil = requestedUntil
					remainingWorkMin := int(utils.NoNegativeDuration(requestedUntil.Sub(jstNow)).Minutes())
					replyMessage += t("work-duration", changeOption.DurationMin, realtimeEntryDurationMin, remainingWorkMin)
				}
			case repository.BreakState:
				// 休憩時間を変更
				realtimeBreakDuration := utils.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt))
				requestedUntil := currentSeat.CurrentStateStartedAt.Add(time.Duration(changeOption.DurationMin) * time.Minute)

				if requestedUntil.Before(jstNow) {
					// もし現在時刻が指定時間を経過していたら却下
					remainingBreakDuration := currentSeat.CurrentStateUntil.Sub(jstNow)
					replyMessage += t("break-duration-before", changeOption.DurationMin, int(realtimeBreakDuration.Minutes()), int(remainingBreakDuration.Minutes()))
				} else { // それ以外ならuntilを変更
					newSeat.CurrentStateUntil = requestedUntil
					remainingBreakDuration := requestedUntil.Sub(jstNow)
					replyMessage += t("break-duration", changeOption.DurationMin, int(realtimeBreakDuration.Minutes()), int(remainingBreakDuration.Minutes()))
				}
			}
		}
		if err := s.Repository.UpdateSeat(ctx, tx, *newSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in UpdateSeats: %w", err)
		}

		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Change()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) More(command *utils.CommandDetails, ctx context.Context) error {
	replyMessage := ""
	t := i18n.GetTFunc("command-more")
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		jstNow := utils.JstNow()

		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		currentSeat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}

		// 以降書き込みのみ
		newSeat := &currentSeat

		replyMessage = i18n.T("common:sir", s.ProcessedUserDisplayName)
		var addedMin int              // 最終的な延長時間（分）
		var remainingUntilExitMin int // 最終的な自動退室予定時刻までの残り時間（分）

		switch currentSeat.State {
		case repository.WorkState:
			// オーバーフロー対策。延長時間が最大作業時間を超えていたら、少なくともアウトなので最大作業時間で上書き。
			if command.MoreOption.DurationMin > s.Configs.Constants.MaxWorkTimeMin {
				command.MoreOption.DurationMin = s.Configs.Constants.MaxWorkTimeMin
			}

			// 作業時間を指定分延長する
			newUntil := currentSeat.Until.Add(time.Duration(command.MoreOption.DurationMin) * time.Minute)
			// もし延長後の時間が最大作業時間を超えていたら、最大作業時間まで延長
			remainingUntilExitMin = int(utils.NoNegativeDuration(newUntil.Sub(jstNow)).Minutes())
			if remainingUntilExitMin > s.Configs.Constants.MaxWorkTimeMin {
				newUntil = jstNow.Add(time.Duration(s.Configs.Constants.MaxWorkTimeMin) * time.Minute)
				replyMessage += t("max-work", s.Configs.Constants.MaxWorkTimeMin)
			}
			addedMin = int(utils.NoNegativeDuration(newUntil.Sub(currentSeat.Until)).Minutes())
			newSeat.Until = newUntil
			newSeat.CurrentStateUntil = newUntil
			remainingUntilExitMin = int(utils.NoNegativeDuration(newUntil.Sub(jstNow)).Minutes())
		case repository.BreakState:
			// 休憩時間を指定分延長する
			newBreakUntil := currentSeat.CurrentStateUntil.Add(time.Duration(command.MoreOption.DurationMin) * time.Minute)
			// もし延長後の休憩時間が最大休憩時間を超えていたら、最大休憩時間まで延長
			if int(utils.NoNegativeDuration(newBreakUntil.Sub(currentSeat.CurrentStateStartedAt)).Minutes()) > s.Configs.Constants.MaxBreakDurationMin {
				newBreakUntil = currentSeat.CurrentStateStartedAt.Add(time.Duration(s.Configs.Constants.MaxBreakDurationMin) * time.Minute)
				replyMessage += t("max-break", strconv.Itoa(s.Configs.Constants.MaxBreakDurationMin))
			}
			addedMin = int(utils.NoNegativeDuration(newBreakUntil.Sub(currentSeat.CurrentStateUntil)).Minutes())
			newSeat.CurrentStateUntil = newBreakUntil
			// もし延長後の休憩時間がUntilを超えていたらUntilもそれに合わせる
			if newBreakUntil.After(currentSeat.Until) {
				newUntil := newBreakUntil
				newSeat.Until = newUntil
				remainingUntilExitMin = int(utils.NoNegativeDuration(newUntil.Sub(jstNow)).Minutes())
			} else {
				remainingUntilExitMin = int(utils.NoNegativeDuration(currentSeat.Until.Sub(jstNow)).Minutes())
			}
		}

		if err := s.Repository.UpdateSeat(ctx, tx, *newSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in s.Repository.UpdateSeats: %w", err)
		}

		switch currentSeat.State {
		case repository.WorkState:
			replyMessage += t("reply-work", addedMin)
		case repository.BreakState:
			remainingBreakDuration := utils.NoNegativeDuration(newSeat.CurrentStateUntil.Sub(jstNow))
			replyMessage += t("reply-break", addedMin, int(remainingBreakDuration.Minutes()))
		}
		realtimeEnteredTimeMin := int(utils.NoNegativeDuration(jstNow.Sub(currentSeat.EnteredAt)).Minutes())
		replyMessage += t("reply", realtimeEnteredTimeMin, remainingUntilExitMin)

		return nil
	})
	if txErr != nil {
		slog.Error("txErr in More()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Break(ctx context.Context, command *utils.CommandDetails) error {
	breakOption := &command.BreakOption
	replyMessage := ""
	t := i18n.GetTFunc("command-break")
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		// stateを確認
		currentSeat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}
		if currentSeat.State != repository.WorkState {
			replyMessage = t("work-only", s.ProcessedUserDisplayName)
			return nil
		}

		// 前回の入室または再開から、最低休憩間隔経っているか？
		currentWorkedMin := int(utils.NoNegativeDuration(utils.JstNow().Sub(currentSeat.CurrentStateStartedAt)).Minutes())
		if currentWorkedMin < s.Configs.Constants.MinBreakIntervalMin {
			replyMessage = t("warn", s.ProcessedUserDisplayName, s.Configs.Constants.MinBreakIntervalMin, currentWorkedMin)
			return nil
		}

		// オプション確認
		if !breakOption.IsDurationMinSet {
			breakOption.DurationMin = s.Configs.Constants.DefaultBreakDurationMin
		}
		if !breakOption.IsWorkNameSet {
			breakOption.WorkName = currentSeat.BreakWorkName
		}

		// 休憩処理
		jstNow := utils.JstNow()
		breakUntil := jstNow.Add(time.Duration(breakOption.DurationMin) * time.Minute)
		workedSec := int(utils.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt)).Seconds())
		cumulativeWorkSec := currentSeat.CumulativeWorkSec + workedSec
		// もし日付を跨いで作業してたら、daily-cumulative-work-secは日付変更からの時間にする
		var dailyCumulativeWorkSec int
		if workedSec > utils.SecondsOfDay(jstNow) {
			dailyCumulativeWorkSec = utils.SecondsOfDay(jstNow)
		} else {
			dailyCumulativeWorkSec = currentSeat.DailyCumulativeWorkSec + workedSec
		}
		currentSeat.State = repository.BreakState
		currentSeat.CurrentStateStartedAt = jstNow
		currentSeat.CurrentStateUntil = breakUntil
		currentSeat.CumulativeWorkSec = cumulativeWorkSec
		currentSeat.DailyCumulativeWorkSec = dailyCumulativeWorkSec
		currentSeat.BreakWorkName = breakOption.WorkName

		if err := s.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in s.Repository.UpdateSeats: %w", err)
		}
		// activityログ記録
		startBreakActivity := repository.UserActivityDoc{
			UserId:       s.ProcessedUserId,
			ActivityType: repository.StartBreakActivity,
			SeatId:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			TakenAt:      utils.JstNow(),
		}
		if err := s.Repository.CreateUserActivityDoc(ctx, tx, startBreakActivity); err != nil {
			return fmt.Errorf("in CreateUserActivityDoc: %w", err)
		}

		var seatIdStr string
		if isInMemberRoom {
			seatIdStr = i18n.T("common:vip-seat-id", currentSeat.SeatId)
		} else {
			seatIdStr = strconv.Itoa(currentSeat.SeatId)
		}

		replyMessage = t("break", s.ProcessedUserDisplayName, breakOption.DurationMin, seatIdStr)
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Break()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Resume(ctx context.Context, command *utils.CommandDetails) error {
	replyMessage := ""
	t := i18n.GetTFunc("command-resume")
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		// stateを確認
		currentSeat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}
		if currentSeat.State != repository.BreakState {
			replyMessage = t("break-only", s.ProcessedUserDisplayName)
			return nil
		}

		// 再開処理
		jstNow := utils.JstNow()
		until := currentSeat.Until
		breakSec := int(utils.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt)).Seconds())
		// もし日付を跨いで休憩してたら、daily-cumulative-work-secは0にリセットする
		var dailyCumulativeWorkSec = currentSeat.DailyCumulativeWorkSec
		if breakSec > utils.SecondsOfDay(jstNow) {
			dailyCumulativeWorkSec = 0
		}
		// 作業名が指定されていなかったら、既存の作業名を引継ぎ
		var workName = command.ResumeOption.WorkName
		if !command.ResumeOption.IsWorkNameSet {
			workName = currentSeat.WorkName
		}

		currentSeat.State = repository.WorkState
		currentSeat.CurrentStateStartedAt = jstNow
		currentSeat.CurrentStateUntil = until
		currentSeat.DailyCumulativeWorkSec = dailyCumulativeWorkSec
		currentSeat.WorkName = workName

		if err := s.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in s.Repository.UpdateSeats: %w", err)
		}
		// activityログ記録
		endBreakActivity := repository.UserActivityDoc{
			UserId:       s.ProcessedUserId,
			ActivityType: repository.EndBreakActivity,
			SeatId:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			TakenAt:      utils.JstNow(),
		}
		if err := s.Repository.CreateUserActivityDoc(ctx, tx, endBreakActivity); err != nil {
			return fmt.Errorf("in CreateUserActivityDoc: %w", err)
		}

		var seatIdStr string
		if isInMemberRoom {
			seatIdStr = i18n.T("common:vip-seat-id", currentSeat.SeatId)
		} else {
			seatIdStr = strconv.Itoa(currentSeat.SeatId)
		}

		untilExitDuration := utils.NoNegativeDuration(until.Sub(jstNow))
		replyMessage = t("work", s.ProcessedUserDisplayName, seatIdStr, int(untilExitDuration.Minutes()))
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Resume()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Rank(_ *utils.CommandDetails, ctx context.Context) error {
	replyMessage := ""
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 変更前のuserDocを読み込んでおく
		userDoc, err := s.Repository.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom

		var currentSeat repository.SeatDoc
		var realtimeTotalStudySec int
		if isInRoom {
			var err error
			currentSeat, err = s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("failed s.CurrentSeat(): %w", err)
			}

			realtimeTotalStudyDuration, _, err := s.GetUserRealtimeTotalStudyDurations(ctx, tx, s.ProcessedUserId)
			if err != nil {
				return fmt.Errorf("in RetrieveRealtimeTotalStudyDuration: %w", err)
			}
			realtimeTotalStudySec = int(realtimeTotalStudyDuration.Seconds())
		}

		// 以降書き込みのみ

		// ランク表示設定のON/OFFを切り替える
		newRankVisible := !userDoc.RankVisible
		if err := s.Repository.UpdateUserRankVisible(tx, s.ProcessedUserId, newRankVisible); err != nil {
			return fmt.Errorf("in UpdateUserRankVisible: %w", err)
		}
		var newValueStr string
		if newRankVisible {
			newValueStr = i18n.T("common:on")
		} else {
			newValueStr = i18n.T("common:off")
		}
		replyMessage = i18n.T("command:rank", s.ProcessedUserDisplayName, newValueStr)

		// 入室中であれば、座席の色も変える
		if isInRoom {
			seatAppearance, err := utils.GetSeatAppearance(realtimeTotalStudySec, newRankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
			if err != nil {
				return fmt.Errorf("in GetSeatAppearance: %w", err)
			}

			// 席の色を更新
			currentSeat.Appearance = seatAppearance
			if err := s.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
				return fmt.Errorf("in s.Repository.UpdateSeat(): %w", err)
			}
		}

		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Rank()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Order(ctx context.Context, command *utils.CommandDetails) error {
	replyMessage := ""
	t := i18n.GetTFunc("command-order")
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		// メンバーでないなら本日の注文回数をチェック
		todayOrderCount, err := s.Repository.CountUserOrdersOfTheDay(ctx, s.ProcessedUserId, utils.JstNow())
		if err != nil {
			return fmt.Errorf("in CountUserOrdersOfTheDay: %w", err)
		}
		if !s.ProcessedUserIsMember && !command.OrderOption.ClearFlag { // 下膳の場合はスキップ
			if todayOrderCount >= int64(s.Configs.Constants.MaxDailyOrderCount) {
				replyMessage = t("too-many-orders", s.ProcessedUserDisplayName, s.Configs.Constants.MaxDailyOrderCount)
				return nil
			}
		}

		currentSeat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}

		// これ以降は書き込みのみ

		if command.OrderOption.ClearFlag {
			// 食器を下げる（注文履歴は削除しない）
			currentSeat.MenuCode = ""
			err := s.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in UpdateSeat: %w", err)
			}
			replyMessage = t("cleared", s.ProcessedUserDisplayName)
			return nil
		}

		targetMenuItem, err := s.GetMenuItemByNumber(command.OrderOption.IntValue)
		if err != nil {
			return fmt.Errorf("in GetMenuItemByNumber: %w", err)
		}

		// 注文履歴を作成
		orderHistoryDoc := repository.OrderHistoryDoc{
			UserId:       s.ProcessedUserId,
			MenuCode:     targetMenuItem.Code,
			SeatId:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			OrderedAt:    utils.JstNow(),
		}
		if err := s.Repository.CreateOrderHistoryDoc(ctx, tx, orderHistoryDoc); err != nil {
			return fmt.Errorf("in CreateOrderHistoryDoc: %w", err)
		}

		// 座席ドキュメントを更新
		currentSeat.MenuCode = targetMenuItem.Code
		err = s.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("in UpdateSeat: %w", err)
		}

		replyMessage = t("ordered", s.ProcessedUserDisplayName, targetMenuItem.Name, todayOrderCount+1)
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Order()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

// RandomAvailableSeatIdForUser
// ルームの席が空いているならその中からランダムな席番号（該当ユーザーの入室上限にかからない範囲に限定）を、
// 空いていないならmax-seatsを増やし、最小の空席番号を返す。
func (s *WorkspaceApp) RandomAvailableSeatIdForUser(ctx context.Context, tx *firestore.Transaction, userId string, isMemberSeat bool) (int,
	error) {
	var seats []repository.SeatDoc
	var err error
	if isMemberSeat {
		seats, err = s.Repository.ReadMemberSeats(ctx)
		if err != nil {
			return 0, fmt.Errorf("in ReadMemberSeats: %w", err)
		}
	} else {
		seats, err = s.Repository.ReadGeneralSeats(ctx)
		if err != nil {
			return 0, fmt.Errorf("in ReadGeneralSeats: %w", err)
		}
	}

	constants, err := s.Repository.ReadSystemConstantsConfig(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("in ReadSystemConstantsConfig: %w", err)
	}
	var maxSeats int
	if isMemberSeat {
		maxSeats = constants.MemberMaxSeats
	} else {
		maxSeats = constants.MaxSeats
	}

	var vacantSeatIdList []int
	for id := 1; id <= maxSeats; id++ {
		isUsed := false
		for _, seatInUse := range seats {
			if id == seatInUse.SeatId {
				isUsed = true
				break
			}
		}
		if !isUsed {
			vacantSeatIdList = append(vacantSeatIdList, id)
		}
	}

	if len(vacantSeatIdList) > 0 {
		// 入室制限にかからない席を選ぶ
		r := rand.New(rand.NewSource(utils.JstNow().UnixNano()))
		r.Shuffle(len(vacantSeatIdList), func(i, j int) { vacantSeatIdList[i], vacantSeatIdList[j] = vacantSeatIdList[j], vacantSeatIdList[i] })
		for _, seatId := range vacantSeatIdList {
			ifSittingTooMuch, err := s.CheckIfUserSittingTooMuchForSeat(ctx, userId, seatId, isMemberSeat)
			if err != nil {
				return -1, fmt.Errorf("in CheckIfUserSittingTooMuchForSeat: %w", err)
			}
			if !ifSittingTooMuch {
				return seatId, nil
			}
		}
	}
	return 0, studyspaceerror.ErrNoSeatAvailable
}

// enterRoom ユーザーを入室させる。
func (s *WorkspaceApp) enterRoom(
	ctx context.Context,
	tx *firestore.Transaction,
	userId string,
	userDisplayName string,
	userProfileImageUrl string,
	seatId int,
	isMemberSeat bool,
	workName string,
	breakWorkName string,
	workMin int,
	seatAppearance repository.SeatAppearance,
	menuCode string,
	state repository.SeatState,
	isContinuousActive bool,
	breakStartedAt time.Time, // set when moving seat
	breakUntil time.Time, // set when moving seat
	enterDate time.Time,
) (int, error) {
	exitDate := enterDate.Add(time.Duration(workMin) * time.Minute)

	var currentStateStartedAt time.Time
	var currentStateUntil time.Time
	switch state {
	case repository.WorkState:
		currentStateStartedAt = enterDate
		currentStateUntil = exitDate
	case repository.BreakState:
		currentStateStartedAt = breakStartedAt
		currentStateUntil = breakUntil
	}

	newSeat := repository.SeatDoc{
		SeatId:                 seatId,
		UserId:                 userId,
		UserDisplayName:        userDisplayName,
		UserProfileImageUrl:    userProfileImageUrl,
		WorkName:               workName,
		BreakWorkName:          breakWorkName,
		EnteredAt:              enterDate,
		Until:                  exitDate,
		Appearance:             seatAppearance,
		MenuCode:               menuCode,
		State:                  state,
		CurrentStateStartedAt:  currentStateStartedAt,
		CurrentStateUntil:      currentStateUntil,
		CumulativeWorkSec:      0,
		DailyCumulativeWorkSec: 0,
	}
	if err := s.Repository.CreateSeat(tx, newSeat, isMemberSeat); err != nil {
		return 0, fmt.Errorf("in CreateSeat: %w", err)
	}

	// 入室時刻を記録
	if err := s.Repository.UpdateUserLastEnteredDate(tx, userId, enterDate); err != nil {
		return 0, fmt.Errorf("in UpdateUserLastEnteredDate: %w", err)
	}
	// activityログ記録
	enterActivity := repository.UserActivityDoc{
		UserId:       userId,
		ActivityType: repository.EnterRoomActivity,
		SeatId:       seatId,
		IsMemberSeat: isMemberSeat,
		TakenAt:      enterDate,
	}
	if err := s.Repository.CreateUserActivityDoc(ctx, tx, enterActivity); err != nil {
		return 0, fmt.Errorf("in CreateUserActivityDoc: %w", err)
	}
	// 久しぶりの入室であれば、isContinuousActiveをtrueに、lastPenaltyImposedDaysを0に更新
	if !isContinuousActive {
		if err := s.Repository.UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(ctx, tx, userId, true, enterDate); err != nil {
			return 0, fmt.Errorf("in UpdateUserIsContinuousActiveAndCurrentActivityStateStarted: %w", err)
		}
		if err := s.Repository.UpdateUserLastPenaltyImposedDays(ctx, tx, userId, 0); err != nil {
			return 0, fmt.Errorf("in UpdateUserLastPenaltyImposedDays: %w", err)
		}
	}

	// 入室から自動退室予定時刻までの時間（分）
	untilExitMin := int(exitDate.Sub(enterDate).Minutes())

	return untilExitMin, nil
}

// exitRoom ユーザーを退室させる。
func (s *WorkspaceApp) exitRoom(
	ctx context.Context,
	tx *firestore.Transaction,
	isMemberSeat bool,
	previousSeat repository.SeatDoc,
	previousUserDoc *repository.UserDoc,
) (int, int, error) {
	// 作業時間を計算
	exitDate := utils.JstNow()
	var addedWorkedTimeSec int
	var addedDailyWorkedTimeSec int
	switch previousSeat.State {
	case repository.BreakState:
		addedWorkedTimeSec = previousSeat.CumulativeWorkSec
		// もし直前の休憩で日付を跨いでたら
		justBreakTimeSec := int(utils.NoNegativeDuration(exitDate.Sub(previousSeat.CurrentStateStartedAt)).Seconds())
		if justBreakTimeSec > utils.SecondsOfDay(exitDate) {
			addedDailyWorkedTimeSec = 0
		} else {
			addedDailyWorkedTimeSec = previousSeat.DailyCumulativeWorkSec
		}
	case repository.WorkState:
		justWorkedTimeSec := int(utils.NoNegativeDuration(exitDate.Sub(previousSeat.CurrentStateStartedAt)).Seconds())
		addedWorkedTimeSec = previousSeat.CumulativeWorkSec + justWorkedTimeSec
		// もし日付変更を跨いで入室してたら、当日の累計時間は日付変更からの時間にする
		if justWorkedTimeSec > utils.SecondsOfDay(exitDate) {
			addedDailyWorkedTimeSec = utils.SecondsOfDay(exitDate)
		} else {
			addedDailyWorkedTimeSec = previousSeat.DailyCumulativeWorkSec + justWorkedTimeSec
		}
	}

	// 退室処理
	if err := s.Repository.DeleteSeat(ctx, tx, previousSeat.SeatId, isMemberSeat); err != nil {
		return 0, 0, fmt.Errorf("in DeleteSeat: %w", err)
	}

	// ログ記録
	exitActivity := repository.UserActivityDoc{
		UserId:       previousSeat.UserId,
		ActivityType: repository.ExitRoomActivity,
		SeatId:       previousSeat.SeatId,
		IsMemberSeat: isMemberSeat,
		TakenAt:      exitDate,
	}
	if err := s.Repository.CreateUserActivityDoc(ctx, tx, exitActivity); err != nil {
		return 0, 0, fmt.Errorf("in CreateUserActivityDoc: %w", err)
	}
	// 退室時刻を記録
	if err := s.Repository.UpdateUserLastExitedDate(tx, previousSeat.UserId, exitDate); err != nil {
		return 0, 0, fmt.Errorf("in UpdateUserLastExitedDate: %w", err)
	}
	// 累計作業時間を更新
	if err := s.UpdateTotalWorkTime(tx, previousSeat.UserId, previousUserDoc, addedWorkedTimeSec, addedDailyWorkedTimeSec); err != nil {
		return 0, 0, fmt.Errorf("in UpdateTotalWorkTime: %w", err)
	}
	// RP更新
	netStudyDuration := time.Duration(addedWorkedTimeSec) * time.Second
	newRP, err := utils.CalcNewRPExitRoom(netStudyDuration, previousSeat.WorkName != "", previousUserDoc.IsContinuousActive, previousUserDoc.CurrentActivityStateStarted, exitDate, previousUserDoc.RankPoint)
	if err != nil {
		return 0, 0, fmt.Errorf("in CalcNewRPExitRoom: %w", err)
	}
	if err := s.Repository.UpdateUserRankPoint(tx, previousSeat.UserId, newRP); err != nil {
		return 0, 0, fmt.Errorf("in UpdateUserRP: %w", err)
	}
	addedRP := newRP - previousUserDoc.RankPoint

	slog.Info("user exited the room.",
		"userId", previousSeat.UserId,
		"seatId", previousSeat.SeatId,
		"addedWorkedTimeSec", addedWorkedTimeSec,
		"addedRP", addedRP,
		"newRP", newRP,
		"previous RP", previousUserDoc.RankPoint)
	return addedWorkedTimeSec, addedRP, nil
}

func (s *WorkspaceApp) moveSeat(
	ctx context.Context,
	tx *firestore.Transaction,
	targetSeatId int,
	latestUserProfileImage string,
	beforeIsMemberSeat,
	afterIsMemberSeat bool,
	option utils.MinutesAndWorkNameOption,
	previousSeat repository.SeatDoc,
	previousUserDoc *repository.UserDoc,
) (int, int, int, error) {
	jstNow := utils.JstNow()

	// 値チェック
	if targetSeatId == previousSeat.SeatId && beforeIsMemberSeat == afterIsMemberSeat {
		return 0, 0, 0, errors.New("targetSeatId == previousSeat.SeatId && beforeIsMemberSeat == afterIsMemberSeat")
	}

	// 退室
	workedTimeSec, addedRP, err := s.exitRoom(ctx, tx, beforeIsMemberSeat, previousSeat, previousUserDoc)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("in exitRoom for %s: %w", s.ProcessedUserId, err)
	}

	// 入室の準備
	var workName string
	var workMin int
	if option.IsWorkNameSet {
		workName = option.WorkName
	} else {
		workName = previousSeat.WorkName
	}
	if option.IsDurationMinSet {
		workMin = option.DurationMin
	} else {
		workMin = int(utils.NoNegativeDuration(previousSeat.Until.Sub(jstNow)).Minutes())
	}
	newTotalStudyDuration := time.Duration(previousUserDoc.TotalStudySec+workedTimeSec) * time.Second
	newRP := previousUserDoc.RankPoint + addedRP
	newSeatAppearance, err := utils.GetSeatAppearance(int(newTotalStudyDuration.Seconds()), previousUserDoc.RankVisible, newRP, previousUserDoc.FavoriteColor)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("in GetSeatAppearance: %w", err)
	}

	// 入室
	untilExitMin, err := s.enterRoom(
		ctx,
		tx,
		previousSeat.UserId,
		previousSeat.UserDisplayName,
		latestUserProfileImage,
		targetSeatId,
		afterIsMemberSeat,
		workName,
		previousSeat.BreakWorkName,
		workMin,
		newSeatAppearance,
		previousSeat.MenuCode,
		previousSeat.State,
		previousUserDoc.IsContinuousActive,
		previousSeat.CurrentStateStartedAt,
		previousSeat.CurrentStateUntil,
		utils.JstNow())
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to enterRoom for %s: %w", previousSeat.UserId, err)
	}

	return workedTimeSec, addedRP, untilExitMin, nil
}
