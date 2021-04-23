package main

import (
	"app.modules/system"
	"bufio"
	"context"
	"fmt"
	"google.golang.org/api/option"
	"log"
	"os"
	"strings"
	"time"
)

func AppEngineMain()  {
	ctx := context.Background()
	_system, err := system.NewSystem(ctx, nil)
	if err != nil {
		_ = _system.LineBot.SendMessageWithError("failed system.NewSystem()", err)
		return
	}
	_ = _system.LineBot.SendMessage("app started.")
	defer func() {
		_ = _system.LineBot.SendMessage("app stopped.")
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
			if strings.HasPrefix(message, system.CommandPrefix) {
				err := _system.Command(message, chatMessage.AuthorDetails.ChannelId, chatMessage.AuthorDetails.DisplayName, ctx)
				if err != nil {
					_ = _system.LineBot.SendMessageWithError("error in system.Command()", err)
				}
			}
		}

		if pollingIntervalMillis > _system.DefaultSleepIntervalMilli {
			sleepIntervalMilli = pollingIntervalMillis + 1000
		} else {
			sleepIntervalMilli = _system.DefaultSleepIntervalMilli
		}
		fmt.Printf("\n%.1f 秒待機\n", float32(sleepIntervalMilli) / 1000.0)
		time.Sleep(time.Duration(sleepIntervalMilli) * time.Millisecond)
	}
}

// ローカル開発用
func DevMain() {
	ctx := context.Background()
	//clientOption := option.WithCredentialsFile("/Users/drew/Development/機密ファイル/GCP/youtube-study-space-c4bcd4edbd8a.json")
	clientOption := option.WithCredentialsFile("C:/Development/GCP Credentials/youtube-study-space-95bb4187aace.json")
	_system, err := system.NewSystem(ctx, clientOption)
	if err != nil {
		_ = _system.LineBot.SendMessageWithError("failed system.NewSystem()", err)
		return
	}
	_ = _system.LineBot.SendMessage("app started.")
	defer func() {
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
			if strings.HasPrefix(message, system.CommandPrefix) {
				err := _system.Command(message, chatMessage.AuthorDetails.ChannelId, chatMessage.AuthorDetails.DisplayName, ctx)
				if err != nil {
					_ = _system.LineBot.SendMessageWithError("error in system.Command()", err)
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

func DevCLIMain()  {
	log.Println("app started.")
	ctx := context.Background()
	//clientOption := option.WithCredentialsFile("/Users/drew/Development/機密ファイル/GCP/youtube-study-space-c4bcd4edbd8a.json")
	clientOption := option.WithCredentialsFile("C:/Development/GCP Credentials/youtube-study-space-95bb4187aace.json")
	_system, err := system.NewSystem(ctx, clientOption)
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer _system.CloseFirestoreClient()
	_ = _system.LineBot.SendMessage("app started.")

	for {
		// チャット取得
		fmt.Printf("\n>> ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		message := scanner.Text()

		// 入力文字列からコマンドを抜き出して処理
		err = _system.Command(message, "test-channel01", "潤", ctx)
		if err != nil {
			log.Println("error in system.Command().")
			log.Println(err.Error())
		}
	}
}

func UpdateRoomLayout() {
	log.Println("app started.")
	ctx := context.Background()
	//clientOption := option.WithCredentialsFile("/Users/drew/Development/機密ファイル/GCP/youtube-study-space-c4bcd4edbd8a.json")
	clientOption := option.WithCredentialsFile("C:/Development/GCP Credentials/youtube-study-space-95bb4187aace.json")
	_system, err := system.NewSystem(ctx, clientOption)
	if err != nil {
		log.Println(err.Error())
		return
	}
	_system.UpdateRoomLayout("C:\\Users\\momom\\Documents\\GitHub\\youtube-study-space\\app-engine\\default-room-layout.json", ctx)
}


func TestSend()  {
	ctx := context.Background()
	clientOption := option.WithCredentialsFile("/Users/drew/Development/機密ファイル/GCP/youtube-study-space-c4bcd4edbd8a.json")
	//clientOption := option.WithCredentialsFile("C:/Development/GCP Credentials/youtube-study-space-95bb4187aace.json")
	_system, err := system.NewSystem(ctx, clientOption)
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer _system.CloseFirestoreClient()
	
	_system.SendLiveChatMessage("hi", ctx)
}

func Test() {
	ctx := context.Background()
	//clientOption := option.WithCredentialsFile("/Users/drew/Development/機密ファイル/GCP/youtube-study-space-c4bcd4edbd8a.json")
	clientOption := option.WithCredentialsFile("C:/Development/GCP Credentials/youtube-study-space-95bb4187aace.json")
	_system, err := system.NewSystem(ctx, clientOption)
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer _system.CloseFirestoreClient()
	
	err = _system.ResetDailyTotalStudyTime(ctx)
	if err != nil {
		log.Println(err.Error())
	}
}

func main() {
	// todo デプロイ時切り替え
	//AppEngineMain()
	DevMain()
	//DevCLIMain()
	//TestSend()
	//Test()
	
	//UpdateRoomLayout()
}

