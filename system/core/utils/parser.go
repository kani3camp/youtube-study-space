package utils

import (
	"github.com/pkg/errors"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"app.modules/core/i18n"
)

var (
	emojiCommandRegex = regexp.MustCompile(EmojiCommandPrefix + `[^` + EmojiSide + `]*` + EmojiSide)
	emojiMinRegex     = regexp.MustCompile(MinString + `[0-9]*` + EmojiSide)
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
		case MatchEmojiCommand(s, OrderClearString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+OrderClearCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, RankString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+RankCommand+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, WorkString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+WorkNameOptionKey+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, MinString):
			minString, err := ReplaceEmojiMinToText(s)
			if err != nil {
				return "", i18n.T("parse:check-option", TimeOptionPrefix)
			}
			fullString = strings.Replace(fullString, s, HalfWidthSpace+minString, 1)
		case MatchEmojiCommand(s, ColorString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+FavoriteColorMyOptionKey+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, RankOnString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+RankVisibleMyOptionKey+HalfWidthSpace+RankVisibleMyOptionOn+HalfWidthSpace, 1)
		case MatchEmojiCommand(s, RankOffString):
			fullString = strings.Replace(fullString, s, HalfWidthSpace+RankVisibleMyOptionKey+HalfWidthSpace+RankVisibleMyOptionOff+HalfWidthSpace, 1)
		}
	}

	return fullString, ""
}

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

func ParseIn(argStr string, isTargetMemberSeat bool, isSeatIdSet bool, seatId int) (*CommandDetails, string) {
	fields := strings.Fields(argStr)

	options := &MinWorkOrderOption{
		IsWorkNameSet:    false,
		IsDurationMinSet: false,
	}
	var err string

	if len(fields) >= 1 {
		options, err = ParseMinWorkOrderOptions(argStr)
		if err != "" {
			return nil, err
		}
	}

	return &CommandDetails{
		CommandType: In,
		InOption: InOption{
			IsSeatIdSet:        isSeatIdSet,
			SeatId:             seatId,
			MinWorkOrderOption: options,
			IsMemberSeat:       isTargetMemberSeat,
		},
	}, ""
}

func ParseSeatIn(seatNum int, commandExcludedStr string, isMemberSeat bool) (*CommandDetails, string) {
	return ParseIn(commandExcludedStr, isMemberSeat, true, seatNum)
}

