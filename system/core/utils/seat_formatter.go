package utils

import (
	"strconv"

	"app.modules/core/i18n"
)

// FormatSeatId formats seat ID for display based on seat type
func FormatSeatId(seatId int, isMemberSeat bool) string {
	if isMemberSeat {
		return i18n.T("common:vip-seat-id", seatId)
	}
	return strconv.Itoa(seatId)
}

// FormatDurationString formats duration in minutes to human-readable string
func FormatDurationString(minutes int) string {
	if minutes < 60 {
		return strconv.Itoa(minutes) + "分"
	}
	hours := minutes / 60
	remainingMinutes := minutes % 60
	if remainingMinutes == 0 {
		return strconv.Itoa(hours) + "時間"
	}
	return strconv.Itoa(hours) + "時間" + strconv.Itoa(remainingMinutes) + "分"
}

// FormatWorkTimeDisplay formats work time in seconds to readable display
func FormatWorkTimeDisplay(sec int) string {
	if sec < 60 {
		return strconv.Itoa(sec) + "秒"
	}
	
	minutes := sec / 60
	remainingSeconds := sec % 60
	
	if minutes < 60 {
		if remainingSeconds == 0 {
			return strconv.Itoa(minutes) + "分"
		}
		return strconv.Itoa(minutes) + "分" + strconv.Itoa(remainingSeconds) + "秒"
	}
	
	hours := minutes / 60
	remainingMinutes := minutes % 60
	
	if remainingMinutes == 0 && remainingSeconds == 0 {
		return strconv.Itoa(hours) + "時間"
	}
	if remainingSeconds == 0 {
		return strconv.Itoa(hours) + "時間" + strconv.Itoa(remainingMinutes) + "分"
	}
	if remainingMinutes == 0 {
		return strconv.Itoa(hours) + "時間" + strconv.Itoa(remainingSeconds) + "秒"
	}
	
	return strconv.Itoa(hours) + "時間" + strconv.Itoa(remainingMinutes) + "分" + strconv.Itoa(remainingSeconds) + "秒"
}