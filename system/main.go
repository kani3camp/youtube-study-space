package main

import (
	"app.modules/core/workspaceapp"
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"time"

	"app.modules/core/youtubebot"
	"github.com/kr/pretty"

	"app.modules/core/utils"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

const MaxRetryIntervalSeconds = 300
const RetryIntervalCalculationBase = 1.2

func Init() (option.ClientOption, context.Context, error) {
	utils.LoadEnv(".env")
	credentialFilePath := os.Getenv("CREDENTIAL_FILE_LOCATION")

	ctx := context.Background()
	clientOption := option.WithCredentialsFile(credentialFilePath)

	// 本番GCPプロジェクトの場合はCLI上で確認
	creds, err := transport.Creds(ctx, clientOption)
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("Project ID: %s\n", creds.ProjectID)
	fmt.Println("Is this the correct project ID? (yes/no)")
	var s string
	_, _ = fmt.Scanln(&s)
	if s != "yes" {
		return nil, nil, errors.New("aborted")
	}

	return clientOption, ctx, nil
}

func CheckLongTimeSitting(ctx context.Context, clientOption option.ClientOption) {
	sys, err := workspaceapp.NewSystem(ctx, false, clientOption)
	if err != nil {
		sys.MessageToOwnerWithError(ctx, "failed core.NewSystem()", err)
		return
	}

	sys.MessageToOwner(ctx, "居座り防止プログラムが起動しました。")

	sys.GoroutineCheckLongTimeSitting(ctx)
}

func CalculateRetryIntervalSec(base float64, numContinuousFailed int) float64 {
	return math.Min(MaxRetryIntervalSeconds, math.Pow(base, float64(numContinuousFailed)))
}

func Bot(ctx context.Context, clientOption option.ClientOption) {
	sys, err := workspaceapp.NewSystem(ctx, true, clientOption)
	if err != nil {
		sys.MessageToOwnerWithError(ctx, "failed core.NewSystem()", err)
		return
	}

	sys.MessageToOwner(ctx, "Botが起動しました。\n"+sys.GetInfoString())
	defer func() { // when error occurred
		sys.CloseFirestoreClient()
		sys.MessageToLiveChat(ctx, "エラーが起きたため終了します。お手数ですが管理者に連絡してください。")
		sys.MessageToOwner(ctx, "app stopped!!")
	}()

	go CheckLongTimeSitting(ctx, clientOption) // 居座り防止処理を並行実行

	checkDesiredMaxSeatsIntervalSec := sys.Configs.Constants.CheckDesiredMaxSeatsIntervalSec

	lastCheckedDesiredMaxSeats := utils.JstNow()

	const MinimumTryTimesToNotify = 2
	numContinuousRetrieveNextPageTokenFailed := 0
	numContinuousListMessagesFailed := 0
	var lastChatFetched time.Time
	var waitAtLeastMilliSec1 float64
	var waitAtLeastMilliSec2 float64
	var sleepInterval time.Duration

	for {
		// max_seatsを変えるか確認
		if utils.JstNow().After(lastCheckedDesiredMaxSeats.Add(time.Duration(checkDesiredMaxSeatsIntervalSec) * time.Second)) {
			slog.Info("checking desired max seats")
			constants, err := sys.Repository.ReadSystemConstantsConfig(ctx, nil)
			if err != nil {
				sys.MessageToOwnerWithError(ctx, "sys.firestoreController.ReadSystemConstantsConfig(ctx)でエラー", err)
			} else {
				if constants.DesiredMaxSeats != constants.MaxSeats || constants.DesiredMemberMaxSeats != constants.MemberMaxSeats {
					if err := sys.AdjustMaxSeats(ctx); err != nil {
						sys.MessageToOwnerWithError(ctx, "failed sys.AdjustMaxSeats()", err)
					}
				}
			}
			lastCheckedDesiredMaxSeats = utils.JstNow()
		}

		// page token取得
		pageToken, err := sys.GetNextPageToken(ctx, nil)
		if err != nil {
			numContinuousRetrieveNextPageTokenFailed += 1
			if numContinuousRetrieveNextPageTokenFailed >= MinimumTryTimesToNotify {
				sys.MessageToOwnerWithError(ctx, "（"+strconv.Itoa(numContinuousRetrieveNextPageTokenFailed)+"回目） failed to retrieve next page token", err)
			}
			waitSeconds := CalculateRetryIntervalSec(RetryIntervalCalculationBase, numContinuousRetrieveNextPageTokenFailed)
			time.Sleep(time.Duration(waitSeconds) * time.Second)
			continue
		} else {
			numContinuousRetrieveNextPageTokenFailed = 0
		}

		// fetch chat messages
		chatMessages, nextPageToken, pollingIntervalMillis, err := sys.ListLiveChatMessages(ctx, pageToken)
		if err != nil {
			numContinuousListMessagesFailed += 1
			if numContinuousListMessagesFailed >= MinimumTryTimesToNotify {
				sys.MessageToOwnerWithError(ctx, "（"+strconv.Itoa(numContinuousListMessagesFailed)+
					"回目） failed to retrieve chat messages", err)
			}
			waitSeconds := CalculateRetryIntervalSec(RetryIntervalCalculationBase, numContinuousListMessagesFailed)
			time.Sleep(time.Duration(waitSeconds) * time.Second)
			continue
		} else {
			numContinuousListMessagesFailed = 0
		}
		lastChatFetched = utils.JstNow()

		// save nextPageToken
		if err := sys.SaveNextPageToken(ctx, nextPageToken); err != nil {
			sys.MessageToOwnerWithError(ctx, "(1回目) failed to save next page token", err)
			// 少し待ってから再試行
			time.Sleep(3 * time.Second)
			err2 := sys.SaveNextPageToken(ctx, nextPageToken)
			if err2 != nil {
				sys.MessageToOwnerWithError(ctx, "(2回目) failed to save next page token", err2)
				// pass
			}
		}

		// chatMessagesを保存
		for _, chatMessage := range chatMessages {
			// only if chatMessage has a text message
			if !youtubebot.HasTextMessageByAuthor(chatMessage) {
				continue
			}

			if err = sys.AddLiveChatHistoryDoc(ctx, chatMessage); err != nil {
				sys.MessageToOwnerWithError(ctx, "(1回目) failed to add live chat history", err)
				time.Sleep(2 * time.Second)
				if err2 := sys.AddLiveChatHistoryDoc(ctx, chatMessage); err2 != nil {
					sys.MessageToOwnerWithError(ctx, "(2回目) failed to add live chat history", err2)
					// pass
				}
			}
		}

		// process the command (includes not command)
		for _, chatMessage := range chatMessages {
			if youtubebot.IsFanFundingEvent(chatMessage) {
				sys.MessageToOwner(ctx, fmt.Sprintf("Fan funding event:\n```%# v```", pretty.Formatter(chatMessage)))
			}

			// only if chatMessage has text message content
			if !youtubebot.HasTextMessageByAuthor(chatMessage) {
				continue
			}

			message := youtubebot.ExtractTextMessageByAuthor(chatMessage)
			channelId := youtubebot.ExtractAuthorChannelId(chatMessage)
			displayName := youtubebot.ExtractAuthorDisplayName(chatMessage)
			profileImageUrl := youtubebot.ExtractAuthorProfileImageUrl(chatMessage)
			isModerator := youtubebot.IsChatMessageByModerator(chatMessage)
			isOwner := youtubebot.IsChatMessageByOwner(chatMessage)
			isMember := isOwner || youtubebot.IsChatMessageByMember(chatMessage)
			slog.Info(chatMessage.AuthorDetails.ChannelId + " (" + chatMessage.AuthorDetails.DisplayName + "): " + message)
			if err := sys.Command(ctx, message, channelId, displayName, profileImageUrl, isModerator, isOwner, isMember); err != nil {
				sys.MessageToOwnerWithError(ctx, "error in Command()", err)
			}
		}

		waitAtLeastMilliSec1 = math.Max(float64((time.Duration(pollingIntervalMillis)*time.Millisecond - utils.
			JstNow().Sub(lastChatFetched)).Milliseconds()), 0)
		waitAtLeastMilliSec2 = math.Max(float64((time.Duration(sys.Configs.Constants.SleepIntervalMilli)*time.Millisecond - utils.JstNow().Sub(lastChatFetched)).Milliseconds()), 0)
		sleepInterval = time.Duration(math.Max(waitAtLeastMilliSec1, waitAtLeastMilliSec2)) * time.Millisecond
		slog.Info(fmt.Sprintf("waiting for %.2f seconds...\n\n", sleepInterval.Seconds()))
		time.Sleep(sleepInterval)
	}
}

func main() {
	clientOption, ctx, err := Init()
	if err != nil {
		panic(err)
	}

	Bot(ctx, clientOption)
}
