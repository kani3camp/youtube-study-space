package core

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"reflect"
	"strconv"
	"time"

	"google.golang.org/api/youtube/v3"

	"app.modules/core/discordbot"
	"app.modules/core/guardians"
	"app.modules/core/i18n"
	"app.modules/core/myfirestore"
	"app.modules/core/myspreadsheet"
	"app.modules/core/studyspaceerror"
	"app.modules/core/utils"
	"app.modules/core/youtubebot"
	"cloud.google.com/go/firestore"
	"github.com/kr/pretty"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewSystem(ctx context.Context, interactive bool, clientOption option.ClientOption) (*System, error) {
	if err := i18n.LoadLocaleFolderFS(); err != nil {
		return nil, fmt.Errorf("in LoadLocaleFolderFS(): %w", err)
	}

	fsController, err := myfirestore.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return nil, fmt.Errorf("in NewFirestoreController(): %w", err)
	}

	// credentials
	credentialsDoc, err := fsController.ReadCredentialsConfig(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("in ReadCredentialsConfig(): %w", err)
	}

	// YouTube live chatbot
	liveChatBot, err := youtubebot.NewYoutubeLiveChatBot(credentialsDoc.YoutubeLiveChatId, fsController, ctx)
	if err != nil {
		return nil, fmt.Errorf("in NewYoutubeLiveChatBot(): %w", err)
	}

	// discord bot for system owner
	discordOwnerBot, err := discordbot.NewDiscordBot(credentialsDoc.DiscordOwnerBotToken, credentialsDoc.DiscordOwnerBotTextChannelId)
	if err != nil {
		return nil, fmt.Errorf("in NewDiscordBot(): %w", err)
	}

	// discord bot for sharing with moderators
	discordSharedBot, err := discordbot.NewDiscordBot(credentialsDoc.DiscordSharedBotToken, credentialsDoc.DiscordSharedBotTextChannelId)
	if err != nil {
		return nil, fmt.Errorf("in NewDiscordBot(): %w", err)
	}

	// discord bot for logging
	discordSharedLogBot, err := discordbot.NewDiscordBot(credentialsDoc.DiscordSharedBotToken, credentialsDoc.DiscordSharedBotLogChannelId)
	if err != nil {
		return nil, fmt.Errorf("in NewDiscordBot(): %w", err)
	}

	// core constant values
	constantsConfig, err := fsController.ReadSystemConstantsConfig(ctx, nil)
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

	ssc, err := myspreadsheet.NewSpreadsheetController(ctx, clientOption, configs.Constants.BotConfigSpreadsheetId, "01", "02")
	if err != nil {
		return nil, fmt.Errorf("in NewSpreadsheetController(): %w", err)
	}
	blockRegexListForChannelName, blockRegexListForChatMessage, err := ssc.GetRegexForBlock()
	if err != nil {
		return nil, fmt.Errorf("in GetRegexForBlock(): %w", err)
	}
	notificationRegexListForChatMessage, notificationRegexListForChannelName, err := ssc.GetRegexForNotification()
	if err != nil {
		return nil, fmt.Errorf("in GetRegexForNotification(): %w", err)
	}

	return &System{
		Configs:                             &configs,
		FirestoreController:                 fsController,
		liveChatBot:                         liveChatBot,
		discordOwnerBot:                     discordOwnerBot,
		discordSharedBot:                    discordSharedBot,
		discordSharedLogBot:                 discordSharedLogBot,
		blockRegexListForChannelName:        blockRegexListForChannelName,
		blockRegexListForChatMessage:        blockRegexListForChatMessage,
		notificationRegexListForChatMessage: notificationRegexListForChatMessage,
		notificationRegexListForChannelName: notificationRegexListForChannelName,
	}, nil
}

func (s *System) RunTransaction(ctx context.Context, f func(ctx context.Context, tx *firestore.Transaction) error) error {
	return s.FirestoreController.FirestoreClient().RunTransaction(ctx, f)
}

func (s *System) SetProcessedUser(userId string, userDisplayName string, userProfileImageUrl string, isChatModerator bool, isChatOwner bool, isChatMember bool) {
	s.ProcessedUserId = userId
	s.ProcessedUserDisplayName = userDisplayName
	s.ProcessedUserProfileImageUrl = userProfileImageUrl
	s.ProcessedUserIsModeratorOrOwner = isChatModerator || isChatOwner
	s.ProcessedUserIsMember = isChatMember
}

func (s *System) CloseFirestoreClient() {
	if err := s.FirestoreController.FirestoreClient().Close(); err != nil {
		slog.Error("failed close firestore client.")
	} else {
		slog.Info("successfully closed firestore client.")
	}
}

func (s *System) GetInfoString() string {
	numAllFilteredRegex := len(s.blockRegexListForChatMessage) + len(s.blockRegexListForChannelName) + len(s.notificationRegexListForChatMessage) + len(s.notificationRegexListForChannelName)
	return fmt.Sprintf("全規制ワード数: %d", numAllFilteredRegex)
}

// GoroutineCheckLongTimeSitting 長時間座席占有検出ループ
func (s *System) GoroutineCheckLongTimeSitting(ctx context.Context) {
	minimumInterval := time.Duration(s.Configs.Constants.MinimumCheckLongTimeSittingIntervalMinutes) * time.Minute
	slog.Info("", "居座りチェックの最小間隔", minimumInterval)

	for {
		slog.Info("checking long time sitting.")
		start := utils.JstNow()

		{
			if err := s.CheckLongTimeSitting(ctx, true); err != nil {
				s.MessageToOwnerWithError("in CheckLongTimeSitting", err)
			}
		}
		{
			if err := s.CheckLongTimeSitting(ctx, false); err != nil {
				s.MessageToOwnerWithError("in CheckLongTimeSitting", err)
			}
		}

		end := utils.JstNow()
		duration := end.Sub(start)
		if duration < minimumInterval {
			time.Sleep(utils.NoNegativeDuration(minimumInterval - duration))
		}
	}
}

func (s *System) CheckIfUnwantedWordIncluded(ctx context.Context, userId, message, channelName string) (bool, error) {
	// ブロック対象チェック
	found, index, err := utils.ContainsRegexWithIndex(s.blockRegexListForChatMessage, message)
	if err != nil {
		return false, err
	}
	if found {
		if err := s.BanUser(ctx, userId); err != nil {
			return false, fmt.Errorf("in BanUser(): %w", err)
		}
		return true, s.LogToSharedDiscord("発言から禁止ワードを検出、ユーザーをブロックしました。" +
			"\n禁止ワード: `" + s.blockRegexListForChatMessage[index] + "`" +
			"\nチャンネル名: `" + channelName + "`" +
			"\nチャンネルURL: https://youtube.com/channel/" + userId +
			"\nチャット内容: `" + message + "`" +
			"\n日時: " + utils.JstNow().String())
	}
	found, index, err = utils.ContainsRegexWithIndex(s.blockRegexListForChannelName, channelName)
	if err != nil {
		return false, fmt.Errorf("in ContainsRegexWithIndex(): %w", err)
	}
	if found {
		if err := s.BanUser(ctx, userId); err != nil {
			return false, fmt.Errorf("in BanUser(): %w", err)
		}
		return true, s.LogToSharedDiscord("チャンネル名から禁止ワードを検出、ユーザーをブロックしました。" +
			"\n禁止ワード: `" + s.blockRegexListForChannelName[index] + "`" +
			"\nチャンネル名: `" + channelName + "`" +
			"\nチャンネルURL: https://youtube.com/channel/" + userId +
			"\nチャット内容: `" + message + "`" +
			"\n日時: " + utils.JstNow().String())
	}

	// 通知対象チェック
	found, index, err = utils.ContainsRegexWithIndex(s.notificationRegexListForChatMessage, message)
	if err != nil {
		return false, fmt.Errorf("in ContainsRegexWithIndex(): %w", err)
	}
	if found {
		return false, s.MessageToSharedDiscord("発言から禁止ワードを検出しました。（通知のみ）" +
			"\n禁止ワード: `" + s.notificationRegexListForChatMessage[index] + "`" +
			"\nチャンネル名: `" + channelName + "`" +
			"\nチャンネルURL: https://youtube.com/channel/" + userId +
			"\nチャット内容: `" + message + "`" +
			"\n日時: " + utils.JstNow().String())
	}
	found, index, err = utils.ContainsRegexWithIndex(s.notificationRegexListForChannelName, channelName)
	if err != nil {
		return false, fmt.Errorf("in ContainsRegexWithIndex(): %w", err)
	}
	if found {
		return false, s.MessageToSharedDiscord("チャンネルから禁止ワードを検出しました。（通知のみ）" +
			"\n禁止ワード: `" + s.notificationRegexListForChannelName[index] + "`" +
			"\nチャンネル名: `" + channelName + "`" +
			"\nチャンネルURL: https://youtube.com/channel/" + userId +
			"\nチャット内容: `" + message + "`" +
			"\n日時: " + utils.JstNow().String())
	}
	return false, nil
}

