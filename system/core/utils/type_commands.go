package utils

type CommandDetails struct {
	CommandType  CommandType
	InOption     InOption
	InfoOption   InfoOption
	MyOptions    []MyOption
	SeatOption   SeatOption
	KickOption   KickOption
	CheckOption  CheckOption
	BlockOption  BlockOption
	ReportOption ReportOption
	ChangeOption MinWorkOrderOption
	MoreOption   MoreOption
	BreakOption  MinWorkOrderOption
	ResumeOption WorkNameOption
	OrderOption  OrderOption
}

type CommandType uint

const (
	NotCommand CommandType = iota
	InvalidCommand
	In     // !in or /in or !{seat No.} or /{seat No.}
	Out    // !out
	Info   // !info
	My     // !my
	Change // !change
	Seat   // !seat
	Report // !report
	Kick   // !kick
	Check  // !check
	Block  // !block
	More   // !more
	Rank   // !rank
	Break  // !break
	Resume // !resume
	Order
	Clear
)

type InfoOption struct {
	ShowDetails bool
}

type MyOptionType uint

const (
	RankVisible MyOptionType = iota
	DefaultStudyMin
	FavoriteColor
)

type InOption struct {
	IsSeatIdSet        bool
	SeatId             int
	MinutesAndWorkName *MinWorkOrderOption
	IsMemberSeat       bool
}

type MyOption struct {
	Type        MyOptionType
	IntValue    int
	BoolValue   bool
	StringValue string
}

type SeatOption struct {
	ShowDetails bool
}

type KickOption struct {
	SeatId             int
	IsTargetMemberSeat bool
}

type CheckOption struct {
	SeatId             int
	IsTargetMemberSeat bool
}

type BlockOption struct {
	SeatId             int
	IsTargetMemberSeat bool
}

type ReportOption struct {
	Message string
}

type MoreOption struct {
	DurationMin int
}

type WorkNameOption struct {
	IsWorkNameSet bool
	WorkName      string
}

type MinWorkOrderOption struct {
	IsWorkNameSet    bool
	IsDurationMinSet bool
	IsOrderSet       bool
	WorkName         string
	DurationMin      int
	OrderNum         int
}

type OrderOption struct {
	IntValue  int
	ClearFlag bool
}

func (o *MinWorkOrderOption) NumOptionsSet() int {
	return NumTrue(o.IsWorkNameSet, o.IsDurationMinSet)
}

type UserIdTotalStudySecSet struct {
	UserId        string `json:"user_id"`
	TotalStudySec int    `json:"total_study_sec"`
}
