package core

import (
	"app.modules/core/customerror"
	"app.modules/core/guardians"
	"app.modules/core/myfirestore"
	"app.modules/core/mylinebot"
	"app.modules/core/utils"
	"app.modules/core/youtubebot"
	"context"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"strconv"
	"strings"
	"time"
)

func NewSystem(ctx context.Context, clientOption option.ClientOption) (System, error) {
	fsController, err := myfirestore.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return System{}, err
	}

	// youtube live chat bot
	youtubeLiveConfig, err := fsController.RetrieveYoutubeLiveConfig(ctx)
	if err != nil {
		return System{}, err
	}
	liveChatBot, err := youtubebot.NewYoutubeLiveChatBot(youtubeLiveConfig.LiveChatId, fsController, ctx)
	if err != nil {
		return System{}, err
	}

	// line bot
	lineBotConfig, err := fsController.RetrieveLineBotConfig(ctx)
	if err != nil {
		return System{}, err
	}
	lineBot, err := mylinebot.NewLineBot(lineBotConfig.ChannelSecret, lineBotConfig.ChannelToken, lineBotConfig.DestinationLineId)
	if err != nil {
		return System{}, err
	}

	// core constant values
	constantsConfig, err := fsController.RetrieveSystemConstantsConfig(ctx)
	if err != nil {
		return System{}, err
	}

	return System{
		FirestoreController:       fsController,
		LiveChatBot:               liveChatBot,
		LineBot:                   lineBot,
		MaxWorkTimeMin:            constantsConfig.MaxWorkTimeMin,
		MinWorkTimeMin:            constantsConfig.MinWorkTimeMin,
		DefaultWorkTimeMin:        constantsConfig.DefaultWorkTimeMin,
		DefaultSleepIntervalMilli: constantsConfig.SleepIntervalMilli,
	}, nil
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

// Command 入力コマンドを解析して実行
func (s *System) Command(commandString string, userId string, userDisplayName string, isChatModerator bool, isChatOwner bool, ctx context.Context) customerror.CustomError {
	s.SetProcessedUser(userId, userDisplayName, isChatModerator, isChatOwner)

	commandDetails, err := s.ParseCommand(commandString)
	if err.IsNotNil() { // これはシステム内部のエラーではなく、コマンドが悪いということなので、return nil
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、"+err.Body.Error(), ctx)
		return customerror.NewNil()
	}
	//log.Printf("parsed command: %# v\n", pretty.Formatter(commandDetails))

	// commandDetailsに基づいて命令処理
	switch commandDetails.CommandType {
	case NotCommand:
		return customerror.NewNil()
	case InvalidCommand:
		// 暫定で何も反応しない
		return customerror.NewNil()
	case In:
		err := s.In(commandDetails, ctx)
		if err != nil {
			return customerror.InProcessFailed.New(err.Error())
		}
		return customerror.NewNil()
	case Out:
		err := s.Out(commandDetails, ctx)
		if err != nil {
			return customerror.OutProcessFailed.New(err.Error())
		}
		return customerror.NewNil()
	case Info:
		err := s.ShowUserInfo(commandDetails, ctx)
		if err != nil {
			return customerror.InfoProcessFailed.New(err.Error())
		}
		return customerror.NewNil()
	case My:
		err := s.My(commandDetails, ctx)
		if err != nil {
			return customerror.MyProcessFailed.New(err.Error())
		}
		return customerror.NewNil()
	case Change:
		err := s.Change(commandDetails, ctx)
		if err != nil {
			return customerror.ChangeProcessFailed.New(err.Error())
		}
	case Seat:
		err := s.ShowSeatInfo(commandDetails, ctx)
		if err != nil {
			return customerror.SeatProcessFailed.New(err.Error())
		}
		return customerror.NewNil()
	case Report:
		err := s.Report(commandDetails, ctx)
		if err != nil {
			return customerror.ReportProcessFailed.New(err.Error())
		}
		return customerror.NewNil()
	case Kick:
		err := s.Kick(commandDetails, ctx)
		if err != nil {
			return customerror.KickProcessFailed.New(err.Error())
		}
		return customerror.NewNil()
	case Add:
		err := s.Add(commandDetails, ctx)
		if err != nil {
			return customerror.AddProcessFailed.New(err.Error())
		}
		return customerror.NewNil()
	case Rank:
		err := s.Rank(commandDetails, ctx)
		if err != nil {
			return customerror.RankProcessFailed.New(err.Error())
		}
		return customerror.NewNil()
	default:
		_ = s.LineBot.SendMessage("Unknown command: " + commandString)
	}
	return customerror.NewNil()
}

// ParseCommand コマンドを解析
func (s *System) ParseCommand(commandString string) (CommandDetails, customerror.CustomError) {
	// 全角スペースを半角に変換
	commandString = strings.Replace(commandString, FullWidthSpace, HalfWidthSpace, -1)
	// 全角イコールを半角に変換
	commandString = strings.Replace(commandString, "＝", "=", -1)

	if strings.HasPrefix(commandString, CommandPrefix) {
		slice := strings.Split(commandString, HalfWidthSpace)
		switch slice[0] {
		case InCommand:
			commandDetails, err := s.ParseIn(commandString)
			if err.IsNotNil() {
				return CommandDetails{}, err
			}
			return commandDetails, customerror.NewNil()
		case OutCommand:
			return CommandDetails{
				CommandType: Out,
				InOptions:   InOptions{},
			}, customerror.NewNil()
		case InfoCommand:
			commandDetails, err := s.ParseInfo(commandString)
			if err.IsNotNil() {
				return CommandDetails{}, err
			}
			return commandDetails, customerror.NewNil()
		case MyCommand:
			commandDetails, err := s.ParseMy(commandString)
			if err.IsNotNil() {
				return CommandDetails{}, err
			}
			return commandDetails, customerror.NewNil()
		case ChangeCommand:
			commandDetails, err := s.ParseChange(commandString)
			if err.IsNotNil() {
				return CommandDetails{}, err
			}
			return commandDetails, customerror.NewNil()
		case SeatCommand:
			return CommandDetails{
				CommandType: Seat,
			}, customerror.NewNil()
		case ReportCommand:
			return CommandDetails{
				CommandType:   Report,
				ReportMessage: commandString,
			}, customerror.NewNil()
		case KickCommand:
			commandDetails, err := s.ParseKick(commandString)
			if err.IsNotNil() {
				return CommandDetails{}, err
			}
			return commandDetails, customerror.NewNil()
		case AddCommand:
			commandDetails, err := s.ParseAdd(commandString)
			if err.IsNotNil() {
				return CommandDetails{}, err
			}
			return commandDetails, customerror.NewNil()
		case RankCommand:
			return CommandDetails{
				CommandType: Rank,
			}, customerror.NewNil()
		case CommandPrefix: // 典型的なミスコマンド「! in」「! out」とか。
			return CommandDetails{}, customerror.InvalidCommand.New("びっくりマークは隣の文字とくっつけてください。")
		default: // 間違いコマンド
			return CommandDetails{
				CommandType: InvalidCommand,
				InOptions:   InOptions{},
			}, customerror.NewNil()
		}
	} else if strings.HasPrefix(commandString, WrongCommandPrefix) {
		return CommandDetails{}, customerror.InvalidCommand.New("びっくりマークは半角にしてください!")
	}
	return CommandDetails{
		CommandType: NotCommand,
		InOptions:   InOptions{},
	}, customerror.NewNil()
}

