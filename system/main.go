package main

import (
	"app.modules/core"
	"app.modules/core/utils"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)


func Init() (option.ClientOption, context.Context, error) {
	core.LoadEnv()
	credentialFilePath := os.Getenv("CREDENTIAL_FILE_LOCATION")
	
	ctx := context.Background()
	clientOption := option.WithCredentialsFile(credentialFilePath)
	
	// 本番GCPプロジェクトの場合はCLI上で確認
	creds, _ := transport.Creds(ctx, clientOption)
	if creds.ProjectID == "youtube-study-space" {
		fmt.Println("本番環境用のcredentialが使われます。よろしいですか？(yes / no)")
		var s string
		_, _ = fmt.Scanf("%s", &s)
		if s != "yes" {
			return nil, nil, errors.New("")
		}
	} else if creds.ProjectID == "test-youtube-study-space" {
		log.Println("credential of test-youtube-study-space")
	} else {
		return nil, nil, errors.New("unknown project id on the credential.")
	}
	return clientOption, ctx, nil
}

// LocalMain ローカル運用
func LocalMain(clientOption option.ClientOption, ctx context.Context) {
	
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		_ = _system.LineBot.SendMessageWithError("failed core.NewSystem()", err)
		return
	}
	
	_ = _system.LineBot.SendMessage("Botが起動しました")
	defer func() {
		_system.SendLiveChatMessage("エラーが起きたため終了します", ctx)
		_ = _system.LineBot.SendMessage("app stopped!!")
	}()
	
	sleepIntervalMilli := _system.DefaultSleepIntervalMilli
	checkDesiredMaxSeatsIntervalSec := _system.CheckDesiredMaxSeatsIntervalSec
	
	lastCheckedDesiredMaxSeats := utils.JstNow()
	
	numContinuousRetrieveNextPageTokenFailed := 0
	numContinuousListMessagesFailed := 0
	
	for {
		// max_seatsを変えるか確認
		if utils.JstNow().After(lastCheckedDesiredMaxSeats.Add(time.Duration(checkDesiredMaxSeatsIntervalSec) * time.Second)) {
			log.Println("checking desired max seats")
			constants, err := _system.FirestoreController.RetrieveSystemConstantsConfig(ctx)
			if err != nil {
				_ = _system.LineBot.SendMessageWithError("_system.FirestoreController.RetrieveSystemConstantsConfig(ctx)でエラー", err)
			} else {
				if constants.DesiredMaxSeats != constants.MaxSeats {
					err := _system.AdjustMaxSeats(ctx)
					if err != nil {
						_ = _system.LineBot.SendMessageWithError("failed _system.AdjustMaxSeats()", err)
					}
				}
			}
			lastCheckedDesiredMaxSeats = utils.JstNow()
		}
		
		// page token取得
		pageToken, err := _system.RetrieveNextPageToken(ctx)
		if err != nil {
			_ = _system.LineBot.SendMessageWithError("（" + strconv.Itoa(numContinuousRetrieveNextPageTokenFailed + 1) + "回目） failed to retrieve next page token", err)
			numContinuousRetrieveNextPageTokenFailed += 1
			if numContinuousRetrieveNextPageTokenFailed > 5 {
				break
			} else {
				continue
			}
		} else {
			numContinuousRetrieveNextPageTokenFailed = 0
		}
		
		// チャット取得
		chatMessages, nextPageToken, pollingIntervalMillis, err := _system.LiveChatBot.ListMessages(pageToken, ctx)
		if err != nil {
			_ = _system.LineBot.SendMessageWithError("（" + strconv.Itoa(numContinuousListMessagesFailed + 1) +
				"回目） failed to retrieve chat messages", err)
			numContinuousListMessagesFailed += 1
			if numContinuousListMessagesFailed > 5 {
				break
			} else {
				continue
			}
		} else {
			numContinuousListMessagesFailed = 0
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
			log.Println(chatMessage.AuthorDetails.ChannelId + " (" + chatMessage.AuthorDetails.DisplayName + "): " + message)
			err := _system.Command(message, chatMessage.AuthorDetails.ChannelId, chatMessage.AuthorDetails.DisplayName, chatMessage.AuthorDetails.IsChatModerator, chatMessage.AuthorDetails.IsChatOwner, ctx)
			if err.IsNotNil() {
				_ = _system.LineBot.SendMessageWithError("error in core.Command()", err.Body)
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



func Test(clientOption option.ClientOption, ctx context.Context) {
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer _system.CloseFirestoreClient()
	
	str := "w-w"
	hasPrefix := strings.HasPrefix(str, core.WorkNameOptionShortPrefixLegacy)
	workName := strings.TrimPrefix(str, core.WorkNameOptionShortPrefixLegacy)
	
	log.Println(hasPrefix)
	log.Println(workName)
}


func main() {
	clientOption, ctx, err := Init()
	if err != nil {
		log.Println(err.Error())
		return
	}
	
	// デプロイ時切り替え
	LocalMain(clientOption, ctx)
	//Test(clientOption, ctx)
	
	//direct_operations.ExportUsersCollectionJson(clientOption, ctx)
	//direct_operations.ExitAllUsersInRoom(clientOption, ctx)
	//direct_operations.ExitSpecificUser("UCN61FE7NtU0URA_u9vWWdjw", clientOption, ctx)
}

