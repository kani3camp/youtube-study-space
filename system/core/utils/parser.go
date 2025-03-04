package utils

import (
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"app.modules/core/i18n"
)

// ParseCommand コマンドを解析
func ParseCommand(fullString string, isMember bool) (*CommandDetails, string) {
	// コマンド解析前に文字列を整形
	fullString = FormatStringToParse(fullString)

	// メンバーの場合は絵文字コマンドを文字に置換
	if isMember {
		var message string
		fullString, message = ReplaceEmojiCommandToText(fullString)
		if message != "" {
			return nil, message
		}
		fullString = FormatStringToParse(fullString)
		slog.Info("Replaced emoji command to text", "fullString", fullString)
	}

	if strings.HasPrefix(fullString, CommandPrefix) || strings.HasPrefix(fullString, MemberCommandPrefix) {
		slice := strings.Split(fullString, HalfWidthSpace)
		switch slice[0] {
		case MemberInCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, MemberInCommand)
			return ParseIn(commandExcludedStr, true, false, 0)
		case InCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, InCommand)
			return ParseIn(commandExcludedStr, false, false, 0)
		case MemberWorkCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, MemberWorkCommand)
			return ParseIn(commandExcludedStr, true, false, 0)
		case WorkCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, WorkCommand)
			return ParseIn(commandExcludedStr, false, false, 0)
		case OutCommand:
			return &CommandDetails{
				CommandType: Out,
			}, ""
		case InfoCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, InfoCommand)
			return ParseInfo(commandExcludedStr)
		case MyCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, MyCommand)
			return ParseMy(commandExcludedStr)
		case ChangeCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, ChangeCommand)
			return ParseChange(commandExcludedStr)
		case SeatCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, SeatCommand)
			return ParseSeat(commandExcludedStr)
		case ReportCommand:
			// NOTE: !reportの場合は全文を送信する。
			return ParseReport(fullString)
		case KickCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, KickCommand)
			return ParseKick(commandExcludedStr, false)
		case MemberKickCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, MemberKickCommand)
			return ParseKick(commandExcludedStr, true)
		case CheckCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, CheckCommand)
			return ParseCheck(commandExcludedStr, false)
		case MemberCheckCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, MemberCheckCommand)
			return ParseCheck(commandExcludedStr, true)
		case BlockCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, BlockCommand)
			return ParseBlock(commandExcludedStr, false)
		case MemberBlockCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, MemberBlockCommand)
			return ParseBlock(commandExcludedStr, true)
		case OkawariCommand, MoreCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, OkawariCommand)
			return ParseMore(commandExcludedStr)
		case RestCommand, ChillCommand, BreakCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, BreakCommand)
			return ParseBreak(commandExcludedStr)
		case ResumeCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, ResumeCommand)
			return ParseResume(commandExcludedStr)
		case RankCommand:
			return &CommandDetails{
				CommandType: Rank,
			}, ""
		case OrderCommand:
			commandExcludedStr := strings.TrimPrefix(fullString, OrderCommand)
			return ParseOrder(commandExcludedStr)
		default: // !席番号 or 間違いコマンド
			// "!席番号" or "/席番号" かも
			if num, err := strconv.Atoi(strings.TrimPrefix(slice[0], CommandPrefix)); err == nil {
				commandExcludedStr := strings.TrimPrefix(fullString, slice[0])
				return ParseSeatIn(num, commandExcludedStr, false)
			} else if num, err := strconv.Atoi(strings.TrimPrefix(slice[0], MemberCommandPrefix)); err == nil {
				commandExcludedStr := strings.TrimPrefix(fullString, slice[0])
				return ParseSeatIn(num, commandExcludedStr, true)
			}

			// 間違いコマンド
			return &CommandDetails{
				CommandType: InvalidCommand,
			}, ""
		}
	}
	return &CommandDetails{
		CommandType: NotCommand,
	}, ""
}

func ReplaceEmojiCommandToText(fullString string) (string, string) {
	// コマンドの置換（オプション除く）
	emojiStrings := emojiCommandRegex.FindAllString(fullString, -1)
	for _, s := range emojiStrings {
		switch true {
		case MatchEmojiCommand(s, InZeroString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+InZeroCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, InString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+InCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, OutString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+OutCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, InfoString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+InfoCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, InfoDString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+InfoDCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, MyString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+MyCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, ChangeString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+ChangeCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, SeatString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+SeatCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, SeatDString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+SeatDCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, MoreString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+MoreCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, BreakString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+BreakCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, ResumeString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+ResumeCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, MemberInString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+MemberInCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, OrderString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+OrderCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, RankString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+RankCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, WorkString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+WorkNameOptionPrefix, 1)
		case MatchEmojiCommand(s, MinString):
			minString, err := ReplaceEmojiMinToText(s)
			if err != nil {
				return "", i18n.T("parse:check-option", TimeOptionPrefix)
			}
			fullString = strings.Replace(fullString, s, HalfWidthSpace+minString, 1)
		case MatchEmojiCommand(s, ColorString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+FavoriteColorMyOptionPrefix, 1)
		case MatchEmojiCommand(s, RankOnString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+RankVisibleMyOptionPrefix+RankVisibleMyOptionOn, 1)
		case MatchEmojiCommand(s, RankOffString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+RankVisibleMyOptionPrefix+RankVisibleMyOptionOff, 1)
		}
	}

	return fullString, ""
}

