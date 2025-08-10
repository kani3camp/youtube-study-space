package presenter

import (
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp/usecase"
)

// BuildClearMessage converts Clear events into a localized response.
// Namespace: others
// Note: Clearはsir接頭辞なし（既存テスト準拠）
func BuildClearMessage(res usecase.Result, displayName string) string {
	msg := ""
	for _, event := range res.Events {
		switch e := event.(type) {
		case usecase.ClearEnterOnly:
			msg += i18nmsg.CommandEnterOnly(displayName)
		case usecase.ClearWork:
			seat := utils.SeatIdStr(e.SeatID, e.IsMemberSeat)
			msg += i18nmsg.OthersClearWork(displayName, seat)
		case usecase.ClearBreak:
			seat := utils.SeatIdStr(e.SeatID, e.IsMemberSeat)
			msg += i18nmsg.OthersClearBreak(displayName, seat)
		}
	}
	return msg
}