func (s *System) ParseIn(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)

	// 追加オプションチェック
	options, err := s.ParseInOptions(slice[1:])
	if err.IsNotNil() {
		return CommandDetails{}, err
	}

	return CommandDetails{
		CommandType: In,
		InOptions:   options,
	}, customerror.NewNil()
}

func (s *System) ParseInOptions(commandSlice []string) (InOptions, customerror.CustomError) {
	workName := ""
	isWorkNameSet := false
	workTimeMin := s.DefaultWorkTimeMin
	isWorkTimeMinSet := false
	for _, str := range commandSlice {
		if strings.HasPrefix(str, WorkNameOptionPrefix) && !isWorkNameSet {
			workName = strings.TrimPrefix(str, WorkNameOptionPrefix)
			isWorkNameSet = true
		} else if strings.HasPrefix(str, WorkNameOptionShortPrefix) && !isWorkNameSet {
			workName = strings.TrimPrefix(str, WorkNameOptionShortPrefix)
			isWorkNameSet = true
		} else if strings.HasPrefix(str, WorkNameOptionPrefixLegacy) && !isWorkNameSet {
			workName = strings.TrimPrefix(str, WorkNameOptionPrefixLegacy)
			isWorkNameSet = true
		} else if strings.HasPrefix(str, WorkNameOptionShortPrefixLegacy) && !isWorkNameSet {
			workName = strings.TrimPrefix(str, WorkNameOptionShortPrefixLegacy)
			isWorkNameSet = true
		} else if strings.HasPrefix(str, WorkTimeOptionPrefix) && !isWorkTimeMinSet {
			num, err := strconv.Atoi(strings.TrimPrefix(str, WorkTimeOptionPrefix))
			if err != nil { // 無効な値
				return InOptions{}, customerror.InvalidCommand.New("「" + WorkTimeOptionPrefix + "」の後の値を確認してください。")
			}
			if s.MinWorkTimeMin <= num && num <= s.MaxWorkTimeMin {
				workTimeMin = num
				isWorkTimeMinSet = true
			} else { // 無効な値
				return InOptions{}, customerror.InvalidCommand.New("最大作業時間（分）は" + strconv.Itoa(s.MinWorkTimeMin) + "～" + strconv.Itoa(s.MaxWorkTimeMin) + "の値にしてください。")
			}
		} else if strings.HasPrefix(str, WorkTimeOptionShortPrefix) && !isWorkTimeMinSet {
			num, err := strconv.Atoi(strings.TrimPrefix(str, WorkTimeOptionShortPrefix))
			if err != nil { // 無効な値
				return InOptions{}, customerror.InvalidCommand.New("「" + WorkTimeOptionShortPrefix + "」の後の値を確認してください。")
			}
			if s.MinWorkTimeMin <= num && num <= s.MaxWorkTimeMin {
				workTimeMin = num
				isWorkTimeMinSet = true
			} else { // 無効な値
				return InOptions{}, customerror.InvalidCommand.New("最大作業時間（分）は" + strconv.Itoa(s.MinWorkTimeMin) + "～" + strconv.Itoa(s.MaxWorkTimeMin) + "の値にしてください。")
			}
		} else if strings.HasPrefix(str, WorkTimeOptionPrefixLegacy) && !isWorkTimeMinSet {
			num, err := strconv.Atoi(strings.TrimPrefix(str, WorkTimeOptionPrefixLegacy))
			if err != nil { // 無効な値
				return InOptions{}, customerror.InvalidCommand.New("「" + WorkTimeOptionPrefixLegacy + "」の後の値を確認してください。")
			}
			if s.MinWorkTimeMin <= num && num <= s.MaxWorkTimeMin {
				workTimeMin = num
				isWorkTimeMinSet = true
			} else { // 無効な値
				return InOptions{}, customerror.InvalidCommand.New("最大作業時間（分）は" + strconv.Itoa(s.MinWorkTimeMin) + "～" + strconv.Itoa(s.MaxWorkTimeMin) + "の値にしてください。")
			}
		} else if strings.HasPrefix(str, WorkTimeOptionShortPrefixLegacy) && !isWorkTimeMinSet {
			num, err := strconv.Atoi(strings.TrimPrefix(str, WorkTimeOptionShortPrefixLegacy))
			if err != nil { // 無効な値
				return InOptions{}, customerror.InvalidCommand.New("「" + WorkTimeOptionShortPrefixLegacy + "」の後の値を確認してください。")
			}
			if s.MinWorkTimeMin <= num && num <= s.MaxWorkTimeMin {
				workTimeMin = num
				isWorkTimeMinSet = true
			} else { // 無効な値
				return InOptions{}, customerror.InvalidCommand.New("最大作業時間（分）は" + strconv.Itoa(s.MinWorkTimeMin) + "～" + strconv.Itoa(s.MaxWorkTimeMin) + "の値にしてください。")
			}
		}
	}
	return InOptions{
		WorkName: workName,
		WorkMin:  workTimeMin,
	}, customerror.NewNil()
}

func (s *System) ParseInfo(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)

	if len(slice) >= 2 {
		if slice[1] == InfoDetailsOption {
			return CommandDetails{
				CommandType: Info,
				InfoOption: InfoOption{
					ShowDetails: true,
				},
			}, customerror.NewNil()
		}
	}
	return CommandDetails{
		CommandType: Info,
	}, customerror.NewNil()
}

