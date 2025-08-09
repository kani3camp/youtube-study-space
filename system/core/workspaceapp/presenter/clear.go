package presenter

import (
	"app.modules/core/i18n"
	"app.modules/core/workspaceapp/usecase"
)

// BuildClearMessage converts Clear events into a localized response.
// Namespace: others
// Note: Clearはsir接頭辞なし（既存テスト準拠）
func BuildClearMessage(res usecase.Result, displayName string) string {
	msg := ""
	for _, ev := range res.Events {
		switch e := ev.(type) {
		case usecase.ClearEnterOnly:
			msg += i18n.T("command:enter-only", displayName)
		case usecase.ClearWork:
			msg += i18n.T("others:clear-work", displayName, e.SeatID)
		case usecase.ClearBreak:
			msg += i18n.T("others:clear-break", displayName, e.SeatID)
		}
	}
	return msg
}
