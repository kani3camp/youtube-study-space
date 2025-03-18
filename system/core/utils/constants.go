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
	OrderClearCommand = "!order -"
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
	OrderClearString   = "OrderClr"

	WorkNameOptionKey = "work"

	TimeOptionPrefix = "min="
	TimeOptionKey    = "min"

	OrderOptionPrefix = "order="
	OrderOptionKey    = "order"

	ShowDetailsOption = "d"
	OrderClearOption  = "-"

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
