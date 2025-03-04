package utils

import (
	"regexp"
	"strconv"
	"strings"

	"app.modules/core/i18n"
)

// ParseCommand コマンドを解析
func ParseCommand(fullString string, isMember bool) (*CommandDetails, string) {
	fullString = strings.Replace(fullString, FullWidthSpace, HalfWidthSpace, -1)
	fullString = strings.Replace(fullString, FullWidthEqualSign, HalfWidthEqualSign, -1)

	if strings.HasPrefix(fullString, CommandPrefix) || strings.HasPrefix(fullString, MemberCommandPrefix) {
		emojis, emojiExcludedString := ExtractAllEmojiCommands(fullString)

		slice := strings.Split(emojiExcludedString, HalfWidthSpace)
		switch slice[0] {
		case MemberInCommand:
			return ParseIn(emojiExcludedString, fullString, isMember, true, emojis)
		case InCommand:
			return ParseIn(emojiExcludedString, fullString, isMember, false, emojis)
		case OutCommand:
			return &CommandDetails{
				CommandType: Out,
			}, ""
		case InfoCommand:
			return ParseInfo(emojiExcludedString, isMember, emojis)
		case MyCommand:
			return ParseMy(emojiExcludedString, fullString, isMember, emojis)
		case ChangeCommand:
			return ParseChange(emojiExcludedString, fullString, isMember, emojis)
		case SeatCommand:
			return ParseSeat(emojiExcludedString, isMember, emojis)
		case ReportCommand:
			return ParseReport(emojiExcludedString)
		case KickCommand:
			return ParseKick(emojiExcludedString, false)
		case MemberKickCommand:
			return ParseKick(emojiExcludedString, true)
		case CheckCommand:
			return ParseCheck(emojiExcludedString, false)
		case MemberCheckCommand:
			return ParseCheck(emojiExcludedString, true)
		case BlockCommand:
			return ParseBlock(emojiExcludedString, false)
		case MemberBlockCommand:
			return ParseBlock(emojiExcludedString, true)
		case OkawariCommand, MoreCommand:
			return ParseMore(emojiExcludedString, fullString, isMember, emojis)
		case RestCommand, ChillCommand, BreakCommand:
			return ParseBreak(emojiExcludedString, fullString, isMember, emojis)
		case ResumeCommand:
			return ParseResume(emojiExcludedString, fullString, isMember, emojis)
		case RankCommand:
			return &CommandDetails{
				CommandType: Rank,
			}, ""
		case OrderCommand:
			return ParseOrder(emojiExcludedString, fullString, isMember, emojis)
		case CommandPrefix: // 典型的なミスコマンド「! in」「! out」とか。
			return nil, i18n.T("parse:isolated-!")
		default: // !席番号 or 間違いコマンド
			// "!席番号" or "/席番号" かも
			if num, err := strconv.Atoi(strings.TrimPrefix(slice[0], CommandPrefix)); err == nil {
				return ParseSeatIn(num, emojiExcludedString, fullString, isMember, false, emojis)
			} else if num, err := strconv.Atoi(strings.TrimPrefix(slice[0], MemberCommandPrefix)); err == nil {
				return ParseSeatIn(num, emojiExcludedString, fullString, isMember, true, emojis)
			}

			// 間違いコマンド
			return &CommandDetails{
				CommandType: InvalidCommand,
			}, ""
		}
	} else if strings.HasPrefix(fullString, WrongCommandPrefix) {
		return nil, i18n.T("parse:non-half-width-!")
	} else if isMember && strings.HasPrefix(fullString, EmojiCommandPrefix) {
		emojis, emojiExcludedString := ExtractAllEmojiCommands(fullString)
		if len(emojis) > 0 {
			switch emojis[0] {
			case EmojiInZero:
				return ParseSeatIn(0, emojiExcludedString, fullString, isMember, false, emojis)
			case EmojiMemberIn:
				return ParseIn(emojiExcludedString, fullString, isMember, true, emojis)
			case EmojiIn:
				return ParseIn(emojiExcludedString, fullString, isMember, false, emojis)
			case EmojiOut:
				return &CommandDetails{
					CommandType: Out,
				}, ""
			case EmojiInfo, EmojiInfoD:
				return ParseInfo(emojiExcludedString, isMember, emojis)
			case EmojiMy:
				return ParseMy(emojiExcludedString, fullString, isMember, emojis)
			case EmojiChange:
				return ParseChange(emojiExcludedString, fullString, isMember, emojis)
			case EmojiSeat, EmojiSeatD:
				return ParseSeat(emojiExcludedString, isMember, emojis)
			case EmojiMore:
				return ParseMore(emojiExcludedString, fullString, isMember, emojis)
			case EmojiBreak:
				return ParseBreak(emojiExcludedString, fullString, isMember, emojis)
			case EmojiResume:
				return ParseResume(emojiExcludedString, fullString, isMember, emojis)
			case EmojiOrder, EmojiOrderCancel:
				return ParseOrder(emojiExcludedString, fullString, isMember, emojis)
			default:
			}
		}
	}
	return &CommandDetails{
		CommandType: NotCommand,
	}, ""
}

