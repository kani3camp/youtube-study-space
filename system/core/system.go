package core

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"reflect"
	"strconv"
	"time"

	"app.modules/core/customerror"
	"app.modules/core/discordbot"
	"app.modules/core/guardians"
	"app.modules/core/i18n"
	"app.modules/core/mybigquery"
	"app.modules/core/myfirestore"
	"app.modules/core/myspreadsheet"
	"app.modules/core/mystorage"
	"app.modules/core/utils"
	"app.modules/core/youtubebot"
	"cloud.google.com/go/firestore"
	"github.com/kr/pretty"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewSystem(ctx context.Context, configCheck bool, clientOption option.ClientOption) (System, error) {
	err := i18n.LoadLocaleFolderFS()
	if err != nil {
		return System{}, err
	}

	fsController, err := myfirestore.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return System{}, err
	}

	// credentials
	credentialsDoc, err := fsController.ReadCredentialsConfig(ctx, nil)
	if err != nil {
		return System{}, err
	}

	// YouTube live chatbot
	liveChatBot, err := youtubebot.NewYoutubeLiveChatBot(credentialsDoc.YoutubeLiveChatId, fsController, ctx)
	if err != nil {
		return System{}, err
	}

	// discord bot owner
	discordOwnerBot, err := discordbot.NewDiscordBot(credentialsDoc.DiscordOwnerBotToken, credentialsDoc.DiscordOwnerBotTextChannelId)
	if err != nil {
		return System{}, err
	}

	// discord bot for share
	discordSharedBot, err := discordbot.NewDiscordBot(credentialsDoc.DiscordSharedBotToken, credentialsDoc.DiscordSharedBotTextChannelId)
	if err != nil {
		return System{}, err
	}

	// discord bot for share log
	discordSharedLogBot, err := discordbot.NewDiscordBot(credentialsDoc.DiscordSharedBotToken, credentialsDoc.DiscordSharedBotLogChannelId)
	if err != nil {
		return System{}, err
	}

	// core constant values
	constantsConfig, err := fsController.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return System{}, err
	}

	configs := SystemConfigs{
		Constants:            constantsConfig,
		LiveChatBotChannelId: credentialsDoc.YoutubeBotChannelId,
	}

	// 全ての項目が初期化できているか確認
	v := reflect.ValueOf(configs.Constants)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).IsZero() {
			if configCheck {
				fieldName := v.Type().Field(i).Name
				fieldValue := fmt.Sprintf("%v", v.Field(i))
				fmt.Println("The field \"" + fieldName + " = " + fieldValue + "\" may not be initialized. " +
					"Continue? (yes / no)")
				var s string
				_, err := fmt.Scanln(&s)
				if err != nil {
					return System{}, err
				}
				if s != "yes" {
					return System{}, errors.New("")
				}
			}
		}
	}

	ssc, err := myspreadsheet.NewSpreadsheetController(ctx, clientOption, configs.Constants.BotConfigSpreadsheetId, "01", "02")
	if err != nil {
		log.Println("failed NewSpreadsheetController", err)
		return System{}, err
	}
	blockRegexListForChannelName, blockRegexListForChatMessage, err := ssc.GetRegexForBlock()
	if err != nil {
		log.Println("failed GetRegexForBlock", err)
		return System{}, err
	}
	notificationRegexListForChatMessage, notificationRegexListForChannelName, err := ssc.GetRegexForNotification()
	if err != nil {
		log.Println("failed GetRegexForNotification", err)
		return System{}, err
	}

	return System{
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
	return s.FirestoreController.FirestoreClient.RunTransaction(ctx, f)
}

func (s *System) SetProcessedUser(userId string, userDisplayName string, userProfileImageUrl string, isChatModerator bool, isChatOwner bool, isChatMember bool) {
	s.ProcessedUserId = userId
	s.ProcessedUserDisplayName = userDisplayName
	s.ProcessedUserProfileImageUrl = userProfileImageUrl
	s.ProcessedUserIsModeratorOrOwner = isChatModerator || isChatOwner
	s.ProcessedUserIsMember = isChatMember
}

func (s *System) CloseFirestoreClient() {
	err := s.FirestoreController.FirestoreClient.Close()
	if err != nil {
		log.Println("failed close firestore client.")
	} else {
		log.Println("successfully closed firestore client.")
	}
}

func (s *System) GetInfoString() string {
	numAllFilteredRegex := len(s.blockRegexListForChatMessage) + len(s.blockRegexListForChannelName) + len(s.notificationRegexListForChatMessage) + len(s.notificationRegexListForChannelName)
	return "全規制ワード数: " + strconv.Itoa(numAllFilteredRegex)
}

