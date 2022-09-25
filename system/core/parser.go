package core

import (
	"app.modules/core/customerror"
	"app.modules/core/i18n"
	"app.modules/core/utils"
	"strconv"
	"strings"
)

// ParseCommand コマンドを解析
func ParseCommand(commandString string) (CommandDetails, customerror.CustomError) {
	commandString = strings.Replace(commandString, utils.FullWidthSpace, utils.HalfWidthSpace, -1)
	commandString = strings.Replace(commandString, utils.FullWidthEqualSign, utils.HalfWidthEqualSign, -1)
	
	if strings.HasPrefix(commandString, utils.CommandPrefix) {
		slice := strings.Split(commandString, utils.HalfWidthSpace)
		switch slice[0] {
		case utils.InCommand:
			return ParseIn(commandString)
		case utils.OutCommand:
			return CommandDetails{
				CommandType: Out,
			}, customerror.NewNil()
		case utils.InfoCommand:
			return ParseInfo(commandString)
		case utils.MyCommand:
			return ParseMy(commandString)
		case utils.ChangeCommand:
			return ParseChange(commandString)
		case utils.SeatCommand:
			return ParseSeat(commandString)
		case utils.ReportCommand:
			return ParseReport(commandString)
		case utils.KickCommand:
			return ParseKick(commandString)
		case utils.CheckCommand:
			return ParseCheck(commandString)
		case utils.BlockCommand:
			return ParseBlock(commandString)
		
		case utils.OkawariCommand:
			fallthrough
		case utils.MoreCommand:
			return ParseMore(commandString)
		
		case utils.RestCommand:
			fallthrough
		case utils.ChillCommand:
			fallthrough
		case utils.BreakCommand:
			return ParseBreak(commandString)
		
		case utils.ResumeCommand:
			return ParseResume(commandString)
		case utils.RankCommand:
			return CommandDetails{
				CommandType: Rank,
			}, customerror.NewNil()
		case utils.CommandPrefix: // 典型的なミスコマンド「! in」「! out」とか。
			return CommandDetails{}, customerror.InvalidCommand.New(i18n.T("parse:isolated-!"))
		default: // !席番号 or 間違いコマンド
			// !席番号かどうか
			num, err := strconv.Atoi(strings.TrimPrefix(slice[0], utils.CommandPrefix))
			if err == nil {
				return ParseSeatIn(num, commandString)
			}
			
			// 間違いコマンド
			return CommandDetails{
				CommandType: InvalidCommand,
			}, customerror.NewNil()
		}
	} else if strings.HasPrefix(commandString, utils.WrongCommandPrefix) {
		return CommandDetails{}, customerror.InvalidCommand.New(i18n.T("parse:non-half-width-!"))
	}
	return CommandDetails{
		CommandType: NotCommand,
	}, customerror.NewNil()
}

func ParseIn(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, utils.HalfWidthSpace)
	
	// 追加オプションチェック
	options, err := ParseMinutesAndWorkNameOptions(slice[1:])
	if err.IsNotNil() {
		return CommandDetails{}, err
	}
	
	return CommandDetails{
		CommandType: In,
		InOption: InOption{
			IsSeatIdSet:        false,
			MinutesAndWorkName: options,
		},
	}, customerror.NewNil()
}

func ParseSeatIn(seatNum int, commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, utils.HalfWidthSpace)
	
	// 追加オプションチェック
	options, err := ParseMinutesAndWorkNameOptions(slice[1:])
	if err.IsNotNil() {
		return CommandDetails{}, err
	}
	
	return CommandDetails{
		CommandType: In,
		InOption: InOption{
			IsSeatIdSet:        true,
			SeatId:             seatNum,
			MinutesAndWorkName: options,
		},
	}, customerror.NewNil()
}

func ParseInfo(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, utils.HalfWidthSpace)
	
	showDetails := false
	if len(slice) >= 2 {
		if slice[1] == utils.ShowDetailsOption {
			showDetails = true
		}
	}
	
	return CommandDetails{
		CommandType: Info,
		InfoOption: InfoOption{
			ShowDetails: showDetails,
		},
	}, customerror.NewNil()
}

func ParseMy(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, utils.HalfWidthSpace)
	
	options, err := ParseMyOptions(slice[1:])
	if err.IsNotNil() {
		return CommandDetails{}, err
	}
	
	return CommandDetails{
		CommandType: My,
		MyOptions:   options,
	}, customerror.NewNil()
}

