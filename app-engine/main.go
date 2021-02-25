package main

import (
	"app.modules/system"
	"bufio"
	"context"
	"fmt"
	"google.golang.org/api/option"
	"os"
	"strings"
	"time"
)

func AppEngineMain()  {
	fmt.Println("app started.")
	ctx := context.Background()
	// todo get credential properly
	clientOption := option.WithCredentialsFile("C:/Development/GCP Credentials/youtube-study-space-95bb4187aace.json")
	_system, err := system.NewSystem(ctx, clientOption)
	if err != nil {
		fmt.Println("failed myfirestore.NewFirestoreController().")
		fmt.Println(err.Error())
		return
	}
	sleepIntervalMilli := _system.LiveChatBot.DefaultSleepIntervalMilli

	for {
		// page token取得
		pageToken, err := _system.RetrieveNextPageToken(ctx)
		if err != nil {
			fmt.Println("failed to retrieve next page token.")
			fmt.Println(err.Error())
			// todo line, livechatで通知
			return
		}
		// チャット取得
		chatMessages, nextPageToken, pollingIntervalMillis, err := _system.LiveChatBot.ListMessages(pageToken)
		if err != nil {
			fmt.Println("failed to retrieve chat messages.")
			fmt.Println(err.Error())
			// todo line, livechatで通知
			return
		}
		// nextPageTokenを保存
		err = _system.SaveNextPageToken(nextPageToken, ctx)
		if err != nil {
			fmt.Println("failed to save next page token.")
			fmt.Println(err.Error())
			// todo line, livechatで通知
			return
		}

		// コマンドを抜き出して各々処理
		for _, chatMessage := range chatMessages {
			message := chatMessage.Snippet.TextMessageDetails.MessageText
			if strings.HasPrefix(message, "!") {
				err := _system.Command(message, chatMessage.AuthorDetails.ChannelId, chatMessage.AuthorDetails.DisplayName, ctx)
				if err != nil {
					fmt.Println("error in system.Command().")
					fmt.Println(err.Error())
					// todo lineで通知
				}
			}
		}

		if pollingIntervalMillis > _system.LiveChatBot.DefaultSleepIntervalMilli {
			sleepIntervalMilli = pollingIntervalMillis + 1000
		} else {
			sleepIntervalMilli = _system.LiveChatBot.DefaultSleepIntervalMilli
		}
		fmt.Printf("\n%.1f 秒待機\n", float32(sleepIntervalMilli) / 1000.0)
		time.Sleep(time.Duration(sleepIntervalMilli) * time.Millisecond)
	}
}

// ローカル開発用
func DevMain()  {
	fmt.Println("app started.")
	ctx := context.Background()
	clientOption := option.WithCredentialsFile("C:/Development/GCP Credentials/youtube-study-space-95bb4187aace.json")
	_system, err := system.NewSystem(ctx, clientOption)
	if err != nil {
		fmt.Println("failed myfirestore.NewFirestoreController().")
		return
	}

	for {
		// チャット取得
		fmt.Printf("\n>> ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		message := scanner.Text()

		// 入力文字列からコマンドを抜き出して処理
		err = _system.Command(message, "test-channel01", "display-name", ctx)
		if err != nil {
			fmt.Println("error in system.Command().")
			fmt.Println(err.Error())
		}
	}
}

func main() {
	// todo デプロイ時切り替え
	//AppEngineMain()
	DevMain()
}