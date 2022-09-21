package core

import (
	"context"
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
	"app.modules/core/mylinebot"
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

func NewSystem(ctx context.Context, clientOption option.ClientOption) (System, error) {
	fsController, err := myfirestore.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return System{}, err
	}

	// credentials
	credentialsDoc, err := fsController.RetrieveCredentialsConfig(ctx, nil)
	if err != nil {
		return System{}, err
	}

	// youtube live chat bot
	liveChatBot, err := youtubebot.NewYoutubeLiveChatBot(credentialsDoc.YoutubeLiveChatId, fsController, ctx)
	if err != nil {
		return System{}, err
	}

	// line bot
	lineBot, err := mylinebot.NewLineBot(credentialsDoc.LineBotChannelSecret, credentialsDoc.LineBotChannelToken, credentialsDoc.LineBotDestinationLineId)
	if err != nil {
		return System{}, err
	}

	// discord bot
	discordBot, err := discordbot.NewDiscordBot(credentialsDoc.DiscordBotToken, credentialsDoc.DiscordBotTextChannelId)
	if err != nil {
		return System{}, err
	}

	// core constant values
	constantsConfig, err := fsController.RetrieveSystemConstantsConfig(ctx, nil)
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
			panic("The field " + v.Type().Field(i).Name + " has not initialized. " +
				"Check if the value on firestore appropriately set.")
		}
	}

	return System{
		Configs:             &configs,
		FirestoreController: fsController,
		liveChatBot:         liveChatBot,
		lineBot:             lineBot,
		discordBot:          discordBot,
	}, nil
}

func (s *System) RunTransaction(ctx context.Context, f func(ctx context.Context, tx *firestore.Transaction) error) error {
	return s.FirestoreController.FirestoreClient.RunTransaction(ctx, f)
}

func (s *System) SetProcessedUser(userId string, userDisplayName string, isChatModerator bool, isChatOwner bool) {
	s.ProcessedUserId = userId
	s.ProcessedUserDisplayName = userDisplayName
	s.ProcessedUserIsModeratorOrOwner = isChatModerator || isChatOwner
}

func (s *System) CloseFirestoreClient() {
	err := s.FirestoreController.FirestoreClient.Close()
	if err != nil {
		log.Println("failed close firestore client.")
	} else {
		log.Println("successfully closed firestore client.")
	}
}

func (s *System) AdjustMaxSeats(ctx context.Context) error {
	log.Println("AdjustMaxSeats()")
	// SetDesiredMaxSeats()などはLambdaからも並列で実行される可能性があるが、競合が起こってもそこまで深刻な問題にはならないため
	//トランザクションは使用しない。

	constants, err := s.FirestoreController.RetrieveSystemConstantsConfig(ctx, nil)
	if err != nil {
		return err
	}
	if constants.DesiredMaxSeats == constants.MaxSeats {
		return nil
	} else if constants.DesiredMaxSeats > constants.MaxSeats { // 席を増やす
		s.MessageToLiveChat(ctx, "ルームを増やします↗")
		return s.FirestoreController.SetMaxSeats(ctx, nil, constants.DesiredMaxSeats)
	} else { // 席を減らす
		// max_seatsを減らしても、空席率が設定値以上か確認
		seats, err := s.FirestoreController.RetrieveSeats(ctx)
		if err != nil {
			return err
		}
		if int(float32(constants.DesiredMaxSeats)*(1.0-constants.MinVacancyRate)) < len(seats) {
			message := "減らそうとしすぎ。desiredは却下し、desired max seats <= current max seatsとします。" +
				"desired: " + strconv.Itoa(constants.DesiredMaxSeats) + ", " +
				"current max seats: " + strconv.Itoa(constants.MaxSeats) + ", " +
				"current seats: " + strconv.Itoa(len(seats))
			log.Println(message)
			return s.FirestoreController.SetDesiredMaxSeats(ctx, nil, constants.MaxSeats)
		} else {
			// 消えてしまう席にいるユーザーを移動させる
			s.MessageToLiveChat(ctx, "人数が減ったためルームを減らします↘ 必要な場合は席を移動してもらうことがあります。")
			for _, seat := range seats {
				if seat.SeatId > constants.DesiredMaxSeats {
					s.SetProcessedUser(seat.UserId, seat.UserDisplayName, false, false)
					// 移動させる
					inCommandDetails := CommandDetails{
						CommandType: In,
						InOption: InOption{
							IsSeatIdSet: true,
							SeatId:      0,
							MinutesAndWorkName: MinutesAndWorkNameOption{
								IsWorkNameSet:    true,
								IsDurationMinSet: true,
								WorkName:         seat.WorkName,
								DurationMin:      int(utils.NoNegativeDuration(seat.Until.Sub(utils.JstNow())).Minutes()),
							},
						},
					}
					err = s.In(ctx, inCommandDetails)
					if err != nil {
						return err
					}
				}
			}
			// max_seatsを更新
			return s.FirestoreController.SetMaxSeats(ctx, nil, constants.DesiredMaxSeats)
		}
	}
}

// Command 入力コマンドを解析して実行
func (s *System) Command(ctx context.Context, commandString string, userId string, userDisplayName string, isChatModerator bool, isChatOwner bool) error {
	if userId == s.Configs.LiveChatBotChannelId {
		return nil
	}
	s.SetProcessedUser(userId, userDisplayName, isChatModerator, isChatOwner)

	// 初回の利用の場合はユーザーデータを初期化
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		isRegistered, err := s.IfUserRegistered(ctx, tx)
		if err != nil {
			s.MessageToLineBotWithError("failed to IfUserRegistered", err)
			return err
		}
		if !isRegistered {
			err := s.InitializeUser(tx)
			if err != nil {
				s.MessageToLineBotWithError("failed to InitializeUser", err)
				return err
			}
		}
		return nil
	})
	if err != nil {
		s.MessageToLiveChat(ctx, s.ProcessedUserDisplayName+"さん、エラーが発生しました")
		return err
	}

	commandDetails, cerr := ParseCommand(commandString)
	if cerr.IsNotNil() { // これはシステム内部のエラーではなく、入力コマンドが不正ということなので、return nil
		s.MessageToLiveChat(ctx, s.ProcessedUserDisplayName+"さん、"+cerr.Body.Error())
		return nil
	}
	//log.Printf("parsed command: %# v\n", pretty.Formatter(commandDetails))

	if cerr := s.ValidateCommand(commandDetails); cerr.IsNotNil() {
		s.MessageToLiveChat(ctx, s.ProcessedUserDisplayName+"さん、"+cerr.Body.Error())
		return nil
	}

	// commandDetailsに基づいて命令処理
	switch commandDetails.CommandType {
	case NotCommand:
		return nil
	case InvalidCommand:
		return nil
	case In:
		return s.In(ctx, commandDetails)
	case Out:
		return s.Out(commandDetails, ctx)
	case Info:
		return s.ShowUserInfo(commandDetails, ctx)
	case My:
		return s.My(commandDetails, ctx)
	case Change:
		return s.Change(commandDetails, ctx)
	case Seat:
		return s.ShowSeatInfo(commandDetails, ctx)
	case Report:
		return s.Report(commandDetails, ctx)
	case Kick:
		return s.Kick(commandDetails, ctx)
	case Check:
		return s.Check(commandDetails, ctx)
	case More:
		return s.More(commandDetails, ctx)
	case Break:
		return s.Break(ctx, commandDetails)
	case Resume:
		return s.Resume(ctx, commandDetails)
	case Rank:
		return s.Rank(commandDetails, ctx)
	default:
		s.MessageToLineBot("Unknown command: " + commandString)
	}
	return nil
}