func (s *System) ParseMy(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)

	options, err := s.ParseMyOptions(slice[1:])
	if err.IsNotNil() {
		return CommandDetails{}, err
	}

	return CommandDetails{
		CommandType: My,
		MyOptions:   options,
	}, customerror.NewNil()
}

func (s *System) ParseMyOptions(commandSlice []string) ([]MyOption, customerror.CustomError) {
	isRankVisibleSet := false

	var options []MyOption

	for _, str := range commandSlice {
		if strings.HasPrefix(str, RankVisibleMyOptionPrefix) && !isRankVisibleSet {
			var rankVisible bool
			rankVisibleStr := strings.TrimPrefix(str, RankVisibleMyOptionPrefix)
			if rankVisibleStr == RankVisibleMyOptionOn {
				rankVisible = true
			} else if rankVisibleStr == RankVisibleMyOptionOff {
				rankVisible = false
			} else {
				return []MyOption{}, customerror.InvalidCommand.New("「" + RankVisibleMyOptionPrefix + "」の後の値を確認してください。")
			}
			options = append(options, MyOption{
				Type:      RankVisible,
				BoolValue: rankVisible,
			})
			isRankVisibleSet = true
		}
	}
	return options, customerror.NewNil()
}

func (s *System) ParseKick(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)

	var kickSeatId int
	if len(slice) >= 2 {
		num, err := strconv.Atoi(slice[1])
		if err != nil {
			return CommandDetails{}, customerror.InvalidCommand.New("有効な席番号を指定してください。")
		}
		kickSeatId = num
	} else {
		return CommandDetails{}, customerror.InvalidCommand.New("席番号を指定してください。")
	}

	return CommandDetails{
		CommandType: Kick,
		KickSeatId:  kickSeatId,
	}, customerror.NewNil()
}

func (s *System) ParseChange(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)

	// 追加オプションチェック
	options, err := s.ParseChangeOptions(slice[1:])
	if err.IsNotNil() {
		return CommandDetails{}, err
	}

	return CommandDetails{
		CommandType:   Change,
		ChangeOptions: options,
	}, customerror.NewNil()
}

func (s *System) ParseChangeOptions(commandSlice []string) ([]ChangeOption, customerror.CustomError) {
	isWorkNameSet := false

	var options []ChangeOption

	for _, str := range commandSlice {
		if strings.HasPrefix(str, WorkNameOptionPrefix) && !isWorkNameSet {
			workName := strings.TrimPrefix(str, WorkNameOptionPrefix)
			options = append(options, ChangeOption{
				Type:        WorkName,
				StringValue: workName,
			})
			isWorkNameSet = true
		} else if strings.HasPrefix(str, WorkNameOptionShortPrefix) && !isWorkNameSet {
			workName := strings.TrimPrefix(str, WorkNameOptionShortPrefix)
			options = append(options, ChangeOption{
				Type:        WorkName,
				StringValue: workName,
			})
			isWorkNameSet = true
		} else if strings.HasPrefix(str, WorkNameOptionPrefixLegacy) && !isWorkNameSet {
			return nil, customerror.InvalidCommand.New("「" + WorkNameOptionPrefixLegacy + "」は使えません。「" + WorkNameOptionPrefix + "」を使ってください。")
		} else if strings.HasPrefix(str, WorkNameOptionShortPrefixLegacy) && !isWorkNameSet {
			return nil, customerror.InvalidCommand.New("「" + WorkNameOptionShortPrefixLegacy + "」は使えません。「" + WorkNameOptionShortPrefix + "」を使ってください。")
		}
	}
	return options, customerror.NewNil()
}

func (s *System) ParseAdd(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)

	// 指定時間
	var workTimeMin int
	if len(slice) >= 2 {
		if strings.HasPrefix(slice[1], WorkTimeOptionPrefix) {
			num, err := strconv.Atoi(strings.TrimPrefix(slice[1], WorkTimeOptionPrefix))
			if err != nil { // 無効な値
				return CommandDetails{}, customerror.InvalidCommand.New("「" + WorkTimeOptionPrefix + "」の後の値を確認してください。")
			}
			if s.MinWorkTimeMin <= num && num <= s.MaxWorkTimeMin {
				workTimeMin = num
			} else { // 無効な値
				return CommandDetails{}, customerror.InvalidCommand.New("延長時間（分）は" + strconv.Itoa(s.MinWorkTimeMin) + "～" + strconv.Itoa(s.MaxWorkTimeMin) + "の値にしてください。")
			}
		} else if strings.HasPrefix(slice[1], WorkTimeOptionShortPrefix) {
			num, err := strconv.Atoi(strings.TrimPrefix(slice[1], WorkTimeOptionShortPrefix))
			if err != nil { // 無効な値
				return CommandDetails{}, customerror.InvalidCommand.New("「" + WorkTimeOptionShortPrefix + "」の後の値を確認してください。")
			}
			if s.MinWorkTimeMin <= num && num <= s.MaxWorkTimeMin {
				workTimeMin = num
			} else { // 無効な値
				return CommandDetails{}, customerror.InvalidCommand.New("延長時間（分）は" + strconv.Itoa(s.MinWorkTimeMin) + "～" + strconv.Itoa(s.MaxWorkTimeMin) + "の値にしてください。")
			}
		} else if strings.HasPrefix(slice[1], WorkTimeOptionPrefixLegacy) {
			num, err := strconv.Atoi(strings.TrimPrefix(slice[1], WorkTimeOptionPrefixLegacy))
			if err != nil { // 無効な値
				return CommandDetails{}, customerror.InvalidCommand.New("「" + WorkTimeOptionPrefixLegacy + "」の後の値を確認してください。")
			}
			if s.MinWorkTimeMin <= num && num <= s.MaxWorkTimeMin {
				workTimeMin = num
			} else { // 無効な値
				return CommandDetails{}, customerror.InvalidCommand.New("延長時間（分）は" + strconv.Itoa(s.MinWorkTimeMin) + "～" + strconv.Itoa(s.MaxWorkTimeMin) + "の値にしてください。")
			}
		} else if strings.HasPrefix(slice[1], WorkTimeOptionShortPrefixLegacy) {
			num, err := strconv.Atoi(strings.TrimPrefix(slice[1], WorkTimeOptionShortPrefixLegacy))
			if err != nil { // 無効な値
				return CommandDetails{}, customerror.InvalidCommand.New("「" + WorkTimeOptionShortPrefixLegacy + "」の後の値を確認してください。")
			}
			if s.MinWorkTimeMin <= num && num <= s.MaxWorkTimeMin {
				workTimeMin = num
			} else { // 無効な値
				return CommandDetails{}, customerror.InvalidCommand.New("延長時間（分）は" + strconv.Itoa(s.MinWorkTimeMin) + "～" + strconv.Itoa(s.MaxWorkTimeMin) + "の値にしてください。")
			}
		}
	} else {
		return CommandDetails{}, customerror.InvalidCommand.New("延長時間（分）を「" + WorkTimeOptionPrefix + "」で指定してください。")
	}
	
	if workTimeMin == 0 {
		return CommandDetails{}, customerror.InvalidCommand.New("オプションが正しく設定されているか確認してください。")
	}

	return CommandDetails{
		CommandType: Add,
		AddMinutes:  workTimeMin,
	}, customerror.NewNil()
}

