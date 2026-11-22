package workspaceapp

import (
	"context"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	mock_repository "app.modules/core/repository/mocks"
)

// 居座りチェックのループは、待機中に ctx がキャンセルされると最小間隔を待たずに戻る。
func TestGoroutineCheckLongTimeSitting_StopsWhenContextIsCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mock_repository.NewMockRepository(ctrl)
	checked := make(chan struct{}, 2)
	mockDB.EXPECT().ReadMemberSeats(gomock.Any()).Return(nil, nil).AnyTimes()
	mockDB.EXPECT().ReadGeneralSeats(gomock.Any()).DoAndReturn(func(context.Context) ([]repository.SeatDoc, error) {
		checked <- struct{}{}
		return nil, nil
	}).AnyTimes()
	app := &WorkspaceApp{
		Configs:       &Configs{Constants: repository.ConstantsConfigDoc{MinimumCheckLongTimeSittingIntervalMinutes: 60}},
		Repository:    mockDB,
		alertOwnerBot: moderatorbot.DummyMessageBot{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		app.GoroutineCheckLongTimeSitting(ctx)
	}()

	select {
	case <-checked:
	case <-time.After(5 * time.Second):
		t.Fatal("居座りチェックが実行されない")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ctx をキャンセルしても居座りチェックのループが止まらない")
	}
}
