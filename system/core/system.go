package core

import (
	"app.modules/core/customerror"
	"app.modules/core/discordbot"
	"app.modules/core/guardians"
	"app.modules/core/mybigquery"
	"app.modules/core/myfirestore"
	"app.modules/core/mylinebot"
	"app.modules/core/mystorage"
	"app.modules/core/utils"
	"app.modules/core/youtubebot"
	"cloud.google.com/go/firestore"
	"context"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"math/rand"
	"reflect"
	"strconv"
	"time"
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
	
	constants := SystemConfigs{
		Constants:            constantsConfig,
		LiveChatBotChannelId: credentialsDoc.YoutubeBotChannelId,
	}
	
	// 全ての項目が初期化できているか確認
	v := reflect.ValueOf(constants)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).IsZero() {
			panic("The field " + v.Type().Field(i).Name + " has not initialized. " +
				"Check if the value on firestore appropriately set.")
		}
	}
	
	return System{
		Configs:             &constants,
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
		s.MessageToLiveChat(ctx, nil, "ルームを増やします↗")
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
			s.MessageToLiveChat(ctx, nil, "人数が減ったためルームを減らします↘　必要な場合は席を移動してもらうことがあります。")
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
	
	commandDetails, cerr := ParseCommand(commandString)
	if cerr.IsNotNil() { // これはシステム内部のエラーではなく、入力コマンドが不正ということなので、return nil
		s.MessageToLiveChat(ctx, nil, s.ProcessedUserDisplayName+"さん、"+cerr.Body.Error())
		return nil
	}
	//log.Printf("parsed command: %# v\n", pretty.Formatter(commandDetails))
	
	if cerr := s.ValidateCommand(commandDetails); cerr.IsNotNil() {
		s.MessageToLiveChat(ctx, nil, s.ProcessedUserDisplayName+"さん、"+cerr.Body.Error())
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
		_ = s.MessageToLineBot("Unknown command: " + commandString)
	}
	return nil
}

func (s *System) In(ctx context.Context, command CommandDetails) error {
	// 初回の利用の場合はユーザーデータを初期化
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		isRegistered, err := s.IfUserRegistered(ctx, tx)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to IfUserRegistered", err)
			return err
		}
		if !isRegistered {
			err := s.InitializeUser(tx)
			if err != nil {
				_ = s.MessageToLineBotWithError("failed to InitializeUser", err)
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed s.IsUserInRoom()", err)
			return err
		}
		var currentSeat myfirestore.SeatDoc
		var customErr customerror.CustomError
		if isInRoom {
			// 現在座っている席を取得
			currentSeat, customErr = s.CurrentSeat(ctx, s.ProcessedUserId)
			if customErr.IsNotNil() {
				_ = s.MessageToLineBotWithError("failed CurrentSeat", customErr.Body)
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました")
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
					_ = s.MessageToLineBotWithError("failed s.MinAvailableSeatIdForUser()", err)
					s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
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
					_ = s.MessageToLineBotWithError("failed s.IfSeatVacant()", err)
					s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
					return err2
				}
				if !isVacant {
					s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、その番号の席は"+"今は使えません。他の空いている席を選ぶか、「"+InCommand+"」で席を指定せずに入室してください")
					return nil
				}
				// ユーザーはその席に対して入室制限を受けてないか？
				isAvailable, err2 = s.CheckSeatAvailabilityForUser(ctx, s.ProcessedUserId, inOption.SeatId)
				if err2 != nil {
					_ = s.MessageToLineBotWithError("failed s.CheckSeatAvailabilityForUser()", err)
					s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
					return err2
				}
				if !isAvailable {
					s.MessageToLiveChat(ctx, tx,
						s.ProcessedUserDisplayName+"さん、その番号の席は"+"長時間入室制限のためしばらく使えません。他の空いている席を選ぶか、「"+InCommand+"」で席を指定せずに入室してください")
					return nil
				}
			}
		} else { // 席の指定なし
			seatId, cerr := s.RandomAvailableSeatIdForUser(ctx, tx, s.ProcessedUserId)
			if cerr.IsNotNil() {
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
					"さん、エラーが発生しました。もう一度試してみてください")
				if cerr.ErrorType == customerror.NoSeatAvailable {
					_ = s.MessageToLineBotWithError("席数がmax seatに達していて、ユーザーが入室できない事象が発生。", cerr.Body)
				}
				return cerr.Body
			}
			inOption.SeatId = seatId
		}
		
		userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to RetrieveUser", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください")
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
			_ = s.MessageToLineBotWithError("failed to RetrieveCurrentUserSeatAppearance", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
			return err
		}
		
		// 動作が決定
		
		// =========== 以降は書き込み処理のみ ===========
		
		if isInRoom {
			if inOption.SeatId == currentSeat.SeatId { // 今と同じ席番号の場合、作業名と自動退室予定時刻を更新
				newSeat := &currentSeat // deep copyは手間がかかるのでポインタ。
				// 作業名を更新
				newSeat.WorkName = inOption.MinutesAndWorkName.WorkName
				// 自動退室予定時刻を更新
				newSeat.Until = utils.JstNow().Add(time.Duration(inOption.MinutesAndWorkName.DurationMin) * time.Minute)
				// 更新したseatsを保存
				err = s.FirestoreController.UpdateSeat(tx, *newSeat)
				if err != nil {
					_ = s.MessageToLineBotWithError("failed to UpdateSeats", err)
					s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
						"さん、エラーが発生しました。もう一度試してみてください")
					return err
				}
				
				// 更新しましたのメッセージ
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さんはすでに"+strconv.Itoa(currentSeat.SeatId)+"番の席に座っています。作業名と入室時間を更新しました")
				return nil
			} else { // 今と別の席番号の場合: 退室させてから、入室させる。
				// 作業名は指定がない場合引き継ぐ。
				if inOption.MinutesAndWorkName.WorkName == "" && currentSeat.WorkName != "" {
					inOption.MinutesAndWorkName.WorkName = currentSeat.WorkName
				}
				
				reply := ""
				
				// 退室処理
				workedTimeSec, addedRP, err := s.exitRoom(tx, currentSeat, &userDoc)
				if err != nil {
					_ = s.MessageToLineBotWithError("failed to exitRoom for "+s.ProcessedUserId, err)
					s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
					return err
				}
				
				// 入室処理
				err = s.enterRoom(tx, s.ProcessedUserId, s.ProcessedUserDisplayName,
					inOption.SeatId, inOption.MinutesAndWorkName.WorkName, inOption.MinutesAndWorkName.DurationMin,
					seatAppearance, myfirestore.WorkState)
				if err != nil {
					_ = s.MessageToLineBotWithError("failed to enter room", err)
					s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
						"さん、エラーが発生しました。もう一度試してみてください")
					return err
				}
				
				var rpEarned string
				if userDoc.RankVisible {
					rpEarned = "（+ " + strconv.Itoa(addedRP) + " RP）"
				}
				reply += s.ProcessedUserDisplayName + "さんが席を移動しました🚶（" +
					strconv.Itoa(currentSeat.SeatId) + "→" + strconv.Itoa(inOption.SeatId) + "番席）" +
					"（+ " + strconv.Itoa(workedTimeSec/60) + "分）" + rpEarned + "（" + strconv.Itoa(inOption.MinutesAndWorkName.DurationMin) + "分後に自動退室）"
				
				// 移動しましたのメッセージ
				s.MessageToLiveChat(ctx, tx, reply)
				return nil
			}
		} else { // 入室のみ
			err = s.enterRoom(tx, s.ProcessedUserId, s.ProcessedUserDisplayName,
				inOption.SeatId, inOption.MinutesAndWorkName.WorkName, inOption.MinutesAndWorkName.DurationMin,
				seatAppearance, myfirestore.WorkState)
			if err != nil {
				_ = s.MessageToLineBotWithError("failed to enter room", err)
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
					"さん、エラーが発生しました。もう一度試してみてください")
				return err
			}
			
			// 入室しましたのメッセージ
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さんが作業を始めました🔥（最大"+strconv.Itoa(inOption.MinutesAndWorkName.DurationMin)+"分、"+strconv.Itoa(inOption.SeatId)+"番席）")
			return nil
		}
	})
}