func ExtractAllEmojiCommands(commandString string) ([]EmojiElement, string) {
	r, _ := regexp.Compile(EmojiCommandPrefix + `[^` + EmojiSide + `]*` + EmojiSide)
	emojis := make([]EmojiElement, 0)
	emojiStrings := r.FindAllString(commandString, -1)
	for _, s := range emojiStrings {
		var m EmojiElement
		switch true {
		case MatchEmojiCommand(s, InZeroString): // must be before InString
			m = EmojiInZero
		case MatchEmojiCommand(s, InString):
			m = EmojiIn
		case MatchEmojiCommand(s, OutString):
			m = EmojiOut
		case MatchEmojiCommand(s, InfoString):
			m = EmojiInfo
		case MatchEmojiCommand(s, InfoDString):
			m = EmojiInfoD
		case MatchEmojiCommand(s, MyString):
			m = EmojiMy
		case MatchEmojiCommand(s, ChangeString):
			m = EmojiChange
		case MatchEmojiCommand(s, SeatString):
			m = EmojiSeat
		case MatchEmojiCommand(s, SeatDString):
			m = EmojiSeatD
		case MatchEmojiCommand(s, MoreString):
			m = EmojiMore
		case MatchEmojiCommand(s, BreakString):
			m = EmojiBreak
		case MatchEmojiCommand(s, ResumeString):
			m = EmojiResume
		case MatchEmojiCommand(s, WorkString):
			m = EmojiWork
		case MatchEmojiCommand(s, MinString):
			m = EmojiMin
		case MatchEmojiCommand(s, ColorString):
			m = EmojiColor
		case MatchEmojiCommand(s, RankOnString):
			m = EmojiRankOn
		case MatchEmojiCommand(s, RankOffString):
			m = EmojiRankOff
		case MatchEmojiCommand(s, MemberInString):
			m = EmojiMemberIn
		case MatchEmojiCommand(s, OrderString):
			m = EmojiOrder
		default:
			continue
		}
		emojis = append(emojis, m)
	}

	emojiExcludedString := r.ReplaceAllString(commandString, HalfWidthSpace)
	emojiExcludedString = strings.TrimLeft(emojiExcludedString, HalfWidthSpace)
	return emojis, emojiExcludedString
}

func ParseIn(emojiExcludedString string, fullString string, isMember bool, isTargetMemberSeat bool, emojis []EmojiElement) (*CommandDetails, string) {
	slice := strings.Split(emojiExcludedString, HalfWidthSpace)

	// 追加オプションチェック
	options, message := ParseMinutesAndWorkNameOptions(slice, fullString, isMember, emojis)
	if message != "" {
		return nil, message
	}

	return &CommandDetails{
		CommandType: In,
		InOption: InOption{
			IsSeatIdSet:        false,
			MinutesAndWorkName: options,
			IsMemberSeat:       isTargetMemberSeat,
		},
	}, ""
}