func (s *System) AdjustMaxSeats(ctx context.Context) error {
	slog.Info(utils.NameOf(s.AdjustMaxSeats))
	// UpdateDesiredMaxSeats()などはLambdaからも並列で実行される可能性があるが、競合が起こってもそこまで深刻な問題にはならないためトランザクションは使用しない。

	constants, err := s.FirestoreController.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return fmt.Errorf("in ReadSystemConstantsConfig(): %w", err)
	}

	// 一般席
	if constants.DesiredMaxSeats > constants.MaxSeats { // 一般席を増やす
		s.MessageToLiveChat(ctx, "席を増やします↗")
		if err := s.FirestoreController.UpdateMaxSeats(ctx, nil, constants.DesiredMaxSeats); err != nil {
			return fmt.Errorf("in UpdateMaxSeats(): %w", err)
		}
	} else if constants.DesiredMaxSeats < constants.MaxSeats { // 一般席を減らす
		if constants.FixedMaxSeatsEnabled { // 空席率に関係なく、max_seatsをdesiredに合わせる
			// なくなる座席にいるユーザーは退出させる
			seats, err := s.FirestoreController.ReadGeneralSeats(ctx)
			if err != nil {
				return err
			}
			s.MessageToLiveChat(ctx, "座席数を"+strconv.Itoa(constants.DesiredMaxSeats)+"に固定します↘ 必要な場合は退出してもらうことがあります。")
			for _, seat := range seats {
				if seat.SeatId > constants.DesiredMaxSeats {
					s.SetProcessedUser(seat.UserId, seat.UserDisplayName, seat.UserProfileImageUrl, false, false, false)
					// 移動させる
					outCommandDetails := &utils.CommandDetails{
						CommandType: utils.Out,
					}
					if err := s.Out(outCommandDetails, ctx); err != nil {
						return fmt.Errorf("in Out(): %w", err)
					}
				}
			}

			// max_seatsを更新
			if err := s.FirestoreController.UpdateMaxSeats(ctx, nil, constants.DesiredMaxSeats); err != nil {
				return fmt.Errorf("in UpdateMaxSeats(): %w", err)
			}
		} else {
			// max_seatsを減らしても、空席率が設定値以上か確認
			seats, err := s.FirestoreController.ReadGeneralSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadGeneralSeats(): %w", err)
			}
			if int(float32(constants.DesiredMaxSeats)*(1.0-constants.MinVacancyRate)) < len(seats) {
				slog.Info("減らそうとしすぎ。desiredは却下します。",
					"desired", constants.DesiredMaxSeats,
					"current max seats", constants.MaxSeats,
					"current seats length", len(seats))
				if err := s.FirestoreController.UpdateDesiredMaxSeats(ctx, nil, constants.MaxSeats); err != nil {
					return fmt.Errorf("in UpdateDesiredMaxSeats(): %w", err)
				}
			} else {
				// 消えてしまう席にいるユーザーを移動させる
				s.MessageToLiveChat(ctx, "人数が減ったため席を減らします↘ 必要な場合は席を移動してもらうことがあります。")
				for _, seat := range seats {
					if seat.SeatId > constants.DesiredMaxSeats {
						s.SetProcessedUser(seat.UserId, seat.UserDisplayName, seat.UserProfileImageUrl, false, false, false)
						// 移動させる
						inCommandDetails := &utils.CommandDetails{
							CommandType: utils.In,
							InOption: utils.InOption{
								IsSeatIdSet: true,
								SeatId:      0,
								MinutesAndWorkName: &utils.MinutesAndWorkNameOption{
									IsWorkNameSet:    true,
									IsDurationMinSet: true,
									WorkName:         seat.WorkName,
									DurationMin:      int(utils.NoNegativeDuration(seat.Until.Sub(utils.JstNow())).Minutes()),
								},
								IsMemberSeat: false,
							},
						}
						if err := s.In(ctx, inCommandDetails); err != nil {
							return fmt.Errorf("in In(): %w", err)
						}
					}
				}
				// max_seatsを更新
				if err := s.FirestoreController.UpdateMaxSeats(ctx, nil, constants.DesiredMaxSeats); err != nil {
					return fmt.Errorf("in UpdateMaxSeats(): %w", err)
				}
			}
		}
	}

	// メンバー席
	if constants.DesiredMemberMaxSeats > constants.MemberMaxSeats { // メンバー席を増やす
		s.MessageToLiveChat(ctx, "メンバー限定の席を増やします↗")
		if err := s.FirestoreController.UpdateMemberMaxSeats(ctx, nil, constants.DesiredMemberMaxSeats); err != nil {
			return fmt.Errorf("in UpdateMemberMaxSeats(): %w", err)
		}
	} else if constants.DesiredMemberMaxSeats < constants.MemberMaxSeats { // メンバー席を減らす
		if constants.FixedMaxSeatsEnabled { // 空席率に関係なく、member_max_seatsをdesiredに合わせる
			// なくなる座席にいるユーザーは退出させる
			seats, err := s.FirestoreController.ReadMemberSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadMemberSeats(): %w", err)
			}
			s.MessageToLiveChat(ctx, "メンバー限定の座席数を"+strconv.Itoa(constants.DesiredMemberMaxSeats)+"に固定します↘ 必要な場合は退出してもらうことがあります。")
			for _, seat := range seats {
				if seat.SeatId > constants.DesiredMemberMaxSeats {
					s.SetProcessedUser(seat.UserId, seat.UserDisplayName, seat.UserProfileImageUrl, false, false, false)
					// 移動させる
					outCommandDetails := &utils.CommandDetails{
						CommandType: utils.Out,
					}
					if err := s.Out(outCommandDetails, ctx); err != nil {
						return fmt.Errorf("in Out(): %w", err)
					}
				}
			}
			// member_max_seatsを更新
			if err := s.FirestoreController.UpdateMemberMaxSeats(ctx, nil, constants.DesiredMemberMaxSeats); err != nil {
				return fmt.Errorf("in UpdateMemberMaxSeats(): %w", err)
			}
		} else {
			// member_max_seatsを減らしても、空席率が設定値以上か確認
			seats, err := s.FirestoreController.ReadMemberSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadMemberSeats(): %w", err)
			}
			if int(float32(constants.DesiredMemberMaxSeats)*(1.0-constants.MinVacancyRate)) < len(seats) {
				slog.Warn("減らそうとしすぎ。desiredは却下します。",
					"desired", constants.DesiredMaxSeats,
					"current member max seats", constants.MemberMaxSeats,
					"current seats length", len(seats))
				if err := s.FirestoreController.UpdateDesiredMemberMaxSeats(ctx, nil, constants.MemberMaxSeats); err != nil {
					return fmt.Errorf("in UpdateDesiredMemberMaxSeats(): %w", err)
				}
			} else {
				// 消えてしまう席にいるユーザーを移動させる
				s.MessageToLiveChat(ctx, "人数が減ったためメンバー限定席を減らします↘ 必要な場合は席を移動してもらうことがあります。")
				for _, seat := range seats {
					if seat.SeatId > constants.DesiredMemberMaxSeats {
						s.SetProcessedUser(seat.UserId, seat.UserDisplayName, seat.UserProfileImageUrl, false, false, true)
						// 移動させる
						inCommandDetails := &utils.CommandDetails{
							CommandType: utils.In,
							InOption: utils.InOption{
								IsSeatIdSet: true,
								SeatId:      0,
								MinutesAndWorkName: &utils.MinutesAndWorkNameOption{
									IsWorkNameSet:    true,
									IsDurationMinSet: true,
									WorkName:         seat.WorkName,
									DurationMin:      int(utils.NoNegativeDuration(seat.Until.Sub(utils.JstNow())).Minutes()),
								},
								IsMemberSeat: true,
							},
						}

						if err = s.In(ctx, inCommandDetails); err != nil {
							return fmt.Errorf("in In(): %w", err)
						}
					}
				}
				// member_max_seatsを更新
				if err := s.FirestoreController.UpdateMemberMaxSeats(ctx, nil, constants.DesiredMemberMaxSeats); err != nil {
					return fmt.Errorf("in UpdateMemberMaxSeats(): %w", err)
				}
			}
		}
	}

	return nil
}

