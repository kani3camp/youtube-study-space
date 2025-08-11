package presenter

import (
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/workspaceapp/usecase"
)

// BuildInMessage converts In usecase result events into a localized response message.
func BuildInMessage(res usecase.Result, displayName string) string {
	msg := ""
	for _, event := range res.Events {
		switch e := event.(type) {
		case usecase.OrderLimitExceeded:
			msg += i18nmsg.CommandInTooManyOrders(e.MaxDailyOrderCount)
		case usecase.MenuOrdered:
			msg += i18nmsg.CommandInOrdered(e.MenuName, e.CountAfter)
		case usecase.SeatMoved:
			rpEarned := ""
			if e.RankVisible && e.AddedRP != 0 {
				rpEarned = i18nmsg.CommandRpEarned(e.AddedRP)
			}
			prevSeat := SeatIDStr(e.FromSeatID, e.FromIsMemberSeat)
			nextSeat := SeatIDStr(e.ToSeatID, e.ToIsMemberSeat)
			msg += i18nmsg.CommandInSeatMove(displayName, e.WorkName, prevSeat, nextSeat, e.WorkedTimeSec/60, rpEarned, e.UntilExitMin)
		case usecase.SeatEntered:
			seat := SeatIDStr(e.SeatID, e.IsMemberSeat)
			msg += i18nmsg.CommandInStart(displayName, e.WorkName, e.UntilExitMin, seat)
		}
	}
	return msg
}