// RetrieveCurrentUserSeatAppearance リアルタイムの現在のランクを求める
func (s *System) RetrieveCurrentUserSeatAppearance(ctx context.Context, tx *firestore.Transaction, userId string) (myfirestore.SeatAppearance, error) {
	userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, userId)
	if err != nil {
		_ = s.MessageToLineBotWithError("failed to RetrieveUser", err)
		return myfirestore.SeatAppearance{}, err
	}
	totalStudyDuration, _, err := s.RetrieveRealtimeTotalStudyDurations(ctx, tx, userId)
	if err != nil {
		return myfirestore.SeatAppearance{}, err
	}
	seatAppearance := utils.GetSeatAppearance(int(totalStudyDuration.Seconds()), userDoc.RankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
	return seatAppearance, nil
}

func (s *System) Out(_ CommandDetails, ctx context.Context) error {
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 今勉強中か？
		isInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to IsUserInRoom()", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
			return err
		}
		if !isInRoom {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、すでに退室しています")
			return nil
		}
		// 現在座っている席を特定
		seat, customErr := s.CurrentSeat(ctx, s.ProcessedUserId)
		if customErr.Body != nil {
			_ = s.MessageToLineBotWithError("failed to s.CurrentSeat", customErr.Body)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください")
			return customErr.Body
		}
		userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to RetrieveUser", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください")
			return err
		}
		
		// 退室処理
		workedTimeSec, addedRP, err := s.exitRoom(tx, seat, &userDoc)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed in s.exitRoom(seatId, ctx)", customErr.Body)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
			return err
		}
		var rpEarned string
		if userDoc.RankVisible {
			rpEarned = "（+ " + strconv.Itoa(addedRP) + " RP）"
		}
		s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さんが退室しました🚶🚪"+
			"（+ "+strconv.Itoa(workedTimeSec/60)+"分、"+strconv.Itoa(seat.SeatId)+"番席）"+rpEarned)
		return nil
	})
}

func (s *System) ShowUserInfo(command CommandDetails, ctx context.Context) error {
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// そのユーザーはドキュメントがあるか？
		isUserRegistered, err := s.IfUserRegistered(ctx, tx)
		if err != nil {
			return err
		}
		if !isUserRegistered {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さんはまだ作業データがありません。「"+InCommand+"」コマンドで作業を始めましょう！")
			return nil
		}
		reply := ""
		totalStudyDuration, dailyTotalStudyDuration, err := s.RetrieveRealtimeTotalStudyDurations(ctx, tx, s.ProcessedUserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed s.RetrieveRealtimeTotalStudyDurations()", err)
			return err
		}
		totalTimeStr := utils.DurationToString(totalStudyDuration)
		dailyTotalTimeStr := utils.DurationToString(dailyTotalStudyDuration)
		reply += s.ProcessedUserDisplayName +
			"さん ［本日の作業時間：" + dailyTotalTimeStr + "］" +
			" ［累計作業時間：" + totalTimeStr + "］"
		
		userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed s.FirestoreController.RetrieveUser", err)
			return err
		}
		
		if userDoc.RankVisible {
			reply += "［ランクポイント：" + strconv.Itoa(userDoc.RankPoint) + " RP］"
		}
		
		if command.InfoOption.ShowDetails {
			switch userDoc.RankVisible {
			case true:
				reply += "［ランク表示：オン］"
			case false:
				reply += "［ランク表示：オフ］"
			}
			
			if userDoc.DefaultStudyMin == 0 {
				reply += "［デフォルト作業時間：なし］"
			} else {
				reply += "［デフォルト作業時間：" + strconv.Itoa(userDoc.DefaultStudyMin) + "分］"
			}
			
			if userDoc.FavoriteColor == "" {
				reply += "［お気に入りカラー：なし］"
			} else {
				reply += "［お気に入りカラー：" + userDoc.FavoriteColor + "］"
			}
			
			reply += "［登録日：" + userDoc.RegistrationDate.Format("2006年01月02日") + "］"
		}
		s.MessageToLiveChat(ctx, tx, reply)
		return nil
	})
}