// GoroutineCheckLongTimeSitting 居座り検出ループ
func (s *System) GoroutineCheckLongTimeSitting(ctx context.Context) {
	minimumInterval := time.Duration(s.Configs.Constants.MinimumCheckLongTimeSittingIntervalMinutes) * time.Minute
	log.Printf("居座りチェックの最小間隔: %v\n", minimumInterval)

	var err error
	for {
		log.Println("checking long time sitting")
		start := utils.JstNow()

		err = s.CheckLongTimeSitting(ctx, true)
		if err != nil {
			s.MessageToOwnerWithError("failed to CheckLongTimeSitting", err)
			log.Println(err)
		}
		err = s.CheckLongTimeSitting(ctx, false)
		if err != nil {
			s.MessageToOwnerWithError("failed to CheckLongTimeSitting", err)
			log.Println(err)
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
		err := s.BanUser(ctx, userId)
		if err != nil {
			return false, err
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
		return false, err
	}
	if found {
		err := s.BanUser(ctx, userId)
		if err != nil {
			return false, err
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
		return false, err
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
		return false, err
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
	log.Println("AdjustMaxSeats()")
	// UpdateDesiredMaxSeats()などはLambdaからも並列で実行される可能性があるが、競合が起こってもそこまで深刻な問題にはならないため
	//トランザクションは使用しない。

	constants, err := s.FirestoreController.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return err
	}

	// 一般席
	if constants.DesiredMaxSeats > constants.MaxSeats { // 一般席を増やす
		s.MessageToLiveChat(ctx, "席を増やします↗")
		return s.FirestoreController.UpdateMaxSeats(ctx, nil, constants.DesiredMaxSeats)
	} else if constants.DesiredMaxSeats < constants.MaxSeats { // 一般席を減らす
		// max_seatsを減らしても、空席率が設定値以上か確認
		seats, err := s.FirestoreController.ReadGeneralSeats(ctx)
		if err != nil {
			return err
		}
		if int(float32(constants.DesiredMaxSeats)*(1.0-constants.MinVacancyRate)) < len(seats) {
			message := "減らそうとしすぎ。desiredは却下し、desired max seats <= current max seatsとします。" +
				"desired: " + strconv.Itoa(constants.DesiredMaxSeats) + ", " +
				"current max seats: " + strconv.Itoa(constants.MaxSeats) + ", " +
				"current seats: " + strconv.Itoa(len(seats))
			log.Println(message)
			return s.FirestoreController.UpdateDesiredMaxSeats(ctx, nil, constants.MaxSeats)
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
					err = s.In(ctx, inCommandDetails)
					if err != nil {
						return err
					}
				}
			}
			// max_seatsを更新
			return s.FirestoreController.UpdateMaxSeats(ctx, nil, constants.DesiredMaxSeats)
		}
	}

	// メンバー席
	if constants.DesiredMemberMaxSeats > constants.MemberMaxSeats { // メンバー席を増やす
		s.MessageToLiveChat(ctx, "メンバー限定の席を増やします↗")
		return s.FirestoreController.UpdateMemberMaxSeats(ctx, nil, constants.DesiredMemberMaxSeats)
	} else if constants.DesiredMemberMaxSeats < constants.MemberMaxSeats { // メンバー席を減らす
		// member_max_seatsを減らしても、空席率が設定値以上か確認
		seats, err := s.FirestoreController.ReadMemberSeats(ctx)
		if err != nil {
			return err
		}
		if int(float32(constants.DesiredMemberMaxSeats)*(1.0-constants.MinVacancyRate)) < len(seats) {
			message := "減らそうとしすぎ。desiredは却下し、desired member max seats <= current member max seatsとします。" +
				"desired: " + strconv.Itoa(constants.DesiredMaxSeats) + ", " +
				"current member max seats: " + strconv.Itoa(constants.MemberMaxSeats) + ", " +
				"current seats: " + strconv.Itoa(len(seats))
			log.Println(message)
			return s.FirestoreController.UpdateDesiredMemberMaxSeats(ctx, nil, constants.MemberMaxSeats)
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
					err = s.In(ctx, inCommandDetails)
					if err != nil {
						return err
					}
				}
			}
			// member_max_seatsを更新
			return s.FirestoreController.UpdateMemberMaxSeats(ctx, nil, constants.DesiredMemberMaxSeats)
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
			s.MessageToOwnerWithError("failed to CheckIfUnwantedWordIncluded", err)
			// continue
		}
		if blocked {
			return nil
		}
	}

	// 初回の利用の場合はユーザーデータを初期化
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		isRegistered, err := s.IfUserRegistered(ctx, tx)
		if err != nil {
			s.MessageToOwnerWithError("failed to IfUserRegistered", err)
			return err
		}
		if !isRegistered {
			err := s.CreateUser(tx)
			if err != nil {
				s.MessageToOwnerWithError("failed to CreateUser", err)
				return err
			}
		}
		return nil
	})
	if err != nil {
		s.MessageToLiveChat(ctx, i18n.T("command:error", s.ProcessedUserDisplayName))
		return err
	}

	commandDetails, cerr := utils.ParseCommand(commandString, isChatMember)
	if cerr.IsNotNil() { // これはシステム内部のエラーではなく、入力コマンドが不正ということなので、return nil
		s.MessageToLiveChat(ctx, i18n.T("common:sir", s.ProcessedUserDisplayName)+cerr.Body.Error())
		return nil
	}
	//log.Printf("parsed command: %# v\n", pretty.Formatter(commandDetails))

	if cerr := s.ValidateCommand(*commandDetails); cerr.IsNotNil() {
		s.MessageToLiveChat(ctx, i18n.T("common:sir", s.ProcessedUserDisplayName)+cerr.Body.Error())
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
		s.MessageToOwner("Unknown command: " + commandString)
	}
	return nil
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

	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 席が指定されているか？
		if inOption.IsSeatIdSet {
			// 0番席だったら最小番号の空席に決定
			if inOption.SeatId == 0 {
				seatId, err := s.MinAvailableSeatIdForUser(ctx, tx, s.ProcessedUserId, isTargetMemberSeat)
				if err != nil {
					s.MessageToOwnerWithError("failed s.MinAvailableSeatIdForUser()", err)
					return err
				}
				inOption.SeatId = seatId
			} else {
				// 以下のように前もってerr2を宣言しておき、このあとのIfSeatVacantとCheckSeatAvailabilityForUserで明示的に同じerr2
				//を使用するようにしておく。すべてerrとした場合、CheckSeatAvailabilityForUserのほうでなぜか上のスコープのerrが使われてしまう
				var isVacant, ifSittingTooMuch bool
				var err2 error
				// その席が空いているか？
				isVacant, err2 = s.IfSeatVacant(ctx, tx, inOption.SeatId, isTargetMemberSeat)
				if err2 != nil {
					s.MessageToOwnerWithError("failed s.IfSeatVacant()", err2)
					return err2
				}
				if !isVacant {
					replyMessage = t("no-seat", s.ProcessedUserDisplayName, utils.InCommand)
					return nil
				}
				// ユーザーはその席に対して入室制限を受けてないか？
				ifSittingTooMuch, err2 = s.CheckIfUserSittingTooMuchForSeat(ctx, s.ProcessedUserId, inOption.SeatId, isTargetMemberSeat)
				if err2 != nil {
					s.MessageToOwnerWithError("failed s.CheckIfUserSittingTooMuchForSeat()", err2)
					return err2
				}
				if ifSittingTooMuch {
					replyMessage = t("no-availability", s.ProcessedUserDisplayName, utils.InCommand)
					return nil
				}
			}
		} else { // 席の指定なし
			seatId, cerr := s.RandomAvailableSeatIdForUser(ctx, tx, s.ProcessedUserId, isTargetMemberSeat)
			if cerr.IsNotNil() {
				if cerr.ErrorType == customerror.NoSeatAvailable {
					s.MessageToOwnerWithError("席数がmax seatに達していて、ユーザーが入室できない事象が発生。", cerr.Body)
				}
				return cerr.Body
			}
			inOption.SeatId = seatId
		}

		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed to ReadUser", err)
			return err
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
			s.MessageToOwnerWithError("failed to GetUserRealtimeSeatAppearance", err)
			return err
		}

		// 動作が決定

		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed IsUserInRoom", err)
			return err
		}
		isInRoom := isInGeneralRoom || isInMemberRoom
		var currentSeat myfirestore.SeatDoc
		if isInRoom { // 現在座っている席を取得
			var customErr customerror.CustomError
			currentSeat, customErr = s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
			if customErr.IsNotNil() {
				s.MessageToOwnerWithError("failed CurrentSeat", customErr.Body)
				return customErr.Body
			}
		}

		// =========== 以降は書き込み処理のみ ===========

		if isInRoom { // 退室させてから、入室させる
			// 席移動処理
			workedTimeSec, addedRP, untilExitMin, err := s.moveSeat(tx, inOption.SeatId, s.ProcessedUserProfileImageUrl, isInMemberRoom, isTargetMemberSeat, *inOption.MinutesAndWorkName, currentSeat, &userDoc)
			if err != nil {
				s.MessageToOwnerWithError(fmt.Sprintf("failed to moveSeat for %s (%s)", s.ProcessedUserDisplayName, s.ProcessedUserId), err)
				return err
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
				time.Time{})
			if err != nil {
				s.MessageToOwnerWithError("failed to enter room", err)
				return err
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
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

// GetUserRealtimeSeatAppearance リアルタイムの現在のランクを求める
func (s *System) GetUserRealtimeSeatAppearance(ctx context.Context, tx *firestore.Transaction, userId string) (myfirestore.SeatAppearance, error) {
	userDoc, err := s.FirestoreController.ReadUser(ctx, tx, userId)
	if err != nil {
		s.MessageToOwnerWithError("failed to ReadUser", err)
		return myfirestore.SeatAppearance{}, err
	}
	totalStudyDuration, _, err := s.GetUserRealtimeTotalStudyDurations(ctx, tx, userId)
	if err != nil {
		return myfirestore.SeatAppearance{}, err
	}
	seatAppearance, err := utils.GetSeatAppearance(int(totalStudyDuration.Seconds()), userDoc.RankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
	if err != nil {
		s.MessageToOwnerWithError("failed to GetSeatAppearance", err)
	}
	return seatAppearance, nil
}

func (s *System) Out(_ *utils.CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-out")
	var replyMessage string
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed IsUserInRoom", err)
			return err
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = t("already-exit", s.ProcessedUserDisplayName)
			return nil
		}

		// 現在座っている席を特定
		seat, customErr := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if customErr.Body != nil {
			s.MessageToOwnerWithError("failed to s.CurrentSeat", customErr.Body)
			return customErr.Body
		}

		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed to ReadUser", err)
			return err
		}

		// 退室処理
		workedTimeSec, addedRP, err := s.exitRoom(tx, isInMemberRoom, seat, &userDoc)
		if err != nil {
			s.MessageToOwnerWithError("failed in s.exitRoom", err)
			return err
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
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) ShowUserInfo(command *utils.CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-user-info")
	var replyMessage string
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		totalStudyDuration, dailyTotalStudyDuration, err := s.GetUserRealtimeTotalStudyDurations(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed s.GetUserRealtimeTotalStudyDurations()", err)
			return err
		}
		totalTimeStr := utils.DurationToString(totalStudyDuration)
		dailyTotalTimeStr := utils.DurationToString(dailyTotalStudyDuration)
		replyMessage += t("base", s.ProcessedUserDisplayName, dailyTotalTimeStr, totalTimeStr)

		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed s.FirestoreController.ReadUser", err)
			return err
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
				} else {
					// 表示しない
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
	if err != nil {
		replyMessage = i18n.T("command:error")
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) ShowSeatInfo(command *utils.CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-seat-info")
	showDetails := command.SeatOption.ShowDetails
	var replyMessage string
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// そのユーザーは入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed IsUserInRoom", err)
			return err
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if isInRoom {
			currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
			if cerr.IsNotNil() {
				s.MessageToOwnerWithError("failed s.CurrentSeat()", cerr.Body)
				return cerr.Body
			}

			realtimeSittingDurationMin := int(utils.NoNegativeDuration(utils.JstNow().Sub(currentSeat.EnteredAt)).Minutes())
			realtimeTotalStudyDurationOfSeat, err := utils.RealTimeTotalStudyDurationOfSeat(currentSeat)
			if err != nil {
				s.MessageToOwnerWithError("failed to RealTimeTotalStudyDurationOfSeat", err)
				return err
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
					s.MessageToOwnerWithError("failed to GetRecentUserSittingTimeForSeat", err)
					return err
				}
				replyMessage += t("details", s.Configs.Constants.RecentRangeMin, seatIdStr, int(recentTotalEntryDuration.Minutes()))
			}
		} else {
			replyMessage = i18n.T("command:not-enter", s.ProcessedUserDisplayName, utils.InCommand)
		}
		return nil
	})
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
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
	err := s.MessageToSharedDiscord(discordMessage)
	if err != nil {
		s.MessageToOwnerWithError("管理者へメッセージが送信できませんでした: \""+discordMessage+"\"", err)
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

	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// ターゲットの座席は誰か使っているか
		isSeatAvailable, err := s.IfSeatVacant(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			return err
		}
		if isSeatAvailable {
			replyMessage = i18n.T("command:unused", s.ProcessedUserDisplayName)
			return nil
		}

		// ユーザーを強制退室させる
		targetSeat, err := s.FirestoreController.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = i18n.T("command:unused", s.ProcessedUserDisplayName)
				return nil
			}
			s.MessageToOwnerWithError("failed to ReadSeat", err)
			return err
		}

		seatIdStr := utils.SeatIdStr(targetSeatId, isTargetMemberSeat)
		replyMessage = t("kick", s.ProcessedUserDisplayName, seatIdStr, targetSeat.UserDisplayName)

		// s.ProcessedUserが処理の対象ではないことに注意。
		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, targetSeat.UserId)
		if err != nil {
			s.MessageToOwnerWithError("failed to ReadUser", err)
			return err
		}

		workedTimeSec, addedRP, exitErr := s.exitRoom(tx, isTargetMemberSeat, targetSeat, &userDoc)
		if exitErr != nil {
			s.MessageToOwnerWithError(s.ProcessedUserDisplayName+"さんのkick退室処理中にエラーが発生しました", exitErr)
			return exitErr
		}
		var rpEarned string
		if userDoc.RankVisible {
			rpEarned = i18n.T("command:rp-earned", addedRP)
		}
		replyMessage += i18n.T("command:exit", targetSeat.UserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)

		err = s.LogToSharedDiscord(s.ProcessedUserDisplayName + "さん、" + strconv.Itoa(targetSeat.
			SeatId) + "番席のユーザーをkickしました。\n" +
			"チャンネル名: " + targetSeat.UserDisplayName + "\n" +
			"作業名: " + targetSeat.WorkName + "\n休憩中の作業名: " + targetSeat.BreakWorkName + "\n" +
			"入室時間: " + strconv.Itoa(workedTimeSec/60) + "分\n" +
			"チャンネルURL: https://youtube.com/channel/" + targetSeat.UserId)
		if err != nil {
			s.MessageToOwnerWithError("failed LogToSharedDiscord()", err)
			return err
		}
		return nil
	})
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) Check(command *utils.CommandDetails, ctx context.Context) error {
	targetSeatId := command.CheckOption.SeatId
	isTargetMemberSeat := command.CheckOption.IsTargetMemberSeat

	var replyMessage string
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// commanderはモデレーターかチャットオーナーか
		if !s.ProcessedUserIsModeratorOrOwner {
			replyMessage = i18n.T("command:permission", s.ProcessedUserDisplayName, utils.CheckCommand)
			return nil
		}

		// ターゲットの座席は誰か使っているか
		isSeatVacant, err := s.IfSeatVacant(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			s.MessageToOwnerWithError("failed to IfSeatVacant", err)
			return err
		}
		if isSeatVacant {
			replyMessage = i18n.T("command:unused", s.ProcessedUserDisplayName)
			return nil
		}
		// 座席情報を表示する
		seat, err := s.FirestoreController.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = i18n.T("command:unused", s.ProcessedUserDisplayName)
				return nil
			}
			s.MessageToOwnerWithError("failed to ReadSeat", err)
			return err
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
		err = s.LogToSharedDiscord(message)
		if err != nil {
			s.MessageToOwnerWithError("failed LogToSharedDiscord()", err)
			return err
		}
		replyMessage = i18n.T("command:sent", s.ProcessedUserDisplayName)
		return nil
	})
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) Block(command *utils.CommandDetails, ctx context.Context) error {
	targetSeatId := command.BlockOption.SeatId
	isTargetMemberSeat := command.BlockOption.IsTargetMemberSeat

	var replyMessage string
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// commanderはモデレーターかチャットオーナーか
		if !s.ProcessedUserIsModeratorOrOwner {
			replyMessage = s.ProcessedUserDisplayName + "さんは" + utils.BlockCommand + "コマンドを使用できません"
			return nil
		}

		// ターゲットの座席は誰か使っているか
		isSeatAvailable, err := s.IfSeatVacant(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			return err
		}
		if isSeatAvailable {
			replyMessage = s.ProcessedUserDisplayName + "さん、その番号の座席は誰も使用していません"
			return nil
		}

		// ユーザーを強制退室させる
		targetSeat, err := s.FirestoreController.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = s.ProcessedUserDisplayName + "さん、その番号の座席は誰も使用していません"
				return nil
			}
			s.MessageToOwnerWithError("failed to ReadSeat", err)
			return err
		}
		replyMessage = s.ProcessedUserDisplayName + "さん、" + strconv.Itoa(targetSeat.SeatId) + "番席の" + targetSeat.UserDisplayName + "さんをブロックします。"

		// s.ProcessedUserが処理の対象ではないことに注意。
		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, targetSeat.UserId)
		if err != nil {
			s.MessageToOwnerWithError("failed to ReadUser", err)
			return err
		}

		workedTimeSec, addedRP, exitErr := s.exitRoom(tx, isTargetMemberSeat, targetSeat, &userDoc)
		if exitErr != nil {
			s.MessageToOwnerWithError(s.ProcessedUserDisplayName+"さんの強制退室処理中にエラーが発生しました", exitErr)
			return exitErr
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
		err = s.BanUser(ctx, targetSeat.UserId)
		if err != nil {
			s.MessageToOwnerWithError("failed to BanUser", err)
			return err
		}

		err = s.LogToSharedDiscord(s.ProcessedUserDisplayName + "さん、" + strconv.Itoa(targetSeat.
			SeatId) + "番席のユーザーをblockしました。\n" +
			"チャンネル名: " + targetSeat.UserDisplayName + "\n" +
			"作業名: " + targetSeat.WorkName + "\n休憩中の作業名: " + targetSeat.BreakWorkName + "\n" +
			"入室時間: " + strconv.Itoa(workedTimeSec/60) + "分\n" +
			"チャンネルURL: https://youtube.com/channel/" + targetSeat.UserId)
		if err != nil {
			s.MessageToOwnerWithError("failed LogToSharedDiscord()", err)
			return err
		}
		return nil
	})
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
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
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 変更前のuserDocを読み込んでおく
		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed to ReadUser", err)
			return err
		}

		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed IsUserInRoom", err)
			return err
		}
		isInRoom := isInMemberRoom || isInGeneralRoom

		var seats []myfirestore.SeatDoc
		if isInMemberRoom {
			seats, err = s.FirestoreController.ReadMemberSeats(ctx)
			if err != nil {
				s.MessageToOwnerWithError("failed to ReadMemberSeats", err)
				return err
			}
		}
		if isInGeneralRoom {
			seats, err = s.FirestoreController.ReadGeneralSeats(ctx)
			if err != nil {
				s.MessageToOwnerWithError("failed to ReadGeneralSeats", err)
				return err
			}
		}
		realTimeTotalStudyDuration, _, err := s.GetUserRealtimeTotalStudyDurations(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed to RetrieveRealtimeTotalStudyDuration", err)
			return err
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
					err := s.FirestoreController.UpdateUserRankVisible(tx, s.ProcessedUserId, newRankVisible)
					if err != nil {
						s.MessageToOwnerWithError("failed to UpdateUserRankVisible", err)
						return err
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
							s.MessageToOwnerWithError("failed to GetSeatAppearance", err)
							return err
						}

						// 席の色を更新
						newSeat, err := utils.GetSeatByUserId(seats, s.ProcessedUserId)
						if err != nil {
							return err
						}
						newSeat.Appearance = seatAppearance
						err = s.FirestoreController.UpdateSeat(tx, newSeat, isInMemberRoom)
						if err != nil {
							s.MessageToOwnerWithError("failed to s.FirestoreController.UpdateSeats", err)
							return err
						}
					}
				}
				currenRankVisible = newRankVisible
			} else if myOption.Type == utils.DefaultStudyMin {
				err := s.FirestoreController.UpdateUserDefaultStudyMin(tx, s.ProcessedUserId, myOption.IntValue)
				if err != nil {
					s.MessageToOwnerWithError("failed to UpdateUserDefaultStudyMin", err)
					return err
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
				err = s.FirestoreController.UpdateUserFavoriteColor(tx, s.ProcessedUserId, colorCode)
				if err != nil {
					s.MessageToOwnerWithError("failed to UpdateUserFavoriteColor", err)
					return err
				}
				replyMessage += t("set-favorite-color")
				if !utils.CanUseFavoriteColor(realTimeTotalStudySec) {
					replyMessage += t("alert-favorite-color", utils.FavoriteColorAvailableThresholdHours)
				}

				// 入室中であれば、座席の色も変える
				if isInRoom {
					newSeat, err := utils.GetSeatByUserId(seats, s.ProcessedUserId)
					if err != nil {
						s.MessageToOwnerWithError("failed to GetSeatByUserId", err)
						return err
					}
					seatAppearance, err := utils.GetSeatAppearance(realTimeTotalStudySec, currenRankVisible, userDoc.RankPoint, colorCode)
					if err != nil {
						s.MessageToOwnerWithError("failed to GetSeatAppearance", err)
						return err
					}

					// 席の色を更新
					newSeat.Appearance = seatAppearance
					err = s.FirestoreController.UpdateSeat(tx, newSeat, isInMemberRoom)
					if err != nil {
						s.MessageToOwnerWithError("failed to s.FirestoreController.UpdateSeat()", err)
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) Change(command *utils.CommandDetails, ctx context.Context) error {
	changeOption := &command.ChangeOption
	jstNow := utils.JstNow()
	replyMessage := ""
	t := i18n.GetTFunc("command-change")
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// そのユーザーは入室中か？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed IsUserInRoom", err)
			return err
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if cerr.IsNotNil() {
			s.MessageToOwnerWithError("failed to s.CurrentSeat(ctx)", cerr.Body)
			return cerr.Body
		}

		// validation
		cerr = s.ValidateChange(*command, currentSeat.State)
		if cerr.IsNotNil() {
			replyMessage = i18n.T("common:sir", s.ProcessedUserDisplayName) + cerr.Body.Error()
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
		err = s.FirestoreController.UpdateSeat(tx, *newSeat, isInMemberRoom)
		if err != nil {
			s.MessageToOwnerWithError("failed to UpdateSeats", err)
			return err
		}

		return nil
	})
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) More(command *utils.CommandDetails, ctx context.Context) error {
	replyMessage := ""
	t := i18n.GetTFunc("command-more")
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		jstNow := utils.JstNow()

		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed IsUserInRoom", err)
			return err
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if cerr.IsNotNil() {
			s.MessageToOwnerWithError("failed to s.CurrentSeat(ctx)", cerr.Body)
			return cerr.Body
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

		err = s.FirestoreController.UpdateSeat(tx, *newSeat, isInMemberRoom)
		if err != nil {
			s.MessageToOwnerWithError("failed to s.FirestoreController.UpdateSeats", err)
			return err
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
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) Break(ctx context.Context, command *utils.CommandDetails) error {
	breakOption := &command.BreakOption
	replyMessage := ""
	t := i18n.GetTFunc("command-break")
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed IsUserInRoom", err)
			return err
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		// stateを確認
		currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if cerr.IsNotNil() {
			s.MessageToOwnerWithError("failed to CurrentSeat()", cerr.Body)
			return cerr.Body
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
			dailyCumulativeWorkSec = workedSec
		}
		currentSeat.State = myfirestore.BreakState
		currentSeat.CurrentStateStartedAt = jstNow
		currentSeat.CurrentStateUntil = breakUntil
		currentSeat.CumulativeWorkSec = cumulativeWorkSec
		currentSeat.DailyCumulativeWorkSec = dailyCumulativeWorkSec
		currentSeat.BreakWorkName = breakOption.WorkName

		err = s.FirestoreController.UpdateSeat(tx, currentSeat, isInMemberRoom)
		if err != nil {
			s.MessageToOwnerWithError("failed to s.FirestoreController.UpdateSeats", err)
			return err
		}
		// activityログ記録
		startBreakActivity := myfirestore.UserActivityDoc{
			UserId:       s.ProcessedUserId,
			ActivityType: myfirestore.StartBreakActivity,
			SeatId:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			TakenAt:      utils.JstNow(),
		}
		err = s.FirestoreController.CreateUserActivityDoc(tx, startBreakActivity)
		if err != nil {
			s.MessageToOwnerWithError("failed to add an user activity", err)
			return err
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
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) Resume(ctx context.Context, command *utils.CommandDetails) error {
	replyMessage := ""
	t := i18n.GetTFunc("command-resume")
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed IsUserInRoom", err)
			return err
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		// stateを確認
		currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if cerr.IsNotNil() {
			s.MessageToOwnerWithError("failed to CurrentSeat()", cerr.Body)
			return cerr.Body
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

		err = s.FirestoreController.UpdateSeat(tx, currentSeat, isInMemberRoom)
		if err != nil {
			s.MessageToOwnerWithError("failed to s.FirestoreController.UpdateSeats", err)
			return err
		}
		// activityログ記録
		endBreakActivity := myfirestore.UserActivityDoc{
			UserId:       s.ProcessedUserId,
			ActivityType: myfirestore.EndBreakActivity,
			SeatId:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			TakenAt:      utils.JstNow(),
		}
		err = s.FirestoreController.CreateUserActivityDoc(tx, endBreakActivity)
		if err != nil {
			s.MessageToOwnerWithError("failed to add an user activity", err)
			return err
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
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) Rank(_ *utils.CommandDetails, ctx context.Context) error {
	replyMessage := ""
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 変更前のuserDocを読み込んでおく
		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed to ReadUser", err)
			return err
		}

		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToOwnerWithError("failed IsUserInRoom", err)
			return err
		}
		isInRoom := isInMemberRoom || isInGeneralRoom

		var currentSeat myfirestore.SeatDoc
		var realtimeTotalStudySec int
		if isInRoom {
			var cerr customerror.CustomError
			currentSeat, cerr = s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
			if cerr.IsNotNil() {
				return cerr.Body
			}

			realtimeTotalStudyDuration, _, err := s.GetUserRealtimeTotalStudyDurations(ctx, tx, s.ProcessedUserId)
			if err != nil {
				s.MessageToOwnerWithError("failed to RetrieveRealtimeTotalStudyDuration", err)
				return err
			}
			realtimeTotalStudySec = int(realtimeTotalStudyDuration.Seconds())
		}

		// 以降書き込みのみ

		// ランク表示設定のON/OFFを切り替える
		newRankVisible := !userDoc.RankVisible
		err = s.FirestoreController.UpdateUserRankVisible(tx, s.ProcessedUserId, newRankVisible)
		if err != nil {
			s.MessageToOwnerWithError("failed to UpdateUserRankVisible", err)
			return err
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
				s.MessageToOwnerWithError("failed to GetSeatAppearance()", err)
				return err
			}

			// 席の色を更新
			currentSeat.Appearance = seatAppearance
			err = s.FirestoreController.UpdateSeat(tx, currentSeat, isInMemberRoom)
			if err != nil {
				s.MessageToOwnerWithError("failed to s.FirestoreController.UpdateSeat()", err)
				return err
			}
		}

		return nil
	})
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

// IsSeatExist 席番号1～max-seatsの席かどうかを判定。
func (s *System) IsSeatExist(ctx context.Context, seatId int, isMemberSeat bool) (bool, error) {
	constants, err := s.FirestoreController.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return false, err
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
				return false, err
			}
			return isExist, nil
		}
		s.MessageToOwnerWithError("failed to ReadSeat", err)
		return false, err
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
			return false, err
		}
	}
	return true, nil
}

