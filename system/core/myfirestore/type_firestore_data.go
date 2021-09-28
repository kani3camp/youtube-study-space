package myfirestore

import (
	"app.modules/core/utils"
	"time"
)


type ConstantsConfigDoc struct {
	MaxWorkTimeMin int `firestore:"max-work-time-min"`
	MinWorkTimeMin int `firestore:"min-work-time-min"`
	DefaultWorkTimeMin int `firestore:"default-work-time-min"`
	SleepIntervalMilli int `firestore:"sleep-interval-milli"`
	LastResetDailyTotalStudySec time.Time `firestore:"last-reset-daily-total-study-sec" json:"last_reset_daily_total_study_sec"`
}

type YoutubeLiveConfigDoc struct {
	LiveChatId string `firestore:"live-chat-id"`
	NextPageToken string `firestore:"next-page-token"`
	OAuthRefreshTokenUrl string `firestore:"o-auth-refresh-token-url"`
}

type YoutubeCredentialDoc struct {
	AccessToken string `firestore:"access-token"`
	ClientId string `firestore:"client-id"`
	ClientSecret string `firestore:"client-secret"`
	ExpirationDate time.Time `firestore:"expiration-date"`
	RefreshToken string `firestore:"refresh-token"`
}

type LineBotConfigDoc struct {
	ChannelSecret string `firestore:"channel-secret"`
	ChannelToken string `firestore:"channel-token"`
	DestinationLineId string `firestore:"destination-line-id"`
}

type DefaultRoomDoc struct {
	Seats []Seat `json:"seats" firestore:"seats"`
}
func NewDefaultRoomDoc() DefaultRoomDoc {
	return DefaultRoomDoc{
		Seats: []Seat{},
	}
}

type Seat struct {
	SeatId int `json:"seat_id" firestore:"seat-id"`
	UserId string `json:"user_id" firestore:"user-id"`
	UserDisplayName string `json:"user_display_name" firestore:"user-display-name"`
	WorkName string `json:"work_name" firestore:"work-name"`
	EnteredAt time.Time `json:"entered_at" firestore:"entered-at"`
	Until time.Time `json:"until" firestore:"until"`
	Rank utils.Rank `json:"rank" firestore:"rank"`
}

type NoSeatRoomDoc struct {
	Seats []Seat `json:"seats" firestore:"seats"`
}
func NewNoSeatRoomDoc() NoSeatRoomDoc {
	return NoSeatRoomDoc{
		Seats: []Seat{},
	}
}

type UserDoc struct {
	DailyTotalStudySec int `json:"daily_total_study_sec" firestore:"daily-total-study-sec"`
	TotalStudySec int `json:"total_study_sec" firestore:"total-study-sec"`
	RegistrationDate time.Time `json:"registration_date" firestore:"registration-date"`
	StatusMessage string `json:"status_message" firestore:"status-message"`
	LastEntered time.Time `json:"last_entered" firestore:"last-entered"`
	LastExited      time.Time `json:"last_exited" firestore:"last-exited"`
	RankVisible     bool      `json:"rank_visible" firestore:"rank-visible"`
	DefaultStudyMin int       `json:"default_study_min" firestore:"default-study-min"`
}

type PartitionShape struct {
	Name   string `json:"name" firestore:"name"`
	Width  int    `json:"width" firestore:"width"`
	Height int    `json:"height" firestore:"height"`
}
type SeatLayout struct {
	Id       int    `json:"id" firestore:"id"`
	X        int    `json:"x" firestore:"x"`
	Y        int    `json:"y" firestore:"y"`
}
type Partition struct {
	Id        int    `json:"id" firestore:"id"`
	X         int    `json:"x" firestore:"x"`
	Y         int    `json:"y" firestore:"y"`
	ShapeType string `json:"shape_type" firestore:"shape-type"`
}
type RoomLayoutDoc struct {
	Version       int     `json:"version" firestore:"version"`
	FontSizeRatio float32 `json:"font_size_ratio" firestore:"font-size-ratio"`
	RoomShape     struct {
		Height int `json:"height" firestore:"height"`
		Width  int `json:"width" firestore:"width"`
	} `json:"room_shape" firestore:"room-shape"`
	SeatShape struct {
		Height int `json:"height" firestore:"height"`
		Width  int `json:"width" firestore:"width"`
	} `json:"seat_shape" firestore:"seat-shape"`
	PartitionShapes []PartitionShape `json:"partition_shapes" firestore:"partition-shapes"`
	Seats []SeatLayout `json:"seats" firestore:"seats"`
	Partitions []Partition `json:"partitions" firestore:"partitions"`
}
func NewRoomLayoutDoc() RoomLayoutDoc {
	return RoomLayoutDoc{
		PartitionShapes: []PartitionShape{},
		Seats: []SeatLayout{},
		Partitions: []Partition{},
	}
}

type UserHistoryDoc struct {
	Action string `json:"action" firestore:"action"`
	Date time.Time `json:"date" firestore:"date"`
	Details interface{} `json:"details" firestore:"details"`
}



