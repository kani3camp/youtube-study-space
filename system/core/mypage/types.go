package mypage

type Status string

const (
	StatusOK            Status = "ok"
	StatusNotRegistered Status = "not_registered"
)

type Viewer struct {
	YouTubeChannelID string `json:"youtubeChannelId"`
	DisplayName      string `json:"displayName"`
	ProfileImageURL  string `json:"profileImageUrl"`
}

type Stats struct {
	DailyWorkSec      int `json:"dailyWorkSec"`
	CumulativeWorkSec int `json:"cumulativeWorkSec"`
}

type CurrentSeat struct {
	SeatID        int    `json:"seatId"`
	IsMemberSeat  bool   `json:"isMemberSeat"`
	State         string `json:"state"`
	WorkName      string `json:"workName"`
	BreakWorkName string `json:"breakWorkName"`

	// StartedAt は現在の state が始まった時刻。
	StartedAt string `json:"startedAt"`

	// Until は現在の state の終了予定時刻。
	// work 中なら作業終了予定、break 中なら休憩終了予定。
	Until string `json:"until"`
}

type Response struct {
	Status Status `json:"status"`
	Viewer Viewer `json:"viewer"`

	// not_registered の場合は省略する。
	Stats *Stats `json:"stats,omitempty"`

	// 登録済みだが未入室の場合は null を返す。
	CurrentSeat *CurrentSeat `json:"currentSeat"`
}
