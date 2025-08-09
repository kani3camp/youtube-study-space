package presenter

import (
	"app.modules/core/i18n"
	"app.modules/core/workspaceapp/usecase"
)

// BuildResumeMessage converts Resume events into a localized response.
// Namespace: command-resume
// Note: Resumeはsir接頭辞なし（テスト準拠）
func BuildResumeMessage(res usecase.Result, displayName string) string {
	t := i18n.GetTFunc("command-resume")
	msg := ""
	for _, event := range res.Events {
		switch e := event.(type) {
		case usecase.ResumeBreakOnly:
			msg += t("break-only", displayName)
		case usecase.ResumeStarted:
			seat := seatIDStr(e.SeatID, e.IsMemberSeat)
			msg += t("work", displayName, seat, e.RemainingUntilExitMin)
		}
	}
	return msg
}
