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
	TimeLimitCommand = "!tl"
	InfoCommand = "!info"

	FullWidthSpace = "　"
	HalfWidthSpace = " "

	InCommandExample = "「!in 席番号 作業名 」（末尾にコメントをつける場合は半角スペースを入れてください）"
	OutCommandExample = "「!out 」（末尾にコメントをつける場合は半角スペースを入れてください）"
	TimeLimitCommandExample = "「!tl タイムリミット分 」（末尾にコメントをつける場合は半角スペースを入れてください）"
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
		case TimeLimitCommand:
			return s.SetTimeLimit(commandString, ctx)
		case InfoCommand:
			return s.ShowUserInfo(commandString, ctx)
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
			"さん、すでに入室しています。まず" + OutCommandExample + "で退室してください。")
		return nil
	}

	// 要素数チェック
	if len(slice) < 3 {
		if strings.Contains(commandString, FullWidthSpace) {
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さん、全角スペースで区切ってませんか？入室コマンドは" + InCommandExample + "のように書いてみてください。")
		} else {
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さん、入室コマンドは" + InCommandExample + "のように書いてみてください。")

		}
		return nil
	}

	// 席番号チェック
	var seatId int
	if slice[1] == "any" {	// 適当な席を希望
		seatId, err = s.RandomAvailableSeatId(ctx)
		if err != nil {
			// todo lineで通知
			return err
		}
	} else {
		seatId, err = strconv.Atoi(slice[1])
		if err != nil {
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さん、座席番号が無効です！半角数字で書いてみてください！")
			return nil
		}
		if seatId < 0 {
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さん、座席番号が無効です！空いている座席の番号を書いてください！")
			return nil
		}
	}

	// 作業名チェックはいらなそう。

	// 座席に座るか座らないか
	if seatId == 0 {	// no-seat-room
		err := s.EnterNoSeatRoom(ctx)
		if err != nil {
			// todo lineで通知
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さん、エラーが発生しました。もう一度試してみてください。")
		} else {
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さんが入室しました！")
		}

	} else {	// default-room
		isOk, customErr := s.IfSeatAvailable(seatId, ctx)
		if customErr.Body != nil {
			if customErr.ErrorType == customerror.SeatNotFound {
				s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
					"さん、その番号の席は存在しません。他の空いている席を選ぶか、席番号を0にして席に座らずに作業を始めましょう！")
				return nil
			} else {
				// todo lineで通知
				return err
			}
		}
		if !isOk {
			err := s.EnterDefaultRoom(seatId, ctx)
			if err != nil {
				// todo lineで通知
				s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
					"さん、エラーが発生しました。もう一度試してみてください。")
			} else {
				s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
					"さんが入室しました！")
			}
		} else {
			s.LiveChatBot.PostMessage(s.ProcessedUserDisplayName +
				"さん、その席は今は使えません！他の座席を指定してみてください！")
		}
	}
	return nil
}

func (s *System) Out(commandString string, ctx context.Context) error {
	slice := strings.Split(commandString, HalfWidthSpace)

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

	// todo 退室処理
	// todo 退室時刻を記録するとともに、合計学習時間を更新
}

func (s *System) SetTimeLimit(commandString string, ctx context.Context) error {
	slice := strings.Split(commandString, HalfWidthSpace)

}

func (s *System) ShowUserInfo(commandString string, ctx context.Context) error {
	slice := strings.Split(commandString, HalfWidthSpace)

	// todo そのユーザーはデータがあるか？
}


func (s *System) IfSeatAvailable(seatId int, ctx context.Context) (bool, customerror.CustomError) {
	defaultRoomData, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
	if err != nil {
		return false, customerror.Unknown.Wrap(err)
	}
	for _, seat := range defaultRoomData.Seats {
		if seat.SeatId == seatId {
			if seat.UserId == "" {
				return true, customerror.CustomError{}
			} else {
				return false, customerror.CustomError{}
			}
		}
	}
	// ここまで来ると指定された番号の席がないということ
	return false, customerror.SeatNotFound.New("seat not found.")
}

func (s *System) IsUserInRoom(ctx context.Context) (bool, error) {
	defaultRoomData, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
	if err != nil {
		return false, err
	}
	noSeatRoomData, err := s.FirestoreController.RetrieveNoSeatRoom(ctx)
	if err != nil {
		return false, err
	}

	for _, seatInDefaultRoom := range defaultRoomData.Seats {
		if s.ProcessedUserId == seatInDefaultRoom.UserId {
			return true, nil
		}
	}
	for _, userInNoSeatRoom := range noSeatRoomData.Users {
		if s.ProcessedUserId == userInNoSeatRoom {
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

func (s *System) EnterDefaultRoom(seatId int, ctx context.Context) error {
	err := s.FirestoreController.SetUserInDefaultRoom(seatId, s.ProcessedUserId, ctx)
	if err != nil {
		return err
	}
	// 入室時刻を記録
	err = s.FirestoreController.SetLastEnteredDate(s.ProcessedUserId, ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *System) EnterNoSeatRoom(ctx context.Context) error {
	err := s.FirestoreController.SetUserInNoSeatRoom(s.ProcessedUserId, ctx)
	if err != nil {
		return err
	}
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

func (s *System) ExitRoom(ctx context.Context) error {
	seatId, err := s.
	err = s.FirestoreController.UnSetUserIn
}

func (s *System) CurrentSeatId(ctx context.Context) (int, error) {
	defaultRoomData, err := s.FirestoreController.RetrieveDefaultRoom(ctx)
	if err != nil {

	}
}


