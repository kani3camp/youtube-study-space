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
	"math/rand"
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

func (s *System) SetProcessedUser(userId string, userDisplayName string) {
	s.ProcessedUserId = userId
	s.ProcessedUserDisplayName = userDisplayName
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
func (s *System) Command(commandString string, userId string, userDisplayName string, ctx context.Context) customerror.CustomError {
	s.SetProcessedUser(userId, userDisplayName)

	commandDetails, err := s.ParseCommand(commandString)
	if err.IsNotNil() {	// これはシステム内部のエラーではなく、コマンドが悪いということなので、return nil
		s.SendLiveChatMessage(s.ProcessedUserDisplayName + "さん、" + err.Body.Error(), ctx)
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
		fallthrough
	case SeatIn:
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
	default:
		_ = s.LineBot.SendMessage("Unknown command type")
	}
	return customerror.NewNil()
}

// ParseCommand コマンドを解析
func (s *System) ParseCommand(commandString string) (CommandDetails, customerror.CustomError) {
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
		default: // !席番号 or 間違いコマンド
			// !席番号かどうか
			num, err := strconv.Atoi(strings.TrimLeft(slice[0], CommandPrefix))
			if err == nil && num >= 0 {
				commandDetails, err := s.ParseSeatIn(num, commandString)
				if err.IsNotNil() {
					return CommandDetails{}, err
				}
				return commandDetails, customerror.NewNil()
			}

			// 間違いコマンド
			return CommandDetails{
				CommandType: InvalidCommand,
				InOptions:   InOptions{},
			}, customerror.NewNil() // TODO: エラーにしたほうがいいかな？
		}
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

func (s *System) ParseSeatIn(seatNum int, commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)

	// 追加オプションチェック
	options, err := s.ParseInOptions(slice[1:])
	if err.IsNotNil() {
		return CommandDetails{}, err
	}

	// 追加オプションに席番号を追加
	options.SeatId = seatNum

	return CommandDetails{
		CommandType: SeatIn,
		InOptions:   options,
	}, customerror.NewNil()
}

func (s *System) ParseInOptions(commandSlice []string) (InOptions, customerror.CustomError) {
	workName := ""
	isWorkNameSet := false
	workTimeMin := s.DefaultWorkTimeMin
	isWorkTimeMinSet := false
	for _, str := range commandSlice {
		if strings.HasPrefix(str, WorkNameOptionPrefixLegacy) && !isWorkNameSet {
			workName = strings.TrimLeft(str, WorkNameOptionPrefixLegacy)
			isWorkNameSet = true
		} else if strings.HasPrefix(str, WorkNameOptionShortPrefixLegacy) && !isWorkNameSet {
			workName = strings.TrimLeft(str, WorkNameOptionShortPrefixLegacy)
			isWorkNameSet = true
		} else if strings.HasPrefix(str, WorkTimeOptionPrefixLegacy) && !isWorkTimeMinSet {
			num, err := strconv.Atoi(strings.TrimLeft(str, WorkTimeOptionPrefixLegacy))
			if err != nil { // 無効な値
				return InOptions{}, customerror.InvalidCommand.New("「" + WorkTimeOptionPrefixLegacy + "」の後の値を確認してください。")
			}
			if s.MinWorkTimeMin <= num && num <= s.MaxWorkTimeMin {
				workTimeMin = num
				isWorkTimeMinSet = true
			} else { // 無効な値
				return InOptions{}, customerror.InvalidCommand.New("作業時間（分）は" + strconv.Itoa(s.MinWorkTimeMin) + "～" + strconv.Itoa(s.MaxWorkTimeMin) + "の値にしてください。")
			}
		} else if strings.HasPrefix(str, WorkTimeOptionShortPrefixLegacy) && !isWorkTimeMinSet {
			num, err := strconv.Atoi(strings.TrimLeft(str, WorkTimeOptionShortPrefixLegacy))
			if err != nil { // 無効な値
				return InOptions{}, customerror.InvalidCommand.New("「" + WorkTimeOptionShortPrefixLegacy + "」の後の値を確認してください。")
			}
			if s.MinWorkTimeMin <= num && num <= s.MaxWorkTimeMin {
				workTimeMin = num
				isWorkTimeMinSet = true
			} else { // 無効な値
				return InOptions{}, customerror.InvalidCommand.New("作業時間（分）は" + strconv.Itoa(s.MinWorkTimeMin) + "～" + strconv.Itoa(s.MaxWorkTimeMin) + "の値にしてください。")
			}
		}
	}
	return InOptions{
		SeatId:   -1,
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
		MyOptions: options,
	}, customerror.NewNil()
}

