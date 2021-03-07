package myfirestore

import "time"


type ConstantsConfigDoc struct {
	MaxWorkTimeMin int `firestore:"max-work-time-min"`
	MinWorkTimeMin int `firestore:"min-work-time-min"`
	SleepIntervalMilli int `firestore:"sleep-interval-milli"`
}

type YoutubeLiveConfigDoc struct {
	LiveChatId string `firestore:"live-chat-id"`
	NextPageToken string `firestore:"next-page-token"`
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

type Seat struct {
	SeatId int `json:"seatId" firestore:"seat-id"`
	UserId string `json:"userId" firestore:"user-id"`
	UserDisplayName string `json:"userDisplayName" firestore:"user-display-name"`
	WorkName string `json:"workName" firestore:"work-name"`
	Until time.Time `json:"until" firestore:"until"`
}

type NoSeatRoomDoc struct {
	Seats []Seat `json:"seats" firestore:"seats"`
}


type UserDoc struct {
	DailyTotalStudySec int `json:"dailyTotalStudySec" firestore:"daily-total-study-sec"`
	TotalStudySec int `json:"totalStudySec" firestore:"total-study-sec"`
	RegistrationDate time.Time `json:"registrationDate" firestore:"registration-date"`
	StatusMessage string `json:"statusMessage" firestore:"status-message"`
	LastEntered time.Time `json:"lastEntered" firestore:"last-entered"`
	LastExited time.Time `json:"lastExited" firestore:"last-exited"`
}

type RoomLayoutDoc struct {
	Version       int     `json:"version" firestore:"version"`
	FontSizeRatio float32 `json:"fontSizeRatio" firestore:"font-size-ratio"`
	RoomShape     struct {
		Height int `json:"height" firestore:"height"`
		Width  int `json:"width" firestore:"width"`
	} `json:"roomShape" firestore:"room-shape"`
	SeatShape struct {
		Height int `json:"height" firestore:"height"`
		Width  int `json:"width" firestore:"width"`
	} `json:"seatShape" firestore:"seat-shape"`
	PartitionShapes []struct {
		Name   string `json:"name" firestore:"name"`
		Width  int    `json:"width" firestore:"width"`
		Height int    `json:"height" firestore:"height"`
	} `json:"partitionShapes" firestore:"partition-shapes"`
	Seats []struct {
		Id       int    `json:"id" firestore:"id"`
		X        int    `json:"x" firestore:"x"`
		Y        int    `json:"y" firestore:"y"`
	} `json:"seats" firestore:"seats"`
	Partitions []struct {
		Id        int    `json:"id" firestore:"id"`
		X         int    `json:"x" firestore:"x"`
		Y         int    `json:"y" firestore:"y"`
		ShapeType string `json:"shapeType" firestore:"shape-type"`
	} `json:"partitions" firestore:"partitions"`
}

type UserHistoryDoc struct {
	Action string `json:"action" firestore:"action"`
	Date time.Time `json:"date" firestore:"date"`
	Details interface{} `json:"details" firestore:"details"`
}



