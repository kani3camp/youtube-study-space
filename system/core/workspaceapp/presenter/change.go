package presenter

import (
	"app.modules/core/i18n"
	"app.modules/core/workspaceapp/usecase"
)

// BuildChangeMessage converts Change usecase events into a localized response.
func BuildChangeMessage(res usecase.Result, displayName string) string {
	t := i18n.GetTFunc("command-change")
	msg := i18n.T("common:sir", displayName)
	for _, event := range res.Events {
		switch e := event.(type) {
		case usecase.ChangeValidationError:
			msg += e.Message
		case usecase.ChangeUpdatedWork:
			seat := seatIDStr(e.SeatID, e.IsMemberSeat)
			msg += t("update-work", e.WorkName, seat)
		case usecase.ChangeUpdatedBreak:
			seat := seatIDStr(e.SeatID, e.IsMemberSeat)
			msg += t("update-break", e.WorkName, seat)
		case usecase.ChangeWorkDurationRejectedBefore:
			msg += t("work-duration-before", e.RequestedMin, e.RealtimeEntryDurationMin, e.RemainingWorkMin)
		case usecase.ChangeWorkDurationRejectedAfter:
			msg += t("work-duration-after", e.MaxWorkTimeMin, e.RealtimeEntryDurationMin, e.RemainingWorkMin)
		case usecase.ChangeWorkDurationUpdated:
			msg += t("work-duration", e.RequestedMin, e.RealtimeEntryDurationMin, e.RemainingWorkMin)
		case usecase.ChangeBreakDurationRejectedBefore:
			msg += t("break-duration-before", e.RequestedMin, e.RealtimeBreakDurationMin, e.RemainingBreakMin)
		case usecase.ChangeBreakDurationUpdated:
			msg += t("break-duration", e.RequestedMin, e.RealtimeBreakDurationMin, e.RemainingBreakMin)
		}
	}
	return msg
}