func (s *System) In(command CommandDetails, ctx context.Context) error {
	// 初回の利用の場合はユーザーデータを初期化
	isRegistered, err := s.IfUserRegistered(ctx)
	if err != nil {
		return err
	}
	if !isRegistered {
		err := s.InitializeUser(ctx)
		if err != nil {
			return err
		}
	}

	// すでに入室している場合
	isInRoom, err := s.IsUserInRoom(ctx)
	if err != nil {
		_ = s.LineBot.SendMessageWithError("failed s.IsUserInRoom()", err)
		return err
	}
	if isInRoom {
		currentSeat, customErr := s.CurrentSeat(ctx)
		if customErr.IsNotNil() {
			_ = s.LineBot.SendMessageWithError("failed CurrentSeatId", customErr.Body)
			s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、エラーが発生しました。", ctx)
			return customErr.Body
		}

		if command.InOptions.WorkName != "" {
			// 作業名を書きかえ
			err := s.UpdateWorkName(command.InOptions.WorkName, ctx)
			if err != nil {
				_ = s.LineBot.SendMessageWithError("failed to UpdateWorkName", err)
				s.SendLiveChatMessage(s.ProcessedUserDisplayName+
					"さん、エラーが発生しました。もう一度試してみてください。", ctx)
				return err
			}
			s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さんの作業名を更新しました（"+strconv.Itoa(currentSeat.SeatId)+"番席）。", ctx)
		} else {
			s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、すでに入室しています（"+strconv.Itoa(currentSeat.SeatId)+"番席）。", ctx)
		}
		return nil
	}

	// ここまで来ると入室処理は確定

	// 席番号を決定
	seatId, err := s.MinAvailableSeatId(ctx)
	if err != nil {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+
			"さん、エラーが発生しました。もう一度試してみてください。", ctx)
		return err
	}

	// ランクから席の色を決定
	var seatColorCode string
	userDoc, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
	if err != nil {
		_ = s.LineBot.SendMessageWithError("failed to RetrieveUser", err)
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+
			"さん、エラーが発生しました。もう一度試してみてください。", ctx)
		return err
	}
	if userDoc.RankVisible {
		rank, err := utils.GetRank(userDoc.TotalStudySec)
		if err != nil {
			_ = s.LineBot.SendMessageWithError("failed to GetRank", err)
			s.SendLiveChatMessage(s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください。", ctx)
			return err
		}
		seatColorCode = rank.ColorCode
	} else {
		rank := utils.GetInvisibleRank()
		seatColorCode = rank.ColorCode
	}

	// 入室
	err = s.EnterRoom(seatId, command.InOptions.WorkName, command.InOptions.WorkMin, seatColorCode, ctx)
	if err != nil {
		_ = s.LineBot.SendMessageWithError("failed to enter room", err)
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+
			"さん、エラーが発生しました。もう一度試してみてください。", ctx)
		return err
	}
	s.SendLiveChatMessage(s.ProcessedUserDisplayName+
		"さんが作業を始めました🔥（最大"+strconv.Itoa(command.InOptions.WorkMin)+"分、"+strconv.Itoa(seatId)+"番席）", ctx)

	// 入室時刻を記録
	err = s.FirestoreController.SetLastEnteredDate(s.ProcessedUserId, ctx)
	if err != nil {
		_ = s.LineBot.SendMessageWithError("failed to set last entered date", err)
		return err
	}
	return nil
}

func (s *System) Out(_ CommandDetails, ctx context.Context) error {
	// 今勉強中か？
	isInRoom, err := s.IsUserInRoom(ctx)
	if err != nil {
		_ = s.LineBot.SendMessageWithError("failed IsUserInRoom()", err)
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください。", ctx)
		return err
	}
	if !isInRoom {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、すでに退室しています。", ctx)
		return nil
	}
	// 現在座っている席を特定
	seatId, customErr := s.CurrentSeatId(ctx)
	if customErr.Body != nil {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+
			"さん、残念ながらエラーが発生しました。もう一度試してみてください。", ctx)
		return customErr.Body
	}
	// 退室処理
	workedTimeSec, err := s.ExitRoom(seatId, ctx)
	if err != nil {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください。", ctx)
		return err
	} else {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さんが退室しました！"+
			"（+ "+strconv.Itoa(workedTimeSec/60)+"分）", ctx)
		return nil
	}
}

func (s *System) ShowUserInfo(command CommandDetails, ctx context.Context) error {
	// そのユーザーはドキュメントがあるか？
	isUserRegistered, err := s.IfUserRegistered(ctx)
	if err != nil {
		return err
	}
	if isUserRegistered {
		liveChatMessage := ""
		totalTimeStr, dailyTotalTimeStr, err := s.TotalStudyTimeStrings(ctx)
		if err != nil {
			_ = s.LineBot.SendMessageWithError("failed s.TotalStudyTimeStrings()", err)
			return err
		}
		liveChatMessage += s.ProcessedUserDisplayName +
			"さん　［本日の作業時間：" + dailyTotalTimeStr + "］" +
			" ［累計作業時間：" + totalTimeStr + "］"

		if command.InfoOption.ShowDetails {
			userDoc, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
			if err != nil {
				_ = s.LineBot.SendMessageWithError("failed fetch user doc", err)
				return err
			}
			
			switch userDoc.RankVisible {
			case true:
				liveChatMessage += "［ランク表示：オン］"
			case false:
				liveChatMessage += "［ランク表示：オフ］"
			}
			
			liveChatMessage += "［登録日：" + userDoc.RegistrationDate.Format("2006年01月02日") + "］"
		}
		s.SendLiveChatMessage(liveChatMessage, ctx)
	} else {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+
			"さんはまだ作業データがありません。「"+InCommand+"」コマンドで作業を始めましょう！", ctx)
	}
	return nil
}

