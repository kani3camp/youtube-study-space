package presenter

import (
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/workspaceapp/usecase"
)

// BuildOrderMessage converts Order events into a localized response.
// Namespace: command-order
// Note: Orderはsir接頭辞なし（既存テスト準拠）
func BuildOrderMessage(res usecase.Result, displayName string) string {
	msg := ""
	for _, event := range res.Events {
		switch e := event.(type) {
		case usecase.OrderEnterOnly:
			msg += i18nmsg.CommandEnterOnly(displayName)
		case usecase.OrderTooMany:
			msg += i18nmsg.CommandOrderTooManyOrders(displayName, e.MaxDailyOrderCount)
		case usecase.OrderCleared:
			msg += i18nmsg.CommandOrderCleared(displayName)
		case usecase.OrderOrdered:
			msg += i18nmsg.CommandOrderOrdered(displayName, e.MenuName, e.CountAfter)
		}
	}
	return msg
}