// IsUserInRoom そのユーザーがルーム内にいるか？登録済みかに関わらず。
func (s *System) IsUserInRoom(ctx context.Context, userId string) (isInMemberRoom bool, isInGeneralRoom bool, err error) {
	isInMemberRoom = true
	isInGeneralRoom = true
	_, err = s.FirestoreController.ReadSeatWithUserId(ctx, userId, true)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			isInMemberRoom = false
		} else {
			return false, false, err
		}
	}
	_, err = s.FirestoreController.ReadSeatWithUserId(ctx, userId, false)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			isInGeneralRoom = false
		} else {
			return false, false, err
		}
	}
	if isInGeneralRoom && isInMemberRoom {
		s.MessageToOwner("isInGeneralRoom && isInMemberRoom")
		return false, false, errors.New("isInGeneralRoom && isInMemberRoom")
	}
	return isInMemberRoom, isInGeneralRoom, nil
}

func (s *System) CreateUser(tx *firestore.Transaction) error {
	log.Println("CreateUser()")
	userData := myfirestore.UserDoc{
		DailyTotalStudySec: 0,
		TotalStudySec:      0,
		RegistrationDate:   utils.JstNow(),
	}
	return s.FirestoreController.CreateUser(tx, s.ProcessedUserId, userData)
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
	customerror.CustomError) {
	var seats []myfirestore.SeatDoc
	var err error
	if isMemberSeat {
		seats, err = s.FirestoreController.ReadMemberSeats(ctx)
		if err != nil {
			return 0, customerror.Unknown.Wrap(err)
		}
	} else {
		seats, err = s.FirestoreController.ReadGeneralSeats(ctx)
		if err != nil {
			return 0, customerror.Unknown.Wrap(err)
		}
	}

	constants, err := s.FirestoreController.ReadSystemConstantsConfig(ctx, tx)
	if err != nil {
		return 0, customerror.Unknown.Wrap(err)
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
		rand.Seed(utils.JstNow().UnixNano())
		rand.Shuffle(len(vacantSeatIdList), func(i, j int) { vacantSeatIdList[i], vacantSeatIdList[j] = vacantSeatIdList[j], vacantSeatIdList[i] })
		for _, seatId := range vacantSeatIdList {
			ifSittingTooMuch, err := s.CheckIfUserSittingTooMuchForSeat(ctx, userId, seatId, isMemberSeat)
			if err != nil {
				return -1, customerror.Unknown.Wrap(err)
			}
			if !ifSittingTooMuch {
				return seatId, customerror.NewNil()
			}
		}
	}
	return 0, customerror.NoSeatAvailable.New("no seat available.")
}