func (s *System) In(ctx context.Context, command CommandDetails) error {
	var replyMessage string
	t := i18n.GetTFunc("command-in")
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToLineBotWithError("failed s.IsUserInRoom()", err)
			return err
		}
		var currentSeat myfirestore.SeatDoc
		var customErr customerror.CustomError
		if isInRoom {
			// 現在座っている席を取得
			currentSeat, customErr = s.CurrentSeat(ctx, s.ProcessedUserId)
			if customErr.IsNotNil() {
				s.MessageToLineBotWithError("failed CurrentSeat", customErr.Body)
				return customErr.Body
			}
		}

		inOption := &command.InOption

		// 席が指定されているか？
		if inOption.IsSeatIdSet {
			// 0番席だったら最小番号の空席に決定
			if inOption.SeatId == 0 {
				seatId, err := s.MinAvailableSeatIdForUser(ctx, tx, s.ProcessedUserId)
				if err != nil {
					s.MessageToLineBotWithError("failed s.MinAvailableSeatIdForUser()", err)
					return err
				}
				inOption.SeatId = seatId
			} else {
				// 以下のように前もってerr2を宣言しておき、このあとのIfSeatVacantとCheckSeatAvailabilityForUserで明示的に同じerr2
				//を使用するようにしておかないとCheckSeatAvailabilityForUserのほうでなぜか上のスコープのerrが使われてしまう（すべてerrとした場合）
				var isVacant, isAvailable bool
				var err2 error
				// その席が空いているか？
				isVacant, err2 = s.IfSeatVacant(ctx, tx, inOption.SeatId)
				if err2 != nil {
					s.MessageToLineBotWithError("failed s.IfSeatVacant()", err)
					return err2
				}
				if !isVacant {
					replyMessage = t("no-seat", s.ProcessedUserDisplayName, InCommand)
					return nil
				}
				// ユーザーはその席に対して入室制限を受けてないか？
				isAvailable, err2 = s.CheckSeatAvailabilityForUser(ctx, s.ProcessedUserId, inOption.SeatId)
				if err2 != nil {
					s.MessageToLineBotWithError("failed s.CheckSeatAvailabilityForUser()", err)
					return err2
				}
				if !isAvailable {
					replyMessage = t("no-availability", s.ProcessedUserDisplayName, InCommand)
					return nil
				}
			}
		} else { // 席の指定なし
			seatId, cerr := s.RandomAvailableSeatIdForUser(ctx, tx, s.ProcessedUserId)
			if cerr.IsNotNil() {
				if cerr.ErrorType == customerror.NoSeatAvailable {
					s.MessageToLineBotWithError("席数がmax seatに達していて、ユーザーが入室できない事象が発生。", cerr.Body)
				}
				return cerr.Body
			}
			inOption.SeatId = seatId
		}

		userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToLineBotWithError("failed to RetrieveUser", err)
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
		seatAppearance, err := s.RetrieveCurrentUserSeatAppearance(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToLineBotWithError("failed to RetrieveCurrentUserSeatAppearance", err)
			return err
		}

		// 動作が決定

		// =========== 以降は書き込み処理のみ ===========

		if isInRoom {
			if inOption.SeatId == currentSeat.SeatId { // 今と同じ席番号の場合、作業名と自動退室予定時刻を更新
				// 今と同じ席番号を指定した場合、「今はその席は使えません」ではじかれるので、ここまで到達しないはず。
				s.MessageToLineBot("到達しないはずのinOption.SeatId == currentSeat.SeatId") // TODO: 消す

				newSeat := &currentSeat // deep copyは手間がかかるのでポインタ。
				// 作業名を更新
				newSeat.WorkName = inOption.MinutesAndWorkName.WorkName
				// 自動退室予定時刻を更新
				newSeat.Until = utils.JstNow().Add(time.Duration(inOption.MinutesAndWorkName.DurationMin) * time.Minute)
				// 更新したseatsを保存
				err = s.FirestoreController.UpdateSeat(tx, *newSeat)
				if err != nil {
					s.MessageToLineBotWithError("failed to UpdateSeats", err)
					return err
				}

				// 更新しましたのメッセージ
				replyMessage = t("already-seat", s.ProcessedUserDisplayName, strconv.Itoa(currentSeat.SeatId))
				return nil
			} else { // 今と別の席番号の場合: 退室させてから、入室させる。
				// 席移動処理
				workedTimeSec, addedRP, untilExitMin, err := s.moveSeat(tx, inOption.SeatId, inOption.MinutesAndWorkName, currentSeat, &userDoc)
				if err != nil {
					s.MessageToLineBotWithError("failed to moveSeat for "+s.ProcessedUserId, err)
					return err
				}

				var rpEarned string
				if userDoc.RankVisible {
					rpEarned = i18n.T("command:rp-earned", addedRP)
				}
				replyMessage += t("seat-move", s.ProcessedUserDisplayName, currentSeat.SeatId, inOption.SeatId, workedTimeSec/60, rpEarned, untilExitMin)

				return nil
			}
		} else { // 入室のみ
			untilExitMin, err := s.enterRoom(tx, s.ProcessedUserId, s.ProcessedUserDisplayName,
				inOption.SeatId, inOption.MinutesAndWorkName.WorkName, "", inOption.MinutesAndWorkName.DurationMin,
				seatAppearance, myfirestore.WorkState, userDoc.IsContinuousActive, time.Time{}, time.Time{})
			if err != nil {
				s.MessageToLineBotWithError("failed to enter room", err)
				return err
			}

			// 入室しましたのメッセージ
			replyMessage = t("start", s.ProcessedUserDisplayName, untilExitMin, inOption.SeatId)
			return nil
		}
	})
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

// RetrieveCurrentUserSeatAppearance リアルタイムの現在のランクを求める
func (s *System) RetrieveCurrentUserSeatAppearance(ctx context.Context, tx *firestore.Transaction, userId string) (myfirestore.SeatAppearance, error) {
	userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, userId)
	if err != nil {
		s.MessageToLineBotWithError("failed to RetrieveUser", err)
		return myfirestore.SeatAppearance{}, err
	}
	totalStudyDuration, _, err := s.RetrieveRealtimeTotalStudyDurations(ctx, tx, userId)
	if err != nil {
		return myfirestore.SeatAppearance{}, err
	}
	seatAppearance, err := utils.GetSeatAppearance(int(totalStudyDuration.Seconds()), userDoc.RankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
	if err != nil {
		s.MessageToLineBotWithError("failed to GetSeatAppearance", err)
	}
	return seatAppearance, nil
}

func (s *System) Out(_ CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-out")
	var replyMessage string
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 今勉強中か？
		isInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToLineBotWithError("failed to IsUserInRoom()", err)
			return err
		}
		if !isInRoom {
			replyMessage = t("already-exit", s.ProcessedUserDisplayName)
			return nil
		}
		// 現在座っている席を特定
		seat, customErr := s.CurrentSeat(ctx, s.ProcessedUserId)
		if customErr.Body != nil {
			s.MessageToLineBotWithError("failed to s.CurrentSeat", customErr.Body)
			return customErr.Body
		}
		userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToLineBotWithError("failed to RetrieveUser", err)
			return err
		}

		// 退室処理
		workedTimeSec, addedRP, err := s.exitRoom(tx, seat, &userDoc)
		if err != nil {
			s.MessageToLineBotWithError("failed in s.exitRoom(seatId, ctx)", customErr.Body)
			return err
		}
		var rpEarned string
		if userDoc.RankVisible {
			rpEarned = t("rp-earned", addedRP)
		}
		replyMessage = t("exit", s.ProcessedUserDisplayName, workedTimeSec/60, seat.SeatId, rpEarned)
		return nil
	})
	if err != nil {
		replyMessage = t("error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) ShowUserInfo(command CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-user-info")
	var replyMessage string
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		totalStudyDuration, dailyTotalStudyDuration, err := s.RetrieveRealtimeTotalStudyDurations(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToLineBotWithError("failed s.RetrieveRealtimeTotalStudyDurations()", err)
			return err
		}
		totalTimeStr := utils.DurationToString(totalStudyDuration)
		dailyTotalTimeStr := utils.DurationToString(dailyTotalStudyDuration)
		replyMessage += t("base", s.ProcessedUserDisplayName, dailyTotalTimeStr, totalTimeStr)

		userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToLineBotWithError("failed s.FirestoreController.RetrieveUser", err)
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

func (s *System) ShowSeatInfo(_ CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-seat-info")
	var replyMessage string
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// そのユーザーは入室しているか？
		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return err
		}
		if isUserInRoom {
			currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId)
			if cerr.IsNotNil() {
				s.MessageToLineBotWithError("failed s.CurrentSeat()", cerr.Body)
				return cerr.Body
			}

			realtimeSittingDurationMin := int(utils.NoNegativeDuration(utils.JstNow().Sub(currentSeat.EnteredAt)).Minutes())
			realtimeTotalStudyDurationOfSeat, err := RealTimeTotalStudyDurationOfSeat(currentSeat)
			if err != nil {
				s.MessageToLineBotWithError("failed to RealTimeTotalStudyDurationOfSeat", err)
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
			replyMessage = t("base", s.ProcessedUserDisplayName, currentSeat.SeatId, stateStr, realtimeSittingDurationMin, int(realtimeTotalStudyDurationOfSeat.Minutes()), remainingMinutes, breakUntilStr)
		} else {
			replyMessage = i18n.T("command:not-enter", s.ProcessedUserDisplayName, InCommand)
		}
		return nil
	})
	if err != nil {
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) Report(command CommandDetails, ctx context.Context) error {
	t := i18n.GetTFunc("command-report")
	if command.ReportOption.Message == "" { // !reportのみは不可
		s.MessageToLiveChat(ctx, t("no-message", s.ProcessedUserDisplayName))
		return nil
	}

	lineMessage := t("line", ReportCommand, s.ProcessedUserId, s.ProcessedUserDisplayName, command.ReportOption.Message)
	s.MessageToLineBot(lineMessage)

	discordMessage := t("discord", ReportCommand, s.ProcessedUserDisplayName, command.ReportOption.Message)
	err := s.MessageToDiscordBot(discordMessage)
	if err != nil {
		s.MessageToLineBotWithError("管理者へメッセージが送信できませんでした: \""+discordMessage+"\"", err)
	}

	s.MessageToLiveChat(ctx, t("alert", s.ProcessedUserDisplayName))
	return nil
}

