package system

import (
	"app.modules/system/customerror"
	"app.modules/system/myfirestore"
	"app.modules/system/youtubebot"
	"context"
	"google.golang.org/api/option"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

const (
	InCommand = "!in"
	OutCommand = "!out"
	InfoCommand = "!info"
	CommandPrefix = "!"

	WorkNameOptionPrefix = "work-"
	WorkTimeOptionPrefix = "min-"

	MinWorkTime = 5
	MaxWorkTime = 360

	FullWidthSpace = "　"
	HalfWidthSpace = " "

)

type System struct {
	FirestoreController *myfirestore.FirestoreController
	LiveChatBot *youtubebot.YoutubeLiveChatBot
	ProcessedUserId string
	ProcessedUserDisplayName string
}

func NewSystem(ctx context.Context, clientOption option.ClientOption) (System, error) {
	fsController, err := myfirestore.NewFirestoreController(ctx, ProjectId, clientOption)
	if err != nil {
		return System{}, err
	}

	youtubeLiveInfo, err := fsController.RetrieveYoutubeLiveInfo(ctx)
	if err != nil {
		return System{}, err
	}
	bot, err := youtubebot.NewYoutubeLiveChatBot(youtubeLiveInfo.LiveChatId, youtubeLiveInfo.SleepIntervalMilli, ctx)
	if err != nil {
		return System{}, err
	}

	return System{
		FirestoreController: fsController,
		LiveChatBot: bot,
	}, nil
}

func (s *System) SetProcessedUser(userDisplayName string, userId string) {
	s.ProcessedUserId = userId
	s.ProcessedUserDisplayName = userDisplayName
}

// Command: 入力コマンドを解析して実行
func (s *System) Command(commandString string, userId string, userDisplayName string, ctx context.Context) error {
	if strings.HasPrefix(commandString, "!") {
		s.SetProcessedUser(userId, userDisplayName)
		slice := strings.Split(commandString, HalfWidthSpace)
		switch slice[0] {
		case InCommand:
			return s.In(commandString, ctx)
		case OutCommand:
			return s.Out(commandString, ctx)
		case InfoCommand:
			return s.ShowUserInfo(commandString, ctx)
		default:
			// !席番号
			num, err := strconv.Atoi(strings.TrimLeft(slice[0], CommandPrefix))
			if err == nil && num > 0 {
				return s.In(commandString, ctx)
			}
		}
	}
	return nil
}

func (s *System) In(commandString string, ctx context.Context) error {
	slice := strings.Split(commandString, HalfWidthSpace)

	// すでに入室している場合
	isInRoom, err := s.IsUserInRoom(ctx)
	if err != nil {
		// todo lineで通知
		return err
	}
	if isInRoom {
		s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
			"さん、すでに入室しています。まず「" + OutCommand + "」で退室してください。")
		return nil
	}

	// 席を指定しているかどうか
	var seatId int
	num, err := strconv.Atoi(strings.TrimLeft(slice[0], CommandPrefix))
	if err != nil {	// !in
		seatId, err = s.RandomAvailableSeatId(ctx)
		if err != nil {
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さん、エラーが発生しました。もう一度試してみてください。")
			return err
		}
	} else {	// 指定された座席番号が有効かチェック
		seatId = num
		// その席番号が存在するか
		isSeatExist, err := s.IsSeatExist(seatId, ctx)
		if err != nil {
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さん、エラーが発生しました。もう一度試してみてください。")
			return err
		} else if ! isSeatExist {
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さん、その番号の席は存在しません。他の空いている席を選ぶか、「" + InfoCommand + "」で席を指定せずに入室してください！")
			return nil
		}
		// その席が空いているか
		isOk, err := s.IfSeatAvailable(seatId, ctx)
		if err != nil {
			// todo lineで通知
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さん、エラーが発生しました。もう一度試してみてください。")
			return err
		}
		if ! isOk {
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さん、その席には今は座れません！空いている座席の番号を書いてください！")
			return nil
		}
	}

	// 追加オプションチェック
	workName := ""
	workTimeMin := 120
	for i, _ := range slice[1:] {
		if strings.HasPrefix(slice[i], WorkNameOptionPrefix) {
			workName = strings.TrimLeft(slice[i], WorkNameOptionPrefix)
		} else if strings.HasPrefix(slice[i], WorkTimeOptionPrefix) {
			num, err = strconv.Atoi(strings.TrimLeft(slice[i], WorkTimeOptionPrefix))
			if err != nil {
				if 5 <= num && num <= 360 {
					workTimeMin = num
				} else {
					s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
						"さん、作業時間（分）は" + strconv.Itoa(MinWorkTime) + "～" + strconv.Itoa(MaxWorkTime) + "の値にしてください。")
				}
			}
		}
	}

	// 入室
	if seatId == 0 {	// no-seat-room
		err = s.EnterNoSeatRoom(workName, workTimeMin, ctx)
	} else { // default-room
		err = s.EnterDefaultRoom(seatId, workName, workTimeMin, ctx)
	}
	if err != nil {
		// todo lineで通知
		s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
			"さん、エラーが発生しました。もう一度試してみてください。")
		return err
	} else {
		s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
			"さんが作業を始めました！（" + strconv.Itoa(workTimeMin) + "分）")
	}
	return nil
}