// enterRoom ユーザーを入室させる。
func (s *System) enterRoom(
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
) (int, error) {
	enterDate := utils.JstNow()
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
	err := s.FirestoreController.CreateSeat(tx, newSeat, isMemberSeat)
	if err != nil {
		return 0, err
	}

	// 入室時刻を記録
	err = s.FirestoreController.UpdateUserLastEnteredDate(tx, userId, enterDate)
	if err != nil {
		s.MessageToOwnerWithError("failed to set last entered date", err)
		return 0, err
	}
	// activityログ記録
	enterActivity := myfirestore.UserActivityDoc{
		UserId:       userId,
		ActivityType: myfirestore.EnterRoomActivity,
		SeatId:       seatId,
		IsMemberSeat: isMemberSeat,
		TakenAt:      enterDate,
	}
	err = s.FirestoreController.CreateUserActivityDoc(tx, enterActivity)
	if err != nil {
		s.MessageToOwnerWithError("failed to add an user activity", err)
		return 0, err
	}
	// 久しぶりの入室であれば、isContinuousActiveをtrueに、lastPenaltyImposedDaysを0に更新
	if !isContinuousActive {
		err = s.FirestoreController.UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(tx, userId, true, enterDate)
		if err != nil {
			s.MessageToOwnerWithError("failed to UpdateUserIsContinuousActiveAndCurrentActivityStateStarted", err)
			return 0, err
		}
		err = s.FirestoreController.UpdateUserLastPenaltyImposedDays(tx, userId, 0)
		if err != nil {
			s.MessageToOwnerWithError("failed to UpdateUserLastPenaltyImposedDays", err)
			return 0, err
		}
	}

	// 入室から自動退室予定時刻までの時間（分）
	untilExitMin := int(exitDate.Sub(enterDate).Minutes())

	return untilExitMin, nil
}

