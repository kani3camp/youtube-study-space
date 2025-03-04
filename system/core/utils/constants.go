package utils

const (
	CommandPrefix                = "!"
	CommandPrefixFullWidth       = "！"
	MemberCommandPrefix          = "/"
	MemberCommandPrefixFullWidth = "／"

	InCommand      = "!in"
	InZeroCommand  = "!0"
	OutCommand     = "!out"
	InfoCommand    = "!info"
	InfoDCommand   = "!info d"
	MyCommand      = "!my"
	ChangeCommand  = "!change"
	SeatCommand    = "!seat"
	SeatDCommand   = "!seat d"
	ReportCommand  = "!report"
	MoreCommand    = "!more"
	OkawariCommand = "!okawari"
	RankCommand    = "!rank"
	BreakCommand   = "!break"
	RestCommand    = "!rest"
	ChillCommand   = "!chill"
	ResumeCommand  = "!resume"
	OrderCommand   = "!order"
	WorkCommand    = "!work"

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

	TimeOptionPrefix            = "min="
	TimeOptionShortPrefix       = "m="
	TimeOptionPrefixLegacy      = "min-"
	TimeOptionShortPrefixLegacy = "m-"

	OrderOptionPrefix      = "order="
	OrderOptionShortPrefix = "o="

	ShowDetailsOption = "d"
	OrderCancelOption = "-"

	RankVisibleMyOptionPrefix = "rank="
	RankVisibleMyOptionOn     = "on"
	RankVisibleMyOptionOff    = "off"

	FavoriteColorMyOptionPrefix = "color="

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
