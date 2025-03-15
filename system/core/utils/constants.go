package utils

const (
	CommandPrefix                = "!"
	CommandPrefixFullWidth       = "！"
	MemberCommandPrefix          = "/"
	MemberCommandPrefixFullWidth = "／"

	InCommand         = "!in"
	InZeroCommand     = "!0"
	OutCommand        = "!out"
	InfoCommand       = "!info"
	InfoDCommand      = "!info d"
	MyCommand         = "!my"
	ChangeCommand     = "!change"
	SeatCommand       = "!seat"
	SeatDCommand      = "!seat d"
	ReportCommand     = "!report"
	MoreCommand       = "!more"
	OkawariCommand    = "!okawari"
	RankCommand       = "!rank"
	BreakCommand      = "!break"
	RestCommand       = "!rest"
	ChillCommand      = "!chill"
	ResumeCommand     = "!resume"
	OrderCommand      = "!order"
	WorkCommand       = "!work"
	ClearCommand      = "!clear"
	ClearShortCommand = "!clr"

	KickCommand  = "!kick"
	CheckCommand = "!check"
	BlockCommand = "!block"

	MemberInCommand   = "/in"
	MemberWorkCommand = "/work"

	MemberKickCommand  = "/kick"
	MemberCheckCommand = "/check"
	MemberBlockCommand = "/block"

	EmojiSide          = ":"
	EmojiCommandPrefix = EmojiSide + "_command"
	InString           = "In"
	InZeroString       = "InZero"
	OutString          = "Out"
	InfoString         = "Info"
	InfoDString        = "InfoD"
	MyString           = "My"
	ChangeString       = "Change"
	SeatString         = "Seat"
	SeatDString        = "SeatD"
	MoreString         = "More"
	BreakString        = "Break"
	ResumeString       = "Resume"
	RankString         = "Rank"
	WorkString         = "Work"
	MinString          = "Min"
	ColorString        = "Color"
	RankOnString       = "RankOn"
	RankOffString      = "RankOff"
	MemberInString     = "MemberIn"
	OrderString        = "Order"

	WorkNameOptionPrefix            = "work="
	WorkNameOptionShortPrefix       = "w="
	WorkNameOptionPrefixLegacy      = "work-"
	WorkNameOptionShortPrefixLegacy = "w-"
	WorkNameOptionKey               = "work"

	TimeOptionPrefix            = "min="
	TimeOptionShortPrefix       = "m="
	TimeOptionPrefixLegacy      = "min-"
	TimeOptionShortPrefixLegacy = "m-"
	TimeOptionKey               = "min"

	OrderOptionPrefix = "order="
	OrderOptionKey    = "order"

	ShowDetailsOption = "d"
	OrderCancelOption = "-"

	RankVisibleMyOptionPrefix = "rank="
	RankVisibleMyOptionKey    = "rank"
	RankVisibleMyOptionOn     = "on"
	RankVisibleMyOptionOff    = "off"

	FavoriteColorMyOptionPrefix = "color="
	FavoriteColorMyOptionKey    = "color"

	FullWidthSpace     = "　"
	HalfWidthSpace     = " "
	FullWidthEqualSign = "＝"
	HalfWidthEqualSign = "="
)

type EmojiElement int

const (
	EmojiIn EmojiElement = iota
	EmojiInZero
	EmojiOut
	EmojiInfo
	EmojiInfoD
	EmojiMy
	EmojiChange
	EmojiSeat
	EmojiSeatD
	EmojiMore
	EmojiBreak
	EmojiResume
	EmojiMemberIn
	EmojiOrder
	EmojiOrderCancel

	EmojiWork
	EmojiMin
	EmojiColor
	EmojiRankOn
	EmojiRankOff
)
