package presenter

import (
	"app.modules/core/i18n"
	"app.modules/core/workspaceapp/usecase"
)

// BuildMoreMessage converts More events into a localized response.
// Namespace: command-more
func BuildMoreMessage(res usecase.Result, displayName string) string {
	t := i18n.GetTFunc("command-more")
	msg := i18n.T("common:sir", displayName)
	for _, event := range res.Events {
		switch e := event.(type) {
		case usecase.MoreMaxWork:
			msg += t("max-work", e.MaxWorkTimeMin)
		case usecase.MoreWorkExtended:
			msg += t("reply-work", e.AddedMin)
		case usecase.MoreMaxBreak:
			msg += t("max-break", e.MaxBreakDurationMin)
		case usecase.MoreBreakExtended:
			msg += t("reply-break", e.AddedMin, e.RemainingBreakMin)
		case usecase.MoreSummary:
			msg += t("reply", e.RealtimeEnteredMin, e.RemainingUntilExitMin)
		}
	}
	return msg
}
