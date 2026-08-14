package workspaceapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/repository"
	mock_repository "app.modules/core/repository/mocks"
	"app.modules/core/timeutil"
)

func TestEnsureProcessedUserRegisteredTx_AlreadyRegistered(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockDB := mock_repository.NewMockRepository(ctrl)
	tx := &firestore.Transaction{}
	mockDB.EXPECT().ReadUser(gomock.Any(), tx, "user-1").Return(repository.UserDoc{}, nil).Times(1)
	app := WorkspaceApp{Repository: mockDB, ProcessedUserID: "user-1"}

	require.NoError(t, app.ensureProcessedUserRegisteredTx(context.Background(), tx))
}

func TestEnsureProcessedUserRegisteredTx_CreatesMissingUser(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockDB := mock_repository.NewMockRepository(ctrl)
	tx := &firestore.Transaction{}
	fixedNow := time.Date(2026, 8, 15, 10, 0, 0, 0, timeutil.JapanLocation())
	mockDB.EXPECT().ReadUser(gomock.Any(), tx, "user-1").Return(
		repository.UserDoc{},
		status.Error(codes.NotFound, "missing"),
	).Times(1)
	mockDB.EXPECT().CreateUser(gomock.Any(), tx, "user-1", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ *firestore.Transaction, _ string, user repository.UserDoc) error {
			assert.Equal(t, fixedNow, user.RegistrationDate)
			assert.Zero(t, user.DailyTotalStudySec)
			assert.Zero(t, user.TotalStudySec)
			return nil
		},
	).Times(1)
	app := WorkspaceApp{
		Repository:      mockDB,
		ProcessedUserID: "user-1",
		nowFunc:         func() time.Time { return fixedNow },
	}

	require.NoError(t, app.ensureProcessedUserRegisteredTx(context.Background(), tx))
}

func TestEnsureProcessedUserRegisteredTx_PropagatesReadFailure(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockDB := mock_repository.NewMockRepository(ctrl)
	tx := &firestore.Transaction{}
	readErr := errors.New("read failed")
	mockDB.EXPECT().ReadUser(gomock.Any(), tx, "user-1").Return(repository.UserDoc{}, readErr).Times(1)
	app := WorkspaceApp{Repository: mockDB, ProcessedUserID: "user-1"}

	err := app.ensureProcessedUserRegisteredTx(context.Background(), tx)
	require.Error(t, err)
	assert.ErrorContains(t, err, readErr.Error())
}

func TestEnsureProcessedUserRegisteredTx_PropagatesCreateFailure(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockDB := mock_repository.NewMockRepository(ctrl)
	tx := &firestore.Transaction{}
	createErr := errors.New("create failed")
	mockDB.EXPECT().ReadUser(gomock.Any(), tx, "user-1").Return(
		repository.UserDoc{},
		status.Error(codes.NotFound, "missing"),
	).Times(1)
	mockDB.EXPECT().CreateUser(gomock.Any(), tx, "user-1", gomock.Any()).Return(createErr).Times(1)
	app := WorkspaceApp{Repository: mockDB, ProcessedUserID: "user-1"}

	err := app.ensureProcessedUserRegisteredTx(context.Background(), tx)
	require.Error(t, err)
	assert.ErrorContains(t, err, createErr.Error())
}