func (s *System) ShowSeatInfo(_ CommandDetails, ctx context.Context) error {
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// そのユーザーは入室しているか？
		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return err
		}
		if isUserInRoom {
			currentSeat, err := s.CurrentSeat(ctx, s.ProcessedUserId)
			if err.IsNotNil() {
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
				_ = s.MessageToLineBotWithError("failed s.CurrentSeat()", err.Body)
			}
			
			realtimeSittingDurationMin := int(utils.NoNegativeDuration(utils.JstNow().Sub(currentSeat.EnteredAt)).Minutes())
			remainingMinutes := int(utils.NoNegativeDuration(currentSeat.Until.Sub(utils.JstNow())).Minutes())
			var stateStr string
			var breakUntilStr string
			switch currentSeat.State {
			case myfirestore.WorkState:
				stateStr = "作業中"
				breakUntilStr = ""
			case myfirestore.BreakState:
				stateStr = "休憩中"
				breakUntilDuration := utils.NoNegativeDuration(currentSeat.CurrentStateUntil.Sub(utils.JstNow()))
				breakUntilStr = "作業再開まで" + strconv.Itoa(int(breakUntilDuration.Minutes())) + "分です"
			}
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さんは"+strconv.Itoa(currentSeat.SeatId)+
				"番の席で"+stateStr+"です。現在"+strconv.Itoa(realtimeSittingDurationMin)+"分入室中。自動退室まで残り"+
				strconv.Itoa(remainingMinutes)+"分です。"+breakUntilStr)
		} else {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さんは入室していません。「"+InCommand+"」コマンドで入室しましょう！")
		}
		return nil
	})
}

func (s *System) Report(command CommandDetails, ctx context.Context) error {
	if command.ReportOption.Message == "" { // !reportのみは不可
		s.MessageToLiveChat(ctx, nil, s.ProcessedUserDisplayName+"さん、スペースを空けてメッセージを書いてください。")
		return nil
	}
	
	lineMessage := "【" + ReportCommand + "受信】\n" +
		"チャンネルID: " + s.ProcessedUserId + "\n" +
		"チャンネル名: " + s.ProcessedUserDisplayName + "\n\n" +
		command.ReportOption.Message
	err := s.MessageToLineBot(lineMessage)
	if err != nil {
		s.MessageToLiveChat(ctx, nil, s.ProcessedUserDisplayName+"さん、エラーが発生しました")
		log.Println(err)
	}
	
	discordMessage := "【" + ReportCommand + "受信】\n" +
		"チャンネル名: `" + s.ProcessedUserDisplayName + "`\n" +
		"メッセージ: `" + command.ReportOption.Message + "`"
	err = s.MessageToDiscordBot(discordMessage)
	if err != nil {
		_ = s.MessageToLineBotWithError("discordへメッセージが送信できませんでした: \""+discordMessage+"\"", err)
	}
	
	s.MessageToLiveChat(ctx, nil, s.ProcessedUserDisplayName+"さん、管理者へメッセージを送信しました⚠")
	return nil
}

func (s *System) Kick(command CommandDetails, ctx context.Context) error {
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// commanderはモデレーターかチャットオーナーか
		if !s.ProcessedUserIsModeratorOrOwner {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さんは「"+KickCommand+"」コマンドを使用できません")
			return nil
		}
		
		// ターゲットの座席は誰か使っているか
		isSeatAvailable, err := s.IfSeatVacant(ctx, tx, command.KickOption.SeatId)
		if err != nil {
			return err
		}
		if isSeatAvailable {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、その番号の座席は誰も使用していません")
			return nil
		}
		
		// ユーザーを強制退室させる
		seat, err := s.FirestoreController.RetrieveSeat(ctx, tx, command.KickOption.SeatId)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、その番号の座席は誰も使用していません")
				return nil
			}
			_ = s.MessageToLineBotWithError("failed to RetrieveSeat", err)
			return err
		}
		s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、"+strconv.Itoa(seat.SeatId)+"番席の"+seat.UserDisplayName+"さんを退室させます")
		
		// s.ProcessedUserが処理の対象ではないことに注意。
		userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, seat.UserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to RetrieveUser", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください")
			return err
		}
		
		workedTimeSec, addedRP, exitErr := s.exitRoom(tx, seat, &userDoc)
		if exitErr != nil {
			_ = s.MessageToLineBotWithError(s.ProcessedUserDisplayName+"さんのkick退室処理中にエラーが発生しました", exitErr)
			return exitErr
		}
		var rpEarned string
		if userDoc.RankVisible {
			rpEarned = "（+ " + strconv.Itoa(addedRP) + " RP）"
		}
		s.MessageToLiveChat(ctx, tx, seat.UserDisplayName+"さんが退室しました🚶🚪"+
			"（+ "+strconv.Itoa(workedTimeSec/60)+"分、"+strconv.Itoa(seat.SeatId)+"番席）"+rpEarned)
		
		err = s.MessageToDiscordBot(s.ProcessedUserDisplayName + "さん、" + strconv.Itoa(seat.
			SeatId) + "番席のユーザーをkickしました。\n" +
			"チャンネル名: " + seat.UserDisplayName + "\n" +
			"作業名: " + seat.WorkName + "\n休憩中の作業名: " + seat.BreakWorkName + "\n" +
			"入室時間: " + strconv.Itoa(workedTimeSec/60) + "分\n" +
			"チャンネルURL: https://youtube.com/channel/" + seat.UserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed MessageToDiscordBot()", err)
			return err
		}
		return nil
	})
}

func (s *System) Check(command CommandDetails, ctx context.Context) error {
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// commanderはモデレーターかチャットオーナーか
		if !s.ProcessedUserIsModeratorOrOwner {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さんは「"+CheckCommand+"」コマンドを使用できません")
			return nil
		}
		
		// ターゲットの座席は誰か使っているか
		isSeatAvailable, err := s.IfSeatVacant(ctx, tx, command.CheckOption.SeatId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to IfSeatVacant", err)
			return err
		}
		if isSeatAvailable {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、その番号の座席は誰も使用していません")
			return nil
		}
		// 座席情報を表示する
		seat, err := s.FirestoreController.RetrieveSeat(ctx, tx, command.CheckOption.SeatId)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、その番号の座席は誰も使用していません")
				return nil
			}
			_ = s.MessageToLineBotWithError("failed to RetrieveSeat", err)
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
			_ = s.MessageToLineBotWithError("failed MessageToDiscordBot()", err)
			return err
		}
		return nil
	})
}

