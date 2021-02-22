package myfirestore

type ConfigCollection struct {
	YoutubeLive YoutubeLiveDoc	`firestore:"youtube-live"`
}

type YoutubeLiveDoc struct {
	LiveChatId string `firestore:"live-chat-id"`
	SleepIntervalMilli int `firestore:"sleep-interval-milli"`
	NextPageToken string `firestore:"next-page-token"`
}