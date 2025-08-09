package presenter

import (
    "strconv"

    "app.modules/core/i18n"
)

func seatIDStr(seatID int, isMemberSeat bool) string {
    if isMemberSeat {
        return i18n.T("common:vip-seat-id", seatID)
    }
    return strconv.Itoa(seatID)
}


