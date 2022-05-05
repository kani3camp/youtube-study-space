package core

import (
	"app.modules/core/customerror"
	"app.modules/core/utils"
	"math"
	"strconv"
	"strings"
)

func (s *System) ParseIn(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)
	
	// 追加オプションチェック
	options, err := s.ParseInOptions(slice[1:])
	if err.IsNotNil() {
		return CommandDetails{}, err
	}
	
	return CommandDetails{
		CommandType: In,
		InOption:    options,
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
		InOption:    options,
	}, customerror.NewNil()
}

func (s *System) ParseInOptions(commandSlice []string) (InOptions, customerror.CustomError) {
	workName := ""
	isWorkNameSet := false
	workTimeMin := 0
	isWorkTimeMinSet := false
	for _, str := range commandSlice {
		if HasWorkNameOptionPrefix(str) && !isWorkNameSet {
			workName = TrimWorkNameOptionPrefix(str)
			isWorkNameSet = true
		} else if HasTimeOptionPrefix(str) && !isWorkTimeMinSet {
			durationMin, cerr := s.ParseDurationMinOption(TrimTimeOptionPrefix(str), s.Constants.MinWorkTimeMin, s.Constants.MaxWorkTimeMin)
			if cerr.IsNotNil() {
				return InOptions{}, cerr
			}
			workTimeMin = durationMin
			isWorkTimeMinSet = true
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
		MyOptions:   options,
	}, customerror.NewNil()
}

func (s *System) ParseMyOptions(commandSlice []string) ([]MyOption, customerror.CustomError) {
	isRankVisibleSet := false
	isDefaultStudyMinSet := false
	isFavoriteColorSet := false
	
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
				return []MyOption{}, customerror.InvalidCommand.New("「" + RankVisibleMyOptionPrefix + "」の後の値を確認してください")
			}
			options = append(options, MyOption{
				Type:      RankVisible,
				BoolValue: rankVisible,
			})
			isRankVisibleSet = true
		} else if HasTimeOptionPrefix(str) && !isDefaultStudyMinSet {
			// TODO: 0だったらリセット。
			
			durationMin, cerr := s.ParseDurationMinOption(TrimTimeOptionPrefix(str), s.Constants.MinWorkTimeMin, s.Constants.MaxWorkTimeMin)
			if cerr.IsNotNil() {
				return []MyOption{}, cerr
			}
			options = append(options, MyOption{
				Type:     DefaultStudyMin,
				IntValue: durationMin,
			})
			isDefaultStudyMinSet = true
		} else if strings.HasPrefix(str, FavoriteColorMyOptionPrefix) && !isFavoriteColorSet {
			var paramStr = strings.TrimPrefix(str, FavoriteColorMyOptionPrefix)
			if paramStr == "" {
				// 空文字列であればリセット
				options = append(options, MyOption{
					Type:        FavoriteColor,
					StringValue: "",
				})
				isFavoriteColorSet = true
			} else {
				// 整数に変換できるか
				num, err := strconv.Atoi(paramStr)
				if err != nil {
					return []MyOption{}, customerror.InvalidCommand.New("「" + FavoriteColorMyOptionPrefix + "」の後の値は半角数字にしてください")
				}
				if num < 0 {
					return []MyOption{}, customerror.InvalidCommand.New("「" + FavoriteColorMyOptionPrefix + "」の後の値は0以上にしてください")
				}
				colorCode := utils.TotalStudyHoursToColorCode(num)
				options = append(options, MyOption{
					Type:        FavoriteColor,
					StringValue: colorCode,
				})
				isFavoriteColorSet = true
			}
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
			return CommandDetails{}, customerror.ParseFailed.New("有効な席番号を指定してください")
		}
		kickSeatId = num
	} else {
		return CommandDetails{}, customerror.InvalidCommand.New("席番号を指定してください")
	}
	
	return CommandDetails{
		CommandType: Kick,
		KickOption: KickOption{
			SeatId: kickSeatId,
		},
	}, customerror.NewNil()
}

func (s *System) ParseCheck(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)
	
	var targetSeatId int
	if len(slice) >= 2 {
		num, err := strconv.Atoi(slice[1])
		if err != nil {
			return CommandDetails{}, customerror.InvalidCommand.New("有効な席番号を指定してください")
		}
		targetSeatId = num
	} else {
		return CommandDetails{}, customerror.InvalidCommand.New("席番号を指定してください")
	}
	
	return CommandDetails{
		CommandType: Check,
		CheckOption: CheckOption{
			SeatId: targetSeatId,
		},
	}, customerror.NewNil()
}

