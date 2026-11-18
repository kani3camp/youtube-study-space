package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/youtube/v3"

	"app.modules/core/workspaceapp"

	"github.com/kr/pretty"

	"app.modules/core/wordsreader"
	"app.modules/core/youtubebot"

	"google.golang.org/api/option"
	"google.golang.org/api/transport"

	"app.modules/core/timeutil"
	"app.modules/core/utils"
)

const (
	MaxRetryIntervalSeconds      = 300
	RetryIntervalCalculationBase = 1.2
)

func Init() (option.ClientOption, context.Context, error) {
	utils.LoadEnv(".env")
	credentialFilePath := os.Getenv("CREDENTIAL_FILE_LOCATION")

	ctx := context.Background()
	//nolint:staticcheck // Credential file path is repo/operator-controlled and restricted to the service account JSON used for this app.
	clientOption := option.WithCredentialsFile(credentialFilePath)

	creds, err := transport.Creds(ctx, clientOption)
	if err != nil {
		return nil, nil, fmt.Errorf("load Google credentials: %w", err)
	}
	fmt.Printf("Project ID: %s\n", creds.ProjectID)
	fmt.Println("Is this the correct project ID? (yes/no)")
	var s string
	if _, err := fmt.Scanln(&s); err != nil {
		return nil, nil, fmt.Errorf("failed to read project confirmation: %w", err)
	}
	if s != "yes" {
		return nil, nil, errors.New("aborted")
	}

	return clientOption, ctx, nil
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

func Bot(ctx context.Context, clientOption option.ClientOption) {
	app, err := workspaceapp.NewWorkspaceApp(ctx, true, clientOption)
	if err != nil {
		slog.ErrorContext(ctx, "failed core.NewWorkspaceApp()", "error", err)
		return
	}
	defer app.CloseFirestoreClient()

	utils.LogSeatAppearanceWriterCapability(ctx)

	ngWordConfig, err := loadNGWordConfig(ctx, clientOption, app.Configs.Constants.BotConfigSpreadsheetID)
	if err != nil {
		app.MessageToOwnerWithError(ctx, "failed loadNGWordConfig()", err)
		return
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

		result, err := pollLiveChatOnce(ctx, app, func(ctx context.Context, chatMessage *youtube.LiveChatMessage) {
			processChatMessage(ctx, app, ngWordConfig, chatMessage)
		})
		switch {
		case errors.Is(err, errReadNextPageToken):
			numContinuousRetrieveNextPageTokenFailed += 1
			if numContinuousRetrieveNextPageTokenFailed >= MinimumTryTimesToNotify {
				app.MessageToOwnerWithError(ctx, "（"+strconv.Itoa(numContinuousRetrieveNextPageTokenFailed)+"回目） failed to retrieve next page token", err)
			}
			waitSeconds := CalculateRetryIntervalSec(RetryIntervalCalculationBase, numContinuousRetrieveNextPageTokenFailed)
			time.Sleep(time.Duration(waitSeconds) * time.Second)
			continue
		case errors.Is(err, errListLiveChatMessages):
			numContinuousRetrieveNextPageTokenFailed = 0
			numContinuousListMessagesFailed += 1
			if numContinuousListMessagesFailed >= MinimumTryTimesToNotify {
				app.MessageToOwnerWithError(ctx, "（"+strconv.Itoa(numContinuousListMessagesFailed)+
					"回目） failed to retrieve chat messages", err)
			}
			waitSeconds := CalculateRetryIntervalSec(RetryIntervalCalculationBase, numContinuousListMessagesFailed)
			time.Sleep(time.Duration(waitSeconds) * time.Second)
			continue
		}
		numContinuousRetrieveNextPageTokenFailed = 0
		numContinuousListMessagesFailed = 0
		lastChatFetched = result.fetchedAt

		waitAtLeastMilliSec1 = math.Max(float64((time.Duration(result.pollingIntervalMillis)*time.Millisecond - timeutil.
			JstNow().Sub(lastChatFetched)).Milliseconds()), 0)
		waitAtLeastMilliSec2 = math.Max(float64((time.Duration(app.Configs.Constants.SleepIntervalMilli)*time.Millisecond - timeutil.JstNow().Sub(lastChatFetched)).Milliseconds()), 0)
		sleepInterval = time.Duration(math.Max(waitAtLeastMilliSec1, waitAtLeastMilliSec2)) * time.Millisecond
		slog.Info(fmt.Sprintf("waiting for %.2f seconds...\n\n", sleepInterval.Seconds()))
		time.Sleep(sleepInterval)
	}
}

var (
	errReadNextPageToken    = errors.New("read next page token")
	errListLiveChatMessages = errors.New("list live chat messages")
)

// liveChatPoller はチャット取得1回分に必要な操作。テストでは取得位置の保存先を差し替える。
type liveChatPoller interface {
	GetNextPageToken(ctx context.Context, tx *firestore.Transaction) (string, error)
	ListLiveChatMessages(ctx context.Context, pageToken string) ([]*youtube.LiveChatMessage, string, int, error)
	SaveNextPageToken(ctx context.Context, nextPageToken string) error
	MessageToOwnerWithError(ctx context.Context, message string, argErr error)
}

type liveChatPollResult struct {
	fetchedAt             time.Time
	pollingIntervalMillis int
}

// pollLiveChatOnce は保存済みの取得位置からチャットを1回取得する。
// 取得に失敗したときは取得位置を更新せず、メッセージも処理しない。次回は同じ取得位置から再取得する。
// 返すエラーは errReadNextPageToken か errListLiveChatMessages のどちらかを含む。
func pollLiveChatOnce(
	ctx context.Context,
	poller liveChatPoller,
	handleMessage func(context.Context, *youtube.LiveChatMessage),
) (liveChatPollResult, error) {
	pageToken, err := poller.GetNextPageToken(ctx, nil)
	if err != nil {
		return liveChatPollResult{}, fmt.Errorf("%w: %w", errReadNextPageToken, err)
	}

	chatMessages, nextPageToken, pollingIntervalMillis, err := poller.ListLiveChatMessages(ctx, pageToken)
	if err != nil {
		return liveChatPollResult{}, fmt.Errorf("%w: %w", errListLiveChatMessages, err)
	}
	result := liveChatPollResult{
		fetchedAt:             timeutil.JstNow(),
		pollingIntervalMillis: pollingIntervalMillis,
	}

	if err := poller.SaveNextPageToken(ctx, nextPageToken); err != nil {
		poller.MessageToOwnerWithError(ctx, "(1回目) failed to save next page token", err)
		// 少し待ってから再試行
		time.Sleep(3 * time.Second)
		if err2 := poller.SaveNextPageToken(ctx, nextPageToken); err2 != nil {
			poller.MessageToOwnerWithError(ctx, "(2回目) failed to save next page token", err2)
			// pass
		}
	}

	for _, chatMessage := range chatMessages {
		handleMessage(ctx, chatMessage)
	}
	return result, nil
}

// processChatMessage はチャット1件を処理する（コマンド以外の投稿も含む）。
func processChatMessage(
	ctx context.Context,
	app *workspaceapp.WorkspaceApp,
	ngWordConfig workspaceapp.NGWordConfig,
	chatMessage *youtube.LiveChatMessage,
) {
	if youtubebot.IsFanFundingEvent(chatMessage) {
		app.MessageToOwner(ctx, fmt.Sprintf("Fan funding event:\n```%# v```", pretty.Formatter(chatMessage)))
	}

	// only if chatMessage has text message content
	if !youtubebot.HasTextMessageByAuthor(chatMessage) {
		return
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
	clientOption, ctx, err := Init()
	if err != nil {
		panic(err)
	}

	Bot(ctx, clientOption)
}