// exitRoom ユーザーを退室させる。
func (s *System) exitRoom(
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
	err := s.FirestoreController.DeleteSeat(tx, previousSeat.SeatId, isMemberSeat)
	if err != nil {
		return 0, 0, err
	}

	// ログ記録
	exitActivity := myfirestore.UserActivityDoc{
		UserId:       previousSeat.UserId,
		ActivityType: myfirestore.ExitRoomActivity,
		SeatId:       previousSeat.SeatId,
		IsMemberSeat: isMemberSeat,
		TakenAt:      exitDate,
	}
	err = s.FirestoreController.CreateUserActivityDoc(tx, exitActivity)
	if err != nil {
		s.MessageToOwnerWithError("failed to add an user activity", err)
	}
	// 退室時刻を記録
	err = s.FirestoreController.UpdateUserLastExitedDate(tx, previousSeat.UserId, exitDate)
	if err != nil {
		s.MessageToOwnerWithError("failed to update last-exited-date", err)
		return 0, 0, err
	}
	// 累計作業時間を更新
	err = s.UpdateTotalWorkTime(tx, previousSeat.UserId, previousUserDoc, addedWorkedTimeSec, addedDailyWorkedTimeSec)
	if err != nil {
		s.MessageToOwnerWithError("failed to update total study time", err)
		return 0, 0, err
	}
	// RP更新
	netStudyDuration := time.Duration(addedWorkedTimeSec) * time.Second
	newRP, err := utils.CalcNewRPExitRoom(netStudyDuration, previousSeat.WorkName != "", previousUserDoc.IsContinuousActive, previousUserDoc.CurrentActivityStateStarted, exitDate, previousUserDoc.RankPoint)
	if err != nil {
		s.MessageToOwnerWithError("failed to CalcNewRPExitRoom", err)
		return 0, 0, err
	}
	err = s.FirestoreController.UpdateUserRankPoint(tx, previousSeat.UserId, newRP)
	if err != nil {
		s.MessageToOwnerWithError("failed to UpdateUserRP", err)
		return 0, 0, err
	}
	addedRP := newRP - previousUserDoc.RankPoint

	log.Println(previousSeat.UserId + " exited the room. seat id: " + strconv.Itoa(previousSeat.SeatId) + " (+ " + strconv.Itoa(addedWorkedTimeSec) + "秒)")
	log.Println(fmt.Sprintf("addedRP: %d, newRP: %d, previous RP: %d", addedRP, newRP, previousUserDoc.RankPoint))
	return addedWorkedTimeSec, addedRP, nil
}

func (s *System) moveSeat(tx *firestore.Transaction, targetSeatId int, latestUserProfileImage string, beforeIsMemberSeat, afterIsMemberSeat bool, option utils.MinutesAndWorkNameOption, previousSeat myfirestore.SeatDoc, previousUserDoc *myfirestore.UserDoc) (int, int, int, error) {
	jstNow := utils.JstNow()

	// 値チェック
	if targetSeatId == previousSeat.SeatId && beforeIsMemberSeat == afterIsMemberSeat {
		return 0, 0, 0, errors.New("targetSeatId == previousSeat.SeatId")
	}

	// 退室
	workedTimeSec, addedRP, err := s.exitRoom(tx, beforeIsMemberSeat, previousSeat, previousUserDoc)
	if err != nil {
		s.MessageToOwnerWithError("failed to exitRoom for "+s.ProcessedUserId, err)
		return 0, 0, 0, err
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
		s.MessageToOwnerWithError("failed to GetSeatAppearance", err)
		return 0, 0, 0, err
	}

	// 入室
	untilExitMin, err := s.enterRoom(
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
		previousSeat.CurrentStateUntil)
	if err != nil {
		s.MessageToOwnerWithError("failed to enter room", err)
		return 0, 0, 0, err
	}

	return workedTimeSec, addedRP, untilExitMin, nil
}

func (s *System) CurrentSeat(ctx context.Context, userId string, isMemberSeat bool) (myfirestore.SeatDoc, customerror.CustomError) {
	seat, err := s.FirestoreController.ReadSeatWithUserId(ctx, userId, isMemberSeat)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return myfirestore.SeatDoc{}, customerror.UserNotInTheRoom.New("the user is not in the room.")
		}
		return myfirestore.SeatDoc{}, customerror.Unknown.Wrap(err)
	}
	return seat, customerror.NewNil()
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
		message := "newTotalSec < previousTotalSec ??!! 処理を中断します。"
		s.MessageToOwner(userId + ": " + message)
		return errors.New(message)
	}

	err := s.FirestoreController.UpdateUserTotalTime(tx, userId, newTotalSec, newDailyTotalSec)
	if err != nil {
		return err
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
		s.MessageToOwnerWithError("failed IsUserInRoom", err)
		return 0, 0, err
	}
	isInRoom := isInMemberRoom || isInGeneralRoom
	if isInRoom {
		// 作業時間を計算
		currentSeat, cerr := s.CurrentSeat(ctx, userId, isInMemberRoom)
		if cerr.IsNotNil() {
			s.MessageToOwnerWithError("failed to CurrentSeat", cerr.Body)
			return 0, 0, cerr.Body
		}

		var err error
		realtimeDuration, err = utils.RealTimeTotalStudyDurationOfSeat(currentSeat)
		if err != nil {
			s.MessageToOwnerWithError("failed to RealTimeTotalStudyDurationOfSeat", err)
			return 0, 0, err
		}
		realtimeDailyDuration, err = utils.RealTimeDailyTotalStudyDurationOfSeat(currentSeat)
		if err != nil {
			s.MessageToOwnerWithError("failed to RealTimeDailyTotalStudyDurationOfSeat", err)
			return 0, 0, err
		}
	}

	userData, err := s.FirestoreController.ReadUser(ctx, tx, userId)
	if err != nil {
		s.MessageToOwnerWithError("failed to ReadUser", err)
		return 0, 0, err
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
				s.MessageToOwnerWithError("failed to ReadGeneralSeats", err)
				return err
			}
		} else {
			seats, err = s.FirestoreController.ReadGeneralSeats(ctx)
			if err != nil {
				s.MessageToOwnerWithError("failed to ReadGeneralSeats", err)
				return err
			}
		}
		if len(seats) == 0 {
			break
		}
		for _, seatCandidate := range seats {
			var message string
			err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
				seat, err := s.FirestoreController.ReadSeat(ctx, tx, seatCandidate.SeatId, isMemberRoom)
				if err != nil {
					return err
				}
				s.SetProcessedUser(seat.UserId, seat.UserDisplayName, seatCandidate.UserProfileImageUrl, false, false, isMemberRoom)
				userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
				if err != nil {
					s.MessageToOwnerWithError("failed to ReadUser", err)
					return err
				}
				// 退室処理
				workedTimeSec, addedRP, err := s.exitRoom(tx, isMemberRoom, seat, &userDoc)
				if err != nil {
					s.MessageToOwnerWithError("failed in s.exitRoom", err)
					return err
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
			if err != nil {
				log.Println(err)
				err = nil
			}
			log.Println(message)
		}
	}
	return nil
}

func (s *System) ListLiveChatMessages(ctx context.Context, pageToken string) ([]*youtube.LiveChatMessage, string, int, error) {
	return s.liveChatBot.ListMessages(ctx, pageToken)
}

func (s *System) MessageToLiveChat(ctx context.Context, message string) {
	err := s.liveChatBot.PostMessage(ctx, message)
	if err != nil {
		s.MessageToOwnerWithError("failed to send live chat message \""+message+"\"\n", err)
	}
	return
}

func (s *System) MessageToOwner(message string) {
	err := s.discordOwnerBot.SendMessage(message)
	if err != nil {
		log.Println("failed to send message to owner: ", err)
	}
	return // これが最終連絡手段のため、エラーは返さずログのみ。
}

func (s *System) MessageToOwnerWithError(message string, argErr error) {
	err := s.discordOwnerBot.SendMessageWithError(message, argErr)
	if err != nil {
		log.Println("failed to send message to owner: ", err)
	}
	return // これが最終連絡手段のため、エラーは返さずログのみ。
}

func (s *System) MessageToSharedDiscord(message string) error {
	return s.discordSharedBot.SendMessage(message)
}

func (s *System) LogToSharedDiscord(logMessage string) error {
	return s.discordSharedLogBot.SendMessage(logMessage)
}

