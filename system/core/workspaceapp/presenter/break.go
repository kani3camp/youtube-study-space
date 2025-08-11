package presenter

import (
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/workspaceapp/usecase"
)

// BuildBreakMessage converts Break events into a localized response.
// Namespace: command-break
func BuildBreakMessage(res usecase.Result, displayName string) string {
	msg := ""
	for _, event := range res.Events {
		switch e := event.(type) {
		case usecase.BreakEnterOnly:
			msg += i18nmsg.CommonSir(displayName) + i18nmsg.CommandEnterOnly()
		case usecase.BreakWorkOnly:
			msg += i18nmsg.CommandBreakWorkOnly(displayName)
		case usecase.BreakWarn:
			msg += i18nmsg.CommandBreakWarn(displayName, e.MinBreakIntervalMin, e.CurrentWorkedMin)
		case usecase.BreakStarted:
			seat := SeatIDStr(e.SeatID, e.IsMemberSeat)
			msg += i18nmsg.CommandBreakBreak(displayName, e.WorkName, e.DurationMin, seat)
		}
	}
	return msg
}