func (s *System) Kick(command CommandDetails, ctx context.Context) error {
	var replyMessage string
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// commanderはモデレーターかチャットオーナーか
		if !s.ProcessedUserIsModeratorOrOwner {
			replyMessage = s.ProcessedUserDisplayName + "さんは「" + KickCommand + "」コマンドを使用できません"
			return nil
		}

		// ターゲットの座席は誰か使っているか
		isSeatAvailable, err := s.IfSeatVacant(ctx, tx, command.KickOption.SeatId)
		if err != nil {
			return err
		}
		if isSeatAvailable {
			replyMessage = s.ProcessedUserDisplayName + "さん、その番号の座席は誰も使用していません"
			return nil
		}

		// ユーザーを強制退室させる
		seat, err := s.FirestoreController.RetrieveSeat(ctx, tx, command.KickOption.SeatId)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = s.ProcessedUserDisplayName + "さん、その番号の座席は誰も使用していません"
				return nil
			}
			s.MessageToLineBotWithError("failed to RetrieveSeat", err)
			return err
		}
		replyMessage = s.ProcessedUserDisplayName + "さん、" + strconv.Itoa(seat.SeatId) + "番席の" + seat.UserDisplayName + "さんを退室させます"

		// s.ProcessedUserが処理の対象ではないことに注意。
		userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, seat.UserId)
		if err != nil {
			s.MessageToLineBotWithError("failed to RetrieveUser", err)
			return err
		}

		workedTimeSec, addedRP, exitErr := s.exitRoom(tx, seat, &userDoc)
		if exitErr != nil {
			s.MessageToLineBotWithError(s.ProcessedUserDisplayName+"さんのkick退室処理中にエラーが発生しました", exitErr)
			return exitErr
		}
		var rpEarned string
		if userDoc.RankVisible {
			rpEarned = "（+ " + strconv.Itoa(addedRP) + " RP）"
		}
		replyMessage += seat.UserDisplayName + "さんが退室しました🚶🚪" +
			"（+ " + strconv.Itoa(workedTimeSec/60) + "分、" + strconv.Itoa(seat.SeatId) + "番席）" + rpEarned

		err = s.MessageToDiscordBot(s.ProcessedUserDisplayName + "さん、" + strconv.Itoa(seat.
			SeatId) + "番席のユーザーをkickしました。\n" +
			"チャンネル名: " + seat.UserDisplayName + "\n" +
			"作業名: " + seat.WorkName + "\n休憩中の作業名: " + seat.BreakWorkName + "\n" +
			"入室時間: " + strconv.Itoa(workedTimeSec/60) + "分\n" +
			"チャンネルURL: https://youtube.com/channel/" + seat.UserId)
		if err != nil {
			s.MessageToLineBotWithError("failed MessageToDiscordBot()", err)
			return err
		}
		return nil
	})
	if err != nil {
		replyMessage = s.ProcessedUserDisplayName + "さん、エラーが発生しました。もう一度試してみてください"
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) Check(command CommandDetails, ctx context.Context) error {
	var replyMessage string
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// commanderはモデレーターかチャットオーナーか
		if !s.ProcessedUserIsModeratorOrOwner {
			replyMessage = s.ProcessedUserDisplayName + "さんは「" + CheckCommand + "」コマンドを使用できません"
			return nil
		}

		// ターゲットの座席は誰か使っているか
		isSeatAvailable, err := s.IfSeatVacant(ctx, tx, command.CheckOption.SeatId)
		if err != nil {
			s.MessageToLineBotWithError("failed to IfSeatVacant", err)
			return err
		}
		if isSeatAvailable {
			replyMessage = s.ProcessedUserDisplayName + "さん、その番号の座席は誰も使用していません"
			return nil
		}
		// 座席情報を表示する
		seat, err := s.FirestoreController.RetrieveSeat(ctx, tx, command.CheckOption.SeatId)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = s.ProcessedUserDisplayName + "さん、その番号の座席は誰も使用していません"
				return nil
			}
			s.MessageToLineBotWithError("failed to RetrieveSeat", err)
			return err
		}
		sinceMinutes := utils.NoNegativeDuration(utils.JstNow().Sub(seat.EnteredAt)).Minutes()
		untilMinutes := utils.NoNegativeDuration(seat.Until.Sub(utils.JstNow())).Minutes()
		message := s.ProcessedUserDisplayName + "さん、" + strconv.Itoa(seat.SeatId) + "番席のユーザー情報です。\n" +
			"チャンネル名: " + seat.UserDisplayName + "\n" + "入室時間: " + strconv.Itoa(int(
			sinceMinutes)) + "分\n" +
			"作業名: " + seat.WorkName + "\n" + "休憩中の作業名: " + seat.BreakWorkName + "\n" +
			"自動退室まで" + strconv.Itoa(int(untilMinutes)) + "分\n" +
			"チャンネルURL: https://youtube.com/channel/" + seat.UserId
		err = s.MessageToDiscordBot(message)
		if err != nil {
			s.MessageToLineBotWithError("failed MessageToDiscordBot()", err)
			return err
		}
		replyMessage = s.ProcessedUserDisplayName + "さん、情報を送信しました"
		return nil
	})
	if err != nil {
		replyMessage = s.ProcessedUserDisplayName + "さん、エラーが発生しました。もう一度試してみてください"
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) My(command CommandDetails, ctx context.Context) error {
	// ユーザードキュメントはすでにあり、登録されていないプロパティだった場合、そのままプロパティを保存したら自動で作成される。
	// また、読み込みのときにそのプロパティがなくても大丈夫。自動で初期値が割り当てられる。
	// ただし、ユーザードキュメントがそもそもない場合は、書き込んでもエラーにはならないが、登録日が記録されないため、要登録。

	// オプションが1つ以上指定されているか？
	if len(command.MyOptions) == 0 {
		s.MessageToLiveChat(ctx, s.ProcessedUserDisplayName+"さん、オプションが正しく設定されているか確認してください")
		return nil
	}

	replyMessage := ""
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 変更前のuserDocを読み込んでおく
		userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToLineBotWithError("failed to RetrieveUser", err)
			return err
		}

		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToLineBotWithError("failed to IsUserInRoom", err)
			return err
		}
		var seats []myfirestore.SeatDoc
		var realTimeTotalStudySec int
		if isUserInRoom {
			seats, err = s.FirestoreController.RetrieveSeats(ctx)
			if err != nil {
				s.MessageToLineBotWithError("failed to CurrentSeat", err)
				return err
			}

			realTimeTotalStudyDuration, _, err := s.RetrieveRealtimeTotalStudyDurations(ctx, tx, s.ProcessedUserId)
			if err != nil {
				s.MessageToLineBotWithError("failed to RetrieveRealtimeTotalStudyDuration", err)
				return err
			}
			realTimeTotalStudySec = int(realTimeTotalStudyDuration.Seconds())
		}

		// これ以降は書き込みのみ

		replyMessage = s.ProcessedUserDisplayName + "さん、"
		currenRankVisible := userDoc.RankVisible
		for _, myOption := range command.MyOptions {
			if myOption.Type == RankVisible {
				newRankVisible := myOption.BoolValue
				// 現在の値と、設定したい値が同じなら、変更なし
				if userDoc.RankVisible == newRankVisible {
					var rankVisibleString string
					if userDoc.RankVisible {
						rankVisibleString = "オン"
					} else {
						rankVisibleString = "オフ"
					}
					replyMessage += "ランク表示モードはすでに" + rankVisibleString + "です。"
				} else { // 違うなら、切替
					err := s.FirestoreController.SetMyRankVisible(tx, s.ProcessedUserId, newRankVisible)
					if err != nil {
						s.MessageToLineBotWithError("failed to SetMyRankVisible", err)
						return err
					}
					var newValueString string
					if newRankVisible {
						newValueString = "オン"
					} else {
						newValueString = "オフ"
					}
					replyMessage += "ランク表示を" + newValueString + "にしました。"

					// 入室中であれば、座席の色も変える
					if isUserInRoom {
						seatAppearance, err := utils.GetSeatAppearance(realTimeTotalStudySec, newRankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
						if err != nil {
							s.MessageToLineBotWithError("failed to GetSeatAppearance", err)
							return err
						}

						// 席の色を更新
						newSeat, err := GetSeatByUserId(seats, s.ProcessedUserId)
						if err != nil {
							return err
						}
						newSeat.Appearance = seatAppearance
						err = s.FirestoreController.UpdateSeat(tx, newSeat)
						if err != nil {
							s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeats()", err)
							return err
						}
					}
				}
				currenRankVisible = newRankVisible
			} else if myOption.Type == DefaultStudyMin {
				err := s.FirestoreController.SetMyDefaultStudyMin(tx, s.ProcessedUserId, myOption.IntValue)
				if err != nil {
					s.MessageToLineBotWithError("failed to SetMyDefaultStudyMin", err)
					return err
				}
				// 値が0はリセットのこと。
				if myOption.IntValue == 0 {
					replyMessage += "デフォルトの作業時間をリセットしました。"
				} else {
					replyMessage += "デフォルトの作業時間を" + strconv.Itoa(myOption.IntValue) + "分に設定しました。"
				}
			} else if myOption.Type == FavoriteColor {
				// 値が-1はリセットのこと。
				var colorCode string
				if myOption.IntValue == -1 {
					colorCode = ""
					err = s.FirestoreController.SetMyFavoriteColor(tx, s.ProcessedUserId, colorCode)
					if err != nil {
						s.MessageToLineBotWithError("failed to SetMyFavoriteColor", err)
						return err
					}
					replyMessage += "お気に入りカラーをリセットしました。"
				} else {
					colorCode, err = utils.TotalStudyHoursToColorCode(myOption.IntValue)
					if err != nil {
						s.MessageToLineBotWithError("failed to TotalStudyHoursToColorCode", err)
						return err
					}
					err = s.FirestoreController.SetMyFavoriteColor(tx, s.ProcessedUserId, colorCode)
					if err != nil {
						s.MessageToLineBotWithError("failed to SetMyFavoriteColor", err)
						return err
					}
					replyMessage += "お気に入りカラーを更新しました。"
					if !utils.CanUseFavoriteColor(realTimeTotalStudySec) {
						replyMessage += "（累計作業時間が" + strconv.Itoa(utils.FavoriteColorAvailableThresholdHours) + "時間を超えるまでお気に入りカラーは使えません）"
					}
				}

				// 入室中であれば、座席の色も変える
				if isUserInRoom {
					newSeat, err := GetSeatByUserId(seats, s.ProcessedUserId)
					if err != nil {
						s.MessageToLineBotWithError("failed to GetSeatByUserId", err)
						return err
					}
					seatAppearance, err := utils.GetSeatAppearance(realTimeTotalStudySec, currenRankVisible, userDoc.RankPoint, colorCode)
					if err != nil {
						s.MessageToLineBotWithError("failed to GetSeatAppearance", err)
						return err
					}

					// 席の色を更新
					newSeat.Appearance = seatAppearance
					err = s.FirestoreController.UpdateSeat(tx, newSeat)
					if err != nil {
						s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeat()", err)
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		replyMessage = s.ProcessedUserDisplayName + "さん、エラーが発生しました。もう一度試してみてください"
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) Change(command CommandDetails, ctx context.Context) error {
	changeOption := &command.ChangeOption
	jstNow := utils.JstNow()
	replyMessage := ""
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// そのユーザーは入室中か？
		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToLineBotWithError("failed to IsUserInRoom()", err)
			return err
		}
		if !isUserInRoom {
			replyMessage = s.ProcessedUserDisplayName + "さん、入室中のみ使えるコマンドです"
			return nil
		}

		currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId)
		if cerr.IsNotNil() {
			s.MessageToLineBotWithError("failed to s.CurrentSeat(ctx)", cerr.Body)
			return cerr.Body
		}

		// validation
		cerr = s.ValidateChange(command, currentSeat.State)
		if cerr.IsNotNil() {
			replyMessage = s.ProcessedUserDisplayName + "さん、" + cerr.Body.Error()
			return nil
		}

		// これ以降は書き込みのみ可。
		newSeat := &currentSeat

		replyMessage = s.ProcessedUserDisplayName + "さん、"
		if changeOption.IsWorkNameSet {
			// 作業名もしくは休憩作業名を書きかえ
			switch currentSeat.State {
			case myfirestore.WorkState:
				newSeat.WorkName = changeOption.WorkName
				replyMessage += "作業内容を更新しました（" + strconv.Itoa(currentSeat.SeatId) + "番席）。"
			case myfirestore.BreakState:
				newSeat.BreakWorkName = changeOption.WorkName
				replyMessage += "休憩内容を更新しました（" + strconv.Itoa(currentSeat.SeatId) + "番席）。"
			}
		}
		if changeOption.IsDurationMinSet {
			switch currentSeat.State {
			case myfirestore.WorkState:
				// 作業時間（入室時間から自動退室までの時間）を変更
				realtimeEntryDurationMin := utils.NoNegativeDuration(jstNow.Sub(currentSeat.EnteredAt)).Minutes()
				requestedUntil := currentSeat.EnteredAt.Add(time.Duration(changeOption.DurationMin) * time.Minute)

				if requestedUntil.Before(jstNow) {
					// もし現在時刻が指定時間を経過していたら却下
					remainingWorkMin := currentSeat.Until.Sub(jstNow).Minutes()
					replyMessage += "すでに" + strconv.Itoa(changeOption.DurationMin) + "分以上入室しています。現在" + utils.Ftoa(realtimeEntryDurationMin) +
						"分入室中。自動退室まで残り" + utils.Ftoa(remainingWorkMin) + "分です"
				} else if requestedUntil.After(jstNow.Add(time.Duration(s.Configs.Constants.MaxWorkTimeMin) * time.Minute)) {
					// もし現在時刻より最大延長可能時間以上後なら却下
					remainingWorkMin := currentSeat.Until.Sub(jstNow).Minutes()
					replyMessage += "自動退室までの時間は現在時刻から" + strconv.Itoa(s.Configs.Constants.MaxWorkTimeMin) + "分後まで設定できます。現在" +
						utils.Ftoa(realtimeEntryDurationMin) + "分入室中。自動退室まで残り" + utils.Ftoa(remainingWorkMin) + "分です"
				} else { // それ以外なら延長
					newSeat.Until = requestedUntil
					newSeat.CurrentStateUntil = requestedUntil
					remainingWorkMin := utils.NoNegativeDuration(requestedUntil.Sub(jstNow)).Minutes()
					replyMessage += "入室時間を" + strconv.Itoa(changeOption.DurationMin) + "分に変更しました。現在" + utils.Ftoa(realtimeEntryDurationMin) +
						"分入室中。自動退室まで残り" + utils.Ftoa(remainingWorkMin) + "分です。"
				}
			case myfirestore.BreakState:
				// 休憩時間を変更
				realtimeBreakDuration := utils.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt))
				requestedUntil := currentSeat.CurrentStateStartedAt.Add(time.Duration(changeOption.DurationMin) * time.Minute)

				if requestedUntil.Before(jstNow) {
					// もし現在時刻が指定時間を経過していたら却下
					remainingBreakDuration := currentSeat.CurrentStateUntil.Sub(jstNow)
					replyMessage += "すでに" + strconv.Itoa(changeOption.DurationMin) + "分以上休憩しています。現在" + utils.Ftoa(realtimeBreakDuration.Minutes()) +
						"分休憩中。作業再開まで残り" + utils.Ftoa(remainingBreakDuration.Minutes()) + "分です"
				} else { // それ以外ならuntilを変更
					newSeat.CurrentStateUntil = requestedUntil
					remainingBreakDuration := requestedUntil.Sub(jstNow)
					replyMessage += "休憩時間を" + strconv.Itoa(changeOption.DurationMin) + "分に変更しました。現在" + utils.Ftoa(realtimeBreakDuration.Minutes()) +
						"分休憩中。作業再開まで残り" + utils.Ftoa(remainingBreakDuration.Minutes()) + "分です。"
				}
			}
		}
		err = s.FirestoreController.UpdateSeat(tx, *newSeat)
		if err != nil {
			s.MessageToLineBotWithError("failed to UpdateSeats", err)
			return err
		}

		return nil
	})
	if err != nil {
		replyMessage = s.ProcessedUserDisplayName + "さん、エラーが発生しました。もう一度試してみてください"
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) More(command CommandDetails, ctx context.Context) error {
	replyMessage := ""
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		jstNow := utils.JstNow()

		// 入室しているか？
		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToLineBotWithError("failed to IsUserInRoom()", err)
			return err
		}
		if !isUserInRoom {
			replyMessage = s.ProcessedUserDisplayName + "さん、入室中のみ使えるコマンドです"
			return nil
		}

		currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId)
		if cerr.IsNotNil() {
			s.MessageToLineBotWithError("failed to s.CurrentSeat(ctx)", cerr.Body)
			return cerr.Body
		}

		// 以降書き込みのみ
		newSeat := &currentSeat

		replyMessage = s.ProcessedUserDisplayName + "さん、"
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
				replyMessage += "現在時刻から" + strconv.Itoa(s.Configs.Constants.
					MaxWorkTimeMin) + "分後までのみ作業時間を延長可能です。延長できる最大の時間で設定します。"
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
				replyMessage += "休憩は最大" + strconv.Itoa(s.Configs.Constants.
					MaxBreakDurationMin) + "分まで可能です。延長できる最大の時間で設定します。"
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

		err = s.FirestoreController.UpdateSeat(tx, *newSeat)
		if err != nil {
			s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeats", err)
			return err
		}

		switch currentSeat.State {
		case myfirestore.WorkState:
			replyMessage += "自動退室までの時間を" + strconv.Itoa(addedMin) + "分延長しました。"
		case myfirestore.BreakState:
			replyMessage += "休憩時間を" + strconv.Itoa(addedMin) + "分延長しました。"
			remainingBreakDuration := utils.NoNegativeDuration(newSeat.CurrentStateUntil.Sub(jstNow))
			replyMessage += "作業再開まで残り" + utils.Ftoa(remainingBreakDuration.Minutes()) + "分。"
		}
		realtimeEnteredTimeMin := utils.NoNegativeDuration(jstNow.Sub(currentSeat.EnteredAt)).Minutes()
		replyMessage += "現在" + utils.Ftoa(realtimeEnteredTimeMin) + "分入室中。自動退室まで残り" + strconv.Itoa(remainingUntilExitMin) + "分です"

		return nil
	})
	if err != nil {
		replyMessage = s.ProcessedUserDisplayName + "さん、エラーが発生しました。もう一度試してみてください。"
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) Break(ctx context.Context, command CommandDetails) error {
	breakOption := &command.BreakOption
	replyMessage := ""
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return err
		}
		if !isUserInRoom {
			replyMessage = s.ProcessedUserDisplayName + "さん、入室中のみ使えるコマンドです"
			return nil
		}

		// stateを確認
		currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId)
		if cerr.IsNotNil() {
			s.MessageToLineBotWithError("failed to CurrentSeat()", cerr.Body)
			return cerr.Body
		}
		if currentSeat.State != myfirestore.WorkState {
			replyMessage = s.ProcessedUserDisplayName + "さん、作業中のみ使えるコマンドです。"
			return nil
		}

		// 前回の入室または再開から、最低休憩間隔経っているか？
		currentWorkedMin := utils.NoNegativeDuration(utils.JstNow().Sub(currentSeat.CurrentStateStartedAt)).Minutes()
		if int(currentWorkedMin) < s.Configs.Constants.MinBreakIntervalMin {
			replyMessage = s.ProcessedUserDisplayName + "さん、作業を始めてから" + strconv.Itoa(s.Configs.Constants.
				MinBreakIntervalMin) + "分間は休憩できません。現在" + utils.Ftoa(currentWorkedMin) + "分作業中"
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

		err = s.FirestoreController.UpdateSeat(tx, currentSeat)
		if err != nil {
			s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeats", err)
			return err
		}
		// activityログ記録
		startBreakActivity := myfirestore.UserActivityDoc{
			UserId:       s.ProcessedUserId,
			ActivityType: myfirestore.StartBreakActivity,
			SeatId:       currentSeat.SeatId,
			TakenAt:      utils.JstNow(),
		}
		err = s.FirestoreController.AddUserActivityDoc(tx, startBreakActivity)
		if err != nil {
			s.MessageToLineBotWithError("failed to add an user activity", err)
			return err
		}

		replyMessage = s.ProcessedUserDisplayName + "さんが休憩します（最大" + strconv.Itoa(breakOption.DurationMin) + "分、" +
			strconv.Itoa(currentSeat.SeatId) + "番席）"
		return nil
	})
	if err != nil {
		replyMessage = s.ProcessedUserDisplayName + "さん、エラーが発生しました。もう一度試してみてください"
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) Resume(ctx context.Context, command CommandDetails) error {
	replyMessage := ""
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return err
		}
		if !isUserInRoom {
			replyMessage = s.ProcessedUserDisplayName + "さん、入室中のみ使えるコマンドです"
			return nil
		}

		// stateを確認
		currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId)
		if cerr.IsNotNil() {
			s.MessageToLineBotWithError("failed to CurrentSeat()", cerr.Body)
			return cerr.Body
		}
		if currentSeat.State != myfirestore.BreakState {
			replyMessage = s.ProcessedUserDisplayName + "さん、座席で休憩中のみ使えるコマンドです。"
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

		err = s.FirestoreController.UpdateSeat(tx, currentSeat)
		if err != nil {
			s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeats", err)
			return err
		}
		// activityログ記録
		endBreakActivity := myfirestore.UserActivityDoc{
			UserId:       s.ProcessedUserId,
			ActivityType: myfirestore.EndBreakActivity,
			SeatId:       currentSeat.SeatId,
			TakenAt:      utils.JstNow(),
		}
		err = s.FirestoreController.AddUserActivityDoc(tx, endBreakActivity)
		if err != nil {
			s.MessageToLineBotWithError("failed to add an user activity", err)
			return err
		}

		untilExitDuration := utils.NoNegativeDuration(until.Sub(jstNow))
		replyMessage = s.ProcessedUserDisplayName + "さんが作業を再開します（" + strconv.Itoa(currentSeat.SeatId) + "番席、自動退室まで" +
			utils.Ftoa(untilExitDuration.Minutes()) + "分）"
		return nil
	})
	if err != nil {
		replyMessage = s.ProcessedUserDisplayName + "さん、エラーが発生しました。もう一度試してみてください"
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

func (s *System) Rank(_ CommandDetails, ctx context.Context) error {
	replyMessage := ""
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 変更前のuserDocを読み込んでおく
		userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			s.MessageToLineBotWithError("failed to RetrieveUser", err)
			return err
		}

		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			s.MessageToLineBotWithError("failed to IsUserInRoom", err)
			return err
		}
		var currentSeat myfirestore.SeatDoc
		var realtimeTotalStudySec int
		if isUserInRoom {
			var cerr customerror.CustomError
			currentSeat, cerr = s.CurrentSeat(ctx, s.ProcessedUserId)
			if cerr.IsNotNil() {
				return cerr.Body
			}

			realtimeTotalStudyDuration, _, err := s.RetrieveRealtimeTotalStudyDurations(ctx, tx, s.ProcessedUserId)
			if err != nil {
				s.MessageToLineBotWithError("failed to RetrieveRealtimeTotalStudyDuration", err)
				return err
			}
			realtimeTotalStudySec = int(realtimeTotalStudyDuration.Seconds())
		}

		// 以降書き込みのみ

		// ランク表示設定のON/OFFを切り替える
		newRankVisible := !userDoc.RankVisible
		err = s.FirestoreController.SetMyRankVisible(tx, s.ProcessedUserId, newRankVisible)
		if err != nil {
			s.MessageToLineBotWithError("failed to SetMyRankVisible", err)
			return err
		}
		var newValueString string
		if newRankVisible {
			newValueString = "オン"
		} else {
			newValueString = "オフ"
		}
		replyMessage = s.ProcessedUserDisplayName + "さんのランク表示を" + newValueString + "にしました"

		// 入室中であれば、座席の色も変える
		if isUserInRoom {
			seatAppearance, err := utils.GetSeatAppearance(realtimeTotalStudySec, newRankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
			if err != nil {
				s.MessageToLineBotWithError("failed to GetSeatAppearance()", err)
				return err
			}

			// 席の色を更新
			currentSeat.Appearance = seatAppearance
			err = s.FirestoreController.UpdateSeat(tx, currentSeat)
			if err != nil {
				s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeat()", err)
				return err
			}
		}

		return nil
	})
	if err != nil {
		replyMessage = s.ProcessedUserDisplayName + "さん、エラーが発生しました。もう一度試してみてください"
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return err
}

