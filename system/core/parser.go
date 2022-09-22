package core

import (
	"app.modules/core/customerror"
	"strconv"
	"strings"
)

// ParseCommand コマンドを解析
func ParseCommand(commandString string) (CommandDetails, customerror.CustomError) {
	commandString = strings.Replace(commandString, FullWidthSpace, HalfWidthSpace, -1)
	commandString = strings.Replace(commandString, FullWidthEqualSign, HalfWidthEqualSign, -1)
	
	if strings.HasPrefix(commandString, CommandPrefix) {
		slice := strings.Split(commandString, HalfWidthSpace)
		switch slice[0] {
		case InCommand:
			return ParseIn(commandString)
		case OutCommand:
			return CommandDetails{
				CommandType: Out,
			}, customerror.NewNil()
		case InfoCommand:
			return ParseInfo(commandString)
		case MyCommand:
			return ParseMy(commandString)
		case ChangeCommand:
			return ParseChange(commandString)
		case SeatCommand:
			return ParseSeat(commandString)
		case ReportCommand:
			return ParseReport(commandString)
		case KickCommand:
			return ParseKick(commandString)
		case CheckCommand:
			return ParseCheck(commandString)
		case BlockCommand:
			return ParseBlock(commandString)
		
		case OkawariCommand:
			fallthrough
		case MoreCommand:
			return ParseMore(commandString)
		
		case RestCommand:
			fallthrough
		case ChillCommand:
			fallthrough
		case BreakCommand:
			return ParseBreak(commandString)
		
		case ResumeCommand:
			return ParseResume(commandString)
		case RankCommand:
			return CommandDetails{
				CommandType: Rank,
			}, customerror.NewNil()
		case CommandPrefix: // 典型的なミスコマンド「! in」「! out」とか。
			return CommandDetails{}, customerror.InvalidCommand.New("びっくりマークは隣の文字とくっつけてください")
		default: // !席番号 or 間違いコマンド
			// !席番号かどうか
			num, err := strconv.Atoi(strings.TrimPrefix(slice[0], CommandPrefix))
			if err == nil {
				return ParseSeatIn(num, commandString)
			}
			
			// 間違いコマンド
			return CommandDetails{
				CommandType: InvalidCommand,
			}, customerror.NewNil()
		}
	} else if strings.HasPrefix(commandString, WrongCommandPrefix) {
		return CommandDetails{}, customerror.InvalidCommand.New("びっくりマークは半角にしてください")
	}
	return CommandDetails{
		CommandType: NotCommand,
	}, customerror.NewNil()
}

func ParseIn(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)
	
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
	slice := strings.Split(commandString, HalfWidthSpace)
	
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
	slice := strings.Split(commandString, HalfWidthSpace)
	
	showDetails := false
	if len(slice) >= 2 {
		if slice[1] == ShowDetailsOption {
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
	slice := strings.Split(commandString, HalfWidthSpace)
	
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
			var durationMin int
			// 0もしくは空欄ならリセットなので、空欄も許可。リセットは内部的には0で扱う。
			if IsEmptyTimeOption(str) {
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
		} else if strings.HasPrefix(str, FavoriteColorMyOptionPrefix) && !isFavoriteColorSet {
			var paramStr = strings.TrimPrefix(str, FavoriteColorMyOptionPrefix)
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
					return []MyOption{}, customerror.InvalidCommand.New("「" + FavoriteColorMyOptionPrefix + "」の後の値は半角数字にしてください")
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

func ParseCheck(commandString string) (CommandDetails, customerror.CustomError) {
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

func ParseBlock(commandString string) (CommandDetails, customerror.CustomError) {
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
		CommandType: Block,
		BlockOption: BlockOption{
			SeatId: targetSeatId,
		},
	}, customerror.NewNil()
}

func ParseReport(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)
	
	var reportMessage string
	if len(slice) == 1 {
		return CommandDetails{}, customerror.InvalidCommand.New("!reportの右にスペースを空けてメッセージを書いてください。")
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
	slice := strings.Split(commandString, HalfWidthSpace)
	
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
	slice := strings.Split(commandString, HalfWidthSpace)
	
	showDetails := false
	if len(slice) >= 2 {
		if slice[1] == ShowDetailsOption {
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
	slice := strings.Split(commandString, HalfWidthSpace)
	
	// 延長時間
	var durationMin int
	if len(slice) >= 2 {
		var cerr customerror.CustomError
		durationMin, cerr = ParseDurationMinOption(slice[1:], true)
		if cerr.IsNotNil() {
			return CommandDetails{}, cerr
		}
	} else {
		return CommandDetails{}, customerror.InvalidCommand.New("オプションに延長時間（分）を指定してください")
	}
	
	return CommandDetails{
		CommandType: More,
		MoreOption: MoreOption{
			DurationMin: durationMin,
		},
	}, customerror.NewNil()
}

func ParseBreak(commandString string) (CommandDetails, customerror.CustomError) {
	slice := strings.Split(commandString, HalfWidthSpace)
	
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
	slice := strings.Split(commandString, HalfWidthSpace)
	
	// 作業名
	option := ParseWorkNameOption(slice[1:])
	
	return CommandDetails{
		CommandType:  Resume,
		ResumeOption: option,
	}, customerror.NewNil()
}

func ParseWorkNameOption(strSlice []string) WorkNameOption {
	for _, str := range strSlice {
		if HasWorkNameOptionPrefix(str) {
			workName := TrimWorkNameOptionPrefix(str)
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
		if HasTimeOptionPrefix(str) {
			num, err := strconv.Atoi(TrimTimeOptionPrefix(str))
			if err != nil {
				return 0, customerror.InvalidCommand.New("時間（分）の値を確認してください")
			}
			return num, customerror.NewNil()
		} else if allowNonPrefix {
			num, err := strconv.Atoi(str)
			if err != nil {
				return num, customerror.ParseFailed.New("オプションが正しく設定されているか確認してください")
			}
			return num, customerror.NewNil()
		}
	}
	return 0, customerror.InvalidCommand.New("時間（分）のオプションをつけてください")
}

func ParseMinutesAndWorkNameOptions(commandSlice []string) (MinutesAndWorkNameOption,
	customerror.CustomError) {
	var options MinutesAndWorkNameOption
	
	for _, str := range commandSlice {
		if (HasWorkNameOptionPrefix(str)) && !options.IsWorkNameSet {
			workName := TrimWorkNameOptionPrefix(str)
			options.WorkName = workName
			options.IsWorkNameSet = true
		} else if (HasTimeOptionPrefix(str)) && !options.IsDurationMinSet {
			num, err := strconv.Atoi(TrimTimeOptionPrefix(str))
			if err != nil { // 無効な値
				return MinutesAndWorkNameOption{}, customerror.InvalidCommand.New("時間（分）の値を確認してください")
			}
			options.DurationMin = num
			options.IsDurationMinSet = true
		}
	}
	return options, customerror.NewNil()
}
