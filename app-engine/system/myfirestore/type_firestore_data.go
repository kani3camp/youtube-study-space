package myfirestore

import "time"

type ConfigCollection struct {
	YoutubeLive YoutubeLiveDoc	`firestore:"youtube-live"`
}

type YoutubeLiveDoc struct {
	LiveChatId string `firestore:"live-chat-id"`
	SleepIntervalMilli int `firestore:"sleep-interval-milli"`
	NextPageToken string `firestore:"next-page-token"`
}

type DefaultRoomDoc struct {
	Seats []Seat `firestore:"seats"`
}

type Seat struct {
	SeatId int `firestore:"seat-id"`
	UserId string `firestore:"user-id"`
	WorkName string `firestore:"work-name"`
	Until time.Time `firestore:"until"`
}

type NoSeatRoomDoc struct {
	Seats []Seat `firestore:"users"`
}


type UserDoc struct {
	DailyTotalStudySec int `firestore:"daily-total-study-sec"`
	TotalStudySec int `firestore:"total-study-sec"`
	RegistrationDate time.Time `firestore:"registration-date"`
	StatusMessage string `firestore:"status-message"`
	LastEntered time.Time `firestore:"last-entered"`
	LastExited time.Time `firestore:"last-exited"`
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
	PartitionShapes []struct {
		Name   string `json:"name" firestore:"name"`
		Width  int    `json:"width" firestore:"width"`
		Height int    `json:"height" firestore:"height"`
	} `json:"partition_shapes" firestore:"partition-shapes"`
	Seats []struct {
		Id       int    `json:"id" firestore:"id"`
		X        int    `json:"x" firestore:"x"`
		Y        int    `json:"y" firestore:"y"`
	} `json:"seats" firestore:"seats"`
	Partitions []struct {
		Id        int    `json:"id" firestore:"id"`
		X         int    `json:"x" firestore:"x"`
		Y         int    `json:"y" firestore:"y"`
		ShapeType string `json:"shape_type" firestore:"shape-type"`
	} `json:"partitions" firestore:"partitions"`
}

type UserHistoryDoc struct {
	Action string `firestore:"action"`
	Date time.Time `firestore:"date"`
	Details interface{} `firestore:"details"`
}



