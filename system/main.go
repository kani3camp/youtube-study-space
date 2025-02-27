package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"time"

	"app.modules/core/youtubebot"
	"github.com/kr/pretty"

	"app.modules/core"
	"app.modules/core/utils"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	"google.golang.org/api/youtube/v3"
)

const (
	MaxRetryIntervalSeconds      = 300
	RetryIntervalCalculationBase = 1.2
	MinimumTryTimesToNotify      = 2
)

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
	sys, err := core.NewSystem(ctx, false, clientOption)
	if err != nil {
		slog.Error("failed to initialize system for long time sitting check", "error", err)
		return
	}

	sys.MessageToOwner("居座り防止プログラムが起動しました。")

	sys.GoroutineCheckLongTimeSitting(ctx)
}

func CalculateRetryIntervalSec(base float64, numContinuousFailed int) float64 {
	return math.Min(MaxRetryIntervalSeconds, math.Pow(base, float64(numContinuousFailed)))
}

// Bot メインのボット処理を実行する
func Bot(ctx context.Context, clientOption option.ClientOption) {
	sys, err := core.NewSystem(ctx, true, clientOption)
	if err != nil {
		slog.Error("failed to initialize system for bot", "error", err)
		return
	}

	sys.MessageToOwner("Botが起動しました。\n" + sys.GetInfoString())
	defer func() { // when error occurred
		sys.CloseFirestoreClient()
		sys.MessageToLiveChat(ctx, "エラーが起きたため終了します。お手数ですが管理者に連絡してください。")
		sys.MessageToOwner("app stopped!!")
	}()

	go CheckLongTimeSitting(ctx, clientOption) // 居座り防止処理を並行実行

	botState := &BotState{
		lastCheckedDesiredMaxSeats:               utils.JstNow(),
		checkDesiredMaxSeatsIntervalSec:          sys.Configs.Constants.CheckDesiredMaxSeatsIntervalSec,
		numContinuousRetrieveNextPageTokenFailed: 0,
		numContinuousListMessagesFailed:          0,
	}

	for {
		// max_seatsを変えるか確認
		checkAndAdjustMaxSeats(ctx, sys, botState)

		// ライブチャットメッセージを取得して処理
		if !fetchAndProcessMessages(ctx, sys, botState) {
			continue
		}

		// 次のポーリングまで待機
		sleepInterval := calculateSleepInterval(botState, sys.Configs.Constants.SleepIntervalMilli)
		slog.Info(fmt.Sprintf("waiting for %.2f seconds...\n\n", sleepInterval.Seconds()))
		time.Sleep(sleepInterval)
	}
}

// BotState ボットの状態を保持する構造体
type BotState struct {
	lastCheckedDesiredMaxSeats               time.Time
	checkDesiredMaxSeatsIntervalSec          int
	numContinuousRetrieveNextPageTokenFailed int
	numContinuousListMessagesFailed          int
	lastChatFetched                          time.Time
	pollingIntervalMillis                    int
}

// checkAndAdjustMaxSeats 必要に応じて最大席数を調整する
func checkAndAdjustMaxSeats(ctx context.Context, sys *core.System, state *BotState) {
	if !utils.JstNow().After(state.lastCheckedDesiredMaxSeats.Add(time.Duration(state.checkDesiredMaxSeatsIntervalSec) * time.Second)) {
		return
	}

	slog.Info("checking desired max seats")
	constants, err := sys.FirestoreController.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		sys.MessageToOwnerWithError("sys.firestoreController.ReadSystemConstantsConfig(ctx)でエラー", err)
	} else {
		if constants.DesiredMaxSeats != constants.MaxSeats || constants.DesiredMemberMaxSeats != constants.MemberMaxSeats {
			if err := sys.AdjustMaxSeats(ctx); err != nil {
				sys.MessageToOwnerWithError("failed sys.AdjustMaxSeats()", err)
			}
		}
	}
	state.lastCheckedDesiredMaxSeats = utils.JstNow()
}

// fetchAndProcessMessages ライブチャットメッセージを取得して処理する
// 成功した場合はtrue、失敗した場合はfalseを返す
func fetchAndProcessMessages(ctx context.Context, sys *core.System, state *BotState) bool {
	// page token取得
	pageToken, err := sys.GetNextPageToken(ctx, nil)
	if err != nil {
		state.numContinuousRetrieveNextPageTokenFailed++
		if state.numContinuousRetrieveNextPageTokenFailed >= MinimumTryTimesToNotify {
			sys.MessageToOwnerWithError("（"+strconv.Itoa(state.numContinuousRetrieveNextPageTokenFailed)+"回目） failed to retrieve next page token", err)
		}
		waitSeconds := CalculateRetryIntervalSec(RetryIntervalCalculationBase, state.numContinuousRetrieveNextPageTokenFailed)
		time.Sleep(time.Duration(waitSeconds) * time.Second)
		return false
	}
	state.numContinuousRetrieveNextPageTokenFailed = 0

	// fetch chat messages
	chatMessages, nextPageToken, pollingIntervalMillis, err := sys.ListLiveChatMessages(ctx, pageToken)
	if err != nil {
		state.numContinuousListMessagesFailed++
		if state.numContinuousListMessagesFailed >= MinimumTryTimesToNotify {
			sys.MessageToOwnerWithError("（"+strconv.Itoa(state.numContinuousListMessagesFailed)+
				"回目） failed to retrieve chat messages", err)
		}
		waitSeconds := CalculateRetryIntervalSec(RetryIntervalCalculationBase, state.numContinuousListMessagesFailed)
		time.Sleep(time.Duration(waitSeconds) * time.Second)
		return false
	}
	state.numContinuousListMessagesFailed = 0
	state.lastChatFetched = utils.JstNow()
	state.pollingIntervalMillis = pollingIntervalMillis

	// save nextPageToken
	saveNextPageTokenWithRetry(ctx, sys, nextPageToken)

	// メッセージを保存して処理
	saveAndProcessMessages(ctx, sys, chatMessages)

	return true
}