func (s *System) ShowSeatInfo(_ CommandDetails, ctx context.Context) error {
	// そのユーザーは入室しているか？
	isUserInRoom, err := s.IsUserInRoom(ctx)
	if err != nil {
		return err
	}
	if isUserInRoom {
		currentSeat, err := s.CurrentSeat(ctx)
		if err.IsNotNil() {
			s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください。", ctx)
			_ = s.LineBot.SendMessageWithError("failed s.CurrentSeat()", err.Body)
		}

		remainingMinutes := int(currentSeat.Until.Sub(utils.JstNow()).Minutes())
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さんは"+strconv.Itoa(currentSeat.SeatId)+"番の席に座っています。自動退室まで残り"+strconv.Itoa(remainingMinutes)+"分です。", ctx)
	} else {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+
			"さんは入室していません。「"+InCommand+"」コマンドで入室しましょう！", ctx)
	}
	return nil
}

func (s *System) Report(command CommandDetails, ctx context.Context) error {
	err := s.LineBot.SendMessage(s.ProcessedUserId + "（" + s.ProcessedUserDisplayName + "）さんから" + ReportCommand + "を受信しました。\n\n" + command.ReportMessage)
	if err != nil {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、エラーが発生しました。", ctx)
		return err
	}
	s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、管理者へメッセージを送信しました。", ctx)
	return nil
}

func (s *System) Kick(command CommandDetails, ctx context.Context) error {
	// commanderはモデレーターかチャットオーナーか
	if s.ProcessedUserIsModeratorOrOwner {
		// ターゲットの座席は誰か使っているか
		isSeatAvailable, err := s.IfSeatAvailable(command.KickSeatId, ctx)
		if err != nil {
			return err
		}
		if !isSeatAvailable {
			// ユーザーを強制退室させる
			seat, cerr := s.RetrieveSeatBySeatId(command.KickSeatId, ctx)
			if cerr.IsNotNil() {
				return cerr.Body
			}
			s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、"+strconv.Itoa(seat.SeatId)+"番席の"+seat.UserDisplayName+"さんを退室させます。", ctx)

			s.SetProcessedUser(seat.UserId, seat.UserDisplayName, false, false)
			outCommandDetails := CommandDetails{
				CommandType: Out,
				InOptions:   InOptions{},
			}

			err := s.Out(outCommandDetails, ctx)
			if err != nil {
				return err
			}
		} else {
			s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、その番号の座席は誰も使用していません。", ctx)
		}
	} else {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さんは「"+KickCommand+"」コマンドを使用できません。", ctx)
	}
	return nil
}

func (s *System) My(command CommandDetails, ctx context.Context) error {
	// ユーザードキュメントはすでにあり、登録されていないプロパティだった場合、そのままプロパティを保存したら自動で作成される。
	// また、読み込みのときにそのプロパティがなくても大丈夫。自動で初期値が割り当てられる。
	// ただし、ユーザードキュメントがそもそもない場合は、書き込んでもエラーにはならないが、登録日が記録されないため、要登録。
	
	// そのユーザーはドキュメントがあるか？
	isUserRegistered, err := s.IfUserRegistered(ctx)
	if err != nil {
		return err
	}
	if !isUserRegistered { // ない場合は作成。
		err := s.InitializeUser(ctx)
		if err != nil {
			return err
		}
	}

	// オプションが1つ以上指定されているか？
	if len(command.MyOptions) == 0 {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、オプションが正しく設定されているか確認してください。", ctx)
		return nil
	}

	for _, myOption := range command.MyOptions {
		if myOption.Type == RankVisible {
			userDoc, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
			if err != nil {
				_ = s.LineBot.SendMessageWithError("faield  s.FirestoreController.RetrieveUser()", err)
				s.SendLiveChatMessage(s.ProcessedUserDisplayName+
					"さん、エラーが発生しました。もう一度試してみてください。", ctx)
				return err
			}
			// 現在の値と、設定したい値が同じなら、変更なし
			if userDoc.RankVisible == myOption.BoolValue {
				var rankVisibleString string
				if userDoc.RankVisible {
					rankVisibleString = "オン"
				} else {
					rankVisibleString = "オフ"
				}
				s.SendLiveChatMessage(s.ProcessedUserDisplayName + "さんのランク表示モードはすでに" + rankVisibleString + "です。", ctx)
			} else {
				// 違うなら、切替
				err := s.ToggleRankVisible(ctx)
				if err != nil {
					_ = s.LineBot.SendMessageWithError("failed to ToggleRankVisible", err)
					s.SendLiveChatMessage(s.ProcessedUserDisplayName+
						"さん、エラーが発生しました。もう一度試してみてください。", ctx)
					return err
				}
			}
		}
		if myOption.Type == DefaultStudyMin {
			err := s.FirestoreController.SetMyDefaultStudyMin(s.ProcessedUserId, myOption.IntValue, ctx)
			if err != nil {
				_ = s.LineBot.SendMessageWithError("failed to set my-default-study-min", err)
				s.SendLiveChatMessage(s.ProcessedUserDisplayName+
					"さん、エラーが発生しました。もう一度試してみてください。", ctx)
				return err
			}
			s.SendLiveChatMessage(s.ProcessedUserDisplayName + "さんのデフォルトの作業時間を" + strconv.Itoa(myOption.IntValue) + "分に設定しました。", ctx)
		}
	}
	return nil
}