func (s *System) ParseReport(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)
	
	var reportMessage string
	if len(slice) == 1 {
		return CommandDetails{}, customerror.InvalidCommand.New("!reportの右にスペースを空けてメッセージを書いてください。")
	} else { // len(slice) > 1
		reportMessage = commandString
	}
	
	return CommandDetails{
		CommandType:  Report,
		ReportOption: ReportOption{Message: reportMessage},
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
	isWorkTimeMinSet := false
	
	var options []ChangeOption
	
	for _, str := range commandSlice {
		if HasWorkNameOptionPrefix(str) && !isWorkNameSet {
			workName := TrimWorkNameOptionPrefix(str)
			options = append(options, ChangeOption{
				Type:        WorkName,
				StringValue: workName,
			})
			isWorkNameSet = true
		} else if HasTimeOptionPrefix(str) && !isWorkTimeMinSet {
			// 延長できるシステムなので、上限はなし
			durationMin, cerr := s.ParseDurationMinOption(TrimTimeOptionPrefix(str), s.Constants.MinWorkTimeMin, math.MaxInt)
			if cerr.IsNotNil() {
				return []ChangeOption{}, cerr
			}
			options = append(options, ChangeOption{
				Type:     WorkTime,
				IntValue: durationMin,
			})
			isWorkTimeMinSet = true
		}
	}
	return options, customerror.NewNil()
}

func (s *System) ParseMore(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)
	isTimeOptionSet := false
	
	// 時間オプションチェック
	var durationMin int
	for _, str := range slice {
		if HasTimeOptionPrefix(str) && !isTimeOptionSet {
			var cerr customerror.CustomError
			durationMin, cerr = s.ParseDurationMinOption(TrimTimeOptionPrefix(str), s.Constants.MinWorkTimeMin, s.Constants.MaxWorkTimeMin)
			if cerr.IsNotNil() {
				return CommandDetails{}, cerr
			}
			isTimeOptionSet = true
		}
	}
	
	return CommandDetails{
		CommandType: More,
		MoreOption: MoreOption{
			DurationMin: durationMin,
		},
	}, customerror.NewNil()
}

func (s *System) ParseBreak(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)
	
	// 追加オプションチェック
	options, err := s.ParseMinutesAndWorkNameOptions(slice[1:], s.Constants.MinBreakDurationMin, s.Constants.MaxBreakDurationMin)
	if err.IsNotNil() {
		return CommandDetails{}, err
	}
	
	// 休憩時間の指定がない場合はデフォルト値を設定
	if options.DurationMin == 0 {
		options.DurationMin = s.Constants.DefaultBreakDurationMin
	}
	
	return CommandDetails{
		CommandType: Break,
		BreakOption: options,
	}, customerror.NewNil()
}

func (s *System) ParseResume(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)
	
	// 追加オプションチェック
	// 作業名オプション
	workName := s.ParseWorkNameOption(slice[1:])
	
	return CommandDetails{
		CommandType: Resume,
		ResumeOption: ResumeOption{
			WorkName: workName,
		},
	}, customerror.NewNil()
}

func (s *System) ParseWorkNameOption(commandSlice []string) string {
	for _, str := range commandSlice {
		if HasWorkNameOptionPrefix(str) {
			workName := TrimWorkNameOptionPrefix(str)
			return workName
		}
	}
	return ""
}

func (s *System) ParseDurationMinOption(str string, MinDuration, MaxDuration int) (int, customerror.CustomError) {
	num, err := strconv.Atoi(str)
	if err != nil { // 無効な値
		return 0, customerror.InvalidCommand.New("時間（分）の値を確認してください")
	}
	if MinDuration <= num && num <= MaxDuration {
		return num, customerror.NewNil()
	} else { // 無効な値
		return 0, customerror.InvalidCommand.New("時間（分）は" + strconv.Itoa(
			MinDuration) + "～" + strconv.Itoa(MaxDuration) + "の値にしてください")
	}
}

func (s *System) ParseMinutesAndWorkNameOptions(commandSlice []string, MinDuration, MaxDuration int) (MinutesAndWorkNameOption,
	customerror.CustomError) {
	isWorkNameSet := false
	isDurationMinSet := false
	
	var options MinutesAndWorkNameOption
	
	for _, str := range commandSlice {
		if (HasWorkNameOptionPrefix(str)) && !isWorkNameSet {
			workName := TrimWorkNameOptionPrefix(str)
			options.WorkName = workName
			isWorkNameSet = true
		} else if (HasTimeOptionPrefix(str)) && !isDurationMinSet {
			num, err := strconv.Atoi(TrimTimeOptionPrefix(str))
			if err != nil { // 無効な値
				return MinutesAndWorkNameOption{}, customerror.InvalidCommand.New("時間（分）の値を確認してください")
			}
			if MinDuration <= num && num <= MaxDuration {
				options.DurationMin = num
				isDurationMinSet = true
			} else { // 無効な値
				return MinutesAndWorkNameOption{}, customerror.InvalidCommand.New("時間（分）は" + strconv.Itoa(
					MinDuration) + "～" + strconv.Itoa(MaxDuration) + "の値にしてください")
			}
		}
	}
	return options, customerror.NewNil()
}