func ParseMyOptions(commandSlice []string) ([]MyOption, customerror.CustomError) {
	isRankVisibleSet := false
	isDefaultStudyMinSet := false
	isFavoriteColorSet := false
	
	options := make([]MyOption, 0)
	
	for _, str := range commandSlice {
		if strings.HasPrefix(str, utils.RankVisibleMyOptionPrefix) && !isRankVisibleSet {
			var rankVisible bool
			rankVisibleStr := strings.TrimPrefix(str, utils.RankVisibleMyOptionPrefix)
			if rankVisibleStr == utils.RankVisibleMyOptionOn {
				rankVisible = true
			} else if rankVisibleStr == utils.RankVisibleMyOptionOff {
				rankVisible = false
			} else {
				return []MyOption{}, customerror.InvalidCommand.New(i18n.T("parse:check-option", utils.RankVisibleMyOptionPrefix))
			}
			options = append(options, MyOption{
				Type:      RankVisible,
				BoolValue: rankVisible,
			})
			isRankVisibleSet = true
		} else if utils.HasTimeOptionPrefix(str) && !isDefaultStudyMinSet {
			var durationMin int
			// 0もしくは空欄ならリセットなので、空欄も許可。リセットは内部的には0で扱う。
			if utils.IsEmptyTimeOption(str) {
				durationMin = 0
			} else {
				var cerr customerror.CustomError
				durationMin, cerr = ParseDurationMinOption([]string{str}, false)
				if cerr.IsNotNil() {
					return []MyOption{}, cerr
				}
			}
			options = append(options, MyOption{
				Type:     DefaultStudyMin,
				IntValue: durationMin,
			})
			isDefaultStudyMinSet = true
		} else if strings.HasPrefix(str, utils.FavoriteColorMyOptionPrefix) && !isFavoriteColorSet {
			var paramStr = strings.TrimPrefix(str, utils.FavoriteColorMyOptionPrefix)
			if paramStr == "" {
				// 「color=」、つまり空欄の場合はリセット。システム内部では-1として扱う。
				options = append(options, MyOption{
					Type:     FavoriteColor,
					IntValue: -1,
				})
				isFavoriteColorSet = true
			} else {
				// 整数に変換できるか
				num, err := strconv.Atoi(paramStr)
				if err != nil {
					return []MyOption{}, customerror.InvalidCommand.New(i18n.T("parse:non-half-width-digit-option", utils.FavoriteColorMyOptionPrefix))
				}
				options = append(options, MyOption{
					Type:     FavoriteColor,
					IntValue: num,
				})
				isFavoriteColorSet = true
			}
		}
	}
	return options, customerror.NewNil()
}

func ParseKick(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, utils.HalfWidthSpace)
	
	var kickSeatId int
	if len(slice) >= 2 {
		num, err := strconv.Atoi(slice[1])
		if err != nil {
			return CommandDetails{}, customerror.ParseFailed.New(i18n.T("parse:invalid-seat-id"))
		}
		kickSeatId = num
	} else {
		return CommandDetails{}, customerror.InvalidCommand.New(i18n.T("parse:missing-seat-id"))
	}
	
	return CommandDetails{
		CommandType: Kick,
		KickOption: KickOption{
			SeatId: kickSeatId,
		},
	}, customerror.NewNil()
}

func ParseCheck(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, utils.HalfWidthSpace)
	
	var targetSeatId int
	if len(slice) >= 2 {
		num, err := strconv.Atoi(slice[1])
		if err != nil {
			return CommandDetails{}, customerror.InvalidCommand.New(i18n.T("parse:invalid-seat-id"))
		}
		targetSeatId = num
	} else {
		return CommandDetails{}, customerror.InvalidCommand.New(i18n.T("parse:missing-seat-id"))
	}
	
	return CommandDetails{
		CommandType: Check,
		CheckOption: CheckOption{
			SeatId: targetSeatId,
		},
	}, customerror.NewNil()
}

func ParseBlock(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, utils.HalfWidthSpace)
	
	var targetSeatId int
	if len(slice) >= 2 {
		num, err := strconv.Atoi(slice[1])
		if err != nil {
			return CommandDetails{}, customerror.InvalidCommand.New(i18n.T("parse:invalid-seat-id"))
		}
		targetSeatId = num
	} else {
		return CommandDetails{}, customerror.InvalidCommand.New(i18n.T("parse:missing-seat-id"))
	}
	
	return CommandDetails{
		CommandType: Block,
		BlockOption: BlockOption{
			SeatId: targetSeatId,
		},
	}, customerror.NewNil()
}

