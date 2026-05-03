package workspaceapp

import (
	"context"
	"testing"
	"time"

	"app.modules/core/repository"
	mock_myfirestore "app.modules/core/repository/mocks"
	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

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