var (
	emojiCommandRegex = regexp.MustCompile(EmojiCommandPrefix + `[^` + EmojiSide + `]*` + EmojiSide)
	workRegex         = regexp.MustCompile(`(work=|w=|work-|w-)`)
	minRegex          = regexp.MustCompile(`(min=|m=|min-|m-)`)
	minWithValueRegex = regexp.MustCompile(`min=\S+|m=\S+|min-\S+|m-\S+|min= \S+|m= \S+|min- \S+|m- \S+`)
)

// FormatStringToParse
// 全角スペースを半角に変換
// 全角イコールを半角に変換
// 前後のスペースをトリム
// `！`（全角）で始まるなら半角に変換
// `／`（全角）で始まるなら半角に変換
// 複数の空白が連続する場合は1つにする
// `!`や`/`の隣が空白ならその空白を消す
func FormatStringToParse(fullString string) string {
	fullString = strings.Replace(fullString, FullWidthSpace, HalfWidthSpace, -1)
	fullString = strings.Replace(fullString, FullWidthEqualSign, HalfWidthEqualSign, -1)
	fullString = strings.TrimSpace(fullString)

	// プレフィックスが全角なら半角に変換
	if strings.HasPrefix(fullString, CommandPrefixFullWidth) {
		fullString = strings.Replace(fullString, CommandPrefixFullWidth, CommandPrefix, 1)
	}
	if strings.HasPrefix(fullString, MemberCommandPrefixFullWidth) {
		fullString = strings.Replace(fullString, MemberCommandPrefixFullWidth, MemberCommandPrefix, 1)
	}

	// 複数の空白が連続する場合は1つにする
	fullString = strings.Join(strings.Fields(fullString), HalfWidthSpace)

	// `!`や`/`の隣が空白ならその空白を消す
	fullString = strings.ReplaceAll(fullString, CommandPrefix+HalfWidthSpace, CommandPrefix)
	fullString = strings.ReplaceAll(fullString, MemberCommandPrefix+HalfWidthSpace, MemberCommandPrefix)

	return fullString
}

func ExtractAllEmojiCommands(commandString string) ([]EmojiElement, string) {
	emojis := make([]EmojiElement, 0)
	emojiStrings := emojiCommandRegex.FindAllString(commandString, -1)
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

	emojiExcludedString := emojiCommandRegex.ReplaceAllString(commandString, HalfWidthSpace)
	emojiExcludedString = strings.TrimLeft(emojiExcludedString, HalfWidthSpace)
	return emojis, emojiExcludedString
}

func ParseIn(commandExcludedStr string, isTargetMemberSeat bool, isSeatIdSet bool, seatId int) (*CommandDetails, string) {
	fields := strings.Fields(commandExcludedStr)

	options := &MinutesAndWorkNameOption{
		IsWorkNameSet:    false,
		IsDurationMinSet: false,
	}
	var err string

	if len(fields) >= 1 {
		options, err = ParseMinutesAndWorkNameOptions(commandExcludedStr)
		if err != "" {
			return nil, err
		}
	}

	return &CommandDetails{
		CommandType: In,
		InOption: InOption{
			IsSeatIdSet:        isSeatIdSet,
			SeatId:             seatId,
			MinutesAndWorkName: options,
			IsMemberSeat:       isTargetMemberSeat,
		},
	}, ""
}

func ParseSeatIn(seatNum int, commandExcludedStr string, isMemberSeat bool) (*CommandDetails, string) {
	return ParseIn(commandExcludedStr, isMemberSeat, true, seatNum)
}

func ParseInfo(commandString string) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)
	showDetails := false

	if len(slice) >= 2 && slice[1] == ShowDetailsOption {
		showDetails = true
	}

	return &CommandDetails{
		CommandType: Info,
		InfoOption: InfoOption{
			ShowDetails: showDetails,
		},
	}, ""
}