func (s *System) ParseMyOptions(commandSlice []string) ([]MyOption, customerror.CustomError) {
	isRankVisibleSet := false
	
	var options []MyOption

	for _, str := range commandSlice {
		if strings.HasPrefix(str, RankVisibleMyOptionPrefix) && !isRankVisibleSet {
			var rankVisible bool
			rankVisibleStr := strings.TrimLeft(str, RankVisibleMyOptionPrefix)
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
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+
			"さん、すでに入室しています。まず「"+OutCommand+"」で退室してください。", ctx)
		return nil
	}

	// 席を指定している場合
	if command.CommandType == SeatIn {
		// 指定された座席番号が有効かチェック
		switch seatId := command.InOptions.SeatId; seatId {
		case 0:
			break
		default:
			// その席番号が存在するか
			isSeatExist, err := s.IsSeatExist(seatId, ctx)
			if err != nil {
				s.SendLiveChatMessage(s.ProcessedUserDisplayName+
					"さん、エラーが発生しました。もう一度試してみてください。", ctx)
				_ = s.LineBot.SendMessageWithError("failed s.IsSeatExist()", err)
				return err
			} else if !isSeatExist {
				s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、その番号の席は"+
					"存在しません。他の空いている席を選ぶか、「"+InCommand+"」で席を指定せずに入室してください！", ctx)
				return nil
			}
			// その席が空いているか
			isOk, err := s.IfSeatAvailable(seatId, ctx)
			if err != nil {
				_ = s.LineBot.SendMessageWithError("failed s.IfSeatAvailable()", err)
				s.SendLiveChatMessage(s.ProcessedUserDisplayName+
					"さん、エラーが発生しました。もう一度試してみてください。", ctx)
				return err
			}
			if !isOk {
				s.SendLiveChatMessage(s.ProcessedUserDisplayName+
					"さん、その席には今は座れません！空いている座席の番号を書いてください！", ctx)
				return nil
			}
		}
	}

	// 席を指定していない場合
	if command.CommandType == In {
		seatId, err := s.RandomAvailableSeatId(ctx)
		if err != nil {
			s.SendLiveChatMessage(s.ProcessedUserDisplayName+
				"さん、エラーが発生しました。もう一度試してみてください。", ctx)
			return err
		}
		command.InOptions.SeatId = seatId
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
	if command.InOptions.SeatId == 0 {
		err = s.EnterNoSeatRoom(command.InOptions.WorkName, command.InOptions.WorkMin, seatColorCode, ctx)
	} else {
		err = s.EnterDefaultRoom(command.InOptions.SeatId, command.InOptions.WorkName, command.InOptions.WorkMin, seatColorCode, ctx)
	}
	if err != nil {
		_ = s.LineBot.SendMessageWithError("failed to enter room", err)
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+
			"さん、エラーが発生しました。もう一度試してみてください。", ctx)
		return err
	}
	s.SendLiveChatMessage(s.ProcessedUserDisplayName+
		"さんが作業を始めました！（最大"+strconv.Itoa(command.InOptions.WorkMin)+"分）", ctx)

	// 入室時刻を記録
	err = s.FirestoreController.SetLastEnteredDate(s.ProcessedUserId, ctx)
	if err != nil {
		_ = s.LineBot.SendMessageWithError("failed to set last entered date", err)
		return err
	}
	return nil
}