func ParseSeatIn(seatNum int, commandString string, fullString string, isMember bool, isMemberSeat bool, emojis []EmojiElement) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)

	// 追加オプションチェック
	options, message := ParseMinutesAndWorkNameOptions(slice, fullString, isMember, emojis)
	if message != "" {
		return nil, message
	}

	return &CommandDetails{
		CommandType: In,
		InOption: InOption{
			IsSeatIdSet:        true,
			SeatId:             seatNum,
			MinutesAndWorkName: options,
			IsMemberSeat:       isMemberSeat,
		},
	}, ""
}

func ParseInfo(commandString string, isMember bool, emojis []EmojiElement) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)
	showDetails := false

	if isMember && ContainsEmojiElement(emojis, EmojiInfoD) {
		showDetails = true
	} else if isMember && ContainsEmojiElement(emojis, EmojiInfo) {
		showDetails = Contains(slice, ShowDetailsOption)
	} else if len(slice) >= 2 && slice[1] == ShowDetailsOption {
		showDetails = true
	}

	return &CommandDetails{
		CommandType: Info,
		InfoOption: InfoOption{
			ShowDetails: showDetails,
		},
	}, ""
}

func ParseMy(commandString string, fullString string, isMember bool, emojis []EmojiElement) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)

	options, message := ParseMyOptions(slice[1:], fullString, isMember, emojis)
	if message != "" {
		return nil, message
	}

	return &CommandDetails{
		CommandType: My,
		MyOptions:   options,
	}, ""
}

func ParseMyOptions(strSlice []string, fullString string, isMember bool, emojis []EmojiElement) ([]MyOption, string) {
	isRankVisibleSet := false
	isDefaultStudyMinSet := false
	isFavoriteColorSet := false

	options := make([]MyOption, 0)

	if isMember {
		for _, emoji := range emojis {
			// rank visible
			if emoji == EmojiRankOn && !isRankVisibleSet {
				options = append(options, MyOption{
					Type:      RankVisible,
					BoolValue: true,
				})
				isRankVisibleSet = true
			} else if emoji == EmojiRankOff && !isRankVisibleSet {
				options = append(options, MyOption{
					Type:      RankVisible,
					BoolValue: false,
				})
				isRankVisibleSet = true
			} else if emoji == EmojiMin && !isDefaultStudyMinSet {
				num, err := ParseEmojiDurationMinOption(fullString, true)
				if err != nil {
					return nil, i18n.T("parse:check-option", TimeOptionPrefix)
				}
				options = append(options, MyOption{
					Type:     DefaultStudyMin,
					IntValue: num,
				})
				isDefaultStudyMinSet = true
			} else if emoji == EmojiColor && !isFavoriteColorSet {
				colorName := ParseEmojiColorNameOption(fullString)
				options = append(options, MyOption{
					Type:        FavoriteColor,
					StringValue: colorName,
				})
				isFavoriteColorSet = true
			}
		}
	}

	for _, str := range strSlice {
		if strings.HasPrefix(str, RankVisibleMyOptionPrefix) && !isRankVisibleSet {
			var rankVisible bool
			rankVisibleStr := strings.TrimPrefix(str, RankVisibleMyOptionPrefix)
			if rankVisibleStr == RankVisibleMyOptionOn {
				rankVisible = true
			} else if rankVisibleStr == RankVisibleMyOptionOff {
				rankVisible = false
			} else {
				return []MyOption{}, i18n.T("parse:check-option", RankVisibleMyOptionPrefix)
			}
			options = append(options, MyOption{
				Type:      RankVisible,
				BoolValue: rankVisible,
			})
			isRankVisibleSet = true
		} else if HasTimeOptionPrefix(str) && !isDefaultStudyMinSet {
			var durationMin int
			// 0もしくは空欄ならリセットなので、空欄も許可。リセットは内部的には0で扱う。
			var message string
			durationMin, message = ParseDurationMinOption([]string{str}, fullString, false, true, isMember, emojis)
			if message != "" {
				return nil, message
			}
			options = append(options, MyOption{
				Type:     DefaultStudyMin,
				IntValue: durationMin,
			})
			isDefaultStudyMinSet = true
		} else if strings.HasPrefix(str, FavoriteColorMyOptionPrefix) && !isFavoriteColorSet {
			var paramStr = strings.TrimPrefix(str, FavoriteColorMyOptionPrefix)
			options = append(options, MyOption{
				Type:        FavoriteColor,
				StringValue: paramStr,
			})
			isFavoriteColorSet = true
		}
	}
	return options, ""
}