func ParseReport(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, utils.HalfWidthSpace)
	
	var reportMessage string
	if len(slice) == 1 {
		return CommandDetails{}, customerror.InvalidCommand.New(i18n.T("parse:missing-message", utils.ReportCommand))
	} else { // len(slice) > 1
		reportMessage = commandString
	}
	
	return CommandDetails{
		CommandType: Report,
		ReportOption: ReportOption{
			Message: reportMessage,
		},
	}, customerror.NewNil()
}

func ParseChange(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, utils.HalfWidthSpace)
	
	// 追加オプションチェック
	options, err := ParseMinutesAndWorkNameOptions(slice[1:])
	if err.IsNotNil() {
		return CommandDetails{}, err
	}
	
	return CommandDetails{
		CommandType:  Change,
		ChangeOption: options,
	}, customerror.NewNil()
}

func ParseSeat(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, utils.HalfWidthSpace)
	
	showDetails := false
	if len(slice) >= 2 {
		if slice[1] == utils.ShowDetailsOption {
			showDetails = true
		}
	}
	
	return CommandDetails{
		CommandType: Seat,
		SeatOption: SeatOption{
			ShowDetails: showDetails,
		},
	}, customerror.NewNil()
}

func ParseMore(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, utils.HalfWidthSpace)
	
	// 延長時間
	var durationMin int
	if len(slice) >= 2 {
		var cerr customerror.CustomError
		durationMin, cerr = ParseDurationMinOption(slice[1:], true)
		if cerr.IsNotNil() {
			return CommandDetails{}, cerr
		}
	} else {
		return CommandDetails{}, customerror.InvalidCommand.New(i18n.T("parse:missing-more-option")) // !more doesn't need 'min=' prefix.
	}
	
	return CommandDetails{
		CommandType: More,
		MoreOption: MoreOption{
			DurationMin: durationMin,
		},
	}, customerror.NewNil()
}

func ParseBreak(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, utils.HalfWidthSpace)
	
	// 追加オプションチェック
	options, cerr := ParseMinutesAndWorkNameOptions(slice[1:])
	if cerr.IsNotNil() {
		return CommandDetails{}, cerr
	}
	
	return CommandDetails{
		CommandType: Break,
		BreakOption: options,
	}, customerror.NewNil()
}

func ParseResume(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, utils.HalfWidthSpace)
	
	// 作業名
	option := ParseWorkNameOption(slice[1:])
	
	return CommandDetails{
		CommandType:  Resume,
		ResumeOption: option,
	}, customerror.NewNil()
}

func ParseWorkNameOption(strSlice []string) WorkNameOption {
	for _, str := range strSlice {
		if utils.HasWorkNameOptionPrefix(str) {
			workName := utils.TrimWorkNameOptionPrefix(str)
			return WorkNameOption{
				IsWorkNameSet: true,
				WorkName:      workName,
			}
		}
	}
	return WorkNameOption{
		IsWorkNameSet: false,
	}
}

func ParseDurationMinOption(strSlice []string, allowNonPrefix bool) (int, customerror.CustomError) {
	for _, str := range strSlice {
		if utils.HasTimeOptionPrefix(str) {
			num, err := strconv.Atoi(utils.TrimTimeOptionPrefix(str))
			if err != nil {
				return 0, customerror.InvalidCommand.New(i18n.T("parse:check-option", utils.TimeOptionPrefix))
			}
			return num, customerror.NewNil()
		} else if allowNonPrefix {
			num, err := strconv.Atoi(str)
			if err != nil {
				return num, customerror.ParseFailed.New(i18n.T("parse:invalid-option"))
			}
			return num, customerror.NewNil()
		}
	}
	return 0, customerror.InvalidCommand.New(i18n.T("parse:missing-time-option"))
}

func ParseMinutesAndWorkNameOptions(commandSlice []string) (MinutesAndWorkNameOption,
	customerror.CustomError) {
	var options MinutesAndWorkNameOption
	
	for _, str := range commandSlice {
		if (utils.HasWorkNameOptionPrefix(str)) && !options.IsWorkNameSet {
			workName := utils.TrimWorkNameOptionPrefix(str)
			options.WorkName = workName
			options.IsWorkNameSet = true
		} else if (utils.HasTimeOptionPrefix(str)) && !options.IsDurationMinSet {
			num, err := strconv.Atoi(utils.TrimTimeOptionPrefix(str))
			if err != nil { // 無効な値
				return MinutesAndWorkNameOption{}, customerror.InvalidCommand.New(i18n.T("parse:check-option", utils.TimeOptionPrefix))
			}
			options.DurationMin = num
			options.IsDurationMinSet = true
		}
	}
	return options, customerror.NewNil()
}