// Command 入力コマンドを解析して実行
func (s *System) Command(
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
			s.MessageToOwnerWithError("in CheckIfUnwantedWordIncluded", err)
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

	commandDetails, message := utils.ParseCommand(commandString, isChatMember)
	if message != "" { // これはシステム内部のエラーではなく、入力コマンドが不正ということなので、return nil
		s.MessageToLiveChat(ctx, i18n.T("common:sir", s.ProcessedUserDisplayName)+message)
		return nil
	}

	if message = s.ValidateCommand(*commandDetails); message != "" {
		s.MessageToLiveChat(ctx, i18n.T("common:sir", s.ProcessedUserDisplayName)+message)
		return nil
	}

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
	default:
		return errors.New("Unknown command: " + commandString)
	}
}

func (s *System) In(ctx context.Context, command *utils.CommandDetails) error {
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

		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
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
		var currentSeat myfirestore.SeatDoc
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
				myfirestore.WorkState,
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
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

// GetUserRealtimeSeatAppearance リアルタイムの現在のランクを求める
func (s *System) GetUserRealtimeSeatAppearance(ctx context.Context, tx *firestore.Transaction, userId string) (myfirestore.SeatAppearance, error) {
	userDoc, err := s.FirestoreController.ReadUser(ctx, tx, userId)
	if err != nil {
		return myfirestore.SeatAppearance{}, fmt.Errorf("in ReadUser(): %w", err)
	}
	totalStudyDuration, _, err := s.GetUserRealtimeTotalStudyDurations(ctx, tx, userId)
	if err != nil {
		return myfirestore.SeatAppearance{}, fmt.Errorf("in GetUserRealtimeTotalStudyDurations(): %w", err)
	}
	seatAppearance, err := utils.GetSeatAppearance(int(totalStudyDuration.Seconds()), userDoc.RankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
	if err != nil {
		return myfirestore.SeatAppearance{}, fmt.Errorf("in GetSeatAppearance(): %w", err)
	}
	return seatAppearance, nil
}

func (s *System) Out(_ *utils.CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-out")
	var replyMessage string
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
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
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *System) ShowUserInfo(command *utils.CommandDetails, ctx context.Context) error {
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

		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in s.FirestoreController.ReadUser: %w", err)
		}

		if userDoc.RankVisible {
			replyMessage += t("rank", userDoc.RankPoint)
		}

		if command.InfoOption.ShowDetails {
			switch userDoc.RankVisible {
			case true:
				replyMessage += t("rank-on")
				if userDoc.IsContinuousActive {
					continuousActiveDays := int(utils.JstNow().Sub(userDoc.CurrentActivityStateStarted).Hours() / 24)
					replyMessage += t("rank-on-continuous", continuousActiveDays+1, continuousActiveDays)
				}
			case false:
				replyMessage += t("rank-off")
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
		replyMessage = i18n.T("command:error")
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *System) ShowSeatInfo(command *utils.CommandDetails, ctx context.Context) error {
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
			case myfirestore.WorkState:
				stateStr = i18n.T("common:work")
				breakUntilStr = ""
			case myfirestore.BreakState:
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
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *System) Report(command *utils.CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-report")
	if command.ReportOption.Message == "" { // !reportのみは不可
		s.MessageToLiveChat(ctx, t("no-message", s.ProcessedUserDisplayName))
		return nil
	}

	ownerMessage := t("owner", utils.ReportCommand, s.ProcessedUserId, s.ProcessedUserDisplayName, command.ReportOption.Message)
	s.MessageToOwner(ownerMessage)

	discordMessage := t("discord", utils.ReportCommand, s.ProcessedUserDisplayName, command.ReportOption.Message)
	if err := s.MessageToSharedDiscord(discordMessage); err != nil {
		s.MessageToOwnerWithError("モデレーターへメッセージが送信できませんでした: \""+discordMessage+"\"", err)
	}

	s.MessageToLiveChat(ctx, t("alert", s.ProcessedUserDisplayName))
	return nil
}

func (s *System) Kick(command *utils.CommandDetails, ctx context.Context) error {
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
		targetSeat, err := s.FirestoreController.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
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
		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, targetSeat.UserId)
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
			err := s.LogToSharedDiscord(s.ProcessedUserDisplayName + "さん、" + strconv.Itoa(targetSeat.
				SeatId) + "番席のユーザーをkickしました。\n" +
				"チャンネル名: " + targetSeat.UserDisplayName + "\n" +
				"作業名: " + targetSeat.WorkName + "\n休憩中の作業名: " + targetSeat.BreakWorkName + "\n" +
				"入室時間: " + strconv.Itoa(workedTimeSec/60) + "分\n" +
				"チャンネルURL: https://youtube.com/channel/" + targetSeat.UserId)
			if err != nil {
				return fmt.Errorf("failed LogToSharedDiscord(): %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *System) Check(command *utils.CommandDetails, ctx context.Context) error {
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
		seat, err := s.FirestoreController.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
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
		if err := s.LogToSharedDiscord(message); err != nil {
			return fmt.Errorf("failed LogToSharedDiscord(): %w", err)
		}
		replyMessage = i18n.T("command:sent", s.ProcessedUserDisplayName)
		return nil
	})
	if txErr != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *System) Block(command *utils.CommandDetails, ctx context.Context) error {
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
		targetSeat, err := s.FirestoreController.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = s.ProcessedUserDisplayName + "さん、その番号の座席は誰も使用していません"
				return nil
			}
			s.MessageToOwnerWithError("in ReadSeat", err)
			return fmt.Errorf("in ReadSeat: %w", err)
		}
		replyMessage = s.ProcessedUserDisplayName + "さん、" + strconv.Itoa(targetSeat.SeatId) + "番席の" + targetSeat.UserDisplayName + "さんをブロックします。"

		// s.ProcessedUserが処理の対象ではないことに注意。
		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, targetSeat.UserId)
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
			err := s.LogToSharedDiscord(s.ProcessedUserDisplayName + "さん、" + strconv.Itoa(targetSeat.
				SeatId) + "番席のユーザーをblockしました。\n" +
				"チャンネル名: " + targetSeat.UserDisplayName + "\n" +
				"作業名: " + targetSeat.WorkName + "\n休憩中の作業名: " + targetSeat.BreakWorkName + "\n" +
				"入室時間: " + strconv.Itoa(workedTimeSec/60) + "分\n" +
				"チャンネルURL: https://youtube.com/channel/" + targetSeat.UserId)
			if err != nil {
				return fmt.Errorf("failed LogToSharedDiscord(): %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *System) My(command *utils.CommandDetails, ctx context.Context) error {
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
		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom

		var seats []myfirestore.SeatDoc
		if isInMemberRoom {
			seats, err = s.FirestoreController.ReadMemberSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadMemberSeats: %w", err)
			}
		}
		if isInGeneralRoom {
			seats, err = s.FirestoreController.ReadGeneralSeats(ctx)
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
					if err := s.FirestoreController.UpdateUserRankVisible(tx, s.ProcessedUserId, newRankVisible); err != nil {
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
						if err := s.FirestoreController.UpdateSeat(ctx, tx, newSeat, isInMemberRoom); err != nil {
							return fmt.Errorf("in s.FirestoreController.UpdateSeats: %w", err)
						}
					}
				}
				currenRankVisible = newRankVisible
			} else if myOption.Type == utils.DefaultStudyMin {
				if err := s.FirestoreController.UpdateUserDefaultStudyMin(tx, s.ProcessedUserId, myOption.IntValue); err != nil {
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
				if err := s.FirestoreController.UpdateUserFavoriteColor(tx, s.ProcessedUserId, colorCode); err != nil {
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
					if err := s.FirestoreController.UpdateSeat(ctx, tx, newSeat, isInMemberRoom); err != nil {
						return fmt.Errorf("in s.FirestoreController.UpdateSeat(): %w", err)
					}
				}
			}
		}
		return nil
	})
	if txErr != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *System) Change(command *utils.CommandDetails, ctx context.Context) error {
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
			case myfirestore.WorkState:
				newSeat.WorkName = changeOption.WorkName
				replyMessage += t("update-work", seatIdStr)
			case myfirestore.BreakState:
				newSeat.BreakWorkName = changeOption.WorkName
				replyMessage += t("update-break", seatIdStr)
			}
		}
		if changeOption.IsDurationMinSet {
			switch currentSeat.State {
			case myfirestore.WorkState:
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
			case myfirestore.BreakState:
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
		if err := s.FirestoreController.UpdateSeat(ctx, tx, *newSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in UpdateSeats: %w", err)
		}

		return nil
	})
	if txErr != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *System) More(command *utils.CommandDetails, ctx context.Context) error {
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
		case myfirestore.WorkState:
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
		case myfirestore.BreakState:
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

		if err := s.FirestoreController.UpdateSeat(ctx, tx, *newSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in s.FirestoreController.UpdateSeats: %w", err)
		}

		switch currentSeat.State {
		case myfirestore.WorkState:
			replyMessage += t("reply-work", addedMin)
		case myfirestore.BreakState:
			remainingBreakDuration := utils.NoNegativeDuration(newSeat.CurrentStateUntil.Sub(jstNow))
			replyMessage += t("reply-break", addedMin, int(remainingBreakDuration.Minutes()))
		}
		realtimeEnteredTimeMin := int(utils.NoNegativeDuration(jstNow.Sub(currentSeat.EnteredAt)).Minutes())
		replyMessage += t("reply", realtimeEnteredTimeMin, remainingUntilExitMin)

		return nil
	})
	if txErr != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *System) Break(ctx context.Context, command *utils.CommandDetails) error {
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
		if currentSeat.State != myfirestore.WorkState {
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
		currentSeat.State = myfirestore.BreakState
		currentSeat.CurrentStateStartedAt = jstNow
		currentSeat.CurrentStateUntil = breakUntil
		currentSeat.CumulativeWorkSec = cumulativeWorkSec
		currentSeat.DailyCumulativeWorkSec = dailyCumulativeWorkSec
		currentSeat.BreakWorkName = breakOption.WorkName

		if err := s.FirestoreController.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in s.FirestoreController.UpdateSeats: %w", err)
		}
		// activityログ記録
		startBreakActivity := myfirestore.UserActivityDoc{
			UserId:       s.ProcessedUserId,
			ActivityType: myfirestore.StartBreakActivity,
			SeatId:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			TakenAt:      utils.JstNow(),
		}
		if err := s.FirestoreController.CreateUserActivityDoc(ctx, tx, startBreakActivity); err != nil {
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
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *System) Resume(ctx context.Context, command *utils.CommandDetails) error {
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
		if currentSeat.State != myfirestore.BreakState {
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

		currentSeat.State = myfirestore.WorkState
		currentSeat.CurrentStateStartedAt = jstNow
		currentSeat.CurrentStateUntil = until
		currentSeat.DailyCumulativeWorkSec = dailyCumulativeWorkSec
		currentSeat.WorkName = workName

		if err := s.FirestoreController.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in s.FirestoreController.UpdateSeats: %w", err)
		}
		// activityログ記録
		endBreakActivity := myfirestore.UserActivityDoc{
			UserId:       s.ProcessedUserId,
			ActivityType: myfirestore.EndBreakActivity,
			SeatId:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			TakenAt:      utils.JstNow(),
		}
		if err := s.FirestoreController.CreateUserActivityDoc(ctx, tx, endBreakActivity); err != nil {
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
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *System) Rank(_ *utils.CommandDetails, ctx context.Context) error {
	replyMessage := ""
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 変更前のuserDocを読み込んでおく
		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom

		var currentSeat myfirestore.SeatDoc
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
		if err := s.FirestoreController.UpdateUserRankVisible(tx, s.ProcessedUserId, newRankVisible); err != nil {
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
			if err := s.FirestoreController.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
				return fmt.Errorf("in s.FirestoreController.UpdateSeat(): %w", err)
			}
		}

		return nil
	})
	if txErr != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