func (s *System) My(command CommandDetails, ctx context.Context) error {
	// ユーザードキュメントはすでにあり、登録されていないプロパティだった場合、そのままプロパティを保存したら自動で作成される。
	// また、読み込みのときにそのプロパティがなくても大丈夫。自動で初期値が割り当てられる。
	// ただし、ユーザードキュメントがそもそもない場合は、書き込んでもエラーにはならないが、登録日が記録されないため、要登録。
	
	// 初回の利用の場合はユーザーデータを初期化
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		isRegistered, err := s.IfUserRegistered(ctx, tx)
		if err != nil {
			return err
		}
		if !isRegistered {
			err := s.InitializeUser(tx)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	
	// オプションが1つ以上指定されているか？
	if len(command.MyOptions) == 0 {
		s.MessageToLiveChat(ctx, nil, s.ProcessedUserDisplayName+"さん、オプションが正しく設定されているか確認してください")
		return nil
	}
	
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 変更前のuserDocを読み込んでおく
		userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to RetrieveUser", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください")
			return err
		}
		
		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to IsUserInRoom", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください")
			return err
		}
		var seats []myfirestore.SeatDoc
		var realTimeTotalStudySec int
		if isUserInRoom {
			seats, err = s.FirestoreController.RetrieveSeats(ctx)
			if err != nil {
				_ = s.MessageToLineBotWithError("failed to CurrentSeat", err)
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
					"さん、エラーが発生しました。もう一度試してみてください")
				return err
			}
			
			realTimeTotalStudyDuration, _, err := s.RetrieveRealtimeTotalStudyDurations(ctx, tx, s.ProcessedUserId)
			if err != nil {
				_ = s.MessageToLineBotWithError("failed to RetrieveRealtimeTotalStudyDuration", err)
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
					"さん、エラーが発生しました。もう一度試してみてください")
				return err
			}
			realTimeTotalStudySec = int(realTimeTotalStudyDuration.Seconds())
		}
		
		// これ以降は書き込みのみ
		
		reply := s.ProcessedUserDisplayName + "さん、"
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
					reply += "ランク表示モードはすでに" + rankVisibleString + "です。"
				} else { // 違うなら、切替
					err := s.FirestoreController.SetMyRankVisible(tx, s.ProcessedUserId, newRankVisible)
					if err != nil {
						_ = s.MessageToLineBotWithError("failed to SetMyRankVisible", err)
						s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
							"さん、エラーが発生しました。もう一度試してみてください")
						return err
					}
					var newValueString string
					if newRankVisible {
						newValueString = "オン"
					} else {
						newValueString = "オフ"
					}
					reply += "ランク表示を" + newValueString + "にしました。"
					
					// 入室中であれば、座席の色も変える
					if isUserInRoom {
						seatAppearance := utils.GetSeatAppearance(realTimeTotalStudySec, newRankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
						// 席の色を更新
						newSeat, err := GetSeatByUserId(seats, s.ProcessedUserId)
						if err != nil {
							return err
						}
						newSeat.Appearance = seatAppearance
						err = s.FirestoreController.UpdateSeat(tx, newSeat)
						if err != nil {
							_ = s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeats()", err)
							s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してください")
							return err
						}
					}
				}
				currenRankVisible = newRankVisible
			} else if myOption.Type == DefaultStudyMin {
				err := s.FirestoreController.SetMyDefaultStudyMin(tx, s.ProcessedUserId, myOption.IntValue)
				if err != nil {
					_ = s.MessageToLineBotWithError("failed to SetMyDefaultStudyMin", err)
					s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
						"さん、エラーが発生しました。もう一度試してみてください")
					return err
				}
				// 値が0はリセットのこと。
				if myOption.IntValue == 0 {
					reply += "デフォルトの作業時間をリセットしました。"
				} else {
					reply += "デフォルトの作業時間を" + strconv.Itoa(myOption.IntValue) + "分に設定しました。"
				}
			} else if myOption.Type == FavoriteColor {
				colorCode := utils.TotalStudyHoursToColorCode(myOption.IntValue)
				err := s.FirestoreController.SetMyFavoriteColor(tx, s.ProcessedUserId, colorCode)
				if err != nil {
					_ = s.MessageToLineBotWithError("failed to SetMyFavoriteColor", err)
					s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
					return err
				}
				reply += "お気に入りカラーを更新しました。"
				if !utils.CanUseFavoriteColor(realTimeTotalStudySec) {
					reply += "（累計作業時間が" + strconv.Itoa(utils.FavoriteColorAvailableThresholdHours) + "時間を超えるまでお気に入りカラーは使えません）"
				}
				
				// 入室中であれば、座席の色も変える
				if isUserInRoom {
					newSeat, err := GetSeatByUserId(seats, s.ProcessedUserId)
					if err != nil {
						return err
					}
					seatAppearance := utils.GetSeatAppearance(realTimeTotalStudySec, currenRankVisible, userDoc.RankPoint, colorCode)
					// 席の色を更新
					newSeat.Appearance = seatAppearance
					err = s.FirestoreController.UpdateSeat(tx, newSeat)
					if err != nil {
						_ = s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeats()", err)
						s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してください")
						return err
					}
				}
			}
		}
		s.MessageToLiveChat(ctx, tx, reply)
		return nil
	})
}