func ParseInfo(argStr string) (*CommandDetails, string) {
	fields := strings.Fields(argStr)
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

func ParseMyOptions(argStr string) ([]MyOption, string) {
	fields := strings.Fields(argStr)

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

func ParseKick(argStr string, isTargetMemberSeat bool) (*CommandDetails, string) {
	fields := strings.Fields(argStr)

	var kickSeatId int
	if len(fields) >= 1 {
		num, err := strconv.Atoi(fields[0])
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

func ParseCheck(argStr string, isTargetMemberSeat bool) (*CommandDetails, string) {
	fields := strings.Fields(argStr)

	var targetSeatId int
	if len(fields) >= 1 {
		num, err := strconv.Atoi(fields[0])
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

func ParseBlock(argStr string, isTargetMemberSeat bool) (*CommandDetails, string) {
	fields := strings.Fields(argStr)

	var targetSeatId int
	if len(fields) >= 1 {
		num, err := strconv.Atoi(fields[0])
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
	fields := strings.Fields(fullString)

	var reportMessage string
	if len(fields) == 1 {
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

func ParseChange(argStr string) (*CommandDetails, string) {
	// 追加オプションチェック
	fields := strings.Fields(argStr)
	if len(fields) == 0 {
		return nil, i18n.T("parse:invalid-option")
	}
	options, message := ParseMinWorkOrderOptions(argStr)
	if message != "" {
		return nil, message
	}

	return &CommandDetails{
		CommandType:  Change,
		ChangeOption: *options,
	}, ""
}

func ParseSeat(argStr string) (*CommandDetails, string) {
	fields := strings.Fields(argStr)
	showDetails := false

	if len(fields) >= 1 && fields[0] == ShowDetailsOption {
		showDetails = true
	}

	return &CommandDetails{
		CommandType: Seat,
		SeatOption: SeatOption{
			ShowDetails: showDetails,
		},
	}, ""
}

func ParseMore(argStr string) (*CommandDetails, string) {
	fields := strings.Fields(argStr)

	if len(fields) == 0 || (len(fields) == 1 && fields[0] == "") {
		return &CommandDetails{
			CommandType: More,
			MoreOption: MoreOption{
				IsDurationMinSet: false,
			},
		}, ""
	}

	var durationMin int
	if len(fields) >= 2 {
		if fields[0] == TimeOptionKey {
			value, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, i18n.T("parse:invalid-time-option")
			}
			durationMin = value
		} else {
			value, err := strconv.Atoi(fields[0])
			if err != nil {
				return nil, i18n.T("parse:invalid-time-option")
			}
			durationMin = value
		}
	} else if len(fields) == 1 {
		if fields[0] == TimeOptionKey {
			return nil, i18n.T("parse:check-option", TimeOptionPrefix)
		}
		value, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, i18n.T("parse:invalid-time-option")
		}
		durationMin = value
	} else {
		return nil, i18n.T("parse:missing-more-option")
	}

	return &CommandDetails{
		CommandType: More,
		MoreOption: MoreOption{
			IsDurationMinSet: true,
			DurationMin:      durationMin,
		},
	}, ""
}

func ParseBreak(argStr string) (*CommandDetails, string) {
	// 追加オプションチェック
	options, message := ParseMinWorkOrderOptions(argStr)
	if message != "" {
		return nil, message
	}

	return &CommandDetails{
		CommandType: Break,
		BreakOption: *options,
	}, ""
}

func ParseResume(argStr string) (*CommandDetails, string) {
	// 作業名
	option := ParseWorkNameOption(argStr)

	return &CommandDetails{
		CommandType:  Resume,
		ResumeOption: option,
	}, ""
}

func ParseOrder(argStr string) (*CommandDetails, string) {
	// NOTE: オプションは番号か文字列のどちらかのみ

	option, errMessage := ParseOrderOption(argStr)
	if errMessage != "" {
		return nil, errMessage
	}

	return &CommandDetails{
		CommandType: Order,
		OrderOption: *option,
	}, ""
}

func ParseOrderOption(argStr string) (*OrderOption, string) {
	fields := strings.Fields(argStr)
	if len(fields) == 0 {
		return nil, i18n.T("parse:invalid-option")
	}
	if len(fields) == 1 && fields[0] == "" {
		return nil, i18n.T("parse:invalid-option")
	}

	// cancel flag?
	if fields[0] == OrderClearOption {
		return &OrderOption{
			ClearFlag: true,
		}, ""
	}

	num, err := strconv.Atoi(fields[0])
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
	if len(fields) == 1 && fields[0] == "" {
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

func ParseMinWorkOrderOptions(argStr string) (*MinWorkOrderOption, string) {
	var options MinWorkOrderOption

	const (
		Min = iota
		Order
		Any
		Work
	)

	currentMode := Any // NOTE: 作業内容はオプションwork明示なしもあるので、初期値はWork
	for _, field := range strings.Fields(argStr) {
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

// ReplaceEmojiMinToText は"min="や"min=360"の絵文字をテキストに変換する。
func ReplaceEmojiMinToText(emojiString string) (string, error) {
	tmp := strings.TrimPrefix(emojiString, EmojiCommandPrefix) // ex. "360Min0:"
	loc := emojiMinRegex.FindStringIndex(tmp)
	if len(loc) != 2 {
		return "", errors.New("invalid emoji min string. tmp=" + tmp)
	}
	numString := tmp[:loc[0]] // ex. "360"
	if numString != "" {      // "min=xxx" emoji -> "min xxx"
		num, err := strconv.Atoi(numString)
		if err != nil {
			return "", err
		}
		return TimeOptionKey + HalfWidthSpace + strconv.Itoa(num) + HalfWidthSpace, nil
	}
	// "min=" emoji -> "min "
	return TimeOptionPrefix + HalfWidthSpace, nil
}