func (s *System) Out(commandString string, ctx context.Context) error {
	// 今勉強中か？
	isInRoom, err := s.IsUserInRoom(ctx)
	if err != nil {
		// todo lineで通知
		return err
	}
	if ! isInRoom {
		s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
			"さん、すでに退室してます！")
		return nil
	}
	// 退室処理
	workedTimeSec, err := s.ExitRoom(ctx)
	if err != nil {
		s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
			"さん、エラーが発生しました。もう一度試してみてください。")
		return err
	} else {
		// 退室時刻を記録
		err = s.FirestoreController.SetLastExitedDate(s.ProcessedUserId, ctx)
		if err != nil {
			// todo lineで通知
			return err
		}
		// 累計学習時間を更新
		err = s.UpdateTotalWorkTime(workedTimeSec, ctx)
		if err != nil {
			// todo lineで通知
			return err
		}
		return nil
	}
}

func (s *System) ShowUserInfo(commandString string, ctx context.Context) error {
	// todo そのユーザーはデータがあるか？
	// todo 情報を返信
}

// IfSeatAvailable: 席番号がseatIdの席が空いているかどうか。seatIdは存在するという前提
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

func (s *System) IsUserInRoom(ctx context.Context) (bool, error) {
	defaultRoomData, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
	if err != nil {
		return false, err
	}
	for _, seatInDefaultRoom := range defaultRoomData.Seats {
		if seatInDefaultRoom.UserId == s.ProcessedUserId  {
			return true, nil
		}
	}

	noSeatRoomData, err := s.FirestoreController.RetrieveNoSeatRoom(ctx)
	if err != nil {
		return false, err
	}
	for _, seatInNoSeatRoom := range noSeatRoomData.Seats {
		if seatInNoSeatRoom.UserId == s.ProcessedUserId  {
			return true, nil
		}
	}
	return false, nil
}

func (s *System) RetrieveYoutubeLiveInfo(ctx context.Context) (myfirestore.YoutubeLiveDoc, error) {
	return s.FirestoreController.RetrieveYoutubeLiveInfo(ctx)
}

func (s *System) RetrieveNextPageToken(ctx context.Context) (string, error) {
	return s.FirestoreController.RetrieveNextPageToken(ctx)
}

func (s *System) SaveNextPageToken(nextPageToken string, ctx context.Context) error {
	return s.FirestoreController.SaveNextPageToken(nextPageToken, ctx)
}

