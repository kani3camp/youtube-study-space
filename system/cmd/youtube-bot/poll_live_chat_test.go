package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/i18n"
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/repository"
	mockrepository "app.modules/core/repository/mocks"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp"
	"app.modules/core/youtubebot"
)

// YouTube API が一時的に HTTP 500 を返しても取得位置を失わず、復旧後は新着分だけを処理する。
func TestPollLiveChatPageResumesFromSavedTokenAfterHTTP500(t *testing.T) {
	const (
		savedToken  = "saved-page-token"
		nextToken   = "next-page-token"
		userID      = "new-viewer-channel-id"
		displayName = "new viewer"
	)
	require.NoError(t, i18n.LoadLocaleFolderFS())
	ctx := t.Context()

	var listCalls, postCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/youtube/v3/liveChat/messages", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			assert.Equal(t, savedToken, r.URL.Query().Get("pageToken"))
			if listCalls.Add(1) == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, err := io.WriteString(w, `{"error":{"code":500,"message":"backendError"}}`)
				assert.NoError(t, err)
				return
			}

			err := json.NewEncoder(w).Encode(youtube.LiveChatMessageListResponse{
				NextPageToken:         nextToken,
				PollingIntervalMillis: 2000,
				Items: []*youtube.LiveChatMessage{{
					Id: "new-message-id",
					Snippet: &youtube.LiveChatMessageSnippet{
						Type:               youtubebot.TextMessageEvent,
						TextMessageDetails: &youtube.LiveChatTextMessageDetails{MessageText: utils.SeatCommand},
					},
					AuthorDetails: &youtube.LiveChatMessageAuthorDetails{ChannelId: userID, DisplayName: displayName},
				}},
			})
			assert.NoError(t, err)
		case http.MethodPost:
			postCalls.Add(1)
			var message youtube.LiveChatMessage
			err := json.NewDecoder(r.Body).Decode(&message)
			if assert.NoError(t, err) && assert.NotNil(t, message.Snippet) && assert.NotNil(t, message.Snippet.TextMessageDetails) {
				assert.Equal(t, i18nmsg.CommandNotEnter(displayName, utils.InCommand), message.Snippet.TextMessageDetails.MessageText)
			}
			_, err = io.WriteString(w, `{}`)
			assert.NoError(t, err)
		default:
			t.Errorf("unexpected YouTube request method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)
	service, err := youtube.NewService(ctx, option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(server.Client()), option.WithoutAuthentication())
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	store := mockrepository.NewMockRepository(ctrl)
	// 取得位置の保存は復旧後の1回だけ。失敗時に保存しようとすると予期しない呼び出しとしてテストが失敗する。
	gomock.InOrder(
		store.EXPECT().ReadNextPageToken(ctx, nil).Return(savedToken, nil),
		store.EXPECT().ReadNextPageToken(ctx, nil).Return(savedToken, nil),
		store.EXPECT().UpdateNextPageToken(ctx, nextToken).Return(nil),
	)
	db := mockrepository.NewMockDBClient(ctrl)
	store.EXPECT().FirestoreClient().Return(db).Times(2)
	db.EXPECT().RunTransaction(ctx, gomock.Any()).DoAndReturn(
		func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, _ ...firestore.TransactionOption) error {
			return f(ctx, &firestore.Transaction{})
		},
	).Times(2)
	// 以下は新着の !seat を1回だけ処理するときの読み取り。トランザクション内の読み取りにはトランザクション用の ctx が渡る。
	store.EXPECT().ReadUser(gomock.Any(), gomock.Any(), userID).Return(repository.UserDoc{}, nil)
	store.EXPECT().ReadSeatWithUserID(gomock.Any(), userID, true).Return(repository.SeatDoc{}, status.Error(codes.NotFound, "no member seat"))
	store.EXPECT().ReadSeatWithUserID(gomock.Any(), userID, false).Return(repository.SeatDoc{}, status.Error(codes.NotFound, "no general seat"))
	app := &workspaceapp.WorkspaceApp{
		Configs:     &workspaceapp.Configs{},
		Repository:  store,
		LiveChatBot: &youtubebot.YoutubeLiveChatBot{LiveChatID: "live-chat-id", BotYoutubeService: service},
	}
	var ngWords workspaceapp.NGWordConfig

	// HTTP 500: エラーを返し、取得位置を保存せず、返信もしない。
	pageToken, err := app.GetNextPageToken(ctx, nil)
	require.NoError(t, err)
	_, _, err = pollLiveChatPage(ctx, app, ngWords, pageToken)
	var apiErr *googleapi.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	assert.Equal(t, int32(1), listCalls.Load())
	assert.Zero(t, postCalls.Load())

	// 復旧後: 同じ取得位置で再取得し、新着分に1回だけ返信して取得位置を進める。
	pageToken, err = app.GetNextPageToken(ctx, nil)
	require.NoError(t, err)
	_, pollingIntervalMillis, err := pollLiveChatPage(ctx, app, ngWords, pageToken)
	require.NoError(t, err)
	assert.Equal(t, 2000, pollingIntervalMillis)
	assert.Equal(t, int32(2), listCalls.Load())
	assert.Equal(t, int32(1), postCalls.Load())
}
