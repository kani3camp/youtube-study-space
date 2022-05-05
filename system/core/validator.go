package core

import (
	"app.modules/core/customerror"
	"strconv"
)

func (s *System) ValidateIn(command CommandDetails) (bool, customerror.CustomError) {
	if command.CommandType != In {
		return false, customerror.InvalidParsedCommand.New("this is not a In command.")
	}
	
	// 作業時間の値
	inputWorkMin := command.InOption.WorkMin
	if inputWorkMin != 0 {
		expect := s.Constants.MinWorkTimeMin <= inputWorkMin && inputWorkMin <= s.Constants.MaxWorkTimeMin
		if !expect {
			return false, customerror.InvalidCommand.New("作業時間（分）は" + strconv.Itoa(s.Constants.MinWorkTimeMin) + "～" + strconv.Itoa(s.Constants.MaxWorkTimeMin) + "の値にしてください。")
		}
	}
	
	// 作業名は特に制限はない
	
	return true, customerror.NewNil()
}

// ValidateSeatIn 返却のcerrはTypeがInvalidCommandの場合のみユーザー向けにメッセージを流すこと。
// Unknownだったらエラーメッセージをチャットに流さないように。
func (s *System) ValidateSeatIn(command CommandDetails) (bool, customerror.CustomError) {
	if command.CommandType != SeatIn {
		return false, customerror.InvalidParsedCommand.New("this is not a SeatIn command.")
	}
	
	// 作業時間の値
	inputWorkMin := command.InOption.WorkMin
	if inputWorkMin != 0 {
		expect := s.Constants.MinWorkTimeMin <= inputWorkMin && inputWorkMin <= s.Constants.MaxWorkTimeMin
		if !expect {
			return false, customerror.InvalidCommand.New("作業時間（分）は" + strconv.Itoa(s.Constants.MinWorkTimeMin) + "～" + strconv.Itoa(s.Constants.MaxWorkTimeMin) + "の値にしてください。")
		}
	}
	
	// 席番号
	if command.InOption.SeatId < 0 {
		return false, customerror.InvalidCommand.New("座席番号は0以上の値にしてください。")
	}
	
	// 作業名は特に制限はない
	
	return true, customerror.NewNil()
}

func (s *System) ValidateInfo(command CommandDetails) (bool, customerror.CustomError) {
	if command.CommandType != Info {
		return false, customerror.InvalidParsedCommand.New("this is not a Info command.")
	}
	
	// 特になし
	
	return true, customerror.NewNil()
}

func (s *System) ValidateMy(command CommandDetails) (bool, customerror.CustomError) {
	if command.CommandType != My {
		return false, customerror.InvalidParsedCommand.New("this is not a My command.")
	}
	
	var isRankVisibleSet, isDefaultStudyMinSet, isFavoriteColorSet bool
	
	for _, option := range command.MyOptions {
		switch option.Type {
		case RankVisible:
			if isRankVisibleSet {
				return false, customerror.InvalidParsedCommand.New("more than 2 RankVisible options.")
			}
			isRankVisibleSet = true
		case DefaultStudyMin:
			if isDefaultStudyMinSet {
				return false, customerror.InvalidParsedCommand.New("more than 2 DefaultStudyMin options.")
			}
			inputDefaultStudyMin := option.IntValue
			if inputDefaultStudyMin != 0 {
				expect := s.Constants.MinWorkTimeMin <= inputDefaultStudyMin && inputDefaultStudyMin <= s.Constants.MaxWorkTimeMin
				if !expect {
					return false, customerror.InvalidCommand.New("作業時間（分）は" + strconv.Itoa(s.Constants.MinWorkTimeMin) + "～" + strconv.Itoa(s.Constants.MaxWorkTimeMin) + "の値にしてください。")
				}
			}
			isDefaultStudyMinSet = true
		case FavoriteColor:
			if isFavoriteColorSet {
				return false, customerror.InvalidParsedCommand.New("more than 2 FavoriteColor options.")
			}
			// 「color=」、つまり空欄の場合は-1として扱う。
			expect := option.IntValue == -1 || 0 <= option.IntValue
			if !expect {
				return false, customerror.InvalidParsedCommand.New(FavoriteColorMyOptionPrefix + "の値は設定したい色になる累計時間（0以上）を指定してください。リセットする場合は空欄にしてください。")
			}
			isFavoriteColorSet = true
		default:
			return false, customerror.InvalidParsedCommand.New("there is an unknown option in command.MyOptions")
		}
	}
	return true, customerror.NewNil()
}