// IsSeatExist 席番号1～max-seatsの席かどうかを判定。
func (s *System) IsSeatExist(ctx context.Context, seatId int) (bool, error) {
	constants, err := s.FirestoreController.RetrieveSystemConstantsConfig(ctx, nil)
	if err != nil {
		return false, err
	}
	return 1 <= seatId && seatId <= constants.MaxSeats, nil
}

// IfSeatVacant 席番号がseatIdの席が空いているかどうか。
func (s *System) IfSeatVacant(ctx context.Context, tx *firestore.Transaction, seatId int) (bool, error) {
	_, err := s.FirestoreController.RetrieveSeat(ctx, tx, seatId)
	if err != nil {
		if status.Code(err) == codes.NotFound { // その座席のドキュメントは存在しない
			// maxSeats以内かどうか
			isExist, err := s.IsSeatExist(ctx, seatId)
			if err != nil {
				return false, err
			}
			return isExist, nil
		}
		s.MessageToLineBotWithError("failed to RetrieveSeat", err)
		return false, err
	}
	// ここまで来ると指定された番号の席が使われてるということ
	return false, nil
}

func (s *System) IfUserRegistered(ctx context.Context, tx *firestore.Transaction) (bool, error) {
	_, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
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
func (s *System) IsUserInRoom(ctx context.Context, userId string) (bool, error) {
	_, err := s.FirestoreController.RetrieveSeatWithUserId(ctx, userId)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *System) InitializeUser(tx *firestore.Transaction) error {
	log.Println("InitializeUser()")
	userData := myfirestore.UserDoc{
		DailyTotalStudySec: 0,
		TotalStudySec:      0,
		RegistrationDate:   utils.JstNow(),
	}
	return s.FirestoreController.InitializeUser(tx, s.ProcessedUserId, userData)
}

func (s *System) RetrieveNextPageToken(ctx context.Context, tx *firestore.Transaction) (string, error) {
	return s.FirestoreController.RetrieveNextPageToken(ctx, tx)
}

func (s *System) SaveNextPageToken(ctx context.Context, nextPageToken string) error {
	return s.FirestoreController.SaveNextPageToken(ctx, nextPageToken)
}

// RandomAvailableSeatIdForUser roomの席が空いているならその中からランダムな席番号（該当ユーザーの入室上限にかからない範囲に限定）を、
// 空いていないならmax-seatsを増やし、最小の空席番号を返す。
func (s *System) RandomAvailableSeatIdForUser(ctx context.Context, tx *firestore.Transaction, userId string) (int,
	customerror.CustomError) {
	seats, err := s.FirestoreController.RetrieveSeats(ctx)
	if err != nil {
		return 0, customerror.Unknown.Wrap(err)
	}

	constants, err := s.FirestoreController.RetrieveSystemConstantsConfig(ctx, tx)
	if err != nil {
		return 0, customerror.Unknown.Wrap(err)
	}

	var vacantSeatIdList []int
	for id := 1; id <= constants.MaxSeats; id++ {
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
		// TODO このfor range意味不明
		for range vacantSeatIdList {
			rand.Seed(utils.JstNow().UnixNano())
			selectedSeatId := vacantSeatIdList[rand.Intn(len(vacantSeatIdList))]
			ifSeatAvailableForUser, err := s.CheckSeatAvailabilityForUser(ctx, userId, selectedSeatId)
			if err != nil {
				return -1, customerror.Unknown.Wrap(err)
			}
			if ifSeatAvailableForUser {
				return selectedSeatId, customerror.NewNil()
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
	seatId int,
	workName string,
	breakWorkName string,
	workMin int,
	seatAppearance myfirestore.SeatAppearance,
	state myfirestore.SeatState,
	isContinuousActive bool,
	breakStartedAt time.Time,
	breakUntil time.Time,
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
	err := s.FirestoreController.AddSeat(tx, newSeat)
	if err != nil {
		return 0, err
	}

	// 入室時刻を記録
	err = s.FirestoreController.SetLastEnteredDate(tx, userId, enterDate)
	if err != nil {
		s.MessageToLineBotWithError("failed to set last entered date", err)
		return 0, err
	}
	// activityログ記録
	enterActivity := myfirestore.UserActivityDoc{
		UserId:       userId,
		ActivityType: myfirestore.EnterRoomActivity,
		SeatId:       seatId,
		TakenAt:      enterDate,
	}
	err = s.FirestoreController.AddUserActivityDoc(tx, enterActivity)
	if err != nil {
		s.MessageToLineBotWithError("failed to add an user activity", err)
		return 0, err
	}
	// 久しぶりの入室であれば、isContinuousActiveをtrueに更新
	if !isContinuousActive {
		err = s.FirestoreController.UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(tx, userId, true, enterDate)
		if err != nil {
			s.MessageToLineBotWithError("failed to UpdateUserIsContinuousActiveAndCurrentActivityStateStarted", err)
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
	err := s.FirestoreController.RemoveSeat(tx, previousSeat.SeatId)
	if err != nil {
		return 0, 0, err
	}

	// ログ記録
	exitActivity := myfirestore.UserActivityDoc{
		UserId:       previousSeat.UserId,
		ActivityType: myfirestore.ExitRoomActivity,
		SeatId:       previousSeat.SeatId,
		TakenAt:      exitDate,
	}
	err = s.FirestoreController.AddUserActivityDoc(tx, exitActivity)
	if err != nil {
		s.MessageToLineBotWithError("failed to add an user activity", err)
	}
	// 退室時刻を記録
	err = s.FirestoreController.SetLastExitedDate(tx, previousSeat.UserId, exitDate)
	if err != nil {
		s.MessageToLineBotWithError("failed to update last-exited-date", err)
		return 0, 0, err
	}
	// 累計作業時間を更新
	err = s.UpdateTotalWorkTime(tx, previousSeat.UserId, previousUserDoc, addedWorkedTimeSec, addedDailyWorkedTimeSec)
	if err != nil {
		s.MessageToLineBotWithError("failed to update total study time", err)
		return 0, 0, err
	}
	// RP更新
	netStudyDuration := time.Duration(addedWorkedTimeSec) * time.Second
	newRP, err := utils.CalcNewRPExitRoom(netStudyDuration, previousSeat.WorkName != "", previousUserDoc.IsContinuousActive, previousUserDoc.CurrentActivityStateStarted, exitDate, previousUserDoc.RankPoint)
	if err != nil {
		s.MessageToLineBotWithError("failed to CalcNewRPExitRoom", err)
		return 0, 0, err
	}
	err = s.FirestoreController.UpdateUserRankPoint(tx, previousSeat.UserId, newRP)
	if err != nil {
		s.MessageToLineBotWithError("failed to UpdateUserRP", err)
		return 0, 0, err
	}
	addedRP := newRP - previousUserDoc.RankPoint

	log.Println(previousSeat.UserId + " exited the room. seat id: " + strconv.Itoa(previousSeat.SeatId) + " (+ " + strconv.Itoa(addedWorkedTimeSec) + "秒)")
	log.Println("addedRP: " + strconv.Itoa(addedRP) + ", newRP: " + strconv.Itoa(newRP) + ", previous RP: " + strconv.Itoa(previousUserDoc.RankPoint))
	return addedWorkedTimeSec, addedRP, nil
}

func (s *System) moveSeat(tx *firestore.Transaction, targetSeatId int, option MinutesAndWorkNameOption, previousSeat myfirestore.SeatDoc, previousUserDoc *myfirestore.UserDoc) (int, int, int, error) {
	jstNow := utils.JstNow()

	// 値チェック
	if targetSeatId == previousSeat.SeatId {
		return 0, 0, 0, errors.New("targetSeatId == previousSeat.SeatId")
	}

	// 退室
	workedTimeSec, addedRP, err := s.exitRoom(tx, previousSeat, previousUserDoc)
	if err != nil {
		s.MessageToLineBotWithError("failed to exitRoom for "+s.ProcessedUserId, err)
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
		s.MessageToLineBotWithError("failed to GetSeatAppearance", err)
		return 0, 0, 0, err
	}

	// 入室
	untilExitMin, err := s.enterRoom(tx, previousSeat.UserId, previousSeat.UserDisplayName, targetSeatId, workName, previousSeat.BreakWorkName,
		workMin, newSeatAppearance, previousSeat.State, previousUserDoc.IsContinuousActive, previousSeat.CurrentStateStartedAt, previousSeat.CurrentStateUntil)
	if err != nil {
		s.MessageToLineBotWithError("failed to enter room", err)
		return 0, 0, 0, err
	}

	return workedTimeSec, addedRP, untilExitMin, nil
}

func (s *System) CurrentSeat(ctx context.Context, userId string) (myfirestore.SeatDoc, customerror.CustomError) {
	seat, err := s.FirestoreController.RetrieveSeatWithUserId(ctx, userId)
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
		s.MessageToLineBot(userId + ": " + message)
		return errors.New(message)
	}

	err := s.FirestoreController.UpdateTotalTime(tx, userId, newTotalSec, newDailyTotalSec)
	if err != nil {
		return err
	}
	return nil
}

// RetrieveRealtimeTotalStudyDurations リアルタイムの累積作業時間・当日累積作業時間を返す。
func (s *System) RetrieveRealtimeTotalStudyDurations(ctx context.Context, tx *firestore.Transaction, userId string) (time.Duration, time.Duration, error) {
	// 入室中ならばリアルタイムの作業時間も加算する
	realtimeDuration := time.Duration(0)
	realtimeDailyDuration := time.Duration(0)
	isInRoom, err := s.IsUserInRoom(ctx, userId)
	if err != nil {
		s.MessageToLineBotWithError("failed to IsUserInRoom", err)
		return 0, 0, err
	}
	if isInRoom {
		// 作業時間を計算
		currentSeat, cerr := s.CurrentSeat(ctx, userId)
		if cerr.IsNotNil() {
			s.MessageToLineBotWithError("failed to CurrentSeat", cerr.Body)
			return 0, 0, cerr.Body
		}

		var err error
		realtimeDuration, err = RealTimeTotalStudyDurationOfSeat(currentSeat)
		if err != nil {
			s.MessageToLineBotWithError("failed to RealTimeTotalStudyDurationOfSeat", err)
			return 0, 0, err
		}
		realtimeDailyDuration, err = RealTimeDailyTotalStudyDurationOfSeat(currentSeat)
		if err != nil {
			s.MessageToLineBotWithError("failed to RealTimeDailyTotalStudyDurationOfSeat", err)
			return 0, 0, err
		}
	}

	userData, err := s.FirestoreController.RetrieveUser(ctx, tx, userId)
	if err != nil {
		s.MessageToLineBotWithError("failed to RetrieveUser", err)
		return 0, 0, err
	}

	// 累計
	totalDuration := realtimeDuration + time.Duration(userData.TotalStudySec)*time.Second

	// 当日の累計
	dailyTotalDuration := realtimeDailyDuration + time.Duration(userData.DailyTotalStudySec)*time.Second

	return totalDuration, dailyTotalDuration, nil
}

// ExitAllUserInRoom roomの全てのユーザーを退室させる。
func (s *System) ExitAllUserInRoom(ctx context.Context) error {
	finished := false
	for {
		if finished {
			break
		}
		return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			seats, err := s.FirestoreController.RetrieveSeats(ctx)
			if err != nil {
				s.MessageToLineBotWithError("failed to RetrieveSeats", err)
				return err
			}
			if len(seats) > 0 {
				seat := seats[0]
				s.SetProcessedUser(seat.UserId, seat.UserDisplayName, false, false)
				userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
				if err != nil {
					s.MessageToLineBotWithError("failed to RetrieveUser", err)
					return err
				}
				_, _, err = s.exitRoom(tx, seat, &userDoc)
				if err != nil {
					s.MessageToLineBotWithError("failed to exitRoom", err)
					return err
				}
			} else if len(seats) == 0 {
				finished = true
			}
			return nil
		})
	}
	return nil
}

func (s *System) ListLiveChatMessages(ctx context.Context, pageToken string) ([]*youtube.LiveChatMessage, string, int, error) {
	return s.liveChatBot.ListMessages(ctx, pageToken)
}

func (s *System) MessageToLiveChat(ctx context.Context, message string) {
	err := s.liveChatBot.PostMessage(ctx, message)
	if err != nil {
		s.MessageToLineBotWithError("failed to send live chat message \""+message+"\"\n", err)
	}
	return
}

func (s *System) MessageToLineBot(message string) {
	err := s.lineBot.SendMessage(message)
	if err != nil {
		log.Println("failed to send message to the LINE: ", err)
	}
	return // LINEが最終連絡手段のため、エラーは返さずログのみ。
}

func (s *System) MessageToLineBotWithError(message string, argErr error) {
	err := s.lineBot.SendMessageWithError(message, argErr)
	if err != nil {
		log.Println("failed to send message to the LINE: ", err)
	}
	return // LINEが最終連絡手段のため、エラーは返さずログのみ。
}

func (s *System) MessageToDiscordBot(message string) error {
	return s.discordBot.SendMessage(message)
}

// OrganizeDatabase 1分ごとに処理を行う。
// - untilを過ぎているルーム内のユーザーを退室させる。
// - CurrentStateUntilを過ぎている休憩中のユーザーを作業再開させる。
func (s *System) OrganizeDatabase(ctx context.Context) error {
	var seatsSnapshot []myfirestore.SeatDoc
	var err error

	// 自動退室
	log.Println("自動退室")
	// 全座席のスナップショットをとる（トランザクションなし）
	seatsSnapshot, err = s.FirestoreController.RetrieveSeats(ctx)
	if err != nil {
		s.MessageToLineBotWithError("failed to RetrieveSeats", err)
		return err
	}
	err = s.OrganizeDBAutoExit(ctx, seatsSnapshot)
	if err != nil {
		s.MessageToLineBotWithError("failed to OrganizeDBAutoExit", err)
		return err
	}

	// 作業再開
	log.Println("作業再開")
	// 全座席のスナップショットをとる（トランザクションなし）
	seatsSnapshot, err = s.FirestoreController.RetrieveSeats(ctx)
	if err != nil {
		s.MessageToLineBotWithError("failed to RetrieveSeats", err)
		return err
	}
	err = s.OrganizeDBResume(ctx, seatsSnapshot)
	if err != nil {
		s.MessageToLineBotWithError("failed to OrganizeDBResume", err)
		return err
	}

	return nil
}

func (s *System) OrganizeDBAutoExit(ctx context.Context, seatsSnapshot []myfirestore.SeatDoc) error {
	log.Println(strconv.Itoa(len(seatsSnapshot)) + "人")
	for _, seatSnapshot := range seatsSnapshot {
		liveChatMessage := ""
		err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			s.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, false, false)

			// 現在も存在しているか
			seat, err := s.FirestoreController.RetrieveSeat(ctx, tx, seatSnapshot.SeatId)
			if err != nil {
				if status.Code(err) == codes.NotFound {
					log.Println("すぐ前に退室したということなのでスルー")
					return nil
				}
				s.MessageToLineBotWithError("failed to RetrieveSeat", err)
				return err
			}
			if !reflect.DeepEqual(seat, seatSnapshot) {
				log.Println("その座席に少しでも変更が加えられているのでスルー")
				return nil
			}

			userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
			if err != nil {
				s.MessageToLineBotWithError("failed to RetrieveUser", err)
				return err
			}

			autoExit := seat.Until.Before(utils.JstNow()) // 自動退室時刻を過ぎていたら自動退室

			// 以下書き込みのみ

			// 自動退室時刻による退室処理
			if autoExit {
				workedTimeSec, addedRP, err := s.exitRoom(tx, seat, &userDoc)
				if err != nil {
					s.MessageToLineBotWithError(s.ProcessedUserDisplayName+"さん（"+s.ProcessedUserId+"）の退室処理中にエラーが発生しました", err)
					return err
				}
				var rpEarned string
				if userDoc.RankVisible {
					rpEarned = "（+ " + strconv.Itoa(addedRP) + " RP）"
				}
				liveChatMessage = s.ProcessedUserDisplayName + "さんが退室しました🚶🚪" +
					"（+ " + strconv.Itoa(workedTimeSec/60) + "分、" + strconv.Itoa(seat.SeatId) + "番席）" + rpEarned
			}

			return nil
		})
		if err != nil {
			s.MessageToLineBotWithError("failed transaction", err)
			continue // err != nil でもreturnではなく次に進む
		}
		if liveChatMessage != "" {
			s.MessageToLiveChat(ctx, liveChatMessage)
		}
	}
	return nil
}

