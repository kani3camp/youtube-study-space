package presenter

import (
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/workspaceapp/usecase"
)

// BuildChangeMessage converts Change usecase events into a localized response.
func BuildChangeMessage(res usecase.Result, displayName string) string {
	msg := i18nmsg.CommonSir(displayName)
	for _, event := range res.Events {
		switch e := event.(type) {
		case usecase.ChangeValidationError:
			msg += e.Message
		case usecase.ChangeUpdatedWork:
			seat := seatIDStr(e.SeatID, e.IsMemberSeat)
			msg += i18nmsg.CommandChangeUpdateWork(e.WorkName, seat)
		case usecase.ChangeUpdatedBreak:
			seat := seatIDStr(e.SeatID, e.IsMemberSeat)
			msg += i18nmsg.CommandChangeUpdateBreak(e.WorkName, seat)
		case usecase.ChangeWorkDurationRejectedBefore:
			msg += i18nmsg.CommandChangeWorkDurationBefore(e.RequestedMin, e.RealtimeEntryDurationMin, e.RemainingWorkMin)
		case usecase.ChangeWorkDurationRejectedAfter:
			msg += i18nmsg.CommandChangeWorkDurationAfter(e.MaxWorkTimeMin, e.RealtimeEntryDurationMin, e.RemainingWorkMin)
		case usecase.ChangeWorkDurationUpdated:
			msg += i18nmsg.CommandChangeWorkDuration(e.RequestedMin, e.RealtimeEntryDurationMin, e.RemainingWorkMin)
		case usecase.ChangeBreakDurationRejectedBefore:
			msg += i18nmsg.CommandChangeBreakDurationBefore(e.RequestedMin, e.RealtimeBreakDurationMin, e.RemainingBreakMin)
		case usecase.ChangeBreakDurationUpdated:
			msg += i18nmsg.CommandChangeBreakDuration(e.RequestedMin, e.RealtimeBreakDurationMin, e.RemainingBreakMin)
		}
	}
	return msg
}