func (s *System) Change(command CommandDetails, ctx context.Context) error {
	// そのユーザーは入室中か？
	isUserInRoom, err := s.IsUserInRoom(ctx)
	if err != nil {
		return err
	}
	if !isUserInRoom {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、入室中のみ使えるコマンドです。", ctx)
		return nil
	}
	currentSeatId, customErr := s.CurrentSeatId(ctx)
	if customErr.IsNotNil() {
		_ = s.LineBot.SendMessageWithError("failed CurrentSeatId", customErr.Body)
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、エラーが発生しました。", ctx)
		return customErr.Body
	}

	// オプションが1つ以上指定されているか？
	if len(command.ChangeOptions) == 0 {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、オプションが正しく設定されているか確認してください。", ctx)
		return nil
	}

	for _, changeOption := range command.ChangeOptions {
		if changeOption.Type == WorkName {
			// 作業名を書きかえ
			err := s.UpdateWorkName(changeOption.StringValue, ctx)
			if err != nil {
				_ = s.LineBot.SendMessageWithError("failed to UpdateWorkName", err)
				s.SendLiveChatMessage(s.ProcessedUserDisplayName+
					"さん、エラーが発生しました。もう一度試してみてください。", ctx)
				return err
			}
		}
	}
	s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さんの作業名を更新しました（"+strconv.Itoa(currentSeatId)+"番席）。", ctx)
	return nil
}

func (s *System) Add(command CommandDetails, ctx context.Context) error {
	// 入室しているか？
	isUserInRoom, err := s.IsUserInRoom(ctx)
	if err != nil {
		return err
	}
	if isUserInRoom {
		// 時間を指定分延長
		currentSeat, cerr := s.CurrentSeat(ctx)
		if cerr.IsNotNil() {
			return cerr.Body
		}
		newUntil := currentSeat.Until.Add(time.Duration(command.AddMinutes) * time.Minute)
		// もし延長後の時間が最大作業時間を超えていたら、却下
		if int(newUntil.Sub(utils.JstNow()).Minutes()) > s.MaxWorkTimeMin {
			remainingWorkMin := int(currentSeat.Until.Sub(utils.JstNow()).Minutes())
			s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、現在時刻から"+
				strconv.Itoa(s.MaxWorkTimeMin)+"分後までのみ作業時間を延長することができます。現在の自動退室までの残り時間は"+
				strconv.Itoa(remainingWorkMin)+"分です。", ctx)
			return nil
		}

		err := s.FirestoreController.UpdateSeatUntil(newUntil, s.ProcessedUserId, ctx)
		if err != nil {
			return err
		}
		remainingWorkMin := int(newUntil.Sub(utils.JstNow()).Minutes())
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、自動退室までの時間を"+strconv.Itoa(command.AddMinutes)+"分延長しました。自動退室まで残り" + strconv.Itoa(remainingWorkMin) + "分です。", ctx)
	} else {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、入室中のみ使えるコマンドです。", ctx)
	}

	return nil
}