func (s *System) Change(command CommandDetails, ctx context.Context) error {
	changeOption := &command.ChangeOption
	jstNow := utils.JstNow()
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// そのユーザーは入室中か？
		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to IsUserInRoom()", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました")
			return err
		}
		if !isUserInRoom {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、入室中のみ使えるコマンドです")
			return nil
		}
		
		currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId)
		if cerr.IsNotNil() {
			_ = s.MessageToLineBotWithError("failed to s.CurrentSeat(ctx)", cerr.Body)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください")
			return cerr.Body
		}
		
		// validation
		cerr = s.ValidateChange(command, currentSeat.State)
		if cerr.IsNotNil() {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、"+cerr.Body.Error())
			return nil
		}
		
		// これ以降は書き込みのみ可。
		newSeat := &currentSeat
		
		reply := s.ProcessedUserDisplayName + "さん、"
		if changeOption.IsWorkNameSet {
			// 作業名もしくは休憩作業名を書きかえ
			switch currentSeat.State {
			case myfirestore.WorkState:
				newSeat.WorkName = changeOption.WorkName
				reply += "作業内容を更新しました（" + strconv.Itoa(currentSeat.SeatId) + "番席）。"
			case myfirestore.BreakState:
				newSeat.BreakWorkName = changeOption.WorkName
				reply += "休憩内容を更新しました（" + strconv.Itoa(currentSeat.SeatId) + "番席）。"
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
					reply += "すでに" + strconv.Itoa(changeOption.DurationMin) + "分以上入室しています。現在" + utils.Ftoa(realtimeEntryDurationMin) +
						"分入室中。自動退室まで残り" + utils.Ftoa(remainingWorkMin) + "分です"
				} else if requestedUntil.After(jstNow.Add(time.Duration(s.Configs.Constants.MaxWorkTimeMin) * time.Minute)) {
					// もし現在時刻より最大延長可能時間以上後なら却下
					remainingWorkMin := currentSeat.Until.Sub(jstNow).Minutes()
					reply += "自動退室までの時間は現在時刻から" + strconv.Itoa(s.Configs.Constants.MaxWorkTimeMin) + "分後まで設定できます。現在" +
						utils.Ftoa(realtimeEntryDurationMin) + "分入室中。自動退室まで残り" + utils.Ftoa(remainingWorkMin) + "分です"
				} else { // それ以外なら延長
					newSeat.Until = requestedUntil
					newSeat.CurrentStateUntil = requestedUntil
					remainingWorkMin := utils.NoNegativeDuration(requestedUntil.Sub(jstNow)).Minutes()
					reply += "入室時間を" + strconv.Itoa(changeOption.DurationMin) + "分に変更しました。現在" + utils.Ftoa(realtimeEntryDurationMin) +
						"分入室中。自動退室まで残り" + utils.Ftoa(remainingWorkMin) + "分です。"
				}
			case myfirestore.BreakState:
				// 休憩時間を変更
				realtimeBreakDuration := utils.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt))
				requestedUntil := currentSeat.CurrentStateStartedAt.Add(time.Duration(changeOption.DurationMin) * time.Minute)
				
				if requestedUntil.Before(jstNow) {
					// もし現在時刻が指定時間を経過していたら却下
					remainingBreakDuration := currentSeat.CurrentStateUntil.Sub(jstNow)
					reply += "すでに" + strconv.Itoa(changeOption.DurationMin) + "分以上休憩しています。現在" + utils.Ftoa(realtimeBreakDuration.Minutes()) +
						"分休憩中。作業再開まで残り" + utils.Ftoa(remainingBreakDuration.Minutes()) + "分です"
				} else { // それ以外ならuntilを変更
					newSeat.CurrentStateUntil = requestedUntil
					remainingBreakDuration := requestedUntil.Sub(jstNow)
					reply += "休憩時間を" + strconv.Itoa(changeOption.DurationMin) + "分に変更しました。現在" + utils.Ftoa(realtimeBreakDuration.Minutes()) +
						"分休憩中。作業再開まで残り" + utils.Ftoa(remainingBreakDuration.Minutes()) + "分です。"
				}
			}
		}
		err = s.FirestoreController.UpdateSeat(tx, *newSeat)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to UpdateSeats", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
			return err
		}
		
		s.MessageToLiveChat(ctx, tx, reply)
		return nil
	})
}

func (s *System) More(command CommandDetails, ctx context.Context) error {
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to IsUserInRoom()", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました")
			return err
		}
		if !isUserInRoom {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、入室中のみ使えるコマンドです")
			return nil
		}
		
		currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId)
		if cerr.IsNotNil() {
			_ = s.MessageToLineBotWithError("failed to s.CurrentSeat(ctx)", cerr.Body)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください")
			return cerr.Body
		}
		
		// 以降書き込みのみ
		newSeat := &currentSeat
		
		replyMessage := s.ProcessedUserDisplayName + "さん、"
		var addedMin int
		var remainingUntilExitMin int
		
		switch currentSeat.State {
		case myfirestore.WorkState:
			// 作業時間を指定分延長する
			newUntil := currentSeat.Until.Add(time.Duration(command.MoreOption.DurationMin) * time.Minute)
			// もし延長後の時間が最大作業時間を超えていたら、最大作業時間まで延長
			if int(utils.NoNegativeDuration(newUntil.Sub(utils.JstNow())).Minutes()) > s.Configs.Constants.MaxWorkTimeMin {
				newUntil = utils.JstNow().Add(time.Duration(s.Configs.Constants.MaxWorkTimeMin) * time.Minute)
				replyMessage += "現在時刻から" + strconv.Itoa(s.Configs.Constants.
					MaxWorkTimeMin) + "分後までのみ作業時間を延長可能です。延長できる最大の時間で設定します。"
			}
			addedMin = int(utils.NoNegativeDuration(newUntil.Sub(currentSeat.Until)).Minutes())
			newSeat.Until = newUntil
			newSeat.CurrentStateUntil = newUntil
			remainingUntilExitMin = int(utils.NoNegativeDuration(newUntil.Sub(utils.JstNow())).Minutes())
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
				remainingUntilExitMin = int(utils.NoNegativeDuration(newUntil.Sub(utils.JstNow())).Minutes())
			} else {
				remainingUntilExitMin = int(utils.NoNegativeDuration(currentSeat.Until.Sub(utils.JstNow())).Minutes())
			}
		}
		
		err = s.FirestoreController.UpdateSeat(tx, *newSeat)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeats", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください")
			return err
		}
		
		switch currentSeat.State {
		case myfirestore.WorkState:
			replyMessage += "自動退室までの時間を" + strconv.Itoa(addedMin) + "分延長しました。"
		case myfirestore.BreakState:
			replyMessage += "休憩時間を" + strconv.Itoa(addedMin) + "分延長しました。"
			remainingBreakDuration := utils.NoNegativeDuration(newSeat.CurrentStateUntil.Sub(utils.JstNow()))
			replyMessage += "作業再開まで残り" + utils.Ftoa(remainingBreakDuration.Minutes()) + "分。"
		}
		realtimeEnteredTimeMin := utils.NoNegativeDuration(utils.JstNow().Sub(currentSeat.EnteredAt)).Minutes()
		replyMessage += "現在" + utils.Ftoa(realtimeEnteredTimeMin) + "分入室中。自動退室まで残り" + strconv.Itoa(remainingUntilExitMin) + "分です"
		s.MessageToLiveChat(ctx, tx, replyMessage)
		
		return nil
	})
}

func (s *System) Break(ctx context.Context, command CommandDetails) error {
	breakOption := &command.BreakOption
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return err
		}
		if !isUserInRoom {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、入室中のみ使えるコマンドです")
			return nil
		}
		
		// stateを確認
		currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId)
		if cerr.IsNotNil() {
			_ = s.MessageToLineBotWithError("failed to CurrentSeat()", cerr.Body)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
			return cerr.Body
		}
		if currentSeat.State != myfirestore.WorkState {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、作業中のみ使えるコマンドです。")
			return nil
		}
		
		// 前回の入室または再開から、最低休憩間隔経っているか？
		currentWorkedMin := utils.NoNegativeDuration(utils.JstNow().Sub(currentSeat.CurrentStateStartedAt)).Minutes()
		if int(currentWorkedMin) < s.Configs.Constants.MinBreakIntervalMin {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、作業を始めてから"+strconv.Itoa(s.Configs.Constants.
				MinBreakIntervalMin)+"分間は休憩できません。現在"+utils.Ftoa(currentWorkedMin)+"分作業中")
			return nil
		}
		
		// オプション確認
		if !breakOption.IsDurationMinSet {
			breakOption.DurationMin = s.Configs.Constants.DefaultBreakDurationMin
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
			_ = s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeats", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
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
			_ = s.MessageToLineBotWithError("failed to add an user activity", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
			return err
		}
		
		s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さんが休憩します（最大"+strconv.Itoa(breakOption.DurationMin)+"分、"+
			strconv.Itoa(currentSeat.SeatId)+"番席）")
		
		return nil
	})
}

