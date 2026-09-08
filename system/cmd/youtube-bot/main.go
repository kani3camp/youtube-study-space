package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"app.modules/core/workspaceapp"

	"github.com/kr/pretty"

	"app.modules/core/wordsreader"
	"app.modules/core/youtubebot"

	"google.golang.org/api/option"
	"app.modules/core/timeutil"
)

const (
	MaxRetryIntervalSeconds      = 300
	RetryIntervalCalculationBase = 1.2
)

func Init() (option.ClientOption, context.Context, bool, error) {
	ctx := context.Background()
	clientOption, interactive, err := initGoogleClient(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	return clientOption, ctx, interactive, nil
}

func CheckLongTimeSitting(ctx context.Context, clientOption option.ClientOption) {
	app, err := workspaceapp.NewWorkspaceApp(ctx, false, clientOption)
	if err != nil {
		slog.ErrorContext(ctx, "failed core.NewWorkspaceApp()", "error", err)
		return
	}

	app.MessageToOwner(ctx, "居座り防止プログラムが起動しました。")

	app.GoroutineCheckLongTimeSitting(ctx)
}

func CalculateRetryIntervalSec(base float64, numContinuousFailed int) float64 {
	return math.Min(MaxRetryIntervalSeconds, math.Pow(base, float64(numContinuousFailed)))
}

func Bot(ctx context.Context, clientOption option.ClientOption, interactive bool) error {
	app, err := workspaceapp.NewWorkspaceApp(ctx, interactive, clientOption)
	if err != nil {
		return fmt.Errorf("initialize workspace app: %w", err)
	}
	defer app.CloseFirestoreClient()

	if !interactive {
		uninitializedFields := workspaceapp.UninitializedConstantsFields(app.Configs.Constants)
		if len(uninitializedFields) > 0 {
			return fmt.Errorf(
				"system constants contain zero values in headless mode: %s",
				strings.Join(uninitializedFields, ", "),
			)
		}
	}

	ngWordConfig, err := loadNGWordConfig(ctx, clientOption, app.Configs.Constants.BotConfigSpreadsheetID)
	if err != nil {
		app.MessageToOwnerWithError(ctx, "failed loadNGWordConfig()", err)
		return fmt.Errorf("load NG word config: %w", err)
	}

	app.MessageToOwner(ctx, fmt.Sprintf("Botが起動しました。\n全規制ワード数: %d", ngWordConfig.Count()))
	defer func() { // when error occurred
		app.MessageToLiveChat(ctx, "エラーが起きたため終了します。お手数ですが管理者に連絡してください。")
		app.MessageToOwner(ctx, "app stopped!!")
	}()

	go CheckLongTimeSitting(ctx, clientOption) // 居座り防止処理を並行実行

	checkDesiredMaxSeatsIntervalSec := app.Configs.Constants.CheckDesiredMaxSeatsIntervalSec

	lastCheckedDesiredMaxSeats := timeutil.JstNow()

	const MinimumTryTimesToNotify = 2
	numContinuousRetrieveNextPageTokenFailed := 0
	numContinuousListMessagesFailed := 0
	var lastChatFetched time.Time
	var waitAtLeastMilliSec1 float64
	var waitAtLeastMilliSec2 float64
	var sleepInterval time.Duration

	for {
		// max_seatsを変えるか確認
		if timeutil.JstNow().After(lastCheckedDesiredMaxSeats.Add(time.Duration(checkDesiredMaxSeatsIntervalSec) * time.Second)) {
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
			lastCheckedDesiredMaxSeats = timeutil.JstNow()
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
		lastChatFetched = timeutil.JstNow()

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
			channelID := youtubebot.ExtractAuthorChannelID(chatMessage)
			displayName := youtubebot.ExtractAuthorDisplayName(chatMessage)
			profileImageURL := youtubebot.ExtractAuthorProfileImageURL(chatMessage)
			isModerator := youtubebot.IsChatMessageByModerator(chatMessage)
			isOwner := youtubebot.IsChatMessageByOwner(chatMessage)
			isMember := isOwner || youtubebot.IsChatMessageByMember(chatMessage)
			if err := app.ProcessMessage(ctx, ngWordConfig, message, channelID, displayName, profileImageURL, isModerator, isOwner, isMember); err != nil {
				app.MessageToOwnerWithError(ctx, "error in ProcessMessage()", err)
			}
		}

		waitAtLeastMilliSec1 = math.Max(float64((time.Duration(pollingIntervalMillis)*time.Millisecond - timeutil.
			JstNow().Sub(lastChatFetched)).Milliseconds()), 0)
		waitAtLeastMilliSec2 = math.Max(float64((time.Duration(app.Configs.Constants.SleepIntervalMilli)*time.Millisecond - timeutil.JstNow().Sub(lastChatFetched)).Milliseconds()), 0)
		sleepInterval = time.Duration(math.Max(waitAtLeastMilliSec1, waitAtLeastMilliSec2)) * time.Millisecond
		slog.Info(fmt.Sprintf("waiting for %.2f seconds...\n\n", sleepInterval.Seconds()))
		time.Sleep(sleepInterval)
	}
}

func loadNGWordConfig(
	ctx context.Context,
	clientOption option.ClientOption,
	spreadsheetID string,
) (workspaceapp.NGWordConfig, error) {
	slog.InfoContext(ctx, "initializing spreadsheet reader...")

	wordsReader, err := wordsreader.NewSpreadsheetReader(ctx, clientOption, spreadsheetID, "01", "02")
	if err != nil {
		return workspaceapp.NGWordConfig{}, fmt.Errorf("in NewSpreadsheetReader(): %w", err)
	}

	slog.InfoContext(ctx, "reading block regexes...")

	blockRegexesForChatMessage, blockRegexesForChannelName, err := wordsReader.ReadBlockRegexes(ctx)
	if err != nil {
		return workspaceapp.NGWordConfig{}, fmt.Errorf("in ReadBlockRegexes(): %w", err)
	}

	slog.InfoContext(ctx, "reading notification regexes...")

	notificationRegexesForChatMessage, notificationRegexesForChannelName, err := wordsReader.ReadNotificationRegexes(ctx)
	if err != nil {
		return workspaceapp.NGWordConfig{}, fmt.Errorf("in ReadNotificationRegexes(): %w", err)
	}

	return workspaceapp.NewNGWordConfig(
		blockRegexesForChatMessage,
		blockRegexesForChannelName,
		notificationRegexesForChatMessage,
		notificationRegexesForChannelName,
	), nil
}

func main() {
	clientOption, ctx, interactive, err := Init()
	if err != nil {
		panic(err)
	}

	if len(os.Args) == 2 && os.Args[1] == "preflight" {
		if interactive {
			panic("youtube-bot preflight requires WIF/headless mode")
		}
		if err := Preflight(ctx, clientOption, os.Stdout); err != nil {
			panic(err)
		}
		return
	}
	if len(os.Args) != 1 {
		panic("usage: youtube-bot [preflight]")
	}

	if err := Bot(ctx, clientOption, interactive); err != nil {
		panic(err)
	}
}