// IsSeatExist 席番号1～max-seatsの席かどうかを判定。
func (s *System) IsSeatExist(ctx context.Context, seatId int, isMemberSeat bool) (bool, error) {
	constants, err := s.FirestoreController.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("in ReadSystemConstantsConfig: %w", err)
	}
	if isMemberSeat {
		return 1 <= seatId && seatId <= constants.MemberMaxSeats, nil
	} else {
		return 1 <= seatId && seatId <= constants.MaxSeats, nil
	}
}

// IfSeatVacant 席番号がseatIdの席が空いているかどうか。
func (s *System) IfSeatVacant(ctx context.Context, tx *firestore.Transaction, seatId int, isMemberSeat bool) (bool, error) {
	_, err := s.FirestoreController.ReadSeat(ctx, tx, seatId, isMemberSeat)
	if err != nil {
		if status.Code(err) == codes.NotFound { // その座席のドキュメントは存在しない
			// maxSeats以内かどうか
			isExist, err := s.IsSeatExist(ctx, seatId, isMemberSeat)
			if err != nil {
				return false, fmt.Errorf("in IsSeatExist: %w", err)
			}
			return isExist, nil
		}
		return false, fmt.Errorf("in ReadSeat: %w", err)
	}
	// ここまで来ると指定された番号の席が使われてるということ
	return false, nil
}

