package main

import (
	"app.modules/system"
	"app.modules/system/youtubebot"
	"app.modules/system/myfirestore"
	"bufio"
	"context"
	"fmt"
	"google.golang.org/api/option"
	"os"
	"time"
)

//func main()  {
//	fmt.Println("app started.")
//	ctx := context.Background()
//	// todo get credential properly
//	clientOption := option.WithCredentialsFile("C:/Development/GCP Credentials/youtube-study-space-95bb4187aace.json")
//	fsController, err := myfirestore.NewFirestoreController(ctx, system.ProjectId, clientOption)
//
//	youtubeLiveInfo, err := fsController.RetrieveYoutubeLiveInfo(ctx)
//	if err != nil {
//		fmt.Println("failed to retrieve youtube live info.")
//		fmt.Println(err.Error())
//		// todo line, livechatで通知
//		return
//	}
//	bot, customErr := youtubebot.NewYoutubeLiveChatBot(youtubeLiveInfo.LiveChatId, youtubeLiveInfo.SleepIntervalMilli, ctx)
//	if customErr.Body != nil {
//		fmt.Println(customErr.Body.Error())
//		// todo line, livechatで通知
//		return
//	}
//	sleepIntervalMilli := bot.SleepIntervalMilli
//	for {
//		// チャット取得
//		chatMessages, err := bot.ListMessages()
//		if err != nil {
//			fmt.Println("failed to retrieve chat messages.")
//			fmt.Println(customErr.Body.Error())
//			// todo line, livechatで通知
//			return
//		}
//		// todo チャットからコマンドを抜き出す
//		// todo コマンドを各々処理
//		if int(response.PollingIntervalMillis) > bot.SleepIntervalMilli {
//			sleepIntervalMilli = int(response.PollingIntervalMillis) + 1000
//		} else {
//			sleepIntervalMilli = bot.SleepIntervalMilli
//		}
//		fmt.Printf("\n%.1f 秒待機\n", float32(sleepIntervalMilli) / 1000.0)
//		time.Sleep(time.Duration(sleepIntervalMilli) * time.Millisecond)
//	}
//}

// ローカル開発用
func main()  {
	fmt.Println("app started.")
	ctx := context.Background()
	clientOption := option.WithCredentialsFile("C:/Development/GCP Credentials/youtube-study-space-95bb4187aace.json")
	fsController, err := myfirestore.NewFirestoreController(ctx, system.ProjectId, clientOption)

	youtubeLiveInfo, err := fsController.RetrieveYoutubeLiveInfo(ctx)
	if err != nil {
		fmt.Println("failed to retrieve youtube live info.")
		fmt.Println(err.Error())
		return
	}
	bot, customErr := youtubebot.NewYoutubeLiveChatBot(youtubeLiveInfo.LiveChatId, youtubeLiveInfo.SleepIntervalMilli, ctx)
	if customErr.Body != nil {
		fmt.Println(customErr.Body.Error())
		return
	}
	sleepIntervalMilli := bot.SleepIntervalMilli
	for {
		// チャット取得
		fmt.Printf(">> ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()

		// todo チャットからコマンドを抜き出す

		// todo コマンドを各々処理
		if int(response.PollingIntervalMillis) > bot.SleepIntervalMilli {
			sleepIntervalMilli = int(response.PollingIntervalMillis) + 1000
		} else {
			sleepIntervalMilli = bot.SleepIntervalMilli
		}
		fmt.Printf("\n%.1f 秒待機\n", float32(sleepIntervalMilli) / 1000.0)
		time.Sleep(time.Duration(sleepIntervalMilli) * time.Millisecond)
	}
}
