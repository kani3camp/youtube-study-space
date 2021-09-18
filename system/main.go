package main

import (
	"app.modules/core"
	"app.modules/core/utils"
	direct_operations "app.modules/direct-operations"
	"context"
	"fmt"
	"google.golang.org/api/option"
	"log"
	"os"
	"strings"
	"time"
)


// LocalMain ローカル運用
func LocalMain(credentialFilePath string) {
	ctx := context.Background()
	clientOption := option.WithCredentialsFile("/Users/drew/Dev/機密ファイル/GCP/youtube-study-space-c4bcd4edbd8a.json")
	// clientOption := option.WithCredentialsFile("C:/Dev/GCP credentials/youtube-study-space-a3516f96e3f8.json")
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		_ = _system.LineBot.SendMessageWithError("failed core.NewSystem()", err)
		return
	}
	_system.SendLiveChatMessage("起動しました。", ctx)
	_ = _system.LineBot.SendMessage("app started.")
	defer func() {
		_system.SendLiveChatMessage("寝ます。", ctx)
		_ = _system.LineBot.SendMessage("app stopped!!")
	}()
	sleepIntervalMilli := _system.DefaultSleepIntervalMilli
	
	for {
		// page token取得
		pageToken, err := _system.RetrieveNextPageToken(ctx)
		if err != nil {
			_ = _system.LineBot.SendMessageWithError("failed to retrieve next page token", err)
			return
		}
		// チャット取得
		chatMessages, nextPageToken, pollingIntervalMillis, err := _system.LiveChatBot.ListMessages(pageToken, ctx)
		if err != nil {
			_ = _system.LineBot.SendMessageWithError("failed to retrieve chat messages", err)
			return
		}
		// nextPageTokenを保存
		err = _system.SaveNextPageToken(nextPageToken, ctx)
		if err != nil {
			_ = _system.LineBot.SendMessageWithError("failed to save next page token", err)
			return
		}
		
		// コマンドを抜き出して各々処理
		for _, chatMessage := range chatMessages {
			message := chatMessage.Snippet.TextMessageDetails.MessageText
			log.Println(chatMessage.AuthorDetails.DisplayName + ": " + message)
			if strings.HasPrefix(message, core.CommandPrefix) {
				err := _system.Command(message, chatMessage.AuthorDetails.ChannelId, chatMessage.AuthorDetails.DisplayName, ctx)
				if err.IsNotNil() {
					_ = _system.LineBot.SendMessageWithError("error in core.Command()", err.Body)
				}
			}
		}
		
		if pollingIntervalMillis > _system.DefaultSleepIntervalMilli {
			sleepIntervalMilli = pollingIntervalMillis + 1000
		} else {
			sleepIntervalMilli = _system.DefaultSleepIntervalMilli
		}
		fmt.Println()
		log.Printf("waiting for %.1f seconds...\n", float32(sleepIntervalMilli) / 1000.0)
		time.Sleep(time.Duration(sleepIntervalMilli) * time.Millisecond)
	}
}



func Test(credentialFilePath string) {
	ctx := context.Background()
	clientOption := option.WithCredentialsFile(credentialFilePath)
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer _system.CloseFirestoreClient()
	
	fmt.Println(utils.JstNow().Format(time.RFC3339))
	fmt.Println(utils.JstNow().Format(time.RFC3339))
}


func main() {
	core.LoadEnv()
	credentialFilePath := os.Getenv("CREDENTIAL_FILE_LOCATION")
	
	// TODO: デプロイ時切り替え
	//LocalMain(credentialFilePath)
	// Test(credentialFilePath)
	
	// UpdateRoomLayout(credentialFilePath)
	direct_operations.ExportUsersCollectionJson(credentialFilePath)
	//direct_operations.ExitAllUsersAllRoom(credentialFilePath)
}