func (s *System) IfUserRegistered(ctx context.Context, tx *firestore.Transaction) (bool, error) {
	_, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		} else {
			return false, fmt.Errorf("in ReadUser: %w", err)
		}
	}
	return true, nil
}

// IsUserInRoom そのユーザーがルーム内にいるか？登録済みかに関わらず。
func (s *System) IsUserInRoom(ctx context.Context, userId string) (isInMemberRoom bool, isInGeneralRoom bool, returnErr error) {
	isInMemberRoom = true
	isInGeneralRoom = true
	if _, err := s.FirestoreController.ReadSeatWithUserId(ctx, userId, true); err != nil {
		if status.Code(err) == codes.NotFound {
			isInMemberRoom = false
		} else {
			return false, false, fmt.Errorf("in ReadSeatWithUserId: %w", err)
		}
	}
	if _, err := s.FirestoreController.ReadSeatWithUserId(ctx, userId, false); err != nil {
		if status.Code(err) == codes.NotFound {
			isInGeneralRoom = false
		} else {
			return false, false, fmt.Errorf("in ReadSeatWithUserId: %w", err)
		}
	}
	if isInGeneralRoom && isInMemberRoom {
		return false, false, errors.New("isInGeneralRoom && isInMemberRoom")
	}
	return isInMemberRoom, isInGeneralRoom, nil
}

func (s *System) CreateUser(ctx context.Context, tx *firestore.Transaction) error {
	slog.Info(utils.NameOf(s.CreateUser))
	userData := myfirestore.UserDoc{
		DailyTotalStudySec: 0,
		TotalStudySec:      0,
		RegistrationDate:   utils.JstNow(),
	}
	return s.FirestoreController.CreateUser(ctx, tx, s.ProcessedUserId, userData)
}

func (s *System) GetNextPageToken(ctx context.Context, tx *firestore.Transaction) (string, error) {
	return s.FirestoreController.ReadNextPageToken(ctx, tx)
}

func (s *System) SaveNextPageToken(ctx context.Context, nextPageToken string) error {
	return s.FirestoreController.UpdateNextPageToken(ctx, nextPageToken)
}