func (s *System) Resume(ctx context.Context, command CommandDetails) error {
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return err
		}
		if !isUserInRoom {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、入室中のみ使えるコマンドです")
			return nil
		}
		
		// stateを確認
		currentSeat, cerr := s.CurrentSeat(ctx, s.ProcessedUserId)
		if cerr.IsNotNil() {
			_ = s.MessageToLineBotWithError("failed to CurrentSeat()", cerr.Body)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
			return cerr.Body
		}
		if currentSeat.State != myfirestore.BreakState {
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、座席で休憩中のみ使えるコマンドです。")
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
		if command.ResumeOption.WorkName == "" {
			workName = currentSeat.WorkName
		}
		
		currentSeat.State = myfirestore.WorkState
		currentSeat.CurrentStateStartedAt = jstNow
		currentSeat.CurrentStateUntil = until
		currentSeat.DailyCumulativeWorkSec = dailyCumulativeWorkSec
		currentSeat.WorkName = workName
		
		err = s.FirestoreController.UpdateSeat(tx, currentSeat)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeats", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください")
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
			_ = s.MessageToLineBotWithError("failed to add an user activity", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください")
			return err
		}
		
		untilExitDuration := utils.NoNegativeDuration(until.Sub(jstNow))
		s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さんが作業を再開します（"+strconv.Itoa(currentSeat.SeatId)+"番席、自動退室まで"+
			utils.Ftoa(untilExitDuration.Minutes())+"分）")
		
		return nil
	})
}

