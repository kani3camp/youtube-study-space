package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"app.modules/core/workspaceapp"

	"app.modules/core/youtubebot"
	"github.com/kr/pretty"

	"app.modules/core/utils"
)

const MaxRetryIntervalSeconds = 300
const RetryIntervalCalculationBase = 1.2

func CalculateRetryIntervalSec(base float64, numContinuousFailed int) float64 {
	return math.Min(MaxRetryIntervalSeconds, math.Pow(base, float64(numContinuousFailed)))
}

func Bot(ctx context.Context) {
	app, err := workspaceapp.NewWorkspaceApp(ctx)
	if err != nil {
		app.MessageToOwnerWithError(ctx, "failed core.NewWorkspaceApp()", err)
		return
	}

	app.MessageToOwner(ctx, "Botが起動しました。\n"+app.GetInfoString())
	defer func() { // when error occurred
		app.MessageToLiveChat(ctx, "エラーが起きたため終了します。お手数ですが管理者に連絡してください。")
		app.MessageToOwner(ctx, "app stopped!!")
	}()

	checkDesiredMaxSeatsIntervalSec := app.Configs.Constants.CheckDesiredMaxSeatsIntervalSec

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
			constants, err := app.Repository.ReadSystemConstantsConfig(ctx, nil)
			if err != nil {
				app.MessageToOwnerWithError(ctx, "app.firestoreController.ReadSystemConstantsConfig(ctx)でエラー", err)
			} else {
				if constants.DesiredMaxSeats != constants.MaxSeats || constants.DesiredMemberMaxSeats != constants.MemberMaxSeats {
					if err := app.AdjustMaxSeats(ctx); err != nil {
						app.MessageToOwnerWithError(ctx, "failed app.AdjustMaxSeats()", err)
					}
				}
			}
			lastCheckedDesiredMaxSeats = utils.JstNow()
		}

		// page token取得
		pageToken, err := app.GetNextPageToken(ctx, nil)
		if err != nil {
			numContinuousRetrieveNextPageTokenFailed += 1
			if numContinuousRetrieveNextPageTokenFailed >= MinimumTryTimesToNotify {
				app.MessageToOwnerWithError(ctx, "（"+strconv.Itoa(numContinuousRetrieveNextPageTokenFailed)+"回目） failed to retrieve next page token", err)
			}
			waitSeconds := CalculateRetryIntervalSec(RetryIntervalCalculationBase, numContinuousRetrieveNextPageTokenFailed)
			time.Sleep(time.Duration(waitSeconds) * time.Second)
			continue
		} else {
			numContinuousRetrieveNextPageTokenFailed = 0
		}

		// fetch chat messages
		chatMessages, nextPageToken, pollingIntervalMillis, err := app.ListLiveChatMessages(ctx, pageToken)
		if err != nil {
			numContinuousListMessagesFailed += 1
			if numContinuousListMessagesFailed >= MinimumTryTimesToNotify {
				app.MessageToOwnerWithError(ctx, "（"+strconv.Itoa(numContinuousListMessagesFailed)+
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
		if err := app.SaveNextPageToken(ctx, nextPageToken); err != nil {
			app.MessageToOwnerWithError(ctx, "(1回目) failed to save next page token", err)
			// 少し待ってから再試行
			time.Sleep(3 * time.Second)
			err2 := app.SaveNextPageToken(ctx, nextPageToken)
			if err2 != nil {
				app.MessageToOwnerWithError(ctx, "(2回目) failed to save next page token", err2)
				// pass
			}
		}

		// chatMessagesを保存
		for _, chatMessage := range chatMessages {
			// only if chatMessage has a text message
			if !youtubebot.HasTextMessageByAuthor(chatMessage) {
				continue
			}

			if err = app.AddLiveChatHistoryDoc(ctx, chatMessage); err != nil {
				app.MessageToOwnerWithError(ctx, "(1回目) failed to add live chat history", err)
				time.Sleep(2 * time.Second)
				if err2 := app.AddLiveChatHistoryDoc(ctx, chatMessage); err2 != nil {
					app.MessageToOwnerWithError(ctx, "(2回目) failed to add live chat history", err2)
					// pass
				}
			}
		}

		// process the command (includes not command)
		for _, chatMessage := range chatMessages {
			if youtubebot.IsFanFundingEvent(chatMessage) {
				app.MessageToOwner(ctx, fmt.Sprintf("Fan funding event:\n```%# v```", pretty.Formatter(chatMessage)))
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
			if err := app.ProcessMessage(ctx, message, channelId, displayName, profileImageUrl, isModerator, isOwner, isMember); err != nil {
				app.MessageToOwnerWithError(ctx, "error in ProcessMessage()", err)
			}
		}

		waitAtLeastMilliSec1 = math.Max(float64((time.Duration(pollingIntervalMillis)*time.Millisecond - utils.
			JstNow().Sub(lastChatFetched)).Milliseconds()), 0)
		waitAtLeastMilliSec2 = math.Max(float64((time.Duration(app.Configs.Constants.SleepIntervalMilli)*time.Millisecond - utils.JstNow().Sub(lastChatFetched)).Milliseconds()), 0)
		sleepInterval = time.Duration(math.Max(waitAtLeastMilliSec1, waitAtLeastMilliSec2)) * time.Millisecond
		slog.Info(fmt.Sprintf("waiting for %.2f seconds...\n\n", sleepInterval.Seconds()))
		time.Sleep(sleepInterval)
	}
}

func main() {
	utils.LoadEnv(".env")
	ctx := context.Background()
	Bot(ctx)
}
