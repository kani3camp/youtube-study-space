package presenter

import (
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/workspaceapp/usecase"
)

// BuildResumeMessage converts Resume events into a localized response.
// Namespace: command-resume
// Note: Resumeはsir接頭辞なし（テスト準拠）
func BuildResumeMessage(res usecase.Result, displayName string) string {
	msg := ""
	for _, event := range res.Events {
		switch e := event.(type) {
		case usecase.ResumeBreakOnly:
			msg += i18nmsg.CommandResumeBreakOnly(displayName)
		case usecase.ResumeStarted:
			seat := seatIDStr(e.SeatID, e.IsMemberSeat)
			msg += i18nmsg.CommandResumeWork(displayName, seat, e.RemainingUntilExitMin)
		}
	}
	return msg
}