// RandomAvailableSeatIdForUser
// ルームの席が空いているならその中からランダムな席番号（該当ユーザーの入室上限にかからない範囲に限定）を、
// 空いていないならmax-seatsを増やし、最小の空席番号を返す。
func (s *System) RandomAvailableSeatIdForUser(ctx context.Context, tx *firestore.Transaction, userId string, isMemberSeat bool) (int,
	error) {
	var seats []myfirestore.SeatDoc
	var err error
	if isMemberSeat {
		seats, err = s.FirestoreController.ReadMemberSeats(ctx)
		if err != nil {
			return 0, fmt.Errorf("in ReadMemberSeats: %w", err)
		}
	} else {
		seats, err = s.FirestoreController.ReadGeneralSeats(ctx)
		if err != nil {
			return 0, fmt.Errorf("in ReadGeneralSeats: %w", err)
		}
	}

	constants, err := s.FirestoreController.ReadSystemConstantsConfig(ctx, tx)
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
func (s *System) enterRoom(
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
	seatAppearance myfirestore.SeatAppearance,
	state myfirestore.SeatState,
	isContinuousActive bool,
	breakStartedAt time.Time, // set when moving seat
	breakUntil time.Time, // set when moving seat
	enterDate time.Time,
) (int, error) {
	exitDate := enterDate.Add(time.Duration(workMin) * time.Minute)

	var currentStateStartedAt time.Time
	var currentStateUntil time.Time
	switch state {
	case myfirestore.WorkState:
		currentStateStartedAt = enterDate
		currentStateUntil = exitDate
	case myfirestore.BreakState:
		currentStateStartedAt = breakStartedAt
		currentStateUntil = breakUntil
	}

	newSeat := myfirestore.SeatDoc{
		SeatId:                 seatId,
		UserId:                 userId,
		UserDisplayName:        userDisplayName,
		UserProfileImageUrl:    userProfileImageUrl,
		WorkName:               workName,
		BreakWorkName:          breakWorkName,
		EnteredAt:              enterDate,
		Until:                  exitDate,
		Appearance:             seatAppearance,
		State:                  state,
		CurrentStateStartedAt:  currentStateStartedAt,
		CurrentStateUntil:      currentStateUntil,
		CumulativeWorkSec:      0,
		DailyCumulativeWorkSec: 0,
	}
	if err := s.FirestoreController.CreateSeat(tx, newSeat, isMemberSeat); err != nil {
		return 0, fmt.Errorf("in CreateSeat: %w", err)
	}

	// 入室時刻を記録
	if err := s.FirestoreController.UpdateUserLastEnteredDate(tx, userId, enterDate); err != nil {
		return 0, fmt.Errorf("in UpdateUserLastEnteredDate: %w", err)
	}
	// activityログ記録
	enterActivity := myfirestore.UserActivityDoc{
		UserId:       userId,
		ActivityType: myfirestore.EnterRoomActivity,
		SeatId:       seatId,
		IsMemberSeat: isMemberSeat,
		TakenAt:      enterDate,
	}
	if err := s.FirestoreController.CreateUserActivityDoc(ctx, tx, enterActivity); err != nil {
		return 0, fmt.Errorf("in CreateUserActivityDoc: %w", err)
	}
	// 久しぶりの入室であれば、isContinuousActiveをtrueに、lastPenaltyImposedDaysを0に更新
	if !isContinuousActive {
		if err := s.FirestoreController.UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(ctx, tx, userId, true, enterDate); err != nil {
			return 0, fmt.Errorf("in UpdateUserIsContinuousActiveAndCurrentActivityStateStarted: %w", err)
		}
		if err := s.FirestoreController.UpdateUserLastPenaltyImposedDays(ctx, tx, userId, 0); err != nil {
			return 0, fmt.Errorf("in UpdateUserLastPenaltyImposedDays: %w", err)
		}
	}

	// 入室から自動退室予定時刻までの時間（分）
	untilExitMin := int(exitDate.Sub(enterDate).Minutes())

	return untilExitMin, nil
}

// exitRoom ユーザーを退室させる。
func (s *System) exitRoom(
	ctx context.Context,
	tx *firestore.Transaction,
	isMemberSeat bool,
	previousSeat myfirestore.SeatDoc,
	previousUserDoc *myfirestore.UserDoc,
) (int, int, error) {
	// 作業時間を計算
	exitDate := utils.JstNow()
	var addedWorkedTimeSec int
	var addedDailyWorkedTimeSec int
	switch previousSeat.State {
	case myfirestore.BreakState:
		addedWorkedTimeSec = previousSeat.CumulativeWorkSec
		// もし直前の休憩で日付を跨いでたら
		justBreakTimeSec := int(utils.NoNegativeDuration(exitDate.Sub(previousSeat.CurrentStateStartedAt)).Seconds())
		if justBreakTimeSec > utils.SecondsOfDay(exitDate) {
			addedDailyWorkedTimeSec = 0
		} else {
			addedDailyWorkedTimeSec = previousSeat.DailyCumulativeWorkSec
		}
	case myfirestore.WorkState:
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
	if err := s.FirestoreController.DeleteSeat(ctx, tx, previousSeat.SeatId, isMemberSeat); err != nil {
		return 0, 0, fmt.Errorf("in DeleteSeat: %w", err)
	}

	// ログ記録
	exitActivity := myfirestore.UserActivityDoc{
		UserId:       previousSeat.UserId,
		ActivityType: myfirestore.ExitRoomActivity,
		SeatId:       previousSeat.SeatId,
		IsMemberSeat: isMemberSeat,
		TakenAt:      exitDate,
	}
	if err := s.FirestoreController.CreateUserActivityDoc(ctx, tx, exitActivity); err != nil {
		return 0, 0, fmt.Errorf("in CreateUserActivityDoc: %w", err)
	}
	// 退室時刻を記録
	if err := s.FirestoreController.UpdateUserLastExitedDate(tx, previousSeat.UserId, exitDate); err != nil {
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
	if err := s.FirestoreController.UpdateUserRankPoint(tx, previousSeat.UserId, newRP); err != nil {
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

func (s *System) moveSeat(
	ctx context.Context,
	tx *firestore.Transaction,
	targetSeatId int,
	latestUserProfileImage string,
	beforeIsMemberSeat,
	afterIsMemberSeat bool,
	option utils.MinutesAndWorkNameOption,
	previousSeat myfirestore.SeatDoc,
	previousUserDoc *myfirestore.UserDoc,
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

func (s *System) CurrentSeat(ctx context.Context, userId string, isMemberSeat bool) (myfirestore.SeatDoc, error) {
	seat, err := s.FirestoreController.ReadSeatWithUserId(ctx, userId, isMemberSeat)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return myfirestore.SeatDoc{}, studyspaceerror.ErrUserNotInTheRoom
		}
		return myfirestore.SeatDoc{}, fmt.Errorf("in ReadSeatWithUserId: %w", err)
	}
	return seat, nil
}

func (s *System) UpdateTotalWorkTime(tx *firestore.Transaction, userId string, previousUserDoc *myfirestore.UserDoc, newWorkedTimeSec int, newDailyWorkedTimeSec int) error {
	// 更新前の値
	previousTotalSec := previousUserDoc.TotalStudySec
	previousDailyTotalSec := previousUserDoc.DailyTotalStudySec
	// 更新後の値
	newTotalSec := previousTotalSec + newWorkedTimeSec
	newDailyTotalSec := previousDailyTotalSec + newDailyWorkedTimeSec

	// 累計作業時間が減るなんてことがないか確認
	if newTotalSec < previousTotalSec {
		return errors.New(fmt.Sprintf("newTotalSec < previousTotalSec ??!! 処理を中断します。userId: %s,newTotalSec: %d, previousTotalSec: %d", userId, newTotalSec, previousTotalSec))
	}

	if err := s.FirestoreController.UpdateUserTotalTime(tx, userId, newTotalSec, newDailyTotalSec); err != nil {
		return fmt.Errorf("in UpdateUserTotalTime: %w", err)
	}
	return nil
}

// GetUserRealtimeTotalStudyDurations リアルタイムの累積作業時間・当日累積作業時間を返す。
func (s *System) GetUserRealtimeTotalStudyDurations(ctx context.Context, tx *firestore.Transaction, userId string) (time.Duration, time.Duration, error) {
	// 入室中ならばリアルタイムの作業時間も加算する
	realtimeDuration := time.Duration(0)
	realtimeDailyDuration := time.Duration(0)
	isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
	if err != nil {
		return 0, 0, fmt.Errorf("failed IsUserInRoom: %w", err)
	}
	if isInMemberRoom || isInGeneralRoom {
		// 作業時間を計算
		currentSeat, err := s.CurrentSeat(ctx, userId, isInMemberRoom)
		if err != nil {
			return 0, 0, fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}

		realtimeDuration, err = utils.RealTimeTotalStudyDurationOfSeat(currentSeat, utils.JstNow())
		if err != nil {
			return 0, 0, fmt.Errorf("in RealTimeTotalStudyDurationOfSeat: %w", err)
		}
		realtimeDailyDuration, err = utils.RealTimeDailyTotalStudyDurationOfSeat(currentSeat, utils.JstNow())
		if err != nil {
			return 0, 0, fmt.Errorf("in RealTimeDailyTotalStudyDurationOfSeat: %w", err)
		}
	}

	userData, err := s.FirestoreController.ReadUser(ctx, tx, userId)
	if err != nil {
		return 0, 0, fmt.Errorf("in ReadUser: %w", err)
	}

	// 累計
	totalDuration := realtimeDuration + time.Duration(userData.TotalStudySec)*time.Second

	// 当日の累計
	dailyTotalDuration := realtimeDailyDuration + time.Duration(userData.DailyTotalStudySec)*time.Second

	return totalDuration, dailyTotalDuration, nil
}

// ExitAllUsersInRoom roomの全てのユーザーを退室させる。
func (s *System) ExitAllUsersInRoom(ctx context.Context, isMemberRoom bool) error {
	for {
		var seats []myfirestore.SeatDoc
		var err error
		if isMemberRoom {
			seats, err = s.FirestoreController.ReadMemberSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadMemberSeats: %w", err)
			}
		} else {
			seats, err = s.FirestoreController.ReadGeneralSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadGeneralSeats: %w", err)
			}
		}
		if len(seats) == 0 {
			break
		}
		for _, seatCandidate := range seats {
			var message string
			txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
				seat, err := s.FirestoreController.ReadSeat(ctx, tx, seatCandidate.SeatId, isMemberRoom)
				if err != nil {
					return fmt.Errorf("in ReadSeat: %w", err)
				}
				s.SetProcessedUser(seat.UserId, seat.UserDisplayName, seatCandidate.UserProfileImageUrl, false, false, isMemberRoom)
				userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
				if err != nil {
					return fmt.Errorf("in ReadUser: %w", err)
				}
				// 退室処理
				workedTimeSec, addedRP, err := s.exitRoom(ctx, tx, isMemberRoom, seat, &userDoc)
				if err != nil {
					return fmt.Errorf("failed to exitRoom for %s: %w", s.ProcessedUserId, err)
				}
				var rpEarned string
				var seatIdStr string
				if userDoc.RankVisible {
					rpEarned = i18n.T("command:rp-earned", addedRP)
				}
				if isMemberRoom {
					seatIdStr = i18n.T("common:vip-seat-id", seat.SeatId)
				} else {
					seatIdStr = strconv.Itoa(seat.SeatId)
				}
				message = i18n.T("command:exit", s.ProcessedUserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)
				return nil
			})
			if txErr != nil { // log txErr but continues
				slog.Error("error in transaction", "txErr", txErr)
			}
			slog.Info(message)
		}
	}
	return nil
}

func (s *System) ListLiveChatMessages(ctx context.Context, pageToken string) ([]*youtube.LiveChatMessage, string, int, error) {
	return s.liveChatBot.ListMessages(ctx, pageToken)
}

func (s *System) MessageToLiveChat(ctx context.Context, message string) {
	if err := s.liveChatBot.PostMessage(ctx, message); err != nil {
		s.MessageToOwnerWithError("failed to send live chat message \""+message+"\"\n", err)
	}
}

func (s *System) MessageToOwner(message string) {
	if err := s.discordOwnerBot.SendMessage(message); err != nil {
		slog.Error("failed to send message to owner.", "err", err)
	}
	// これが最終連絡手段のため、エラーは返さずログのみ。
}

func (s *System) MessageToOwnerWithError(message string, argErr error) {
	if err := s.discordOwnerBot.SendMessageWithError(message, argErr); err != nil {
		slog.Error("failed to send message to owner.", "err", err)
	}
	// これが最終連絡手段のため、エラーは返さずログのみ。
}