func ParseKick(commandString string, isTargetMemberSeat bool) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)

	var kickSeatId int
	if len(slice) >= 2 {
		num, err := strconv.Atoi(slice[1])
		if err != nil {
			return nil, i18n.T("parse:invalid-seat-id")
		}
		kickSeatId = num
	} else {
		return nil, i18n.T("parse:missing-seat-id")
	}

	return &CommandDetails{
		CommandType: Kick,
		KickOption: KickOption{
			SeatId:             kickSeatId,
			IsTargetMemberSeat: isTargetMemberSeat,
		},
	}, ""
}

func ParseCheck(commandString string, isTargetMemberSeat bool) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)

	var targetSeatId int
	if len(slice) >= 2 {
		num, err := strconv.Atoi(slice[1])
		if err != nil {
			return nil, i18n.T("parse:invalid-seat-id")
		}
		targetSeatId = num
	} else {
		return nil, i18n.T("parse:missing-seat-id")
	}

	return &CommandDetails{
		CommandType: Check,
		CheckOption: CheckOption{
			SeatId:             targetSeatId,
			IsTargetMemberSeat: isTargetMemberSeat,
		},
	}, ""
}

func ParseBlock(commandString string, isTargetMemberSeat bool) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)

	var targetSeatId int
	if len(slice) >= 2 {
		num, err := strconv.Atoi(slice[1])
		if err != nil {
			return nil, i18n.T("parse:invalid-seat-id")
		}
		targetSeatId = num
	} else {
		return nil, i18n.T("parse:missing-seat-id")
	}

	return &CommandDetails{
		CommandType: Block,
		BlockOption: BlockOption{
			SeatId:             targetSeatId,
			IsTargetMemberSeat: isTargetMemberSeat,
		},
	}, ""
}

func ParseReport(commandString string) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)

	var reportMessage string
	if len(slice) == 1 {
		return nil, i18n.T("parse:missing-message", ReportCommand)
	} else { // len(slice) > 1
		reportMessage = commandString
	}

	return &CommandDetails{
		CommandType: Report,
		ReportOption: ReportOption{
			Message: reportMessage,
		},
	}, ""
}

func ParseChange(commandString string, fullString string, isMember bool, emojis []EmojiElement) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)

	// 追加オプションチェック
	options, message := ParseMinutesAndWorkNameOptions(slice, fullString, isMember, emojis)
	if message != "" {
		return nil, message
	}

	return &CommandDetails{
		CommandType:  Change,
		ChangeOption: *options,
	}, ""
}

func ParseSeat(commandString string, isMember bool, emojis []EmojiElement) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)
	showDetails := false

	if isMember && ContainsEmojiElement(emojis, EmojiSeatD) {
		showDetails = true
	} else if isMember && ContainsEmojiElement(emojis, EmojiSeat) {
		showDetails = Contains(slice, ShowDetailsOption)
	} else if len(slice) >= 2 && slice[1] == ShowDetailsOption {
		showDetails = true
	}

	return &CommandDetails{
		CommandType: Seat,
		SeatOption: SeatOption{
			ShowDetails: showDetails,
		},
	}, ""
}

