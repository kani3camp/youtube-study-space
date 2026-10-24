package workspaceapp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"app.modules/core/repository"
	mock_myfirestore "app.modules/core/repository/mocks"
)

func TestGenerateWorkNameTrendRankingsDoesNotLogWorkNamesOrResponse(t *testing.T) {
	const workName = "PRIVATE_WORK_TITLE_481"
	const responseExample = "PRIVATE_MODEL_EXAMPLE_731"
	responseText := `{"rankings":[{"rank":1,"genre":"study","count":1,"examples":["` + responseExample + `"]}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
		}
		if !bytes.Contains(body, []byte(workName)) {
			t.Error("expected synthetic work name in provider request")
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"output": []any{map[string]any{
				"type": "message", "content": []any{map[string]any{"type": "output_text", "text": responseText}},
			}},
		}); err != nil {
			t.Errorf("write provider response: %v", err)
		}
	}))
	defer server.Close()

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	client := openai.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("synthetic-test-key"))
	rankings, err := generateWorkNameTrendRankingsWithClient(context.Background(), client, []string{workName})
	require.NoError(t, err)
	require.Len(t, rankings, 1)
	assert.Equal(t, responseExample, rankings[0].Examples[0])
	assert.Contains(t, logs.String(), `"work_name_count":1`)
	assert.Contains(t, logs.String(), `"ranking_count":1`)
	assert.NotContains(t, logs.String(), workName)
	assert.NotContains(t, logs.String(), responseExample)
	assert.NotContains(t, logs.String(), responseText)
}

func TestGenerateWorkNameTrendRankingsProviderErrorDoesNotLeakResponse(t *testing.T) {
	const workName = "PRIVATE_WORK_TITLE_482"
	const providerBody = "PRIVATE_PROVIDER_ERROR_BODY_732"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if _, err := io.WriteString(w, `{"error":{"message":"`+providerBody+`","type":"invalid_request_error"}}`); err != nil {
			t.Errorf("write provider error: %v", err)
		}
	}))
	defer server.Close()

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	client := openai.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("synthetic-test-key"))
	_, err := generateWorkNameTrendRankingsWithClient(context.Background(), client, []string{workName})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), providerBody)
	assert.Contains(t, logs.String(), `"error_class":"provider_request_failed"`)
	for _, secret := range []string{workName, providerBody, "synthetic-test-key"} {
		assert.False(t, strings.Contains(logs.String(), secret), "logs contain synthetic private value")
	}
}

func TestGenerateWorkNameTrendRankingsInvalidResponseDoesNotLogOutput(t *testing.T) {
	const privateOutput = "PRIVATE_INVALID_MODEL_OUTPUT_733"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"output": []any{map[string]any{
				"type": "message", "content": []any{map[string]any{"type": "output_text", "text": privateOutput}},
			}},
		}); err != nil {
			t.Errorf("write provider response: %v", err)
		}
	}))
	defer server.Close()

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	client := openai.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("synthetic-test-key"))
	_, err := generateWorkNameTrendRankingsWithClient(context.Background(), client, []string{"PRIVATE_WORK_TITLE_483"})
	require.Error(t, err)
	assert.Contains(t, logs.String(), `"error_class":"invalid_response"`)
	assert.NotContains(t, logs.String(), privateOutput)
	assert.NotContains(t, logs.String(), "PRIVATE_WORK_TITLE_483")
}

func TestUpdateWorkNameTrend_EmptyWorkNames(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	fixedNow := time.Date(2026, time.January, 1, 10, 0, 0, 0, time.UTC)
	mockDB := mock_myfirestore.NewMockRepository(ctrl)
	mockFirestoreClient := mock_myfirestore.NewMockDBClient(ctrl)

	mockDB.EXPECT().ReadActiveWorkNameSeats(gomock.Any(), true).Return([]repository.SeatDoc{}, nil).Times(1)
	mockDB.EXPECT().ReadActiveWorkNameSeats(gomock.Any(), false).Return([]repository.SeatDoc{}, nil).Times(1)
	mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).Times(1)

	var savedWorkNameTrend *repository.WorkNameTrendDoc
	mockDB.EXPECT().UpdateWorkNameTrend(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, tx *firestore.Transaction, workNameTrend repository.WorkNameTrendDoc) error {
			savedWorkNameTrend = &workNameTrend
			return nil
		}).
		Times(1)
	mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, opts ...firestore.TransactionOption) error {
			tx := &firestore.Transaction{}
			return f(ctx, tx)
		}).
		Times(1)

	app := WorkspaceApp{
		Repository: mockDB,
		nowFunc:    func() time.Time { return fixedNow },
	}

	err := app.UpdateWorkNameTrend(ctx, "dummy-api-key")

	require.NoError(t, err)
	require.NotNil(t, savedWorkNameTrend)
	assert.NotNil(t, savedWorkNameTrend.Ranking)
	assert.Empty(t, savedWorkNameTrend.Ranking)
	assert.Equal(t, fixedNow, savedWorkNameTrend.RankedAt)
}

func TestParseWorkNameTrendRankings(t *testing.T) {
	t.Run("rankings欠落は空スライスに正規化する", func(t *testing.T) {
		rankings, err := parseWorkNameTrendRankings(`{}`)

		require.NoError(t, err)
		assert.NotNil(t, rankings)
		assert.Empty(t, rankings)
	})

	t.Run("rankings nullは空スライスに正規化する", func(t *testing.T) {
		rankings, err := parseWorkNameTrendRankings(`{"rankings":null}`)

		require.NoError(t, err)
		assert.NotNil(t, rankings)
		assert.Empty(t, rankings)
	})

	t.Run("rankings空配列は空スライスに正規化する", func(t *testing.T) {
		rankings, err := parseWorkNameTrendRankings(`{"rankings":[]}`)

		require.NoError(t, err)
		assert.NotNil(t, rankings)
		assert.Empty(t, rankings)
	})

	t.Run("valid ranking JSONは値を保持する", func(t *testing.T) {
		rankings, err := parseWorkNameTrendRankings(`{"rankings":[{"rank":1,"genre":"study","count":2,"examples":["math","english"]}]}`)

		require.NoError(t, err)
		assert.Equal(t, []repository.WorkNameTrendRanking{
			{
				Rank:     1,
				Genre:    "study",
				Count:    2,
				Examples: []string{"math", "english"},
			},
		}, rankings)
	})

	t.Run("invalid JSONはエラーを返す", func(t *testing.T) {
		rankings, err := parseWorkNameTrendRankings(`{`)

		require.Error(t, err)
		assert.Nil(t, rankings)
	})
}

func TestNormalizeWorkNameTrendRankings(t *testing.T) {
	t.Run("nilは空スライスに正規化する", func(t *testing.T) {
		rankings := normalizeWorkNameTrendRankings(nil)

		assert.NotNil(t, rankings)
		assert.Empty(t, rankings)
	})

	t.Run("non-nilはそのまま返す", func(t *testing.T) {
		input := []repository.WorkNameTrendRanking{
			{
				Rank:     1,
				Genre:    "study",
				Count:    2,
				Examples: []string{"math", "english"},
			},
		}

		rankings := normalizeWorkNameTrendRankings(input)

		assert.Equal(t, input, rankings)
	})
}