func (s *System) MessageToSharedDiscord(message string) error {
	return s.discordSharedBot.SendMessage(message)
}

func (s *System) LogToSharedDiscord(logMessage string) error {
	return s.discordSharedLogBot.SendMessage(logMessage)
}

// CheckLongTimeSitting 長時間入室しているユーザーを席移動させる。
func (s *System) CheckLongTimeSitting(ctx context.Context, isMemberRoom bool) error {
	// 全座席のスナップショットをとる（トランザクションなし）
	var seatsSnapshot []myfirestore.SeatDoc
	var err error
	if isMemberRoom {
		seatsSnapshot, err = s.FirestoreController.ReadMemberSeats(ctx)
	} else {
		seatsSnapshot, err = s.FirestoreController.ReadGeneralSeats(ctx)
	}
	if err != nil {
		return fmt.Errorf("failed to read seats: %w", err)
	}

	if err := s.OrganizeDBForceMove(ctx, seatsSnapshot, isMemberRoom); err != nil {
		return fmt.Errorf("in OrganizeDBForceMove: %w", err)
	}

	return nil
}

func (s *System) CheckLiveStreamStatus(ctx context.Context) error {
	checker := guardians.NewLiveStreamChecker(s.FirestoreController, s.liveChatBot, s.discordOwnerBot)
	return checker.Check(ctx)
}

func (s *System) GetUserIdsToProcessRP(ctx context.Context) ([]string, error) {
	slog.Info(utils.NameOf(s.GetUserIdsToProcessRP))
	jstNow := utils.JstNow()
	// 過去31日以内に入室したことのあるユーザーをクエリ（本当は退室したことのある人も取得したいが、クエリはORに対応してないため無視）
	_31daysAgo := jstNow.AddDate(0, 0, -31)
	iter := s.FirestoreController.GetUsersActiveAfterDate(ctx, _31daysAgo)

	var userIds []string
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return []string{}, fmt.Errorf("in iter.Next(): %w", err)
		}
		userId := doc.Ref.ID
		userIds = append(userIds, userId)
	}
	return userIds, nil
}

func (s *System) GetAllUsersTotalStudySecList(ctx context.Context) ([]utils.UserIdTotalStudySecSet, error) {
	var set []utils.UserIdTotalStudySecSet

	userDocRefs, err := s.FirestoreController.GetAllUserDocRefs(ctx)
	if err != nil {
		return set, fmt.Errorf("in GetAllUserDocRefs: %w", err)
	}
	for _, userDocRef := range userDocRefs {
		userDoc, err := s.FirestoreController.ReadUser(ctx, nil, userDocRef.ID)
		if err != nil {
			return set, fmt.Errorf("in ReadUser: %w", err)
		}
		set = append(set, utils.UserIdTotalStudySecSet{
			UserId:        userDocRef.ID,
			TotalStudySec: userDoc.TotalStudySec,
		})
	}
	return set, nil
}

// MinAvailableSeatIdForUser 空いている最小の番号の席番号を求める。該当ユーザーの入室上限にかからない範囲に限定。
func (s *System) MinAvailableSeatIdForUser(ctx context.Context, tx *firestore.Transaction, userId string, isMemberSeat bool) (int, error) {
	var seats []myfirestore.SeatDoc
	var err error
	if isMemberSeat {
		seats, err = s.FirestoreController.ReadMemberSeats(ctx)
		if err != nil {
			return -1, fmt.Errorf("in ReadMemberSeats(): %w", err)
		}
	} else {
		seats, err = s.FirestoreController.ReadGeneralSeats(ctx)
		if err != nil {
			return -1, fmt.Errorf("in ReadGeneralSeats(): %w", err)
		}
	}

	constants, err := s.FirestoreController.ReadSystemConstantsConfig(ctx, tx)
	if err != nil {
		return -1, fmt.Errorf("in ReadSystemConstantsConfig(): %w", err)
	}

	// 使用されている座席番号リストを取得
	var usedSeatIds []int
	for _, seat := range seats {
		usedSeatIds = append(usedSeatIds, seat.SeatId)
	}

	// 使用されていない最小の席番号を求める。1から順に探索
	searchingSeatId := 1
	for searchingSeatId <= constants.MaxSeats {
		// searchingSeatIdがusedSeatIdsに含まれているか
		isUsed := false
		for _, usedSeatId := range usedSeatIds {
			if usedSeatId == searchingSeatId {
				isUsed = true
			}
		}
		if !isUsed { // 使われていない
			// 且つ、該当ユーザーが入室制限にかからなければその席番号を返す
			ifSittingTooMuch, err := s.CheckIfUserSittingTooMuchForSeat(ctx, userId, searchingSeatId, isMemberSeat)
			if err != nil {
				return -1, fmt.Errorf("in CheckIfUserSittingTooMuchForSeat(): %w", err)
			}
			if !ifSittingTooMuch {
				return searchingSeatId, nil
			}
		}
		searchingSeatId += 1
	}
	return -1, studyspaceerror.ErrNoSeatAvailable
}

func (s *System) AddLiveChatHistoryDoc(ctx context.Context, chatMessage *youtube.LiveChatMessage) error {
	// example of publishedAt: "2021-11-13T07:21:30.486982+00:00"
	publishedAt, err := time.Parse(time.RFC3339Nano, chatMessage.Snippet.PublishedAt)
	if err != nil {
		return fmt.Errorf("failed to Parse publishedAt: %w", err)
	}
	publishedAt = publishedAt.In(utils.JapanLocation())

	liveChatHistoryDoc := myfirestore.LiveChatHistoryDoc{
		AuthorChannelId:       chatMessage.AuthorDetails.ChannelId,
		AuthorDisplayName:     chatMessage.AuthorDetails.DisplayName,
		AuthorProfileImageUrl: chatMessage.AuthorDetails.ProfileImageUrl,
		AuthorIsChatModerator: chatMessage.AuthorDetails.IsChatModerator,
		Id:                    chatMessage.Id,
		LiveChatId:            chatMessage.Snippet.LiveChatId,
		MessageText:           youtubebot.ExtractTextMessageByAuthor(chatMessage),
		PublishedAt:           publishedAt,
		Type:                  chatMessage.Snippet.Type,
	}
	return s.FirestoreController.CreateLiveChatHistoryDoc(ctx, nil, liveChatHistoryDoc)
}

func (s *System) DeleteCollectionHistoryBeforeDate(ctx context.Context, date time.Time) (int, int, error) {
	// Firestoreでは1回のトランザクションで500件までしか削除できないため、500件ずつ回す
	var numRowsLiveChat, numRowsUserActivity int

	// date以前の全てのlive chat history docsをクエリで取得
	for {
		iter := s.FirestoreController.Get500LiveChatHistoryDocIdsBeforeDate(ctx, date)
		count, err := s.DeleteIteratorDocs(ctx, iter)
		numRowsLiveChat += count
		if err != nil {
			return 0, 0, fmt.Errorf("in DeleteIteratorDocs(): %w", err)
		}
		if count == 0 {
			break
		}
	}

	// date以前の全てのuser activity docをクエリで取得
	for {
		iter := s.FirestoreController.Get500UserActivityDocIdsBeforeDate(ctx, date)
		count, err := s.DeleteIteratorDocs(ctx, iter)
		numRowsUserActivity += count
		if err != nil {
			return 0, 0, fmt.Errorf("in DeleteIteratorDocs(): %w", err)
		}
		if count == 0 {
			break
		}
	}
	return numRowsLiveChat, numRowsUserActivity, nil
}