func (s *System) EnterDefaultRoom(seatId int, workName string, workTimeMin int, ctx context.Context) error {
	exitDate := time.Now().Add(time.Duration(workTimeMin) * time.Minute)
	seat, err := s.FirestoreController.SetSeatInDefaultRoom(seatId, workName, exitDate, s.ProcessedUserId, ctx)
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

func (s *System) EnterNoSeatRoom(workName string, workTimeMin int, ctx context.Context) error {
	exitDate := time.Now().Add(time.Duration(workTimeMin) * time.Minute)
	seat, err := s.FirestoreController.SetSeatInNoSeatRoom(workName, exitDate, s.ProcessedUserId, ctx)
	if err != nil {
		return err
	}
	// 退室時刻を記録
	err = s.FirestoreController.SetLastExitedDate(s.ProcessedUserId, ctx)
	if err != nil {
		return err
	}
	// ログ記録
	err = s.FirestoreController.AddUserHistory(s.ProcessedUserId, ExitAction, seat, ctx)
	return nil
}

func (s *System) RandomAvailableSeatId(ctx context.Context) (int, error) {
	defaultRoomData, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
	if err != nil {
		return 0, err
	}
	var availableSeatIdList []int
	for _, seat := range defaultRoomData.Seats {
		if seat.UserId == "" {
			availableSeatIdList = append(availableSeatIdList, seat.SeatId)
		}
	}
	if len(availableSeatIdList) > 0 {
		rand.Seed(time.Now().UnixNano())
		return availableSeatIdList[rand.Intn(len(availableSeatIdList))], nil
	} else {
		return 0, nil
	}
}

func (s *System) ExitRoom(ctx context.Context) (int, error) {
	seatId, customErr := s.CurrentSeatId(ctx)
	if customErr.Body != nil {
		if customErr.ErrorType == customerror.UserNotInAnyRoom {
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さん、あなたは今ルーム内にはいません。")
			return 0, nil
		} else {
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さん、残念ながらエラーが発生しました。もう一度試してみてください。")
			return 0, customErr.Body
		}
	}
	switch seatId {
	case 0:
		noSeatRoom, err := s.FirestoreController.RetrieveNoSeatRoom(ctx)
		if err != nil {
			return 0, err
		}
		var seat myfirestore.Seat
		for _, seatInNoSeatRoom := range noSeatRoom.Seats {
			if seatInNoSeatRoom.UserId == s.ProcessedUserId {
				seat = seatInNoSeatRoom
			}
		}
		// 作業時間を計算
		userData, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
		if err != nil {
			return 0, err
		}
		workedTimeSec := int(time.Now().Sub(userData.LastEntered).Seconds())
		return workedTimeSec, s.FirestoreController.UnSetSeatInNoSeatRoom(seat, ctx)
	default:
		defaultSeatRoom, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
		if err != nil {
			return 0, err
		}
		var seat myfirestore.Seat
		for _, seatDefaultRoom := range defaultSeatRoom.Seats {
			if seatDefaultRoom.UserId == s.ProcessedUserId {
				seat = seatDefaultRoom
			}
		}
		// 作業時間を計算
		userData, err := s.FirestoreController.RetrieveUser(s.ProcessedUserId, ctx)
		if err != nil {
			return 0, err
		}
		workedTimeSec := int(time.Now().Sub(userData.LastEntered).Seconds())
		return workedTimeSec, s.FirestoreController.UnSetSeatInDefaultRoom(seat, ctx)
	}
}

func (s *System) CurrentSeatId(ctx context.Context) (int, customerror.CustomError) {
	// ますは Default room にいるかどうか
	defaultRoomData, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
	if err != nil {
		return 0, customerror.Unknown.Wrap(err)
	}
	for _, seat := range defaultRoomData.Seats {
		if seat.UserId == s.ProcessedUserId {
			return seat.SeatId, customerror.NewNilCustomError()
		}
	}
	// default room にいなければ、no-seat-room　にいるかどうか
	noSeatRoomData, err := s.FirestoreController.RetrieveNoSeatRoom(ctx)
	if err != nil {
		return 0, customerror.Unknown.Wrap(err)
	}
	for _, seat := range noSeatRoomData.Seats {
		if seat.UserId == s.ProcessedUserId {
			return 0, customerror.NewNilCustomError()
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










