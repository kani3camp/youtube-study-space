package presenter

import (
    "app.modules/core/i18n"
    "app.modules/core/workspaceapp/usecase"
)

// BuildBreakMessage converts Break events into a localized response.
// Namespace: command-break
// Note: Breakの文面は先頭のsir接頭辞を付けない（既存テスト準拠）
func BuildBreakMessage(res usecase.Result, displayName string) string {
    t := i18n.GetTFunc("command-break")
    msg := ""
    for _, ev := range res.Events {
        switch e := ev.(type) {
        case usecase.BreakStarted:
            seat := seatIDStr(e.SeatID, e.IsMemberSeat)
            msg += t("break", displayName, e.WorkName, e.DurationMin, seat)
        }
    }
    return msg
}