// OrganizeDB 1分ごとに処理を行う。
// - 自動退室予定時刻(until)を過ぎているルーム内のユーザーを退室させる。
// - CurrentStateUntilを過ぎている休憩中のユーザーを作業再開させる。
// - 一時着席制限ブラックリスト・ホワイトリストのuntilを過ぎているドキュメントを削除する。
func (s *System) OrganizeDB(ctx context.Context, isMemberRoom bool) error {
	log.Println(utils.FuncNameOf(s.OrganizeDB), "isMemberRoom:", isMemberRoom)
	var err error

	log.Println("自動退室")
	// 全座席のスナップショットをとる（トランザクションなし）
	err = s.OrganizeDBAutoExit(ctx, isMemberRoom)
	if err != nil {
		s.MessageToOwnerWithError("failed to OrganizeDBAutoExit", err)
		return err
	}

	log.Println("作業再開")
	err = s.OrganizeDBResume(ctx, isMemberRoom)
	if err != nil {
		s.MessageToOwnerWithError("failed to OrganizeDBResume", err)
		return err
	}

	log.Println("一時着席制限ブラックリスト・ホワイトリストのクリーニング")
	err = s.OrganizeDBDeleteExpiredSeatLimits(ctx, isMemberRoom)
	if err != nil {
		s.MessageToOwnerWithError("failed to OrganizeDBDeleteExpiredSeatLimits", err)
		return err
	}

	return nil
}

func (s *System) OrganizeDBAutoExit(ctx context.Context, isMemberRoom bool) error {
	jstNow := utils.JstNow()
	candidateSeatsSnapshot, err := s.FirestoreController.ReadSeatsExpiredUntil(ctx, jstNow, isMemberRoom)
	if err != nil {
		s.MessageToOwnerWithError("failed to ReadGeneralSeats", err)
		return err
	}
	log.Println("自動退室候補" + strconv.Itoa(len(candidateSeatsSnapshot)) + "人")

	for _, seatSnapshot := range candidateSeatsSnapshot {
		liveChatMessage := ""
		err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			s.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, seatSnapshot.UserProfileImageUrl, false, false, isMemberRoom)

			// 現在も存在しているか
			seat, err := s.FirestoreController.ReadSeat(ctx, tx, seatSnapshot.SeatId, isMemberRoom)
			if err != nil {
				if status.Code(err) == codes.NotFound {
					log.Println("すぐ前に退室したということなのでスルー")
					return nil
				}
				s.MessageToOwnerWithError("failed to ReadSeat", err)
				return err
			}
			if !reflect.DeepEqual(seat, seatSnapshot) {
				log.Println("その座席に少しでも変更が加えられているのでスルー")
				return nil
			}

			userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
			if err != nil {
				s.MessageToOwnerWithError("failed to ReadUser", err)
				return err
			}

			autoExit := seat.Until.Before(utils.JstNow()) // 自動退室時刻を過ぎていたら自動退室

			// 以下書き込みのみ

			// 自動退室時刻による退室処理
			if autoExit {
				workedTimeSec, addedRP, err := s.exitRoom(tx, isMemberRoom, seat, &userDoc)
				if err != nil {
					s.MessageToOwnerWithError(s.ProcessedUserDisplayName+"さん（"+s.ProcessedUserId+"）の退室処理中にエラーが発生しました", err)
					return err
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
				liveChatMessage = i18n.T("command:exit", s.ProcessedUserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)
			}

			return nil
		})
		if err != nil {
			s.MessageToOwnerWithError("failed transaction", err)
			continue // err != nil でもreturnではなく次に進む
		}
		if liveChatMessage != "" {
			s.MessageToLiveChat(ctx, liveChatMessage)
		}
	}
	return nil
}

func (s *System) OrganizeDBResume(ctx context.Context, isMemberRoom bool) error {
	jstNow := utils.JstNow()
	candidateSeatsSnapshot, err := s.FirestoreController.ReadSeatsExpiredBreakUntil(ctx, jstNow, isMemberRoom)
	if err != nil {
		s.MessageToOwnerWithError("failed to ReadGeneralSeats", err)
		return err
	}
	log.Println("作業再開候補" + strconv.Itoa(len(candidateSeatsSnapshot)) + "人")

	for _, seatSnapshot := range candidateSeatsSnapshot {
		liveChatMessage := ""
		err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			s.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, seatSnapshot.UserProfileImageUrl, false, false, isMemberRoom)

			// 現在も存在しているか
			seat, err := s.FirestoreController.ReadSeat(ctx, tx, seatSnapshot.SeatId, isMemberRoom)
			if err != nil {
				if status.Code(err) == codes.NotFound {
					log.Println("すぐ前に退室したということなのでスルー")
					return nil
				}
				s.MessageToOwnerWithError("failed to ReadSeat", err)
				return err
			}
			if !reflect.DeepEqual(seat, seatSnapshot) {
				log.Println("その座席に少しでも変更が加えられているのでスルー")
				return nil
			}

			resume := seat.State == myfirestore.BreakState && seat.CurrentStateUntil.Before(utils.JstNow())

			// 以下書き込みのみ

			if resume { // 作業再開処理
				jstNow := utils.JstNow()
				until := seat.Until
				breakSec := int(utils.NoNegativeDuration(jstNow.Sub(seat.CurrentStateStartedAt)).Seconds())
				// もし日付を跨いで休憩してたら、daily-cumulative-work-secは0にリセットする
				var dailyCumulativeWorkSec = seat.DailyCumulativeWorkSec
				if breakSec > utils.SecondsOfDay(jstNow) {
					dailyCumulativeWorkSec = 0
				}

				seat.State = myfirestore.WorkState
				seat.CurrentStateStartedAt = jstNow
				seat.CurrentStateUntil = until
				seat.DailyCumulativeWorkSec = dailyCumulativeWorkSec
				err = s.FirestoreController.UpdateSeat(tx, seat, isMemberRoom)
				if err != nil {
					s.MessageToOwnerWithError("failed to s.FirestoreController.UpdateSeat", err)
					return err
				}
				// activityログ記録
				endBreakActivity := myfirestore.UserActivityDoc{
					UserId:       s.ProcessedUserId,
					ActivityType: myfirestore.EndBreakActivity,
					SeatId:       seat.SeatId,
					IsMemberSeat: isMemberRoom,
					TakenAt:      utils.JstNow(),
				}
				err = s.FirestoreController.CreateUserActivityDoc(tx, endBreakActivity)
				if err != nil {
					s.MessageToOwnerWithError("failed to add an user activity", err)
					return err
				}
				var seatIdStr string
				if isMemberRoom {
					seatIdStr = i18n.T("common:vip-seat-id", seat.SeatId)
				} else {
					seatIdStr = strconv.Itoa(seat.SeatId)
				}

				liveChatMessage = i18n.T("command-resume:work", s.ProcessedUserDisplayName, seatIdStr, int(utils.NoNegativeDuration(until.Sub(jstNow)).Minutes()))
			}
			return nil
		})
		if err != nil {
			s.MessageToOwnerWithError("failed transaction", err)
			continue // err != nil でもreturnではなく次に進む
		}
		if liveChatMessage != "" {
			s.MessageToLiveChat(ctx, liveChatMessage)
		}
	}
	return nil
}

func (s *System) OrganizeDBDeleteExpiredSeatLimits(ctx context.Context, isMemberRoom bool) error {
	jstNow := utils.JstNow()
	// white list
	for {
		iter := s.FirestoreController.Get500SeatLimitsAfterUntilInWHITEList(ctx, jstNow, isMemberRoom)
		count, err := s.DeleteIteratorDocs(ctx, iter)
		if err != nil {
			return err
		}
		if count == 0 {
			break
		}
	}

	// black list
	for {
		iter := s.FirestoreController.Get500SeatLimitsAfterUntilInBLACKList(ctx, jstNow, isMemberRoom)
		count, err := s.DeleteIteratorDocs(ctx, iter)
		if err != nil {
			return err
		}
		if count == 0 {
			break
		}
	}
	return nil
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
		s.MessageToOwnerWithError("failed to read seats", err)
		return err
	}
	err = s.OrganizeDBForceMove(ctx, seatsSnapshot, isMemberRoom)
	if err != nil {
		s.MessageToOwnerWithError("failed to OrganizeDBForceMove", err)
		return err
	}
	return nil
}