func (s *System) Out(command CommandDetails, ctx context.Context) error {
	// 今勉強中か？
	isInRoom, err := s.IsUserInRoom(ctx)
	if err != nil {
		_ = s.LineBot.SendMessageWithError("failed IsUserInRoom()", err)
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください。", ctx)
		return err
	}
	if !isInRoom {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、すでに退室してます！", ctx)
		return nil
	}
	// 現在座っている席を特定
	seatId, customErr := s.CurrentSeatId(ctx)
	if customErr.Body != nil {
		if customErr.ErrorType == customerror.UserNotInAnyRoom { // おそらくここには到達しない
			s.SendLiveChatMessage(s.ProcessedUserDisplayName+
				"さん、あなたは今ルーム内にはいません。", ctx)
			return nil
		} else {
			s.SendLiveChatMessage(s.ProcessedUserDisplayName+
				"さん、残念ながらエラーが発生しました。もう一度試してみてください。", ctx)
			return customErr.Body
		}
	}
	// 退室処理
	workedTimeSec, err := s.ExitRoom(seatId, ctx)
	if err != nil {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さん、エラーが発生しました。もう一度試してみてください。", ctx)
		return err
	} else {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さんが退室しました！"+
			"（"+strconv.Itoa(workedTimeSec/60)+"分）", ctx)
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
		liveChatMessage += s.ProcessedUserDisplayName+
			"さんの本日の作業時間は"+dailyTotalTimeStr+"、"+
			"累計作業時間は"+totalTimeStr+"です。"
		
		if command.InfoOption.ShowDetails {
			userDoc, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
			if err != nil {
				_ = s.LineBot.SendMessageWithError("failed fetch user doc", err)
				return err
			}
			switch userDoc.RankVisible {
			case true:
				liveChatMessage += "また、ランク表示はオンです。"
			case false:
				liveChatMessage += "また、ランク表示はオフです。"
			}
		}
		s.SendLiveChatMessage(liveChatMessage, ctx)
	} else {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName+
			"さんはまだ作業データがありません。「"+InCommand+"」コマンドで作業を始めましょう！", ctx)
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
	if !isUserRegistered {	// ない場合は作成。
		err := s.InitializeUser(ctx)
		if err != nil {
			return err
		}
	}
	
	// オプションが1つ以上指定されているか？
	if len(command.MyOptions) == 0 {
		s.SendLiveChatMessage(s.ProcessedUserDisplayName + "さん、オプションが正しく設定されているか確認してください。", ctx)
		return nil
	}
	
	for _, myOption := range command.MyOptions {
		if myOption.Type == RankVisible {
			err := s.FirestoreController.SetMyRankVisible(s.ProcessedUserId, myOption.BoolValue, ctx)
			if err != nil {
				_ = s.LineBot.SendMessageWithError("failed to set my-rank-visible", err)
				s.SendLiveChatMessage(s.ProcessedUserDisplayName+
					"さん、エラーが発生しました。もう一度試してみてください。", ctx)
				return err
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
		}
	}
	s.SendLiveChatMessage(s.ProcessedUserDisplayName + "さんのmy設定を更新しました。", ctx)
	return nil
}

// IfSeatAvailable 席番号がseatIdの席が空いているかどうか。seatIdは存在するという前提
func (s *System) IfSeatAvailable(seatId int, ctx context.Context) (bool, error) {
	defaultRoomData, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
	if err != nil {
		return false, err
	}
	for _, seat := range defaultRoomData.Seats {
		if seat.SeatId == seatId {
			return false, nil
		}
	}
	// ここまで来ると指定された番号の席が使われていないということ
	return true, nil
}

