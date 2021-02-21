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
	response, err := listCall.Do()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(len(response.Items))
	for _, item := range response.Items {
		fmt.Println(item.Snippet.DisplayMessage)
	}
	fmt.Println()

	time.Sleep(5 * time.Second)

	fmt.Println(response.NextPageToken)
	listCall = liveChatMessagesService.List(liveChatId, part).PageToken(response.NextPageToken)
	response, err = listCall.Do()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(len(response.Items))
	for _, item := range response.Items {
		fmt.Println(item.Snippet.DisplayMessage)
	}
}
