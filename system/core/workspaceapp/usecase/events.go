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

// Result aggregates events produced by a usecase execution.
type Result struct {
	Events []Event
}

// Add appends an event to the result.
func (r *Result) Add(e Event) {
	r.Events = append(r.Events, e)
}
