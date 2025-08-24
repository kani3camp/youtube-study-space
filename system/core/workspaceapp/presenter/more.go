package presenter

import (
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/workspaceapp/usecase"
	"strings"
)

// BuildMoreMessage converts More events into a localized response.
// Namespace: command-more
func BuildMoreMessage(res usecase.Result, displayName string) string {
	var builder strings.Builder
	builder.WriteString(i18nmsg.CommonSir(displayName))
	for _, event := range res.Events {
		switch e := event.(type) {
		case usecase.MoreEnterOnly:
			builder.WriteString(i18nmsg.CommandEnterOnly())
		case usecase.MoreMaxWork:
			builder.WriteString(i18nmsg.CommandMoreMaxWork())
		case usecase.MoreWorkExtended:
			builder.WriteString(i18nmsg.CommandMoreReplyWork(e.AddedMin))
		case usecase.MoreMaxBreak:
			builder.WriteString(i18nmsg.CommandMoreMaxBreak())
		case usecase.MoreBreakExtended:
			builder.WriteString(i18nmsg.CommandMoreReplyBreak(e.AddedMin, e.RemainingBreakMin))
		case usecase.MoreSummary:
			builder.WriteString(i18nmsg.CommandMoreReply(e.RealtimeEnteredMin, e.RemainingUntilExitMin))
		}
	}
	return builder.String()
}