func (s *System) OrganizeDBResume(ctx context.Context, seatsSnapshot []myfirestore.SeatDoc) error {
	log.Println(strconv.Itoa(len(seatsSnapshot)) + "人")
	for _, seatSnapshot := range seatsSnapshot {
		liveChatMessage := ""
		err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			s.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, false, false)

			// 現在も存在しているか
			seat, err := s.FirestoreController.RetrieveSeat(ctx, tx, seatSnapshot.SeatId)
			if err != nil {
				if status.Code(err) == codes.NotFound {
					log.Println("すぐ前に退室したということなのでスルー")
					return nil
				}
				s.MessageToLineBotWithError("failed to RetrieveSeat", err)
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
				err = s.FirestoreController.UpdateSeat(tx, seat)
				if err != nil {
					s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeat", err)
					return err
				}
				// activityログ記録
				endBreakActivity := myfirestore.UserActivityDoc{
					UserId:       s.ProcessedUserId,
					ActivityType: myfirestore.EndBreakActivity,
					SeatId:       seat.SeatId,
					TakenAt:      utils.JstNow(),
				}
				err = s.FirestoreController.AddUserActivityDoc(tx, endBreakActivity)
				if err != nil {
					s.MessageToLineBotWithError("failed to add an user activity", err)
					return err
				}
				liveChatMessage = s.ProcessedUserDisplayName + "さんが作業を再開します（自動退室まで" +
					utils.Ftoa(utils.NoNegativeDuration(until.Sub(jstNow)).Minutes()) + "分）"
			}
			return nil
		})
		if err != nil {
			s.MessageToLineBotWithError("failed transaction", err)
			continue // err != nil でもreturnではなく次に進む
		}
		if liveChatMessage != "" {
			s.MessageToLiveChat(ctx, liveChatMessage)
		}
	}
	return nil
}

