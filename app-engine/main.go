package main

import (
	"context"
	"fmt"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"time"
)

func main()  {
	fmt.Println("app started.")

	// ライブチャット取得
	ctx := context.Background()
	sa := option.WithCredentialsFile("C:/Development/GCP Credentials/youtube-study-space-95bb4187aace.json")
	youtubeService, err := youtube.NewService(ctx, sa)
	if err != nil {
		fmt.Println("failed youtube.NewService()")
		fmt.Println(err.Error())
		return
	}
	liveChatMessagesService := youtube.NewLiveChatMessagesService(youtubeService)
	liveChatId := "Cg0KC2dZUnBORVNHc2RVKicKGFVDWHVEMlhtUFRkcFZ5N3ptd2JGVlpXZxILZ1lScE5FU0dzZFU"
	part := []string{
		"snippet",
	}
	listCall := liveChatMessagesService.List(liveChatId, part)
	nextPageToken := ""
	sleepIntervalMilli := 5000
	for {
		fmt.Println(time.Now())
		if nextPageToken != "" {
			listCall = listCall.PageToken(nextPageToken)
		}
		response, err := listCall.Do()
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		fmt.Println(len(response.Items))
		for _, item := range response.Items {
			fmt.Println(item.Snippet.DisplayMessage)
		}
		nextPageToken = response.NextPageToken
		sleepIntervalMilli = int(response.PollingIntervalMillis) + 1000
		fmt.Printf("\n%.1f 秒待機\n", float32(sleepIntervalMilli) / 1000.0)
		time.Sleep(time.Duration(sleepIntervalMilli) * time.Millisecond)
		fmt.Println()
	}
}
