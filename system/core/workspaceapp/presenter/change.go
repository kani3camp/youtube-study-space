package presenter

import (
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/workspaceapp/usecase"
	"strings"
)

// BuildChangeMessage converts Change usecase events into a localized response.
func BuildChangeMessage(res usecase.Result, displayName string) string {
	var builder strings.Builder
	builder.WriteString(i18nmsg.CommonSir(displayName))
	for _, event := range res.Events {
		switch e := event.(type) {
		case usecase.ChangeValidationError:
			builder.WriteString(e.Message)
		case usecase.ChangeUpdatedWork:
			seat := SeatIDStr(e.SeatID, e.IsMemberSeat)
			builder.WriteString(i18nmsg.CommandChangeUpdateWork(e.WorkName, seat))
		case usecase.ChangeUpdatedBreak:
			seat := SeatIDStr(e.SeatID, e.IsMemberSeat)
			builder.WriteString(i18nmsg.CommandChangeUpdateBreak(e.WorkName, seat))
		case usecase.ChangeWorkDurationRejectedBefore:
			builder.WriteString(i18nmsg.CommandChangeWorkDurationBefore(e.RequestedMin, e.RealtimeEntryDurationMin, e.RemainingWorkMin))
		case usecase.ChangeWorkDurationRejectedAfter:
			builder.WriteString(i18nmsg.CommandChangeWorkDurationAfter(e.MaxWorkTimeMin, e.RealtimeEntryDurationMin, e.RemainingWorkMin))
		case usecase.ChangeWorkDurationUpdated:
			builder.WriteString(i18nmsg.CommandChangeWorkDuration(e.RequestedMin, e.RealtimeEntryDurationMin, e.RemainingWorkMin))
		case usecase.ChangeBreakDurationRejectedBefore:
			builder.WriteString(i18nmsg.CommandChangeBreakDurationBefore(e.RequestedMin, e.RealtimeBreakDurationMin, e.RemainingBreakMin))
		case usecase.ChangeBreakDurationUpdated:
			builder.WriteString(i18nmsg.CommandChangeBreakDuration(e.RequestedMin, e.RealtimeBreakDurationMin, e.RemainingBreakMin))
		}
	}
	return builder.String()
}
