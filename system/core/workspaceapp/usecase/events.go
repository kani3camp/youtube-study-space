package usecase

// Event is a marker interface for usecase events.
type Event interface{ isEvent() }

// SeatMoved represents that a user moved seats.
type SeatMoved struct {
	FromSeatID       int
	FromIsMemberSeat bool
	ToSeatID         int
	ToIsMemberSeat   bool
	WorkName         string
	WorkedTimeSec    int
	AddedRP          int
	RankVisible      bool
	UntilExitMin     int
}

func (SeatMoved) isEvent() {}

// SeatEntered represents that a user entered a seat.
type SeatEntered struct {
	SeatID       int
	IsMemberSeat bool
	WorkName     string
	UntilExitMin int
}

func (SeatEntered) isEvent() {}

// MenuOrdered represents that a user ordered a menu item.
type MenuOrdered struct {
	MenuName   string
	CountAfter int64
}

func (MenuOrdered) isEvent() {}

// OrderLimitExceeded represents that order count exceeded the daily limit.
type OrderLimitExceeded struct {
	MaxDailyOrderCount int
}

func (OrderLimitExceeded) isEvent() {}

// ============ Change usecase events ============
// These events are used by the Change handler to describe state changes
// or rejections, and then formatted by presenter/change.go outside the tx.

type ChangeUpdatedWork struct {
	WorkName     string
	SeatID       int
	IsMemberSeat bool
}

func (ChangeUpdatedWork) isEvent() {}

type ChangeUpdatedBreak struct {
	WorkName     string
	SeatID       int
	IsMemberSeat bool
}

func (ChangeUpdatedBreak) isEvent() {}

type ChangeWorkDurationRejectedBefore struct {
	RequestedMin             int
	RealtimeEntryDurationMin int
	RemainingWorkMin         int
}

func (ChangeWorkDurationRejectedBefore) isEvent() {}

type ChangeWorkDurationRejectedAfter struct {
	MaxWorkTimeMin           int
	RealtimeEntryDurationMin int
	RemainingWorkMin         int
}

func (ChangeWorkDurationRejectedAfter) isEvent() {}

type ChangeWorkDurationUpdated struct {
	RequestedMin             int
	RealtimeEntryDurationMin int
	RemainingWorkMin         int
}

func (ChangeWorkDurationUpdated) isEvent() {}

type ChangeBreakDurationRejectedBefore struct {
	RequestedMin             int
	RealtimeBreakDurationMin int
	RemainingBreakMin        int
}

func (ChangeBreakDurationRejectedBefore) isEvent() {}

type ChangeBreakDurationUpdated struct {
	RequestedMin             int
	RealtimeBreakDurationMin int
	RemainingBreakMin        int
}

func (ChangeBreakDurationUpdated) isEvent() {}

// Validation error occurred in Change usecase (message already localized)
type ChangeValidationError struct {
	Message string
}

func (ChangeValidationError) isEvent() {}

// ============ More usecase events ============
// Events produced by the More handler.

type MoreEnterOnly struct{}

func (MoreEnterOnly) isEvent() {}

type MoreMaxWork struct {
	MaxWorkTimeMin int
}

func (MoreMaxWork) isEvent() {}

type MoreWorkExtended struct {
	AddedMin int
}

func (MoreWorkExtended) isEvent() {}

type MoreMaxBreak struct {
	MaxBreakDurationMin int
}

func (MoreMaxBreak) isEvent() {}

type MoreBreakExtended struct {
	AddedMin          int
	RemainingBreakMin int
}

func (MoreBreakExtended) isEvent() {}

type MoreSummary struct {
	RealtimeEnteredMin    int
	RemainingUntilExitMin int
}

func (MoreSummary) isEvent() {}

// ============ Break usecase events ============
// Event produced when a break is successfully started.
type BreakStarted struct {
	SeatID       int
	IsMemberSeat bool
	WorkName     string
	DurationMin  int
}

func (BreakStarted) isEvent() {}

type BreakEnterOnly struct{}

func (BreakEnterOnly) isEvent() {}

type BreakWorkOnly struct{}

func (BreakWorkOnly) isEvent() {}

type BreakWarn struct {
	MinBreakIntervalMin int
	CurrentWorkedMin    int
}

func (BreakWarn) isEvent() {}

// ============ Resume usecase events ============

type ResumeEnterOnly struct{}

func (ResumeEnterOnly) isEvent() {}

type ResumeBreakOnly struct{}

func (ResumeBreakOnly) isEvent() {}

type ResumeStarted struct {
	SeatID                int
	IsMemberSeat          bool
	RemainingUntilExitMin int
}

func (ResumeStarted) isEvent() {}

// ============ Order usecase events ============
type OrderEnterOnly struct{}

func (OrderEnterOnly) isEvent() {}

type OrderTooMany struct {
	MaxDailyOrderCount int
}

func (OrderTooMany) isEvent() {}

type OrderCleared struct{}

func (OrderCleared) isEvent() {}

type OrderOrdered struct {
	MenuName   string
	CountAfter int64
}

func (OrderOrdered) isEvent() {}

// ============ Clear usecase events ============
type ClearEnterOnly struct{}

func (ClearEnterOnly) isEvent() {}

type ClearWork struct {
	SeatID       int
	IsMemberSeat bool
}

func (ClearWork) isEvent() {}

type ClearBreak struct {
	SeatID       int
	IsMemberSeat bool
}

func (ClearBreak) isEvent() {}

// Result aggregates events produced by a usecase execution.
type Result struct {
	Events []Event
}

// Add appends an event to the result.
func (r *Result) Add(e Event) {
	r.Events = append(r.Events, e)
}