// CheckLongTimeSitting 長時間入室しているユーザーを席移動させる。
func (s *System) CheckLongTimeSitting(ctx context.Context) error {
	// 全座席のスナップショットをとる（トランザクションなし）
	seatsSnapshot, err := s.FirestoreController.RetrieveSeats(ctx)
	if err != nil {
		s.MessageToLineBotWithError("failed to RetrieveSeats", err)
		return err
	}
	err = s.OrganizeDBForceMove(ctx, seatsSnapshot)
	if err != nil {
		s.MessageToLineBotWithError("failed to OrganizeDBForceMove", err)
		return err
	}
	return nil
}

func (s *System) OrganizeDBForceMove(ctx context.Context, seatsSnapshot []myfirestore.SeatDoc) error {
	log.Println(strconv.Itoa(len(seatsSnapshot)) + "人")
	for _, seatSnapshot := range seatsSnapshot {
		var forcedMove bool // 長時間入室制限による強制席移動
		err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			s.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, false, false)

			// 現在も存在しているか
			seat, err := s.FirestoreController.RetrieveSeat(ctx, tx, seatSnapshot.SeatId)
			if err != nil {
				if status.Code(err) == codes.NotFound {
					log.Println("すぐ前に退室したということなのでスルー")
					return nil
				}
				s.MessageToLineBotWithError("failed to RetrieveSeat", err)
				return err
			}
			if !reflect.DeepEqual(seat, seatSnapshot) {
				log.Println("その座席に少しでも変更が加えられているのでスルー")
				return nil
			}

			ifNotSittingTooMuch, err := s.CheckSeatAvailabilityForUser(ctx, s.ProcessedUserId, seat.SeatId)
			if err != nil {
				s.MessageToLineBotWithError(s.ProcessedUserDisplayName+"さん（"+s.ProcessedUserId+"）の席移動処理中にエラーが発生しました", err)
				return err
			}
			if !ifNotSittingTooMuch {
				forcedMove = true
			}

			// 以下書き込みのみ

			if forcedMove { // 長時間入室制限による強制席移動
				// nested transactionとならないよう、RunTransactionの外側で実行
			}

			return nil
		})
		if err != nil {
			s.MessageToLineBotWithError("failed transaction", err)
			continue
		}
		// err != nil でもreturnではなく次に進む
		if forcedMove {
			s.MessageToLiveChat(ctx, s.ProcessedUserDisplayName+"さんが"+strconv.Itoa(seatSnapshot.SeatId)+"番席の入室時間の一時上限に達したため席移動します💨")

			inCommandDetails := CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         seatSnapshot.WorkName,
						DurationMin:      int(utils.NoNegativeDuration(seatSnapshot.Until.Sub(utils.JstNow())).Minutes()),
					},
				},
			}
			err = s.In(ctx, inCommandDetails)
			if err != nil {
				s.MessageToLineBotWithError(s.ProcessedUserDisplayName+"さん（"+s.ProcessedUserId+"）の自動席移動処理中にエラーが発生しました", err)
				return err
			}
		}
	}
	return nil
}

