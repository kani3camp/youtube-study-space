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
	}

	before := time.Now()
	err := app.UpdateWorkNameTrend(ctx, "dummy-api-key")
	after := time.Now()

	require.NoError(t, err)
	require.NotNil(t, savedWorkNameTrend)
	assert.NotNil(t, savedWorkNameTrend.Ranking)
	assert.Empty(t, savedWorkNameTrend.Ranking)
	assert.False(t, savedWorkNameTrend.RankedAt.IsZero())
	assert.False(t, savedWorkNameTrend.RankedAt.Before(before))
	assert.False(t, savedWorkNameTrend.RankedAt.After(after))
}