func (s *System) Rank(_ CommandDetails, ctx context.Context) error {
	// そのユーザーはドキュメントがあるか？
	isUserRegistered, err := s.IfUserRegistered(ctx)
	if err != nil {
		return err
	}
	if !isUserRegistered { // ない場合は作成。
		err := s.InitializeUser(ctx)
		if err != nil {
			return err
		}
	}
	
	// ランク表示設定のON/OFFを切り替える
	err = s.ToggleRankVisible(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *System) ToggleRankVisible(ctx context.Context) error {
	// get current value
	userDoc, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
	if err != nil {
		return err
	}
	currentRankVisible := userDoc.RankVisible
	newRankVisible := !currentRankVisible
	
	// set reverse value
	err = s.FirestoreController.SetMyRankVisible(s.ProcessedUserId, newRankVisible, ctx)
	if err != nil {
		return err
	}
	
	var newValueString string
	if newRankVisible {
		newValueString = "オン"
	} else {
		newValueString = "オフ"
	}
	s.SendLiveChatMessage(s.ProcessedUserDisplayName + "さんのランク表示を" + newValueString + "にしました。", ctx)
	
	// 入室中であれば、座席の色も変える
	isUserInRoom, err := s.IsUserInRoom(ctx)
	if isUserInRoom {
		var rank utils.Rank
		if newRankVisible {	// ランクから席の色を取得
			rank, err = utils.GetRank(userDoc.TotalStudySec)
			if err != nil {
				_ = s.LineBot.SendMessageWithError("failed to GetRank", err)
				s.SendLiveChatMessage(s.ProcessedUserDisplayName+
					"さん、エラーが発生しました。もう一度試してみてください。", ctx)
				return err
			}
		} else {	// ランク表示オフの色を取得
			rank = utils.GetInvisibleRank()
		}
		// 席の色を更新
		err := s.FirestoreController.UpdateSeatColorCode(rank.ColorCode, s.ProcessedUserId, ctx)
		if err != nil {
			_ = s.LineBot.SendMessageWithError("failed to s.FirestoreController.UpdateSeatColorCode()", err)
			s.SendLiveChatMessage(s.ProcessedUserDisplayName + "さん、エラーが発生しました。もう一度試してください。", ctx)
			return err
		}
	}
	
	return nil
}

// IfSeatAvailable 席番号がseatIdの席が空いているかどうか。
func (s *System) IfSeatAvailable(seatId int, ctx context.Context) (bool, error) {
	roomData, err := s.FirestoreController.RetrieveRoom(ctx)
	if err != nil {
		return false, err
	}
	for _, seat := range roomData.Seats {
		if seat.SeatId == seatId {
			return false, nil
		}
	}
	// ここまで来ると指定された番号の席が使われていないということ
	return true, nil
}

func (s *System) RetrieveSeatBySeatId(seatId int, ctx context.Context) (myfirestore.Seat, customerror.CustomError) {
	roomDoc, err := s.FirestoreController.RetrieveRoom(ctx)
	if err != nil {
		return myfirestore.Seat{}, customerror.Unknown.Wrap(err)
	}
	for _, seat := range roomDoc.Seats {
		if seat.SeatId == seatId {
			return seat, customerror.NewNil()
		}
	}
	// ここまで来ると指定された番号の席が使われていないということ
	return myfirestore.Seat{}, customerror.SeatNotFound.New("that seat is not used.")
}

// IsUserInRoom そのユーザーがルーム内にいるか？登録済みかに関わらず。
func (s *System) IsUserInRoom(ctx context.Context) (bool, error) {
	roomData, err := s.FirestoreController.RetrieveRoom(ctx)
	if err != nil {
		return false, err
	}
	for _, seat := range roomData.Seats {
		if seat.UserId == s.ProcessedUserId {
			return true, nil
		}
	}
	return false, nil
}

func (s *System) InitializeUser(ctx context.Context) error {
	log.Println("InitializeUser()")
	userData := myfirestore.UserDoc{
		DailyTotalStudySec: 0,
		TotalStudySec:      0,
		RegistrationDate:   utils.JstNow(),
	}
	return s.FirestoreController.InitializeUser(s.ProcessedUserId, userData, ctx)
}

func (s *System) RetrieveYoutubeLiveInfo(ctx context.Context) (myfirestore.YoutubeLiveConfigDoc, error) {
	return s.FirestoreController.RetrieveYoutubeLiveConfig(ctx)
}

func (s *System) RetrieveNextPageToken(ctx context.Context) (string, error) {
	return s.FirestoreController.RetrieveNextPageToken(ctx)
}

func (s *System) SaveNextPageToken(nextPageToken string, ctx context.Context) error {
	return s.FirestoreController.SaveNextPageToken(nextPageToken, ctx)
}

// EnterRoom 入室させる。事前チェックはされている前提。
func (s *System) EnterRoom(seatId int, workName string, workTimeMin int, seatColorCode string, ctx context.Context) error {
	enterDate := utils.JstNow()
	exitDate := enterDate.Add(time.Duration(workTimeMin) * time.Minute)
	seat, err := s.FirestoreController.SetSeat(seatId, workName, enterDate, exitDate, seatColorCode, s.ProcessedUserId, s.ProcessedUserDisplayName, ctx)
	if err != nil {
		return err
	}
	// 入室時刻を記録
	err = s.FirestoreController.SetLastEnteredDate(s.ProcessedUserId, ctx)
	if err != nil {
		return err
	}
	// ログ記録
	err = s.FirestoreController.AddUserHistory(s.ProcessedUserId, EnterAction, seat, ctx)
	if err != nil {
		return err
	}
	return nil
}

// ExitRoom ユーザーを退室させる。事前チェックはされている前提。
func (s *System) ExitRoom(seatId int, ctx context.Context) (int, error) {
	// 作業時間を計算
	userData, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
	if err != nil {
		return 0, err
	}
	workedTimeSec := int(utils.JstNow().Sub(userData.LastEntered).Seconds())
	var dailyWorkedTimeSec int
	jstNow := utils.JstNow()
	// もし日付変更を跨いで入室してたら、当日の累計時間は日付変更からの時間にする
	if workedTimeSec > utils.InSeconds(jstNow) {
		dailyWorkedTimeSec = utils.InSeconds(jstNow)
	} else {
		dailyWorkedTimeSec = workedTimeSec
	}

	var seat myfirestore.Seat
	room, err := s.FirestoreController.RetrieveRoom(ctx)
	if err != nil {
		return 0, err
	}
	for _, seatInRoom := range room.Seats {
		if seatInRoom.UserId == s.ProcessedUserId {
			seat = seatInRoom
		}
	}
	err = s.FirestoreController.UnSetSeatInRoom(seat, ctx)
	if err != nil {
		return 0, err
	}
	// ログ記録
	err = s.FirestoreController.AddUserHistory(s.ProcessedUserId, ExitAction, seat, ctx)
	if err != nil {
		_ = s.LineBot.SendMessageWithError("failed to add an user history", err)
	}
	// 退室時刻を記録
	err = s.FirestoreController.SetLastExitedDate(s.ProcessedUserId, ctx)
	if err != nil {
		_ = s.LineBot.SendMessageWithError("failed to update last-exited-date", err)
		return 0, err
	}
	// 累計学習時間を更新
	err = s.UpdateTotalWorkTime(workedTimeSec, dailyWorkedTimeSec, ctx)
	if err != nil {
		_ = s.LineBot.SendMessageWithError("failed to update total study time", err)
		return 0, err
	}

	log.Println(s.ProcessedUserId + " exited the room. seat id: " + strconv.Itoa(seatId))
	return workedTimeSec, nil
}

func (s *System) CurrentSeatId(ctx context.Context) (int, customerror.CustomError) {
	currentSeat, err := s.CurrentSeat(ctx)
	if err.IsNotNil() {
		return -1, err
	}
	return currentSeat.SeatId, customerror.NewNil()
}

func (s *System) CurrentSeat(ctx context.Context) (myfirestore.Seat, customerror.CustomError) {
	roomData, err := s.FirestoreController.RetrieveRoom(ctx)
	if err != nil {
		return myfirestore.Seat{}, customerror.Unknown.Wrap(err)
	}
	for _, seat := range roomData.Seats {
		if seat.UserId == s.ProcessedUserId {
			return seat, customerror.NewNil()
		}
	}
	// 入室していない
	return myfirestore.Seat{}, customerror.UserNotInAnyRoom.New("the user is not in any room.")
}

func (s *System) UpdateTotalWorkTime(workedTimeSec int, dailyWorkedTimeSec int, ctx context.Context) error {
	userData, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
	if err != nil {
		return err
	}
	// 更新前の値
	previousTotalSec := userData.TotalStudySec
	previousDailyTotalSec := userData.DailyTotalStudySec
	// 更新後の値
	newTotalSec := previousTotalSec + workedTimeSec
	newDailyTotalSec := previousDailyTotalSec + dailyWorkedTimeSec
	err = s.FirestoreController.UpdateTotalTime(s.ProcessedUserId, newTotalSec, newDailyTotalSec, ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *System) IfUserRegistered(ctx context.Context) (bool, error) {
	_, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

// TotalStudyTimeStrings リアルタイムの累積作業時間・当日累積作業時間を文字列で返す。
func (s *System) TotalStudyTimeStrings(ctx context.Context) (string, string, error) {
	// 入室中ならばリアルタイムの作業時間も加算する
	realtimeDuration := time.Duration(0)
	realtimeDailyDuration := time.Duration(0)
	if isInRoom, _ := s.IsUserInRoom(ctx); isInRoom {
		// 作業時間を計算
		jstNow := utils.JstNow()
		userData, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
		if err != nil {
			return "", "", err
		}
		workedTimeSec := int(jstNow.Sub(userData.LastEntered).Seconds())
		realtimeDuration = time.Duration(workedTimeSec) * time.Second

		var dailyWorkedTimeSec int
		if workedTimeSec > utils.InSeconds(jstNow) {
			dailyWorkedTimeSec = utils.InSeconds(jstNow)
		} else {
			dailyWorkedTimeSec = workedTimeSec
		}
		realtimeDailyDuration = time.Duration(dailyWorkedTimeSec) * time.Second
	}

	userData, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
	if err != nil {
		return "", "", err
	}
	// 累計
	var totalStr string
	totalDuration := realtimeDuration + time.Duration(userData.TotalStudySec)*time.Second
	if totalDuration < time.Hour {
		totalStr = strconv.Itoa(int(totalDuration.Minutes())) + "分"
	} else {
		totalStr = strconv.Itoa(int(totalDuration.Hours())) + "時間" +
			strconv.Itoa(int(totalDuration.Minutes())%60) + "分"
	}
	// 当日の累計
	var dailyTotalStr string
	dailyTotalDuration := realtimeDailyDuration + time.Duration(userData.DailyTotalStudySec)*time.Second
	if dailyTotalDuration < time.Hour {
		dailyTotalStr = strconv.Itoa(int(dailyTotalDuration.Minutes())) + "分"
	} else {
		dailyTotalStr = strconv.Itoa(int(dailyTotalDuration.Hours())) + "時間" +
			strconv.Itoa(int(dailyTotalDuration.Minutes())%60) + "分"
	}
	return totalStr, dailyTotalStr, nil
}

// ExitAllUserInRoom roomの全てのユーザーを退室させる。
func (s *System) ExitAllUserInRoom(ctx context.Context) error {
	room, err := s.FirestoreController.RetrieveRoom(ctx)
	if err != nil {
		return err
	}
	for _, seat := range room.Seats {
		s.ProcessedUserId = seat.UserId
		s.ProcessedUserDisplayName = seat.UserDisplayName
		_, err := s.ExitRoom(seat.SeatId, ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *System) SendLiveChatMessage(message string, ctx context.Context) {
	err := s.LiveChatBot.PostMessage(message, ctx)
	if err != nil {
		_ = s.LineBot.SendMessageWithError("failed to send live chat message", err)
	}
	return
}

// OrganizeDatabase untilを過ぎているルーム内のユーザーを退室させる。
func (s *System) OrganizeDatabase(ctx context.Context) error {
	room, err := s.FirestoreController.RetrieveRoom(ctx)
	if err != nil {
		return err
	}
	for _, seat := range room.Seats {
		if seat.Until.Before(utils.JstNow()) {
			s.ProcessedUserId = seat.UserId
			s.ProcessedUserDisplayName = seat.UserDisplayName

			workedTimeSec, err := s.ExitRoom(seat.SeatId, ctx)
			if err != nil {
				_ = s.LineBot.SendMessageWithError(s.ProcessedUserDisplayName+"さん（"+s.ProcessedUserId+"）の退室処理中にエラーが発生しました。", err)
				return err
			} else {
				s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さんが退室しました！"+
					"（+ "+strconv.Itoa(workedTimeSec/60)+"分）", ctx)
				return nil
			}
		}
	}
	return nil
}

func (s *System) CheckLiveStreamStatus(ctx context.Context) error {
	checker := guardians.NewLiveStreamChecker(s.FirestoreController, s.LiveChatBot, s.LineBot)
	return checker.Check(ctx)
}

func (s *System) ResetDailyTotalStudyTime(ctx context.Context) error {
	log.Println("ResetDailyTotalStudyTime()")
	constantsConfig, err := s.FirestoreController.RetrieveSystemConstantsConfig(ctx)
	if err != nil {
		return err
	}
	previousDate := constantsConfig.LastResetDailyTotalStudySec.In(utils.JapanLocation())
	now := utils.JstNow()
	isDifferentDay := now.Year() != previousDate.Year() || now.Month() != previousDate.Month() || now.Day() != previousDate.Day()
	if isDifferentDay && now.After(previousDate) {
		userRefs, err := s.FirestoreController.RetrieveAllUserDocRefs(ctx)
		if err != nil {
			return err
		}
		for _, userRef := range userRefs {
			err := s.FirestoreController.ResetDailyTotalStudyTime(userRef, ctx)
			if err != nil {
				return err
			}
		}
		_ = s.LineBot.SendMessage("successfully reset all user's daily total study time. (" + strconv.Itoa(len(userRefs)) + " users)")
		err = s.FirestoreController.SetLastResetDailyTotalStudyTime(now, ctx)
		if err != nil {
			return err
		}
	} else {
		_ = s.LineBot.SendMessage("all user's daily total study times are already reset today.")
	}
	return nil
}

func (s *System) RetrieveAllUsersTotalStudySecList(ctx context.Context) ([]UserIdTotalStudySecSet, error) {
	var set []UserIdTotalStudySecSet

	userDocRefs, err := s.FirestoreController.RetrieveAllUserDocRefs(ctx)
	if err != nil {
		return set, err
	}
	for _, userDocRef := range userDocRefs {
		userDoc, err := s.FirestoreController.RetrieveUser(userDocRef.ID, ctx)
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

// UpdateWorkName 入室中のユーザーの作業名を更新する。入室中かどうかはチェック済みとする。
func (s *System) UpdateWorkName(workName string, ctx context.Context) error {
	isUserInRoom, err := s.IsUserInRoom(ctx)
	if err != nil {
		return err
	}
	if isUserInRoom {
		err := s.FirestoreController.UpdateWorkNameAtSeat(workName, s.ProcessedUserId, ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// MinAvailableSeatId 空いている最小の番号の席番号を求める
func (s *System) MinAvailableSeatId(ctx context.Context) (int, error) {
	roomDoc, err := s.FirestoreController.RetrieveRoom(ctx)
	if err != nil {
		return -1, err
	}

	if len(roomDoc.Seats) > 0 {
		// 使用されている座席番号リストを取得
		var usedSeatIds []int
		for _, seat := range roomDoc.Seats {
			usedSeatIds = append(usedSeatIds, seat.SeatId)
		}

		// 使用されていない最小の席番号を求める。1から順に探索
		searchingSeatId := 1
		for {
			// searchingSeatIdがusedSeatIdsに含まれているか
			isUsed := false
			for _, usedSeatId := range usedSeatIds {
				if usedSeatId == searchingSeatId {
					isUsed = true
				}
			}
			if !isUsed { // 使われていなければその席番号を返す
				return searchingSeatId, nil
			}
			searchingSeatId += 1
		}
	} else { // 誰も入室していない場合
		return 1, nil
	}
}
