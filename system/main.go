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
	"math"
	"os"
	"strconv"
	"time"
)

func Init() (option.ClientOption, context.Context, error) {
	utils.LoadEnv()
	credentialFilePath := os.Getenv("CREDENTIAL_FILE_LOCATION")
	
	ctx := context.Background()
	clientOption := option.WithCredentialsFile(credentialFilePath)
	
	// 本番GCPプロジェクトの場合はCLI上で確認
	creds, err := transport.Creds(ctx, clientOption)
	if err != nil {
		return nil, nil, err
	}
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

func CheckLongTimeSitting(ctx context.Context, clientOption option.ClientOption) {
	sys, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		sys.MessageToLineBotWithError("failed core.NewSystem()", err)
		return
	}
	
	sys.MessageToLineBot("居座り防止プログラムが起動しました。")
	defer func() {
		sys.CloseFirestoreClient()
		sys.MessageToLiveChat(ctx, "エラーが起きたため終了します。お手数ですが管理者に連絡してください。")
		sys.MessageToLineBot("app stopped!!")
	}()
	
	sys.GoroutineCheckLongTimeSitting(ctx)
}

// Bot ローカル運用
func Bot(ctx context.Context, clientOption option.ClientOption) {
	sys, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		sys.MessageToLineBotWithError("failed core.NewSystem()", err)
		return
	}
	
	sys.MessageToLineBot("Botが起動しました。")
	defer func() { // プログラムが停止してしまうとき。このプログラムは無限なので停止するのはエラーがおこったとき。
		sys.CloseFirestoreClient()
		sys.MessageToLiveChat(ctx, "エラーが起きたため終了します。お手数ですが管理者に連絡してください。")
		sys.MessageToLineBot("app stopped!!")
	}()
	
	checkDesiredMaxSeatsIntervalSec := sys.Configs.Constants.CheckDesiredMaxSeatsIntervalSec
	
	lastCheckedDesiredMaxSeats := utils.JstNow()
	
	numContinuousRetrieveNextPageTokenFailed := 0
	numContinuousListMessagesFailed := 0
	var lastChatFetched time.Time
	var waitAtLeastMilliSec1 float64
	var waitAtLeastMilliSec2 float64
	var sleepInterval time.Duration
	
	for {
		// max_seatsを変えるか確認
		if utils.JstNow().After(lastCheckedDesiredMaxSeats.Add(time.Duration(checkDesiredMaxSeatsIntervalSec) * time.Second)) {
			log.Println("checking desired max seats")
			constants, err := sys.FirestoreController.RetrieveSystemConstantsConfig(ctx, nil)
			if err != nil {
				sys.MessageToLineBotWithError("sys.firestoreController.RetrieveSystemConstantsConfig(ctx)でエラー", err)
			} else {
				if constants.DesiredMaxSeats != constants.MaxSeats {
					err := sys.AdjustMaxSeats(ctx)
					if err != nil {
						sys.MessageToLineBotWithError("failed sys.AdjustMaxSeats()", err)
					}
				}
			}
			lastCheckedDesiredMaxSeats = utils.JstNow()
		}
		
		// page token取得
		pageToken, err := sys.RetrieveNextPageToken(ctx, nil)
		if err != nil {
			sys.MessageToLineBotWithError("（"+strconv.Itoa(numContinuousRetrieveNextPageTokenFailed+1)+"回目） failed to retrieve next page token", err)
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
		chatMessages, nextPageToken, pollingIntervalMillis, err := sys.ListLiveChatMessages(ctx, pageToken)
		if err != nil {
			sys.MessageToLineBotWithError("（"+strconv.Itoa(numContinuousListMessagesFailed+1)+
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
		lastChatFetched = utils.JstNow()
		
		// nextPageTokenを保存
		err = sys.SaveNextPageToken(ctx, nextPageToken)
		if err != nil {
			sys.MessageToLineBotWithError("failed to save next page token", err)
			return
		}
		
		// chatMessagesを保存
		for _, chatMessage := range chatMessages {
			err = sys.AddLiveChatHistoryDoc(ctx, chatMessage)
			if err != nil {
				sys.MessageToLineBotWithError("failed to add live chat history", err)
				return
			}
		}
		
		// コマンドを抜き出して各々処理
		for _, chatMessage := range chatMessages {
			message := chatMessage.Snippet.TextMessageDetails.MessageText
			log.Println(chatMessage.AuthorDetails.ChannelId + " (" + chatMessage.AuthorDetails.DisplayName + "): " + message)
			err := sys.Command(ctx, message, chatMessage.AuthorDetails.ChannelId, chatMessage.AuthorDetails.DisplayName, chatMessage.AuthorDetails.IsChatModerator, chatMessage.AuthorDetails.IsChatOwner)
			if err != nil {
				sys.MessageToLineBotWithError("error in core.Command()", err)
			}
		}
		
		waitAtLeastMilliSec1 = math.Max(float64((time.Duration(pollingIntervalMillis)*time.Millisecond - utils.
			JstNow().Sub(lastChatFetched)).Milliseconds()), 0)
		waitAtLeastMilliSec2 = math.Max(float64((time.Duration(sys.Configs.Constants.SleepIntervalMilli)*time.Millisecond - utils.JstNow().Sub(lastChatFetched)).Milliseconds()), 0)
		sleepInterval = time.Duration(math.Max(waitAtLeastMilliSec1, waitAtLeastMilliSec2)) * time.Millisecond
		log.Printf("waiting for %.2f seconds...\n\n", sleepInterval.Seconds())
		time.Sleep(sleepInterval)
	}
}

func LocalMain(ctx context.Context, clientOption option.ClientOption) {
	// 居座り防止処理を並行実行
	go CheckLongTimeSitting(ctx, clientOption)
	
	Bot(ctx, clientOption)
}

func Test(ctx context.Context, clientOption option.ClientOption) {
	s, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer s.CloseFirestoreClient()
	// === ここまでおまじない ===
	
}

func main() {
	clientOption, ctx, err := Init()
	if err != nil {
		log.Println(err.Error())
		return
	}
	
	// デプロイ時切り替え
	LocalMain(ctx, clientOption)
	//Test(ctx, clientOption)
	
	//direct_operations.ExportUsersCollectionJson(clientOption, ctx)
	//direct_operations.ExitAllUsersInRoom(clientOption, ctx)
	//direct_operations.ExitSpecificUser("UCTYYfHyJLOBDiFqvfpvmUHg", clientOption, ctx)
}
