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

	slog.Info("formatted string", "fullString", fullString)

	if strings.HasPrefix(fullString, CommandPrefix) || strings.HasPrefix(fullString, MemberCommandPrefix) {
		slice := strings.Split(fullString, HalfWidthSpace)
		switch slice[0] {
		case MemberInCommand:
			argStr := strings.TrimPrefix(fullString, MemberInCommand)
			return ParseIn(argStr, true, false, 0)
		case InCommand:
			argStr := strings.TrimPrefix(fullString, InCommand)
			return ParseIn(argStr, false, false, 0)
		case MemberWorkCommand:
			argStr := strings.TrimPrefix(fullString, MemberWorkCommand)
			return ParseIn(argStr, true, false, 0)
		case WorkCommand:
			argStr := strings.TrimPrefix(fullString, WorkCommand)
			return ParseIn(argStr, false, false, 0)
		case OutCommand:
			return &CommandDetails{
				CommandType: Out,
			}, ""
		case InfoCommand:
			argStr := strings.TrimPrefix(fullString, InfoCommand)
			return ParseInfo(argStr)
		case MyCommand:
			argStr := strings.TrimPrefix(fullString, MyCommand)
			return ParseMy(argStr)
		case ChangeCommand:
			argStr := strings.TrimPrefix(fullString, ChangeCommand)
			return ParseChange(argStr)
		case SeatCommand:
			argStr := strings.TrimPrefix(fullString, SeatCommand)
			return ParseSeat(argStr)
		case ReportCommand:
			// NOTE: !reportの場合は全文を送信する。
			return ParseReport(fullString)
		case KickCommand:
			argStr := strings.TrimPrefix(fullString, KickCommand)
			return ParseKick(argStr, false)
		case MemberKickCommand:
			argStr := strings.TrimPrefix(fullString, MemberKickCommand)
			return ParseKick(argStr, true)
		case CheckCommand:
			argStr := strings.TrimPrefix(fullString, CheckCommand)
			return ParseCheck(argStr, false)
		case MemberCheckCommand:
			argStr := strings.TrimPrefix(fullString, MemberCheckCommand)
			return ParseCheck(argStr, true)
		case BlockCommand:
			argStr := strings.TrimPrefix(fullString, BlockCommand)
			return ParseBlock(argStr, false)
		case MemberBlockCommand:
			argStr := strings.TrimPrefix(fullString, MemberBlockCommand)
			return ParseBlock(argStr, true)
		case OkawariCommand:
			argStr := strings.TrimPrefix(fullString, OkawariCommand)
			return ParseMore(argStr)
		case MoreCommand:
			argStr := strings.TrimPrefix(fullString, MoreCommand)
			return ParseMore(argStr)
		case RestCommand:
			argStr := strings.TrimPrefix(fullString, RestCommand)
			return ParseBreak(argStr)
		case ChillCommand:
			argStr := strings.TrimPrefix(fullString, ChillCommand)
			return ParseBreak(argStr)
		case BreakCommand:
			argStr := strings.TrimPrefix(fullString, BreakCommand)
			return ParseBreak(argStr)
		case ResumeCommand:
			argStr := strings.TrimPrefix(fullString, ResumeCommand)
			return ParseResume(argStr)
		case RankCommand:
			return &CommandDetails{
				CommandType: Rank,
			}, ""
		case OrderCommand:
			argStr := strings.TrimPrefix(fullString, OrderCommand)
			return ParseOrder(argStr)
		case ClearCommand, ClearShortCommand:
			return &CommandDetails{
				CommandType: Clear,
			}, ""
		default: // !席番号 or 間違いコマンド
			// "!席番号" or "/席番号" かも
			if num, err := strconv.Atoi(strings.TrimPrefix(slice[0], CommandPrefix)); err == nil {
				argStr := strings.TrimPrefix(fullString, slice[0])
				return ParseSeatIn(num, argStr, false)
			} else if num, err := strconv.Atoi(strings.TrimPrefix(slice[0], MemberCommandPrefix)); err == nil {
				argStr := strings.TrimPrefix(fullString, slice[0])
				return ParseSeatIn(num, argStr, true)
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
)

// FormatStringToParse はコマンド解析のために文字列を整形する
func FormatStringToParse(fullString string) string {
	// 全角スペースを半角に変換
	fullString = strings.Replace(fullString, FullWidthSpace, HalfWidthSpace, -1)

	// 全角イコールを半角に変換
	fullString = strings.Replace(fullString, FullWidthEqualSign, HalfWidthEqualSign, -1)

	// 前後のスペースをトリム
	fullString = strings.TrimSpace(fullString)

	// `！`（全角）で始まるなら半角に変換
	if strings.HasPrefix(fullString, CommandPrefixFullWidth) {
		fullString = strings.Replace(fullString, CommandPrefixFullWidth, CommandPrefix, 1)
	}
	// `／`（全角）で始まるなら半角に変換
	if strings.HasPrefix(fullString, MemberCommandPrefixFullWidth) {
		fullString = strings.Replace(fullString, MemberCommandPrefixFullWidth, MemberCommandPrefix, 1)
	}

	// work=やmin=のようなオプションで=を空白に変換。
	fullString = strings.ReplaceAll(fullString, " work=", " work ")
	fullString = strings.ReplaceAll(fullString, " min=", " min ")
	fullString = strings.ReplaceAll(fullString, " order=", " order ")
	fullString = strings.ReplaceAll(fullString, " w=", " w ")
	fullString = strings.ReplaceAll(fullString, " m=", " m ")
	fullString = strings.ReplaceAll(fullString, " o=", " o ")
	fullString = strings.ReplaceAll(fullString, " work-", " work ")
	fullString = strings.ReplaceAll(fullString, " w-", " work ")
	fullString = strings.ReplaceAll(fullString, " min-", " min ")
	fullString = strings.ReplaceAll(fullString, " m-", " min ")
	fullString = strings.ReplaceAll(fullString, " order-", " order ")
	fullString = strings.ReplaceAll(fullString, " o-", " order ")
	fullString = strings.ReplaceAll(fullString, " rank=", " rank ")
	fullString = strings.ReplaceAll(fullString, " color=", " color ")

	// オプションの短縮系は非短縮に変換
	fullString = strings.ReplaceAll(fullString, " w ", " work ")
	fullString = strings.ReplaceAll(fullString, " m ", " min ")
	fullString = strings.ReplaceAll(fullString, " o ", " order ")

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

	options := &MinWorkOrderOption{
		IsWorkNameSet:    false,
		IsDurationMinSet: false,
	}
	var err string

	if len(fields) >= 1 {
		options, err = ParseMinWorkOrderOptions(commandExcludedStr)
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

func ParseInfo(argText string) (*CommandDetails, string) {
	fields := strings.Fields(argText)
	showDetails := false

	if len(fields) > 0 && fields[0] == ShowDetailsOption {
		showDetails = true
	}

	return &CommandDetails{
		CommandType: Info,
		InfoOption: InfoOption{
			ShowDetails: showDetails,
		},
	}, ""
}

func ParseMy(argText string) (*CommandDetails, string) {
	options, message := ParseMyOptions(argText)
	if message != "" {
		return nil, message
	}

	return &CommandDetails{
		CommandType: My,
		MyOptions:   options,
	}, ""
}

func ParseMyOptions(argText string) ([]MyOption, string) {
	fields := strings.Fields(argText)

	const (
		Rank = iota
		Min
		Color
		Any
	)
	currentMode := Any

	isRankVisibleSet := false
	var rankVisibleValue bool
	isDefaultStudyMinSet := false
	var defaultStudyMinValue int
	isFavoriteColorSet := false
	var favoriteColorValue string

	for _, field := range fields {
		switch currentMode {
		case Rank:
			if field == RankVisibleMyOptionOn {
				rankVisibleValue = true
				isRankVisibleSet = true
			} else if field == RankVisibleMyOptionOff {
				rankVisibleValue = false
				isRankVisibleSet = true
			} else {
				return []MyOption{}, i18n.T("parse:check-option", RankVisibleMyOptionPrefix)
			}
			currentMode = Any
			continue
		case Min:
			// 0もしくは空欄ならリセットとする。リセットは内部的には0で扱う。
			if field == RankVisibleMyOptionOn || field == RankVisibleMyOptionOff {
				defaultStudyMinValue = 0
			} else {
				value, err := strconv.Atoi(field)
				if err != nil {
					return []MyOption{}, i18n.T("parse:check-option", TimeOptionPrefix)
				}
				defaultStudyMinValue = value
			}
			currentMode = Any
			continue
		case Color:
			favoriteColorValue = field
			currentMode = Any
			continue
		default:
			// pass
		}

		if field == RankVisibleMyOptionKey && !isRankVisibleSet {
			currentMode = Rank
		} else if field == TimeOptionKey && !isDefaultStudyMinSet {
			currentMode = Min
			isDefaultStudyMinSet = true // 空白の場合も対応（リセット）するのでここでセット
		} else if field == FavoriteColorMyOptionKey && !isFavoriteColorSet {
			currentMode = Color
			isFavoriteColorSet = true // 空白の場合も対応（リセット）するのでここでセット
		}
	}

	options := make([]MyOption, 0)

	if isRankVisibleSet {
		options = append(options, MyOption{
			Type:      RankVisible,
			BoolValue: rankVisibleValue,
		})
	}
	if isDefaultStudyMinSet {
		options = append(options, MyOption{
			Type:     DefaultStudyMin,
			IntValue: defaultStudyMinValue,
		})
	}
	if isFavoriteColorSet {
		options = append(options, MyOption{
			Type:        FavoriteColor,
			StringValue: favoriteColorValue,
		})
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
	options, message := ParseMinWorkOrderOptions(commandString)
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

func ParseMore(argText string) (*CommandDetails, string) {
	slice := strings.Split(argText, HalfWidthSpace)
	var durationMin int
	var message string

	if len(slice) == 1 && slice[0] == "" {
		return &CommandDetails{
			CommandType: More,
			MoreOption: MoreOption{
				IsDurationMinSet: false,
			},
		}, ""
	}

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
			IsDurationMinSet: true,
			DurationMin:      durationMin,
		},
	}, ""
}

func ParseBreak(commandString string) (*CommandDetails, string) {
	// 追加オプションチェック
	options, message := ParseMinWorkOrderOptions(commandString)
	if message != "" {
		return nil, message
	}

	return &CommandDetails{
		CommandType: Break,
		BreakOption: *options,
	}, ""
}

func ParseResume(argText string) (*CommandDetails, string) {
	// 作業名
	option := ParseWorkNameOption(argText)

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

func ParseWorkNameOption(argText string) WorkNameOption {
	argText = strings.TrimSpace(argText)

	fields := strings.Fields(argText)
	if len(fields) == 0 {
		return WorkNameOption{
			IsWorkNameSet: false,
		}
	}

	workName := strings.TrimPrefix(argText, WorkNameOptionKey)
	workName = strings.TrimSpace(workName)

	return WorkNameOption{
		IsWorkNameSet: true,
		WorkName:      workName,
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

func ParseMinWorkOrderOptions(commandExcludedStr string) (*MinWorkOrderOption, string) {
	var options MinWorkOrderOption

	const (
		Min = iota
		Order
		Any
		Work
	)

	currentMode := Any // NOTE: 作業内容はオプションwork明示なしもあるので、初期値はWork
	for _, field := range strings.Fields(commandExcludedStr) {
		switch currentMode {
		case Min:
			value, err := strconv.Atoi(field)
			if err != nil {
				return nil, i18n.T("parse:check-option", TimeOptionPrefix)
			}
			options.DurationMin = value
			options.IsDurationMinSet = true
			currentMode = Any
			continue
		case Order:
			value, err := strconv.Atoi(field)
			if err != nil {
				return nil, i18n.T("parse:check-option", OrderOptionPrefix)
			}
			options.OrderNum = value
			options.IsOrderSet = true
			currentMode = Any
			continue
		case Work:
			if field == TimeOptionKey || field == OrderOptionKey {
				currentMode = Any
			} else {
				options.WorkName += field + HalfWidthSpace
				continue
			}
		default:
			// pass
		}

		if field == TimeOptionKey && !options.IsDurationMinSet {
			currentMode = Min
		} else if field == OrderOptionKey && !options.IsOrderSet {
			currentMode = Order
		} else if field == WorkNameOptionKey && !options.IsWorkNameSet {
			currentMode = Work
			options.IsWorkNameSet = true // リセット（空文字）の場合もあるので、ここでセット
		} else if !options.IsWorkNameSet {
			currentMode = Work
			options.IsWorkNameSet = true
			options.WorkName += field + HalfWidthSpace
		}
	}

	options.WorkName = strings.TrimSpace(options.WorkName)

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