func (s *System) Rank(_ CommandDetails, ctx context.Context) error {
	// 初回の利用の場合はユーザーデータを初期化
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		isRegistered, err := s.IfUserRegistered(ctx, tx)
		if err != nil {
			return err
		}
		if !isRegistered {
			err := s.InitializeUser(tx)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 変更前のuserDocを読み込んでおく
		userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to RetrieveUser", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください")
			return err
		}
		
		isUserInRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to IsUserInRoom", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください")
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
				_ = s.MessageToLineBotWithError("failed to RetrieveRealtimeTotalStudyDuration", err)
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
					"さん、エラーが発生しました。もう一度試してみてください")
				return err
			}
			realtimeTotalStudySec = int(realtimeTotalStudyDuration.Seconds())
		}
		
		// 以降書き込みのみ
		
		// ランク表示設定のON/OFFを切り替える
		newRankVisible := !userDoc.RankVisible
		err = s.FirestoreController.SetMyRankVisible(tx, s.ProcessedUserId, newRankVisible)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to SetMyRankVisible", err)
			s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください")
			return err
		}
		var newValueString string
		if newRankVisible {
			newValueString = "オン"
		} else {
			newValueString = "オフ"
		}
		s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さんのランク表示を"+newValueString+"にしました")
		
		// 入室中であれば、座席の色も変える
		if isUserInRoom {
			seatAppearance := utils.GetSeatAppearance(realtimeTotalStudySec, newRankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
			// 席の色を更新
			currentSeat.Appearance = seatAppearance
			err := s.FirestoreController.UpdateSeat(tx, currentSeat)
			if err != nil {
				_ = s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeats()", err)
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してください")
				return err
			}
		}
		
		return nil
	})
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
		_ = s.MessageToLineBotWithError("failed to RetrieveSeat", err)
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
	workMin int,
	seatAppearance myfirestore.SeatAppearance,
	state myfirestore.SeatState,
) error {
	enterDate := utils.JstNow()
	exitDate := enterDate.Add(time.Duration(workMin) * time.Minute)
	
	newSeat := myfirestore.SeatDoc{
		SeatId:                 seatId,
		UserId:                 userId,
		UserDisplayName:        userDisplayName,
		WorkName:               workName,
		EnteredAt:              enterDate,
		Until:                  exitDate,
		Appearance:             seatAppearance,
		State:                  state,
		CurrentStateStartedAt:  enterDate,
		CurrentStateUntil:      exitDate,
		CumulativeWorkSec:      0,
		DailyCumulativeWorkSec: 0,
	}
	err := s.FirestoreController.AddSeat(tx, newSeat)
	if err != nil {
		return err
	}
	
	// 入室時刻を記録
	err = s.FirestoreController.SetLastEnteredDate(tx, userId, enterDate)
	if err != nil {
		_ = s.MessageToLineBotWithError("failed to set last entered date", err)
		return err
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
		_ = s.MessageToLineBotWithError("failed to add an user activity", err)
		return err
	}
	return nil
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
		_ = s.MessageToLineBotWithError("failed to add an user activity", err)
	}
	// 退室時刻を記録
	err = s.FirestoreController.SetLastExitedDate(tx, previousSeat.UserId, exitDate)
	if err != nil {
		_ = s.MessageToLineBotWithError("failed to update last-exited-date", err)
		return 0, 0, err
	}
	// 累計作業時間を更新
	err = s.UpdateTotalWorkTime(tx, previousSeat.UserId, previousUserDoc, addedWorkedTimeSec, addedDailyWorkedTimeSec)
	if err != nil {
		_ = s.MessageToLineBotWithError("failed to update total study time", err)
		return 0, 0, err
	}
	// RP更新
	netStudyDuration := time.Duration(addedWorkedTimeSec) * time.Second
	newRP, err := utils.CalcNewRPExitRoom(netStudyDuration, previousSeat.WorkName != "", previousUserDoc.IsContinuousActive, previousUserDoc.CurrentActivityStateStarted, exitDate, previousUserDoc.RankPoint)
	if err != nil {
		_ = s.MessageToLineBotWithError("failed to CalcNewRPExitRoom", err)
		return 0, 0, err
	}
	err = s.FirestoreController.UpdateUserRankPoint(tx, previousSeat.UserId, newRP)
	if err != nil {
		_ = s.MessageToLineBotWithError("failed to UpdateUserRankPoint", err)
		return 0, 0, err
	}
	addedRP := newRP - previousUserDoc.RankPoint
	
	log.Println(previousSeat.UserId + " exited the room. seat id: " + strconv.Itoa(previousSeat.SeatId) + " (+ " + strconv.Itoa(addedWorkedTimeSec) + "秒)")
	log.Println("addedRP: " + strconv.Itoa(addedRP) + ", newRP: " + strconv.Itoa(newRP) + ", previous RP: " + strconv.Itoa(previousUserDoc.RankPoint))
	return addedWorkedTimeSec, addedRP, nil
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
		_ = s.MessageToLineBot(userId + ": " + message)
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
		_ = s.MessageToLineBotWithError("failed to IsUserInRoom", err)
		return 0, 0, err
	}
	if isInRoom {
		// 作業時間を計算
		currentSeat, cerr := s.CurrentSeat(ctx, userId)
		if cerr.IsNotNil() {
			_ = s.MessageToLineBotWithError("failed to CurrentSeat", cerr.Body)
			return 0, 0, cerr.Body
		}
		
		var err error
		realtimeDuration, err = RealTimeTotalStudyDurationOfSeat(currentSeat)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to RealTimeTotalStudyDurationOfSeat", err)
			return 0, 0, err
		}
		realtimeDailyDuration, err = RealTimeDailyTotalStudyDurationOfSeat(currentSeat)
		if err != nil {
			_ = s.MessageToLineBotWithError("failed to RealTimeDailyTotalStudyDurationOfSeat", err)
			return 0, 0, err
		}
	}
	
	userData, err := s.FirestoreController.RetrieveUser(ctx, tx, userId)
	if err != nil {
		_ = s.MessageToLineBotWithError("failed to RetrieveUser", err)
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
				_ = s.MessageToLineBotWithError("failed to RetrieveSeats", err)
				return err
			}
			if len(seats) > 0 {
				seat := seats[0]
				s.SetProcessedUser(seat.UserId, seat.UserDisplayName, false, false)
				userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
				if err != nil {
					_ = s.MessageToLineBotWithError("failed to RetrieveUser", err)
					return err
				}
				_, _, err = s.exitRoom(tx, seat, &userDoc)
				if err != nil {
					_ = s.MessageToLineBotWithError("failed to exitRoom", err)
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

func (s *System) MessageToLiveChat(ctx context.Context, tx *firestore.Transaction, message string) {
	err := s.liveChatBot.PostMessage(ctx, tx, message)
	if err != nil {
		_ = s.MessageToLineBotWithError("failed to send live chat message \""+message+"\"\n", err)
	}
	return
}

func (s *System) MessageToLineBot(message string) error {
	return s.lineBot.SendMessage(message)
}

func (s *System) MessageToLineBotWithError(message string, err error) error {
	return s.lineBot.SendMessageWithError(message, err)
}

func (s *System) MessageToDiscordBot(message string) error {
	return s.discordBot.SendMessage(message)
}

// OrganizeDatabase untilを過ぎているルーム内のユーザーを退室させる。長時間入室しているユーザーを席移動させる。
func (s *System) OrganizeDatabase(ctx context.Context) error {
	// 長時間入室制限のチェックを行うかどうか
	ifCheckLongTimeSitting := int(utils.NoNegativeDuration(utils.JstNow().Sub(s.Configs.Constants.LastLongTimeSittingChecked)).Minutes()) > s.
		Configs.Constants.CheckLongTimeSittingIntervalMinutes
	
	// 一旦全座席のスナップショットをとる（トランザクションなし）
	seatsSnapshot, err := s.FirestoreController.RetrieveSeats(ctx)
	if err != nil {
		_ = s.MessageToLineBotWithError("failed to RetrieveSeats", err)
		return err
	}
	
	// スナップショットの各座席についてトランザクション処理
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
				_ = s.MessageToLineBotWithError("failed to RetrieveSeat", err)
				return err
			}
			if !reflect.DeepEqual(seat, seatSnapshot) {
				log.Println("その座席に少しでも変更が加えられているのでスルー")
				return nil
			}
			
			userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, s.ProcessedUserId)
			if err != nil {
				_ = s.MessageToLineBotWithError("failed to RetrieveUser", err)
				return err
			}
			
			var autoExit bool // 自動退室時刻による自動退室
			var resume bool   // 作業再開
			
			if seat.Until.Before(utils.JstNow()) { // 自動退室時刻を過ぎていたら自動退室
				autoExit = true
			} else if ifCheckLongTimeSitting { // 長時間入室制限に引っかかっていたら強制席移動
				ifNotSittingTooMuch, err := s.CheckSeatAvailabilityForUser(ctx, s.ProcessedUserId, seat.SeatId)
				if err != nil {
					_ = s.MessageToLineBotWithError(s.ProcessedUserDisplayName+"さん（"+s.ProcessedUserId+"）の席移動処理中にエラーが発生しました", err)
					return err
				}
				if !ifNotSittingTooMuch {
					forcedMove = true
				}
			}
			if seat.State == myfirestore.BreakState && seat.CurrentStateUntil.Before(utils.JstNow()) {
				resume = true
			}
			
			// 以下書き込みのみ
			
			// 自動退室時刻による退室処理
			if autoExit {
				workedTimeSec, addedRP, err := s.exitRoom(tx, seat, &userDoc)
				if err != nil {
					_ = s.MessageToLineBotWithError(s.ProcessedUserDisplayName+"さん（"+s.ProcessedUserId+"）の退室処理中にエラーが発生しました", err)
					return err
				}
				var rpEarned string
				if userDoc.RankVisible {
					rpEarned = "（+ " + strconv.Itoa(addedRP) + " RP）"
				}
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さんが退室しました🚶🚪"+
					"（+ "+strconv.Itoa(workedTimeSec/60)+"分、"+strconv.Itoa(seat.SeatId)+"番席）"+rpEarned)
			} else if forcedMove { // 長時間入室制限による強制席移動
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さんが"+strconv.Itoa(seat.SeatId)+"番席の入室時間の一時上限に達したため席移動します💨")
				// nested transactionとならないよう、RunTransactionの外側で実行
			} else if resume { // 作業再開処理
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
					_ = s.MessageToLineBotWithError("failed to s.FirestoreController.UpdateSeat", err)
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
					_ = s.MessageToLineBotWithError("failed to add an user activity", err)
					return err
				}
				s.MessageToLiveChat(ctx, tx, s.ProcessedUserDisplayName+"さんが作業を再開します（自動退室まで"+
					utils.Ftoa(utils.NoNegativeDuration(until.Sub(jstNow)).Minutes())+"分）")
			}
			
			return nil
		})
		if err != nil {
			return err
		}
		if forcedMove {
			inCommandDetails := CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      0,
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
				_ = s.MessageToLineBotWithError(s.ProcessedUserDisplayName+"さん（"+s.ProcessedUserId+"）の自動席移動処理中にエラーが発生しました", err)
				return err
			}
		}
	}
	if ifCheckLongTimeSitting {
		err = s.FirestoreController.SetLastLongTimeSittingChecked(ctx, utils.JstNow())
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *System) CheckLiveStreamStatus(ctx context.Context) error {
	checker := guardians.NewLiveStreamChecker(s.FirestoreController, s.liveChatBot, s.lineBot)
	return checker.Check(ctx)
}

