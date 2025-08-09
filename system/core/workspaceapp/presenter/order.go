package presenter

import (
	"app.modules/core/i18n"
	"app.modules/core/workspaceapp/usecase"
)

// BuildOrderMessage converts Order events into a localized response.
// Namespace: command-order
// Note: Orderはsir接頭辞なし（既存テスト準拠）
func BuildOrderMessage(res usecase.Result, displayName string) string {
	t := i18n.GetTFunc("command-order")
	msg := ""
	for _, ev := range res.Events {
		switch e := ev.(type) {
		case usecase.OrderEnterOnly:
			msg += i18n.T("command:enter-only", displayName)
		case usecase.OrderTooMany:
			msg += t("too-many-orders", displayName, e.MaxDailyOrderCount)
		case usecase.OrderCleared:
			msg += t("cleared", displayName)
		case usecase.OrderOrdered:
			msg += t("ordered", displayName, e.MenuName, e.CountAfter)
		}
	}
	return msg
}
