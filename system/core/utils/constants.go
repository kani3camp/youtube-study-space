package utils

const (
	CommandPrefix       = "!"
	WrongCommandPrefix  = "！"
	MemberCommandPrefix = "/"
	
	InCommand      = "!in"
	OutCommand     = "!out"
	InfoCommand    = "!info"
	MyCommand      = "!my"
	ChangeCommand  = "!change"
	SeatCommand    = "!seat"
	ReportCommand  = "!report"
	MoreCommand    = "!more"
	OkawariCommand = "!okawari"
	RankCommand    = "!rank"
	BreakCommand   = "!break"
	RestCommand    = "!rest"
	ChillCommand   = "!chill"
	ResumeCommand  = "!resume"
	
	KickCommand  = "!kick"
	CheckCommand = "!check"
	BlockCommand = "!block"
	
	MemberInCommand = "/in"
	
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
	WorkString         = "Work"
	MinString          = "Min"
	ColorString        = "Color"
	RankOnString       = "RankOn"
	RankOffString      = "RankOff"
	MemberInString     = "MemberIn"
	
	WorkNameOptionPrefix            = "work="
	WorkNameOptionShortPrefix       = "w="
	WorkNameOptionPrefixLegacy      = "work-"
	WorkNameOptionShortPrefixLegacy = "w-"
	
	TimeOptionPrefix            = "min="
	TimeOptionShortPrefix       = "m="
	TimeOptionPrefixLegacy      = "min-"
	TimeOptionShortPrefixLegacy = "m-"
	
	ShowDetailsOption = "d"
	
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
	
	EmojiWork
	EmojiMin
	EmojiColor
	EmojiRankOn
	EmojiRankOff
)
