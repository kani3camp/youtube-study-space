package presenter

import (
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/workspaceapp/usecase"
)

// BuildMoreMessage converts More events into a localized response.
// Namespace: command-more
func BuildMoreMessage(res usecase.Result, displayName string) string {
	msg := i18nmsg.CommonSir(displayName)
	for _, event := range res.Events {
		switch e := event.(type) {
		case usecase.MoreEnterOnly:
			msg += i18nmsg.CommandEnterOnly()
		case usecase.MoreMaxWork:
			msg += i18nmsg.CommandMoreMaxWork()
		case usecase.MoreWorkExtended:
			msg += i18nmsg.CommandMoreReplyWork(e.AddedMin)
		case usecase.MoreMaxBreak:
			msg += i18nmsg.CommandMoreMaxBreak()
		case usecase.MoreBreakExtended:
			msg += i18nmsg.CommandMoreReplyBreak(e.AddedMin, e.RemainingBreakMin)
		case usecase.MoreSummary:
			msg += i18nmsg.CommandMoreReply(e.RealtimeEnteredMin, e.RemainingUntilExitMin)
		}
	}
	return msg
}