func ParseMy(commandString string) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)

	options, message := ParseMyOptions(slice[1:])
	if message != "" {
		return nil, message
	}

	return &CommandDetails{
		CommandType: My,
		MyOptions:   options,
	}, ""
}

func ParseMyOptions(strSlice []string) ([]MyOption, string) {
	isRankVisibleSet := false
	isDefaultStudyMinSet := false
	isFavoriteColorSet := false

	options := make([]MyOption, 0)

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
			durationMin, message = ParseDurationMinOption([]string{str}, false, true)
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

func ParseReport(fullString string) (*CommandDetails, string) {
	slice := strings.Split(fullString, HalfWidthSpace)

	var reportMessage string
	if len(slice) == 1 {
		return nil, i18n.T("parse:missing-message", ReportCommand)
	} else { // len(slice) > 1
		reportMessage = fullString
	}

	return &CommandDetails{
		CommandType: Report,
		ReportOption: ReportOption{
			Message: reportMessage,
		},
	}, ""
}

func ParseChange(commandString string) (*CommandDetails, string) {
	// 追加オプションチェック
	fields := strings.Fields(commandString)
	if len(fields) == 0 {
		return nil, i18n.T("parse:missing-change-option")
	}
	options, message := ParseMinutesAndWorkNameOptions(commandString)
	if message != "" {
		return nil, message
	}

	return &CommandDetails{
		CommandType:  Change,
		ChangeOption: *options,
	}, ""
}

func ParseSeat(commandString string) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)
	showDetails := false

	if len(slice) >= 2 && slice[1] == ShowDetailsOption {
		showDetails = true
	}

	return &CommandDetails{
		CommandType: Seat,
		SeatOption: SeatOption{
			ShowDetails: showDetails,
		},
	}, ""
}

func ParseMore(commandString string) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)
	var durationMin int
	var message string

	if len(slice) >= 2 {
		durationMin, message = ParseDurationMinOption(slice, true, false)
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

func ParseBreak(commandString string) (*CommandDetails, string) {
	// 追加オプションチェック
	options, message := ParseMinutesAndWorkNameOptions(commandString)
	if message != "" {
		return nil, message
	}

	return &CommandDetails{
		CommandType: Break,
		BreakOption: *options,
	}, ""
}

func ParseResume(commandString string) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)

	// 作業名
	option := ParseWorkNameOption(slice)

	return &CommandDetails{
		CommandType:  Resume,
		ResumeOption: option,
	}, ""
}

func ParseOrder(commandString string) (*CommandDetails, string) {
	slice := strings.Split(commandString, HalfWidthSpace)

	// NOTE: オプションは番号か文字列のどちらかのみ

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

func ParseDurationMinOption(strSlice []string, allowNonPrefix bool, allowEmpty bool) (int, string) {
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

func ParseMinutesAndWorkNameOptions(commandExcludedStr string) (*MinutesAndWorkNameOption, string) {
	var options MinutesAndWorkNameOption

	minLoc := minRegex.FindStringIndex(commandExcludedStr)
	workLoc := workRegex.FindStringIndex(commandExcludedStr)

	// minオプション
	if minLoc != nil {
		// runeのインデックスに変換
		targetStr := commandExcludedStr[minLoc[1]:]
		fields := strings.Fields(targetStr)
		if len(fields) == 0 {
			return nil, i18n.T("parse:check-option", TimeOptionPrefix)
		}
		minValueStr := fields[0]
		minValue, err := strconv.Atoi(strings.TrimSpace(minValueStr))
		if err != nil {
			return nil, i18n.T("parse:check-option", TimeOptionPrefix)
		}
		options.DurationMin = minValue
		options.IsDurationMinSet = true
	}

	// workオプション
	if workLoc != nil {
		// min指定がないか、minLocより後にある
		if minLoc == nil || minLoc[0] < workLoc[0] {
			workNameValue := commandExcludedStr[workLoc[1]:]
			options.WorkName = strings.TrimSpace(workNameValue)
		} else { // minLocより前にある
			workNameValue := commandExcludedStr[workLoc[1]:minLoc[0]]
			options.WorkName = strings.TrimSpace(workNameValue)
		}
		options.IsWorkNameSet = true
	}

	// 明示的なwork=指定なしの場合
	if !options.IsWorkNameSet {
		if minLoc != nil {
			parts := minWithValueRegex.Split(commandExcludedStr, -1)
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					options.WorkName = trimmed
					options.IsWorkNameSet = true
					break
				}
			}
		} else { // min指定がない場合
			options.WorkName = strings.TrimSpace(commandExcludedStr)
			if options.WorkName != "" {
				options.IsWorkNameSet = true
			}
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