func (s *System) OrganizeDBForceMove(ctx context.Context, seatsSnapshot []myfirestore.SeatDoc, isMemberSeat bool) error {
	log.Println(strconv.Itoa(len(seatsSnapshot)) + "人")
	for _, seatSnapshot := range seatsSnapshot {
		var forcedMove bool // 長時間入室制限による強制席移動
		err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			s.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, seatSnapshot.UserProfileImageUrl, false, false, isMemberSeat)

			// 現在も存在しているか
			seat, err := s.FirestoreController.ReadSeat(ctx, tx, seatSnapshot.SeatId, isMemberSeat)
			if err != nil {
				if status.Code(err) == codes.NotFound {
					log.Println("すぐ前に退室したということなのでスルー")
					return nil
				}
				s.MessageToOwnerWithError("failed to ReadSeat", err)
				return err
			}
			if !reflect.DeepEqual(seat, seatSnapshot) {
				log.Println("その座席に少しでも変更が加えられているのでスルー")
				return nil
			}

			ifSittingTooMuch, err := s.CheckIfUserSittingTooMuchForSeat(ctx, s.ProcessedUserId, seat.SeatId, isMemberSeat)
			if err != nil {
				s.MessageToOwnerWithError(s.ProcessedUserDisplayName+"さん（"+s.ProcessedUserId+"）の席移動処理中にエラーが発生しました", err)
				return err
			}
			if ifSittingTooMuch {
				forcedMove = true
			}

			// 以下書き込みのみ

			if forcedMove { // 長時間入室制限による強制席移動
				// nested transactionとならないよう、RunTransactionの外側で実行
			}

			return nil
		})
		if err != nil {
			s.MessageToOwnerWithError("failed transaction", err)
			continue
		}
		// err != nil でもreturnではなく次に進む
		if forcedMove {
			var seatIdStr string
			if isMemberSeat {
				seatIdStr = i18n.T("common:vip-seat-id", seatSnapshot.SeatId)
			} else {
				seatIdStr = strconv.Itoa(seatSnapshot.SeatId)
			}
			s.MessageToLiveChat(ctx, i18n.T("others:force-move", s.ProcessedUserDisplayName, seatIdStr))

			inCommandDetails := &utils.CommandDetails{
				CommandType: utils.In,
				InOption: utils.InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &utils.MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         seatSnapshot.WorkName,
						DurationMin:      int(utils.NoNegativeDuration(seatSnapshot.Until.Sub(utils.JstNow())).Minutes()),
					},
					IsMemberSeat: isMemberSeat,
				},
			}
			err = s.In(ctx, inCommandDetails)
			if err != nil {
				s.MessageToOwnerWithError(s.ProcessedUserDisplayName+"さん（"+s.ProcessedUserId+"）の自動席移動処理中にエラーが発生しました", err)
				return err
			}
		}
	}
	return nil
}

func (s *System) CheckLiveStreamStatus(ctx context.Context) error {
	checker := guardians.NewLiveStreamChecker(s.FirestoreController, s.liveChatBot, s.discordOwnerBot)
	return checker.Check(ctx)
}

func (s *System) DailyOrganizeDB(ctx context.Context) ([]string, error) {
	log.Println("DailyOrganizeDB()")
	var lineMessage string

	log.Println("一時的累計作業時間をリセット")
	dailyResetCount, err := s.ResetDailyTotalStudyTime(ctx)
	if err != nil {
		s.MessageToOwnerWithError("failed to ResetDailyTotalStudyTime", err)
		return []string{}, err
	}
	lineMessage += "\nsuccessfully reset daily total study time. (" + strconv.Itoa(dailyResetCount) + " users)"

	log.Println("RP関連の情報更新・ペナルティ処理を行うユーザーのIDのリストを取得")
	err, userIdsToProcessRP := s.GetUserIdsToProcessRP(ctx)
	if err != nil {
		s.MessageToOwnerWithError("failed to GetUserIdsToProcessRP", err)
		return []string{}, err
	}

	lineMessage += "\n過去31日以内に入室した人数（RP処理対象）: " + strconv.Itoa(len(userIdsToProcessRP))
	lineMessage += "\n本日のDailyOrganizeDatabase()処理が完了しました（RP更新処理以外）。"
	s.MessageToOwner(lineMessage)
	log.Println("finished DailyOrganizeDB().")
	return userIdsToProcessRP, nil
}

func (s *System) ResetDailyTotalStudyTime(ctx context.Context) (int, error) {
	log.Println("ResetDailyTotalStudyTime()")
	// 時間がかかる処理なのでトランザクションはなし
	previousDate := s.Configs.Constants.LastResetDailyTotalStudySec.In(utils.JapanLocation())
	now := utils.JstNow()
	isDifferentDay := now.Year() != previousDate.Year() || now.Month() != previousDate.Month() || now.Day() != previousDate.Day()
	if isDifferentDay && now.After(previousDate) {
		userIter := s.FirestoreController.GetAllNonDailyZeroUserDocs(ctx)
		count := 0
		for {
			doc, err := userIter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return 0, err
			}
			err = s.FirestoreController.ResetDailyTotalStudyTime(ctx, doc.Ref)
			if err != nil {
				return 0, err
			}
			count += 1
		}
		err := s.FirestoreController.UpdateLastResetDailyTotalStudyTime(ctx, now)
		if err != nil {
			s.MessageToOwnerWithError("failed to UpdateLastResetDailyTotalStudyTime", err)
			return 0, err
		}
		return count, nil
	} else {
		s.MessageToOwner("all user's daily total study times are already reset today.")
		return 0, nil
	}
}

func (s *System) GetUserIdsToProcessRP(ctx context.Context) (error, []string) {
	log.Println("GetUserIdsToProcessRP()")
	jstNow := utils.JstNow()
	// 過去31日以内に入室したことのあるユーザーをクエリ（本当は退室したことのある人も取得したいが、クエリはORに対応してないため無視）
	_31daysAgo := jstNow.AddDate(0, 0, -31)
	iter := s.FirestoreController.GetUsersActiveAfterDate(ctx, _31daysAgo)

	var userIds []string
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err, []string{}
		}
		userId := doc.Ref.ID
		userIds = append(userIds, userId)
	}
	return nil, userIds
}

func (s *System) UpdateUserRPBatch(ctx context.Context, userIds []string, timeLimitSeconds int) ([]string, error) {
	jstNow := utils.JstNow()
	startTime := jstNow
	var doneUserIds []string
	for _, userId := range userIds {
		// 時間チェック
		duration := utils.JstNow().Sub(startTime)
		if int(duration.Seconds()) > timeLimitSeconds {
			return userIds, nil
		}

		// 処理
		err := s.UpdateUserRP(ctx, userId, jstNow)
		if err != nil {
			s.MessageToOwnerWithError("failed to UpdateUserRP, while processing "+userId, err)
			// pass. mark user as done
		}
		doneUserIds = append(doneUserIds, userId)
	}

	var remainingUserIds []string
	for _, userId := range userIds {
		if utils.ContainsString(doneUserIds, userId) {
			continue
		} else {
			remainingUserIds = append(remainingUserIds, userId)
		}
	}
	return remainingUserIds, nil
}

func (s *System) UpdateUserRP(ctx context.Context, userId string, jstNow time.Time) error {
	log.Println("[userId: " + userId + "] processing RP.")
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, userId)
		if err != nil {
			s.MessageToOwnerWithError("failed to ReadUser", err)
			return err
		}

		// 同日の重複処理防止チェック
		if utils.DateEqualJST(userDoc.LastRPProcessed, jstNow) {
			log.Println("user " + userId + " is already RP processed today, skipping.")
			return nil
		}

		lastPenaltyImposedDays, isContinuousActive, currentActivityStateStarted, rankPoint, err := utils.DailyUpdateRankPoint(
			userDoc.LastPenaltyImposedDays, userDoc.IsContinuousActive, userDoc.CurrentActivityStateStarted,
			userDoc.RankPoint, userDoc.LastEntered, userDoc.LastExited, jstNow)
		if err != nil {
			s.MessageToOwnerWithError("failed to DailyUpdateRankPoint", err)
			return err
		}

		// 変更項目がある場合のみ変更
		if lastPenaltyImposedDays != userDoc.LastPenaltyImposedDays {
			err := s.FirestoreController.UpdateUserLastPenaltyImposedDays(tx, userId, lastPenaltyImposedDays)
			if err != nil {
				s.MessageToOwnerWithError("failed to UpdateUserLastPenaltyImposedDays", err)
				return err
			}
		}
		if isContinuousActive != userDoc.IsContinuousActive || !currentActivityStateStarted.Equal(userDoc.CurrentActivityStateStarted) {
			err := s.FirestoreController.UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(tx, userId, isContinuousActive, currentActivityStateStarted)
			if err != nil {
				s.MessageToOwnerWithError("failed to UpdateUserIsContinuousActiveAndCurrentActivityStateStarted", err)
				return err
			}
		}
		if rankPoint != userDoc.RankPoint {
			err := s.FirestoreController.UpdateUserRankPoint(tx, userId, rankPoint)
			if err != nil {
				s.MessageToOwnerWithError("failed to UpdateUserRankPoint", err)
				return err
			}
		}

		err = s.FirestoreController.UpdateUserLastRPProcessed(tx, userId, jstNow)
		if err != nil {
			s.MessageToOwnerWithError("failed to UpdateUserLastRPProcessed", err)
			return err
		}

		return nil
	})
}

func (s *System) GetAllUsersTotalStudySecList(ctx context.Context) ([]utils.UserIdTotalStudySecSet, error) {
	var set []utils.UserIdTotalStudySecSet

	userDocRefs, err := s.FirestoreController.GetAllUserDocRefs(ctx)
	if err != nil {
		return set, err
	}
	for _, userDocRef := range userDocRefs {
		userDoc, err := s.FirestoreController.ReadUser(ctx, nil, userDocRef.ID)
		if err != nil {
			return set, err
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
			return -1, err
		}
	} else {
		seats, err = s.FirestoreController.ReadGeneralSeats(ctx)
		if err != nil {
			return -1, err
		}
	}

	constants, err := s.FirestoreController.ReadSystemConstantsConfig(ctx, tx)
	if err != nil {
		return -1, err
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
				return -1, err
			}
			if !ifSittingTooMuch {
				return searchingSeatId, nil
			}
		}
		searchingSeatId += 1
	}
	return -1, errors.New("no available seat")
}