// saveNextPageTokenWithRetry 次のページトークンを保存する（リトライあり）
func saveNextPageTokenWithRetry(ctx context.Context, sys *core.System, nextPageToken string) {
	if err := sys.SaveNextPageToken(ctx, nextPageToken); err != nil {
		sys.MessageToOwnerWithError("(1回目) failed to save next page token", err)
		// 少し待ってから再試行
		time.Sleep(3 * time.Second)
		if err2 := sys.SaveNextPageToken(ctx, nextPageToken); err2 != nil {
			sys.MessageToOwnerWithError("(2回目) failed to save next page token", err2)
			// pass
		}
	}
}

// saveAndProcessMessages チャットメッセージを保存して処理する
func saveAndProcessMessages(ctx context.Context, sys *core.System, chatMessages []*youtube.LiveChatMessage) {
	// chatMessagesを保存
	for _, chatMessage := range chatMessages {
		// only if chatMessage has a text message
		if !youtubebot.HasTextMessageByAuthor(chatMessage) {
			continue
		}

		saveLiveChatHistoryWithRetry(ctx, sys, chatMessage)
	}

	// process the command (includes not command)
	for _, chatMessage := range chatMessages {
		if youtubebot.IsFanFundingEvent(chatMessage) {
			sys.MessageToOwner(fmt.Sprintf("Fan funding event:\n```%# v```", pretty.Formatter(chatMessage)))
		}

		// only if chatMessage has text message content
		if !youtubebot.HasTextMessageByAuthor(chatMessage) {
			continue
		}

		processCommand(ctx, sys, chatMessage)
	}
}

// saveLiveChatHistoryWithRetry ライブチャット履歴を保存する（リトライあり）
func saveLiveChatHistoryWithRetry(ctx context.Context, sys *core.System, chatMessage *youtube.LiveChatMessage) {
	if err := sys.AddLiveChatHistoryDoc(ctx, chatMessage); err != nil {
		sys.MessageToOwnerWithError("(1回目) failed to add live chat history", err)
		time.Sleep(2 * time.Second)
		if err2 := sys.AddLiveChatHistoryDoc(ctx, chatMessage); err2 != nil {
			sys.MessageToOwnerWithError("(2回目) failed to add live chat history", err2)
			// pass
		}
	}
}

// processCommand コマンドを処理する
func processCommand(ctx context.Context, sys *core.System, chatMessage *youtube.LiveChatMessage) {
	message := youtubebot.ExtractTextMessageByAuthor(chatMessage)
	channelId := youtubebot.ExtractAuthorChannelId(chatMessage)
	displayName := youtubebot.ExtractAuthorDisplayName(chatMessage)
	profileImageUrl := youtubebot.ExtractAuthorProfileImageUrl(chatMessage)
	isModerator := youtubebot.IsChatMessageByModerator(chatMessage)
	isOwner := youtubebot.IsChatMessageByOwner(chatMessage)
	isMember := isOwner || youtubebot.IsChatMessageByMember(chatMessage)

	slog.Info(channelId + " (" + displayName + "): " + message)

	if err := sys.Command(ctx, message, channelId, displayName, profileImageUrl, isModerator, isOwner, isMember); err != nil {
		sys.MessageToOwnerWithError("error in Command()", err)
	}
}

// calculateSleepInterval 次のポーリングまでの待機時間を計算する
func calculateSleepInterval(state *BotState, sleepIntervalMilli int) time.Duration {
	waitAtLeastMilliSec1 := math.Max(float64((time.Duration(state.pollingIntervalMillis)*time.Millisecond - utils.
		JstNow().Sub(state.lastChatFetched)).Milliseconds()), 0)
	waitAtLeastMilliSec2 := math.Max(float64((time.Duration(sleepIntervalMilli)*time.Millisecond - utils.JstNow().Sub(state.lastChatFetched)).Milliseconds()), 0)
	return time.Duration(math.Max(waitAtLeastMilliSec1, waitAtLeastMilliSec2)) * time.Millisecond
}

func main() {
	clientOption, ctx, err := Init()
	if err != nil {
		panic(err)
	}

	Bot(ctx, clientOption)
}
