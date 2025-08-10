package presenter

import (
	"strconv"

	i18nmsg "app.modules/core/i18n/typed"
)

func seatIDStr(seatID int, isMemberSeat bool) string {
	if isMemberSeat {
		return i18nmsg.CommonVipSeatId(seatID)
	}
	return strconv.Itoa(seatID)
}