func (s *System) CheckLiveStreamStatus(ctx context.Context) error {
	checker := guardians.NewLiveStreamChecker(s.FirestoreController, s.liveChatBot, s.lineBot)
	return checker.Check(ctx)
}

func (s *System) DailyOrganizeDatabase(ctx context.Context) (error, []string) {
	log.Println("DailyOrganizeDatabase()")
	// 時間がかかる処理なのでトランザクションはなし

	log.Println("一時的累計作業時間をリセット")
	err := s.ResetDailyTotalStudyTime(ctx)
	if err != nil {
		s.MessageToLineBotWithError("failed to ResetDailyTotalStudyTime", err)
		return err, []string{}
	}

	log.Println("RP関連の情報更新・ペナルティ処理")
	err, userIdsToProcessRP := s.RetrieveUserIdsToProcessRP(ctx)
	if err != nil {
		s.MessageToLineBotWithError("failed to RetrieveUserIdsToProcessRP", err)
		return err, []string{}
	}

	s.MessageToLineBot("本日のDailyOrganizeDatabase()処理が完了しました（RP更新処理以外）。")
	log.Println("finished DailyOrganizeDatabase().")
	return nil, userIdsToProcessRP
}

func (s *System) ResetDailyTotalStudyTime(ctx context.Context) error {
	log.Println("ResetDailyTotalStudyTime()")
	// 時間がかかる処理なのでトランザクションはなし
	previousDate := s.Configs.Constants.LastResetDailyTotalStudySec.In(utils.JapanLocation())
	now := utils.JstNow()
	isDifferentDay := now.Year() != previousDate.Year() || now.Month() != previousDate.Month() || now.Day() != previousDate.Day()
	if isDifferentDay && now.After(previousDate) {
		userIter := s.FirestoreController.RetrieveAllNonDailyZeroUserDocs(ctx)
		count := 0
		for {
			doc, err := userIter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return err
			}
			err = s.FirestoreController.ResetDailyTotalStudyTime(ctx, doc.Ref)
			if err != nil {
				return err
			}
			count += 1
		}
		s.MessageToLineBot("successfully reset all non-daily-zero user's daily total study time. (" + strconv.Itoa(count) + " users)")
		err := s.FirestoreController.SetLastResetDailyTotalStudyTime(ctx, now)
		if err != nil {
			return err
		}
	} else {
		s.MessageToLineBot("all user's daily total study times are already reset today.")
	}
	return nil
}