// DeleteIteratorDocs iterは最大500件とすること。
func (s *System) DeleteIteratorDocs(ctx context.Context, iter *firestore.DocumentIterator) (int, error) {
	count := 0 // iterのアイテムの件数
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// forで各docをdeleteしていく
		for {
			doc, err := iter.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return fmt.Errorf("in iter.Next(): %w", err)
			}
			count++
			{
				if err := s.FirestoreController.DeleteDocRef(ctx, tx, doc.Ref); err != nil {
					return fmt.Errorf("in DeleteDocRef(): %w", err)
				}
			}
		}
		return nil
	})
	return count, txErr
}

func (s *System) CheckIfUserSittingTooMuchForSeat(ctx context.Context, userId string, seatId int, isMemberSeat bool) (bool, error) {
	jstNow := utils.JstNow()

	// ホワイトリスト・ブラックリストを検索
	whiteListForUserAndSeat, err := s.FirestoreController.ReadSeatLimitsWHITEListWithSeatIdAndUserId(ctx, seatId, userId, isMemberSeat)
	if err != nil {
		return false, fmt.Errorf("in ReadSeatLimitsWHITEListWithSeatIdAndUserId(): %w", err)
	}
	blackListForUserAndSeat, err := s.FirestoreController.ReadSeatLimitsBLACKListWithSeatIdAndUserId(ctx, seatId, userId, isMemberSeat)
	if err != nil {
		return false, fmt.Errorf("in ReadSeatLimitsBLACKListWithSeatIdAndUserId(): %w", err)
	}

	// もし両方あったら矛盾なのでエラー
	if len(whiteListForUserAndSeat) > 0 && len(blackListForUserAndSeat) > 0 {
		return false, errors.New("len(whiteListForUserAndSeat) > 0 && len(blackListForUserAndSeat) > 0")
	}

	// 片方しかなければチェックは不要
	if len(whiteListForUserAndSeat) > 1 {
		return false, errors.New(fmt.Sprintf("len(whiteListForUserAndSeat) > 1, seatId=%d, userId=%s", seatId, userId))
	} else if len(whiteListForUserAndSeat) == 1 {
		if whiteListForUserAndSeat[0].Until.After(jstNow) {
			slog.Info("[seat " + strconv.Itoa(seatId) + ": " + userId + "] found in white list. skipping.")
			return false, nil
		}
		// ホワイトリストに入っているが、期限切れのためチェックを続行
	}
	if len(blackListForUserAndSeat) > 1 {
		return false, errors.New(fmt.Sprintf("len(blackListForUserAndSeat) > 1, seatId=%d, userId=%s", seatId, userId))
	} else if len(blackListForUserAndSeat) == 1 {
		if blackListForUserAndSeat[0].Until.After(jstNow) {
			slog.Info("[seat " + strconv.Itoa(seatId) + ": " + userId + "] found in black list. skipping.")
			return true, nil
		}
		// ブラックリストに入っているが、期限切れのためチェックを続行
	}

	totalEntryDuration, err := s.GetRecentUserSittingTimeForSeat(ctx, userId, seatId, isMemberSeat)
	if err != nil {
		return false, fmt.Errorf("in GetRecentUserSittingTimeForSeat(): %w", err)
	}

	slog.Info("",
		"userId", userId,
		"seatId", seatId,
		"過去何分", s.Configs.Constants.RecentRangeMin,
		"合計何分", int(totalEntryDuration.Minutes()))

	// 制限値と比較
	ifSittingTooMuch := int(totalEntryDuration.Minutes()) > s.Configs.Constants.RecentThresholdMin

	if !ifSittingTooMuch {
		until := jstNow.Add(time.Duration(s.Configs.Constants.RecentThresholdMin)*time.Minute - totalEntryDuration)
		if until.Sub(jstNow) > time.Duration(s.Configs.Constants.MinimumCheckLongTimeSittingIntervalMinutes)*time.Minute {
			// ホワイトリストに登録
			if err := s.FirestoreController.CreateSeatLimitInWHITEList(ctx, seatId, userId, jstNow, until, isMemberSeat); err != nil {
				return false, fmt.Errorf("in CreateSeatLimitInWHITEList(): %w", err)
			}
			slog.Info("[seat " + strconv.Itoa(seatId) + ": " + userId + "] saved to white list.")
		}
	} else {
		// ブラックリストに登録
		until := jstNow.Add(time.Duration(s.Configs.Constants.LongTimeSittingPenaltyMinutes) * time.Minute)
		if err := s.FirestoreController.CreateSeatLimitInBLACKList(ctx, seatId, userId, jstNow, until, isMemberSeat); err != nil {
			return false, fmt.Errorf("in CreateSeatLimitInBLACKList(): %w", err)
		}
		slog.Info("[seat " + strconv.Itoa(seatId) + ": " + userId + "] saved to black list.")
	}

	return ifSittingTooMuch, nil
}

func (s *System) GetRecentUserSittingTimeForSeat(ctx context.Context, userId string, seatId int, isMemberSeat bool) (time.Duration, error) {
	checkDurationFrom := utils.JstNow().Add(-time.Duration(s.Configs.Constants.RecentRangeMin) * time.Minute)

	// 指定期間の該当ユーザーの該当座席への入退室ドキュメントを取得する
	enterRoomActivities, err := s.FirestoreController.GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx, checkDurationFrom, userId, seatId, isMemberSeat)
	if err != nil {
		return 0, fmt.Errorf("in "+utils.NameOf(s.FirestoreController.GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat)+": %w", err)
	}
	exitRoomActivities, err := s.FirestoreController.GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx, checkDurationFrom, userId, seatId, isMemberSeat)
	if err != nil {
		return 0, fmt.Errorf("in "+utils.NameOf(s.FirestoreController.GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat)+": %w", err)
	}
	activityOnlyEnterExitList := append(enterRoomActivities, exitRoomActivities...)

	// activityListは長さ0の可能性もあることに注意

	// 入室と退室が交互に並んでいるか確認
	utils.SortUserActivityByTakenAtAscending(activityOnlyEnterExitList)
	orderOK := utils.CheckEnterExitActivityOrder(activityOnlyEnterExitList)
	if !orderOK {
		return 0, errors.New("入室activityと退室activityが交互に並んでいない\n" + fmt.Sprintf("%v", pretty.Formatter(activityOnlyEnterExitList)))
	}

	slog.Info("入退室ドキュメント数：" + strconv.Itoa(len(activityOnlyEnterExitList)))

	// 入退室をセットで考え、合計入室時間を求める
	totalEntryDuration := time.Duration(0)
	entryCount := 0 // 退室時（もしくは現在日時）にentryCountをインクリメント。
	lastEnteredTimestamp := checkDurationFrom
	for i, activity := range activityOnlyEnterExitList {
		if activity.ActivityType == myfirestore.EnterRoomActivity {
			lastEnteredTimestamp = activity.TakenAt
			if i+1 == len(activityOnlyEnterExitList) { // 最後のactivityであった場合、現在時刻までの時間を加算
				entryCount += 1
				totalEntryDuration += utils.NoNegativeDuration(utils.JstNow().Sub(activity.TakenAt))
			}
			continue
		} else if activity.ActivityType == myfirestore.ExitRoomActivity {
			entryCount += 1
			totalEntryDuration += utils.NoNegativeDuration(activity.TakenAt.Sub(lastEnteredTimestamp))
		}
	}
	return totalEntryDuration, nil
}

func (s *System) BanUser(ctx context.Context, userId string) error {
	if err := s.liveChatBot.BanUser(ctx, userId); err != nil {
		return fmt.Errorf("in BanUser: %w", err)
	}
	return nil
}