func ParseMore(commandString string, fullString string, isMember bool, emojis []EmojiElement) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)
	var durationMin int
	var message string

	if isMember && ContainsEmojiElement(emojis, EmojiMore) {
		durationMin, message = ParseDurationMinOption(slice, fullString, true, false, isMember, emojis)
		if message != "" {
			return nil, message
		}
	} else if len(slice) >= 2 {
		durationMin, message = ParseDurationMinOption(slice, fullString, true, false, isMember, emojis)
		if message != "" {
			return nil, message
		}
	} else {
		return nil, i18n.T("parse:missing-more-option") // !more doesn't need 'min=' prefix.
	}

	return &CommandDetails{
		CommandType: More,
		MoreOption: MoreOption{
			DurationMin: durationMin,
		},
	}, ""
}

func ParseBreak(commandString string, fullString string, isMember bool, emojis []EmojiElement) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)

	// 追加オプションチェック
	options, message := ParseMinutesAndWorkNameOptions(slice, fullString, isMember, emojis)
	if message != "" {
		return nil, message
	}

	return &CommandDetails{
		CommandType: Break,
		BreakOption: *options,
	}, ""
}

func ParseResume(commandString string, fullString string, isMember bool, emojis []EmojiElement) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)

	// 作業名
	option := ParseWorkNameOption(slice, fullString, isMember, emojis)

	return &CommandDetails{
		CommandType:  Resume,
		ResumeOption: option,
	}, ""
}

func ParseOrder(commandString string, fullString string, isMember bool, emojis []EmojiElement) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)

	// NOTE: オプションは番号か文字列のどちらかのみ

	if isMember && ContainsEmojiElement(emojis, EmojiOrderCancel) {
		return &CommandDetails{
			CommandType: Order,
			OrderOption: OrderOption{
				ClearFlag: true,
			},
		}, ""
	} else if isMember && ContainsEmojiElement(emojis, EmojiOrder) {
		if len(slice) < 2 {
			return nil, i18n.T("parse:invalid-option")
		}

		for _, str := range slice[1:] {
			if str == OrderCancelOption {
				return &CommandDetails{
					CommandType: Order,
					OrderOption: OrderOption{
						ClearFlag: true,
					},
				}, ""
			}

			num, err := strconv.Atoi(str)
			if err == nil {
				return &CommandDetails{
					CommandType: Order,
					OrderOption: OrderOption{
						IntValue: num,
					},
				}, ""
			}
		}
		return nil, i18n.T("parse:invalid-option")
	}

	option, message := ParseOrderOption(slice)
	if message != "" {
		return nil, message
	}

	return &CommandDetails{
		CommandType: Order,
		OrderOption: *option,
	}, ""
}

func ParseOrderOption(strSlice []string) (*OrderOption, string) {
	if len(strSlice) < 2 {
		return nil, i18n.T("parse:invalid-option")
	}

	// cancel flag?
	if strSlice[1] == OrderCancelOption {
		return &OrderOption{
			ClearFlag: true,
		}, ""
	}

	num, err := strconv.Atoi(strSlice[1])
	if err != nil {
		return nil, i18n.T("parse:invalid-option")
	}

	return &OrderOption{
		IntValue: num,
	}, ""
}

