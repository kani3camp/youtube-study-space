package presenter

import (
	"strconv"

	"app.modules/core/i18n"
	"app.modules/core/workspaceapp/usecase"
)

// BuildInMessage converts In usecase result events into a localized response message.
// t must be a localizer with namespace "command-in".
func BuildInMessage(res usecase.Result, t i18n.TFuncType, displayName string) string {
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

func seatIDStr(seatID int, isMemberSeat bool) string {
	if isMemberSeat {
		return i18n.T("common:vip-seat-id", seatID)
	}
	return strconv.Itoa(seatID)
}