// IsUserInRoom そのユーザーがルーム内にいるか？登録済みかに関わらず。
func (s *System) IsUserInRoom(ctx context.Context) (bool, error) {
	defaultRoomData, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
	if err != nil {
		return false, err
	}
	for _, seatInDefaultRoom := range defaultRoomData.Seats {
		if seatInDefaultRoom.UserId == s.ProcessedUserId {
			return true, nil
		}
	}

	noSeatRoomData, err := s.FirestoreController.RetrieveNoSeatRoom(ctx)
	if err != nil {
		return false, err
	}
	for _, seatInNoSeatRoom := range noSeatRoomData.Seats {
		if seatInNoSeatRoom.UserId == s.ProcessedUserId {
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

// EnterDefaultRoom default-roomに入室させる。事前チェックはされている前提。
func (s *System) EnterDefaultRoom(seatId int, workName string, workTimeMin int, seatColorCode string, ctx context.Context) error {
	enterDate := utils.JstNow()
	exitDate := enterDate.Add(time.Duration(workTimeMin) * time.Minute)
	seat, err := s.FirestoreController.SetSeatInDefaultRoom(seatId, workName, enterDate, exitDate, seatColorCode, s.ProcessedUserId, s.ProcessedUserDisplayName, ctx)
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

// EnterNoSeatRoom no-seat-roomに入室させる。事前チェックはされている前提。
func (s *System) EnterNoSeatRoom(workName string, workTimeMin int, seatColorCode string, ctx context.Context) error {
	enterDate := utils.JstNow()
	exitDate := enterDate.Add(time.Duration(workTimeMin) * time.Minute)
	seat, err := s.FirestoreController.SetSeatInNoSeatRoom(workName, enterDate, exitDate, seatColorCode, s.ProcessedUserId, s.ProcessedUserDisplayName, ctx)
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

// RandomAvailableSeatId default-roomの席が空いているならその中からランダムな席番号を、空いていないなら0を返す。
func (s *System) RandomAvailableSeatId(ctx context.Context) (int, error) {
	roomLayout, err := s.FirestoreController.RetrieveDefaultRoomLayout(ctx)
	if err != nil {
		return 0, err
	}
	defaultRoom, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
	if err != nil {
		return 0, err
	}

	var availableSeatIdList []int
	for _, seatInLayout := range roomLayout.Seats {
		isUsed := false
		for _, seatInUse := range defaultRoom.Seats {
			if seatInLayout.Id == seatInUse.SeatId {
				isUsed = true
				break
			}
		}
		if !isUsed {
			availableSeatIdList = append(availableSeatIdList, seatInLayout.Id)
		}
	}

	if len(availableSeatIdList) > 0 {
		rand.Seed(utils.JstNow().UnixNano())
		return availableSeatIdList[rand.Intn(len(availableSeatIdList))], nil
	} else {
		return 0, nil
	}
}

// ExitRoom ユーザーを退室させる。事前チェックはされている前提。
func (s *System) ExitRoom(seatId int, ctx context.Context) (int, error) {
	// 作業時間を計算
	userData, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
	if err != nil {
		return 0, err
	}
	workedTimeSec := int(utils.JstNow().Sub(userData.LastEntered).Seconds())

	var seat myfirestore.Seat
	switch seatId {
	case 0:
		noSeatRoom, err := s.FirestoreController.RetrieveNoSeatRoom(ctx)
		if err != nil {
			return 0, err
		}
		for _, seatInNoSeatRoom := range noSeatRoom.Seats {
			if seatInNoSeatRoom.UserId == s.ProcessedUserId {
				seat = seatInNoSeatRoom
			}
		}
		err = s.FirestoreController.UnSetSeatInNoSeatRoom(seat, ctx)
		if err != nil {
			return 0, err
		}
	default:
		defaultSeatRoom, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
		if err != nil {
			return 0, err
		}
		for _, seatDefaultRoom := range defaultSeatRoom.Seats {
			if seatDefaultRoom.UserId == s.ProcessedUserId {
				seat = seatDefaultRoom
			}
		}
		err = s.FirestoreController.UnSetSeatInDefaultRoom(seat, ctx)
		if err != nil {
			return 0, err
		}
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
	err = s.UpdateTotalWorkTime(workedTimeSec, ctx)
	if err != nil {
		_ = s.LineBot.SendMessageWithError("failed to update total study time", err)
		return 0, err
	}
	
	log.Println(s.ProcessedUserId + " exited the room. seat id: " + strconv.Itoa(seatId))
	return workedTimeSec, nil
}

func (s *System) CurrentSeatId(ctx context.Context) (int, customerror.CustomError) {
	// ますは Default room にいるかどうか
	defaultRoomData, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
	if err != nil {
		return 0, customerror.Unknown.Wrap(err)
	}
	for _, seat := range defaultRoomData.Seats {
		if seat.UserId == s.ProcessedUserId {
			return seat.SeatId, customerror.NewNil()
		}
	}
	// default room にいなければ、no-seat-room　にいるかどうか
	noSeatRoomData, err := s.FirestoreController.RetrieveNoSeatRoom(ctx)
	if err != nil {
		return 0, customerror.Unknown.Wrap(err)
	}
	for _, seat := range noSeatRoomData.Seats {
		if seat.UserId == s.ProcessedUserId {
			return 0, customerror.NewNil()
		}
	}
	// default-roomにもno-seat-roomにもいない
	return -1, customerror.UserNotInAnyRoom.New("the user is not in any room.")
}

func (s *System) IsSeatExist(seatId int, ctx context.Context) (bool, error) {
	// room-layoutを読み込む
	roomLayout, err := s.FirestoreController.RetrieveDefaultRoomLayout(ctx)
	if err != nil {
		return false, err
	}
	for _, seat := range roomLayout.Seats {
		if seat.Id == seatId {
			return true, nil
		}
	}
	return false, nil
}

func (s *System) UpdateTotalWorkTime(workedTimeSec int, ctx context.Context) error {
	userData, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
	if err != nil {
		return err
	}
	previousTotalSec := userData.TotalStudySec
	previousDailyTotalSec := userData.DailyTotalStudySec
	newTotalSec := previousTotalSec + workedTimeSec
	newDailyTotalSec := previousDailyTotalSec + workedTimeSec
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

func (s *System) TotalStudyTimeStrings(ctx context.Context) (string, string, error) {
	userData, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
	if err != nil {
		return "", "", err
	}
	// 累計
	var totalStr string
	totalDuration := time.Duration(userData.TotalStudySec) * time.Second
	if totalDuration < time.Hour {
		totalStr = strconv.Itoa(int(totalDuration.Minutes())) + "分"
	} else {
		totalStr = strconv.Itoa(int(totalDuration.Hours())) + "時間" +
			strconv.Itoa(int(totalDuration.Minutes())%60) + "分"
	}
	// 当日の累計
	var dailyTotalStr string
	dailyTotalDuration := time.Duration(userData.DailyTotalStudySec) * time.Second
	if dailyTotalDuration < time.Hour {
		dailyTotalStr = strconv.Itoa(int(dailyTotalDuration.Minutes())) + "分"
	} else {
		dailyTotalStr = strconv.Itoa(int(dailyTotalDuration.Hours())) + "時間" +
			strconv.Itoa(int(dailyTotalDuration.Minutes())%60) + "分"
	}
	return totalStr, dailyTotalStr, nil
}

// ExitAllUserDefaultRoom default-roomの全てのユーザーを退室させる。
func (s *System) ExitAllUserDefaultRoom(ctx context.Context) error {
	defaultRoom, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
	if err != nil {
		return err
	}
	for _, seat := range defaultRoom.Seats {
		s.ProcessedUserId = seat.UserId
		s.ProcessedUserDisplayName = seat.UserDisplayName
		_, err := s.ExitRoom(seat.SeatId, ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// ExitAllUserNoSeatRoom no-seat-roomの全てのユーザーを退室させる。
func (s *System) ExitAllUserNoSeatRoom(ctx context.Context) error {
	noSeatRoom, err := s.FirestoreController.RetrieveNoSeatRoom(ctx)
	if err != nil {
		return err
	}
	for _, seat := range noSeatRoom.Seats {
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

// OrganizeDatabase untilを過ぎているdefaultルーム内のユーザーを退室させる。
func (s *System) OrganizeDatabase(ctx context.Context) error {
	defaultRoom, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
	if err != nil {
		return err
	}
	for _, seat := range defaultRoom.Seats {
		if seat.Until.Before(utils.JstNow()) {
			s.ProcessedUserId = seat.UserId
			s.ProcessedUserDisplayName = seat.UserDisplayName
			
			workedTimeSec, err := s.ExitRoom(seat.SeatId, ctx)
			if err != nil {
				_ = s.LineBot.SendMessageWithError(s.ProcessedUserDisplayName+"さん（" + s.ProcessedUserId + "）の退室処理中にエラーが発生しました。", err)
				return err
			} else {
				s.SendLiveChatMessage(s.ProcessedUserDisplayName+"さんが退室しました！"+
					"（"+strconv.Itoa(workedTimeSec/60)+"分）", ctx)
				return nil
			}
		}
	}

	// no-seat-roomも同様。
	noSeatRoom, err := s.FirestoreController.RetrieveNoSeatRoom(ctx)
	if err != nil {
		return err
	}
	for _, seat := range noSeatRoom.Seats {
		if seat.Until.Before(utils.JstNow()) {
			s.ProcessedUserId = seat.UserId
			s.ProcessedUserDisplayName = seat.UserDisplayName
			_, err := s.ExitRoom(seat.SeatId, ctx)
			if err != nil {
				return err
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
	previousDate := constantsConfig.LastResetDailyTotalStudySec.Local()
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
		_ = s.LineBot.SendMessage("successfully reset all user's daily total study time.")
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