func (s *System) RetrieveUserIdsToProcessRP(ctx context.Context) (error, []string) {
	log.Println("RetrieveUserIdsToProcessRP()")
	jstNow := utils.JstNow()
	// 過去31日以内に入室したことのあるユーザーをクエリ（本当は退室したことのある人も取得したいが、クエリはORに対応してないため無視）
	_31daysAgo := jstNow.AddDate(0, 0, -31)
	iter := s.FirestoreController.RetrieveUsersActiveAfterDate(ctx, _31daysAgo)

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
	s.MessageToLineBot("過去31日以内に入室した人数: " + strconv.Itoa(len(userIds)))
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
			s.MessageToLineBotWithError("failed to UpdateUserRP, while processing "+userId, err)
			continue // 次のuserから処理は継続
		}
		doneUserIds = append(doneUserIds, userId)
	}

	var remainingUserIds []string
	for _, userId := range userIds {
		if containsString(doneUserIds, userId) {
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
		userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, userId)
		if err != nil {
			s.MessageToLineBotWithError("failed to RetrieveUser", err)
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
			s.MessageToLineBotWithError("failed to DailyUpdateRankPoint", err)
			return err
		}

		// 変更項目がある場合のみ変更
		if lastPenaltyImposedDays != userDoc.LastPenaltyImposedDays {
			err := s.FirestoreController.UpdateUserLastPenaltyImposedDays(tx, userId, lastPenaltyImposedDays)
			if err != nil {
				s.MessageToLineBotWithError("failed to UpdateUserLastPenaltyImposedDays", err)
				return err
			}
		}
		if isContinuousActive != userDoc.IsContinuousActive || !currentActivityStateStarted.Equal(userDoc.CurrentActivityStateStarted) {
			err := s.FirestoreController.UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(tx, userId, isContinuousActive, currentActivityStateStarted)
			if err != nil {
				s.MessageToLineBotWithError("failed to UpdateUserIsContinuousActiveAndCurrentActivityStateStarted", err)
				return err
			}
		}
		if rankPoint != userDoc.RankPoint {
			err := s.FirestoreController.UpdateUserRankPoint(tx, userId, rankPoint)
			if err != nil {
				s.MessageToLineBotWithError("failed to UpdateUserRP", err)
				return err
			}
		}

		err = s.FirestoreController.UpdateUserLastRPProcessed(tx, userId, jstNow)
		if err != nil {
			s.MessageToLineBotWithError("failed to UpdateUserLastRPProcessed", err)
			return err
		}

		return nil
	})
}

func (s *System) RetrieveAllUsersTotalStudySecList(ctx context.Context) ([]UserIdTotalStudySecSet, error) {
	var set []UserIdTotalStudySecSet

	userDocRefs, err := s.FirestoreController.RetrieveAllUserDocRefs(ctx)
	if err != nil {
		return set, err
	}
	for _, userDocRef := range userDocRefs {
		userDoc, err := s.FirestoreController.RetrieveUser(ctx, nil, userDocRef.ID)
		if err != nil {
			return set, err
		}
		set = append(set, UserIdTotalStudySecSet{
			UserId:        userDocRef.ID,
			TotalStudySec: userDoc.TotalStudySec,
		})
	}
	return set, nil
}

// MinAvailableSeatIdForUser 空いている最小の番号の席番号を求める。該当ユーザーの入室上限にかからない範囲に限定。
func (s *System) MinAvailableSeatIdForUser(ctx context.Context, tx *firestore.Transaction, userId string) (int, error) {
	seats, err := s.FirestoreController.RetrieveSeats(ctx)
	if err != nil {
		return -1, err
	}

	constants, err := s.FirestoreController.RetrieveSystemConstantsConfig(ctx, tx)
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
			isAvailable, err := s.CheckSeatAvailabilityForUser(ctx, userId, searchingSeatId)
			if err != nil {
				return -1, err
			}
			if isAvailable {
				return searchingSeatId, nil
			}
		}
		searchingSeatId += 1
	}
	return -1, errors.New("no available seat")
}

func (s *System) AddLiveChatHistoryDoc(ctx context.Context, chatMessage *youtube.LiveChatMessage) error {
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// publishedAtの値の例: "2021-11-13T07:21:30.486982+00:00"
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
			MessageText:           chatMessage.Snippet.TextMessageDetails.MessageText,
			PublishedAt:           publishedAt,
			Type:                  chatMessage.Snippet.Type,
		}
		err = s.FirestoreController.AddLiveChatHistoryDoc(ctx, tx, liveChatHistoryDoc)
		if err != nil {
			return err
		}

		return nil
	})
}

func (s *System) DeleteCollectionHistoryBeforeDate(ctx context.Context, date time.Time) error {
	// Firestoreでは1回のトランザクションで500件までしか削除できないため、500件ずつ回す

	// date以前の全てのlive chat history docsをクエリで取得
	for {
		iter := s.FirestoreController.Retrieve500LiveChatHistoryDocIdsBeforeDate(ctx, date)
		count, err := s.DeleteIteratorDocs(ctx, iter)
		if err != nil {
			return err
		}
		if count == 0 {
			break
		}
	}

	// date以前の全てのuser activity docをクエリで取得
	for {
		iter := s.FirestoreController.Retrieve500UserActivityDocIdsBeforeDate(ctx, date)
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

		projectId, err := GetGcpProjectId(ctx, clientOption)
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
		s.MessageToLineBot("successfully transfer yesterday's live chat history to bigquery.")

		// 一定期間前のライブチャットおよびユーザー行動ログを削除
		// 何日以降分を保持するか求める
		retentionFromDate := utils.JstNow().Add(-time.Duration(s.Configs.Constants.CollectionHistoryRetentionDays*24) * time.
			Hour)
		retentionFromDate = time.Date(retentionFromDate.Year(), retentionFromDate.Month(), retentionFromDate.Day(),
			0, 0, 0, 0, retentionFromDate.Location())

		// ライブチャット・ユーザー行動ログ削除
		err = s.DeleteCollectionHistoryBeforeDate(ctx, retentionFromDate)
		if err != nil {
			return err
		}
		s.MessageToLineBot(strconv.Itoa(int(retentionFromDate.Month())) + "月" + strconv.Itoa(retentionFromDate.Day()) +
			"日より前の日付のライブチャット履歴およびユーザー行動ログをFirestoreから削除しました。")

		err = s.FirestoreController.SetLastTransferCollectionHistoryBigquery(ctx, now)
		if err != nil {
			return err
		}
	} else {
		s.MessageToLineBot("yesterday's collection histories are already reset today.")
	}
	return nil
}

func (s *System) CheckSeatAvailabilityForUser(ctx context.Context, userId string, seatId int) (bool, error) {
	checkDurationFrom := utils.JstNow().Add(-time.Duration(s.Configs.Constants.RecentRangeMin) * time.Minute)

	// 指定期間の該当ユーザーの該当座席への入退室ドキュメントを取得する
	iter := s.FirestoreController.RetrieveAllUserActivityDocIdsAfterDateForUserAndSeat(ctx, checkDurationFrom, userId, seatId)
	var activityAllTypeList []myfirestore.UserActivityDoc
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return false, err
		}
		var activity myfirestore.UserActivityDoc
		err = doc.DataTo(&activity)
		if err != nil {
			return false, err
		}
		activityAllTypeList = append(activityAllTypeList, activity)
	}
	// activityListは長さ0の可能性もあることに注意

	// 入退室以外のactivityは除外
	var activityOnlyEnterExitList []myfirestore.UserActivityDoc
	for _, a := range activityAllTypeList {
		if a.ActivityType == myfirestore.EnterRoomActivity || a.ActivityType == myfirestore.ExitRoomActivity {
			activityOnlyEnterExitList = append(activityOnlyEnterExitList, a)
		}
	}

	// 入室と退室が交互に並んでいるか確認
	orderOK := CheckEnterExitActivityOrder(activityOnlyEnterExitList)
	if !orderOK {
		log.Printf("activity list: \n%v\n", pretty.Formatter(activityOnlyEnterExitList))
		return false, errors.New("入室activityと退室activityが交互に並んでいない")
	}

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

	log.Println("[userId: " + userId + "] 過去" + strconv.Itoa(s.Configs.Constants.RecentRangeMin) + "分以内に" + strconv.Itoa(seatId) + "番席に合計" + strconv.Itoa(int(totalEntryDuration.Minutes())) +
		"分入室")
	// 制限値と比較し、結果を返す
	return int(totalEntryDuration.Minutes()) < s.Configs.Constants.RecentThresholdMin, nil
}
