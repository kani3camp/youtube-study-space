package presenter

import (
	"app.modules/core/i18n"
	"app.modules/core/workspaceapp/usecase"
)

// BuildInMessage converts In usecase result events into a localized response message.
func BuildInMessage(res usecase.Result, displayName string) string {
	t := i18n.GetTFunc("command-in")
	msg := ""
	for _, event := range res.Events {
		switch e := event.(type) {
		case usecase.OrderLimitExceeded:
			msg += t("too-many-orders", e.MaxDailyOrderCount)
		case usecase.MenuOrdered:
			msg += t("ordered", e.MenuName, e.CountAfter)
		case usecase.SeatMoved:
			rpEarned := ""
			if e.RankVisible && e.AddedRP != 0 {
				rpEarned = i18n.T("command:rp-earned", e.AddedRP)
			}
			prevSeat := seatIDStr(e.FromSeatID, e.FromIsMemberSeat)
			nextSeat := seatIDStr(e.ToSeatID, e.ToIsMemberSeat)
			msg += t("seat-move", displayName, e.WorkName, prevSeat, nextSeat, e.WorkedTimeSec/60, rpEarned, e.UntilExitMin)
		case usecase.SeatEntered:
			seat := seatIDStr(e.SeatID, e.IsMemberSeat)
			msg += t("start", displayName, e.WorkName, e.UntilExitMin, seat)
		}
	}
	return msg
}