func (s *System) AddLiveChatHistoryDoc(ctx context.Context, chatMessage *youtube.LiveChatMessage) error {
	// example of publishedAt: "2021-11-13T07:21:30.486982+00:00"
	publishedAt, err := time.Parse(time.RFC3339Nano, chatMessage.Snippet.PublishedAt)
	if err != nil {
		return err
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
			return 0, 0, err
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
			return 0, 0, err
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
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// forで各docをdeleteしていく
		for {
			doc, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return err
			}
			count++
			err = s.FirestoreController.DeleteDocRef(ctx, tx, doc.Ref)
			if err != nil {
				log.Println("failed to DeleteDocRef()")
				return err
			}
		}
		return nil
	})
	return count, err
}

func (s *System) BackupCollectionHistoryFromGcsToBigquery(ctx context.Context, clientOption option.ClientOption) error {
	log.Println("BackupCollectionHistoryFromGcsToBigquery()")
	// 時間がかかる処理なのでトランザクションはなし
	previousDate := s.Configs.Constants.LastTransferCollectionHistoryBigquery.In(utils.JapanLocation())
	now := utils.JstNow()
	isDifferentDay := now.Year() != previousDate.Year() || now.Month() != previousDate.Month() || now.Day() != previousDate.Day()
	if isDifferentDay && now.After(previousDate) {
		gcsClient, err := mystorage.NewStorageClient(ctx, clientOption, s.Configs.Constants.GcpRegion)
		if err != nil {
			return err
		}
		defer gcsClient.CloseClient()

		projectId, err := utils.GetGcpProjectId(ctx, clientOption)
		if err != nil {
			return err
		}
		bqClient, err := mybigquery.NewBigqueryClient(ctx, projectId, clientOption, s.Configs.Constants.GcpRegion)
		if err != nil {
			return err
		}
		defer bqClient.CloseClient()

		gcsTargetFolderName, err := gcsClient.GetGcsYesterdayExportFolderName(ctx, s.Configs.Constants.GcsFirestoreExportBucketName)
		if err != nil {
			return err
		}

		err = bqClient.ReadCollectionsFromGcs(ctx, gcsTargetFolderName, s.Configs.Constants.GcsFirestoreExportBucketName,
			[]string{myfirestore.LiveChatHistory, myfirestore.UserActivities})
		if err != nil {
			return err
		}
		s.MessageToOwner("successfully transfer yesterday's live chat history to bigquery.")

		// 一定期間前のライブチャットおよびユーザー行動ログを削除
		// 何日以降分を保持するか求める
		retentionFromDate := utils.JstNow().Add(-time.Duration(s.Configs.Constants.CollectionHistoryRetentionDays*24) * time.
			Hour)
		retentionFromDate = time.Date(retentionFromDate.Year(), retentionFromDate.Month(), retentionFromDate.Day(),
			0, 0, 0, 0, retentionFromDate.Location())

		// ライブチャット・ユーザー行動ログ削除
		numRowsLiveChat, numRowsUserActivity, err := s.DeleteCollectionHistoryBeforeDate(ctx, retentionFromDate)
		if err != nil {
			return err
		}
		s.MessageToOwner(strconv.Itoa(int(retentionFromDate.Month())) + "月" + strconv.Itoa(retentionFromDate.Day()) +
			"日より前の日付のライブチャット履歴およびユーザー行動ログをFirestoreから削除しました。")
		s.MessageToOwner(fmt.Sprintf("削除したライブチャット件数: %d\n削除したユーザー行動ログ件数: %d", numRowsLiveChat, numRowsUserActivity))

		err = s.FirestoreController.UpdateLastTransferCollectionHistoryBigquery(ctx, now)
		if err != nil {
			return err
		}
	} else {
		s.MessageToOwner("yesterday's collection histories are already reset today.")
	}
	return nil
}

func (s *System) CheckIfUserSittingTooMuchForSeat(ctx context.Context, userId string, seatId int, isMemberSeat bool) (bool, error) {
	jstNow := utils.JstNow()

	// ホワイトリスト・ブラックリストを検索
	whiteListForUserAndSeat, err := s.FirestoreController.ReadSeatLimitsWHITEListWithSeatIdAndUserId(ctx, seatId, userId, isMemberSeat)
	if err != nil {
		return false, err
	}
	blackListForUserAndSeat, err := s.FirestoreController.ReadSeatLimitsBLACKListWithSeatIdAndUserId(ctx, seatId, userId, isMemberSeat)
	if err != nil {
		return false, err
	}

	// もし両方あったら矛盾なのでエラー
	if len(whiteListForUserAndSeat) > 0 && len(blackListForUserAndSeat) > 0 {
		return false, errors.New("len(whiteListForUserAndSeat) > 0 && len(blackListForUserAndSeat) > 0")
	}

	// 片方しかなければチェックは不要
	if len(whiteListForUserAndSeat) > 1 {
		return false, errors.New("len(whiteListForUserAndSeat) > 1")
	} else if len(whiteListForUserAndSeat) == 1 {
		if whiteListForUserAndSeat[0].Until.After(jstNow) {
			log.Println("[seat " + strconv.Itoa(seatId) + ": " + userId + "] found in white list. skipping.")
			return false, nil
		} else {
			// ホワイトリストに入っているが、期限切れのためチェックを続行
		}
	}
	if len(blackListForUserAndSeat) > 1 {
		return false, errors.New("len(blackListForUserAndSeat) > 1")
	} else if len(blackListForUserAndSeat) == 1 {
		if blackListForUserAndSeat[0].Until.After(jstNow) {
			log.Println("[seat " + strconv.Itoa(seatId) + ": " + userId + "] found in black list. skipping.")
			return true, nil
		} else {
			// ブラックリストに入っているが、期限切れのためチェックを続行
		}
	}

	totalEntryDuration, err := s.GetRecentUserSittingTimeForSeat(ctx, userId, seatId, isMemberSeat)
	if err != nil {
		return false, err
	}

	log.Println("[" + userId + "] 過去" + strconv.Itoa(s.Configs.Constants.RecentRangeMin) + "分以内に" + strconv.Itoa(seatId) + "番席に合計" + strconv.Itoa(int(totalEntryDuration.Minutes())) +
		"分入室")

	// 制限値と比較
	ifSittingTooMuch := int(totalEntryDuration.Minutes()) > s.Configs.Constants.RecentThresholdMin

	if !ifSittingTooMuch {
		until := jstNow.Add(time.Duration(s.Configs.Constants.RecentThresholdMin)*time.Minute - totalEntryDuration)
		if until.Sub(jstNow) > time.Duration(s.Configs.Constants.MinimumCheckLongTimeSittingIntervalMinutes)*time.Minute {
			// ホワイトリストに登録
			err := s.FirestoreController.CreateSeatLimitInWHITEList(ctx, seatId, userId, jstNow, until, isMemberSeat)
			if err != nil {
				return false, err
			}
			log.Println("[seat " + strconv.Itoa(seatId) + ": " + userId + "] saved to white list.")
		} else {
			// pass
		}
	} else {
		// ブラックリストに登録
		until := jstNow.Add(time.Duration(s.Configs.Constants.LongTimeSittingPenaltyMinutes) * time.Minute)
		err := s.FirestoreController.CreateSeatLimitInBLACKList(ctx, seatId, userId, jstNow, until, isMemberSeat)
		if err != nil {
			return false, err
		}
		log.Println("[seat " + strconv.Itoa(seatId) + ": " + userId + "] saved to black list.")
	}

	return ifSittingTooMuch, nil
}

func (s *System) GetRecentUserSittingTimeForSeat(ctx context.Context, userId string, seatId int, isMemberSeat bool) (time.Duration, error) {
	checkDurationFrom := utils.JstNow().Add(-time.Duration(s.Configs.Constants.RecentRangeMin) * time.Minute)

	// 指定期間の該当ユーザーの該当座席への入退室ドキュメントを取得する
	enterRoomActivities, err := s.FirestoreController.GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx, checkDurationFrom, userId, seatId, isMemberSeat)
	if err != nil {
		return 0, err
	}
	exitRoomActivities, err := s.FirestoreController.GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx, checkDurationFrom, userId, seatId, isMemberSeat)
	if err != nil {
		return 0, err
	}
	activityOnlyEnterExitList := append(enterRoomActivities, exitRoomActivities...)

	// activityListは長さ0の可能性もあることに注意

	// 入室と退室が交互に並んでいるか確認
	utils.SortUserActivityByTakenAtAscending(activityOnlyEnterExitList)
	orderOK := utils.CheckEnterExitActivityOrder(activityOnlyEnterExitList)
	if !orderOK {
		log.Printf("activity list: \n%v\n", pretty.Formatter(activityOnlyEnterExitList))
		return 0, errors.New("入室activityと退室activityが交互に並んでいない\n" + fmt.Sprintf("%v", pretty.Formatter(activityOnlyEnterExitList)))
	}

	log.Println("入退室ドキュメント数：" + strconv.Itoa(len(activityOnlyEnterExitList)))

	// 入退室をセットで考え、合計入室時間を求める
	totalEntryDuration := time.Duration(0)
	entryCount := 0 // 退室時（もしくは現在日時）にentryCountをインクリメント。
	lastEnteredTimestamp := checkDurationFrom
	for i, activity := range activityOnlyEnterExitList {
		//log.Println(activity.TakenAt.In(utils.JapanLocation()).String() + "に" + string(activity.ActivityType))
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
	err := s.liveChatBot.BanUser(ctx, userId)
	if err != nil {
		s.MessageToOwnerWithError("failed to BanUser", err)
		return err
	}
	return nil
}