func ParseWorkNameOption(strSlice []string, fullString string, isMember bool, emojis []EmojiElement) WorkNameOption {
	if isMember {
		if ContainsEmojiElement(emojis, EmojiWork) {
			workName := ParseEmojiWorkNameOption(fullString)
			return WorkNameOption{
				IsWorkNameSet: true,
				WorkName:      workName,
			}
		}
	}

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

func ParseDurationMinOption(strSlice []string, fullString string, allowNonPrefix bool, allowEmpty bool, isMember bool, emojis []EmojiElement) (int, string) {
	// 絵文字コマンドの処理
	if isMember && ContainsEmojiElement(emojis, EmojiMin) {
		num, err := ParseEmojiDurationMinOption(fullString, allowEmpty)
		if err != nil {
			return 0, i18n.T("parse:check-option", TimeOptionPrefix)
		}
		return num, ""
	}

	// テキストオプションの処理
	for _, str := range strSlice {
		// 空の時間オプション
		if allowEmpty && IsEmptyTimeOption(str) {
			return 0, ""
		} else if HasTimeOptionPrefix(str) { // 時間オプションプレフィックス付き
			num, err := strconv.Atoi(TrimTimeOptionPrefix(str))
			if err == nil {
				return num, ""
			}
		} else if allowNonPrefix { // プレフィックスなしの数値
			num, err := strconv.Atoi(str)
			if err == nil {
				return num, ""
			}
		}
	}

	return 0, i18n.T("parse:missing-time-option", TimeOptionPrefix)
}

func ParseMinutesAndWorkNameOptions(strSlice []string, fullString string, isMember bool, emojis []EmojiElement) (*MinutesAndWorkNameOption, string) {
	var options MinutesAndWorkNameOption

	// 絵文字コマンド
	if isMember {
		// 作業名の処理
		if ContainsEmojiElement(emojis, EmojiWork) && !options.IsWorkNameSet {
			options.WorkName = ParseEmojiWorkNameOption(fullString)
			options.IsWorkNameSet = true
		}

		// 時間の処理
		if ContainsEmojiElement(emojis, EmojiMin) && !options.IsDurationMinSet {
			num, err := ParseEmojiDurationMinOption(fullString, false)
			if err != nil {
				return nil, i18n.T("parse:check-option", TimeOptionPrefix)
			}
			options.DurationMin = num
			options.IsDurationMinSet = true
		}
	}

	// テキストオプションの処理
	for _, str := range strSlice {
		if HasWorkNameOptionPrefix(str) && !options.IsWorkNameSet {
			options.WorkName = TrimWorkNameOptionPrefix(str)
			options.IsWorkNameSet = true
		} else if HasTimeOptionPrefix(str) && !options.IsDurationMinSet {
			num, err := strconv.Atoi(TrimTimeOptionPrefix(str))
			if err != nil { // 無効な値
				return nil, i18n.T("parse:check-option", TimeOptionPrefix)
			}
			options.DurationMin = num
			options.IsDurationMinSet = true
		}
	}

	return &options, ""
}

func ParseEmojiWorkNameOption(fullString string) string {
	emojiLoc := FindEmojiCommandIndex(fullString, WorkString)
	if len(emojiLoc) != 2 {
		return ""
	}
	targetString := fullString[emojiLoc[1]:]
	targetString = ReplaceAnyEmojiCommandStringWithSpace(targetString)
	slice := strings.Split(targetString, HalfWidthSpace)
	if MatchEmojiCommandString(slice[0]) {
		return ""
	}
	return slice[0]
}

// ParseEmojiDurationMinOption parses two types of min emoji. "min=" emoji or "min=xxx" emoji.
func ParseEmojiDurationMinOption(fullString string, allowEmpty bool) (int, error) {
	minEmojiString := ExtractEmojiString(fullString, MinString)
	return ExtractEmojiMinValue(fullString, minEmojiString, allowEmpty)
}

func ParseEmojiColorNameOption(fullString string) string {
	emojiLoc := FindEmojiCommandIndex(fullString, ColorString)
	if len(emojiLoc) != 2 {
		return ""
	}
	targetString := fullString[emojiLoc[1]:]
	targetString = ReplaceAnyEmojiCommandStringWithSpace(targetString)
	slice := strings.Split(targetString, HalfWidthSpace)
	return slice[0]
}