func (s *System) ValidateKick(command CommandDetails) (bool, customerror.CustomError) {
	if command.CommandType != Kick {
		return false, customerror.InvalidParsedCommand.New("this is not a Kick command.")
	}
	
	// 指定座席番号
	if command.KickOption.SeatId <= 0 {
		return false, customerror.InvalidCommand.New("席番号は1以上にしてください。")
	}
	
	return true, customerror.NewNil()
}

func (s *System) ValidateCheck(command CommandDetails) (bool, customerror.CustomError) {
	if command.CommandType != Check {
		return false, customerror.InvalidParsedCommand.New("this is not a Check command.")
	}
	
	// 指定座席番号
	if command.CheckOption.SeatId <= 0 {
		return false, customerror.InvalidCommand.New("席番号は1以上にしてください。")
	}
	
	return true, customerror.NewNil()
}

func (s *System) ValidateReport(command CommandDetails) (bool, customerror.CustomError) {
	if command.CommandType != Report {
		return false, customerror.InvalidParsedCommand.New("this is not a Report command.")
	}
	
	// 空欄でないか
	if command.ReportOption.Message == "" {
		return false, customerror.InvalidCommand.New(ReportCommand + "の右にスペースを空けてメッセージを書いてください。")
	}
	
	return true, customerror.NewNil()
}

func (s *System) ValidateChange(command CommandDetails) (bool, customerror.CustomError) {
	if command.CommandType != Change {
		return false, customerror.InvalidParsedCommand.New("this is not a Change command.")
	}
	
	var isWorkNameSet, isWorkTimeMinSet bool
	
	for _, option := range command.ChangeOptions {
		switch option.Type {
		case WorkName:
			if isWorkNameSet {
				return false, customerror.InvalidParsedCommand.New("more than 2 WorkName options.")
			}
			isWorkNameSet = true
		case WorkTime:
			if isWorkTimeMinSet {
				return false, customerror.InvalidParsedCommand.New("more than 2 WorkTime options.")
			}
			inputWorkTimeMin := option.IntValue
			expect := s.Constants.MinWorkTimeMin <= inputWorkTimeMin && inputWorkTimeMin <= s.Constants.MaxWorkTimeMin
			if !expect {
				return false, customerror.InvalidCommand.New("作業時間（分）は" + strconv.Itoa(s.Constants.MinWorkTimeMin) + "～" + strconv.Itoa(s.Constants.MaxWorkTimeMin) + "の値にしてください。")
			}
			isWorkTimeMinSet = true
		default:
			return false, customerror.InvalidParsedCommand.New("there is an unknown option in command.ChangeOptions")
		}
	}
	return true, customerror.NewNil()
}

func (s *System) ValidateMore(command CommandDetails) (bool, customerror.CustomError) {
	if command.CommandType != More {
		return false, customerror.InvalidParsedCommand.New("this is not a More command.")
	}
	
	// 時間オプション
	if command.MoreOption.DurationMin <= 0 {
		return false, customerror.InvalidCommand.New("延長時間（分）は1以上の値にしてください。")
	}
	
	return true, customerror.NewNil()
}

func (s *System) ValidateBreak(command CommandDetails) (bool, customerror.CustomError) {
	if command.CommandType != Break {
		return false, customerror.InvalidParsedCommand.New("this is not a More command.")
	}
	
	// 休憩内容
	// 特になし
	
	// 休憩時間
	inputDurationMin := command.BreakOption.DurationMin
	if inputDurationMin != 0 {
		expect := s.Constants.MinBreakDurationMin <= inputDurationMin && inputDurationMin <= s.Constants.MaxBreakDurationMin
		if !expect {
			return false, customerror.InvalidCommand.New("休憩時間（分）は" + strconv.Itoa(s.Constants.MinBreakDurationMin) + "～" + strconv.Itoa(s.Constants.MaxBreakDurationMin) + "の値にしてください。")
		}
	}
	
	return true, customerror.NewNil()
}

func (s *System) ValidateResume(command CommandDetails) (bool, customerror.CustomError) {
	if command.CommandType != Resume {
		return false, customerror.InvalidParsedCommand.New("this is not a Resume command.")
	}
	
	// 作業名
	// 特になし
	
	return true, customerror.NewNil()
}