func (s *System) DailyOrganizeDatabase(ctx context.Context) error {
	log.Println("DailyOrganizeDatabase()")
	// 時間がかかる処理なのでトランザクションはなし
	
	// 一時的累計作業時間をリセット
	err := s.ResetDailyTotalStudyTime(ctx)
	if err != nil {
		_ = s.MessageToLineBotWithError("failed to ResetDailyTotalStudyTime", err)
		return err
	}
	
	// RP関連の情報更新・ペナルティ処理
	err = s.UpdateRankProcess(ctx)
	if err != nil {
		_ = s.MessageToLineBotWithError("failed to UpdateRankProcess", err)
		return err
	}
	
	_ = s.MessageToLineBot("本日のDailyOrganizeDatabase()処理が完了しました。")
	log.Println("finished DailyOrganizeDatabase().")
	return nil
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
		_ = s.MessageToLineBot("successfully reset all non-daily-zero user's daily total study time. (" + strconv.Itoa(count) + " users)")
		err := s.FirestoreController.SetLastResetDailyTotalStudyTime(ctx, now)
		if err != nil {
			return err
		}
	} else {
		_ = s.MessageToLineBot("all user's daily total study times are already reset today.")
	}
	return nil
}

func (s *System) UpdateRankProcess(ctx context.Context) error {
	log.Println("UpdateRankProcess()")
	jstNow := utils.JstNow()
	// 過去31日以内に入室したことのあるユーザーをクエリ
	_31daysAgo := jstNow.AddDate(0, 0, -31)
	iter := s.FirestoreController.RetrieveUsersActiveAfterDate(ctx, _31daysAgo)
	
	count := 0
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		count++
		userId := doc.Ref.ID
		err = s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			userDoc, err := s.FirestoreController.RetrieveUser(ctx, tx, userId)
			if err != nil {
				_ = s.MessageToLineBotWithError("failed to RetrieveUser", err)
				return err
			}
			
			// 変更する可能性のある項目
			lastPenaltyImposedDays := userDoc.LastPenaltyImposedDays
			isContinuousActive := userDoc.IsContinuousActive
			currentActivityStateStarted := userDoc.CurrentActivityStateStarted
			rankPoint := userDoc.RankPoint
			
			// アクティブ・非アクティブ状態の更新
			wasUserActiveYesterday := utils.WasUserActiveFromYesterday(userDoc.LastEntered, userDoc.LastExited, jstNow)
			if wasUserActiveYesterday {
				if userDoc.IsContinuousActive {
					// pass
				} else {
					isContinuousActive = true
					currentActivityStateStarted = jstNow
					// 連続非アクティブが終わったのでlastPenaltyImposedDaysは0にリセット
					lastPenaltyImposedDays = 0
				}
			} else {
				if userDoc.IsContinuousActive {
					isContinuousActive = false
					currentActivityStateStarted = jstNow.AddDate(0, 0, -1)
				} else {
					// pass
				}
			}
			
			// lastExitedが一定日数以上前のユーザーはRPペナルティ処理
			lastActiveAt := utils.LastActiveAt(userDoc.LastEntered, userDoc.LastExited, jstNow)
			rankPoint, lastPenaltyImposedDays, err = utils.CalcNewRPContinuousInactivity(rankPoint, lastActiveAt, lastPenaltyImposedDays)
			if err != nil {
				_ = s.MessageToLineBotWithError("failed to CalcNewRPContinuousInactivity", err)
				return err
			}
			
			// 変更項目がある場合のみ変更
			if lastPenaltyImposedDays != userDoc.LastPenaltyImposedDays {
				err := s.FirestoreController.UpdateUserLastPenaltyImposedDays(tx, userId, lastPenaltyImposedDays)
				if err != nil {
					_ = s.MessageToLineBotWithError("failed to UpdateUserLastPenaltyImposedDays", err)
					return err
				}
			}
			if isContinuousActive != userDoc.IsContinuousActive || currentActivityStateStarted != userDoc.CurrentActivityStateStarted {
				err := s.FirestoreController.UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(tx, userId, isContinuousActive, currentActivityStateStarted)
				if err != nil {
					_ = s.MessageToLineBotWithError("failed to UpdateUserIsContinuousActiveAndCurrentActivityStateStarted", err)
					return err
				}
			}
			if rankPoint != userDoc.RankPoint {
				err := s.FirestoreController.UpdateUserRankPoint(tx, userId, rankPoint)
				if err != nil {
					_ = s.MessageToLineBotWithError("failed to UpdateUserRankPoint", err)
					return err
				}
			}
			
			return nil
		})
		if err != nil {
			return err
		}
	}
	_ = s.MessageToLineBot("過去31日以内に入室した人数: " + strconv.Itoa(count))
	log.Println("finished UpdateRankProcess()")
	return nil
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
		_ = s.MessageToLineBot("successfully transfer yesterday's live chat history to bigquery.")
		
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
		_ = s.MessageToLineBot(strconv.Itoa(int(retentionFromDate.Month())) + "月" + strconv.Itoa(retentionFromDate.Day()) +
			"日より前の日付のライブチャット履歴およびユーザー行動ログをFirestoreから削除しました。")
		
		err = s.FirestoreController.SetLastTransferCollectionHistoryBigquery(ctx, now)
		if err != nil {
			return err
		}
	} else {
		_ = s.MessageToLineBot("yesterday's collection histories are already reset today.")
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
	var lastActivityType myfirestore.UserActivityType
	for i, activity := range activityOnlyEnterExitList {
		if i == 0 {
			lastActivityType = activity.ActivityType
			continue
		}
		if activity.ActivityType == lastActivityType {
			return false, errors.New("入室activityと退室activityが交互に並んでいない")
		}
		lastActivityType = activity.ActivityType
	}
	
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
	
	// 制限値と比較し、結果を返す
	return int(totalEntryDuration.Minutes()) < s.Configs.Constants.RecentThresholdMin, nil
}
