//go:build integration

// TODO: GitHub ActionsでFirestore Emulatorを使用するようになったら、このファイルも自動テスト対象に変更する。

package workspaceapp

import (
	"context"
	"os"
	"testing"
	"time"

	"google.golang.org/api/iterator"

	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"

	"github.com/stretchr/testify/assert"
)

func TestEnterRoom(t *testing.T) {
	// 入室ができること

	setEnvErr := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080")
	if setEnvErr != nil {
		t.Fatal(setEnvErr)
	}

	userID := "test_user_id"
	userDisplayName := "test_user_display_name"
	userProfileImageURL := "test_user_profile_image_url"
	inOption := utils.InOption{
		IsSeatIDSet: true,
		SeatID:      1,
		MinWorkOrderOption: &utils.MinWorkOrderOption{
			DurationMin: 30,
			WorkName:    "test_work_name",
		},
		IsMemberSeat: false,
	}
	seatAppearance := repository.SeatAppearance{
		ColorCode1:           "#000000",
		ColorCode2:           "#000000",
		NumStars:             3,
		ColorGradientEnabled: true,
	}
	expectedUntilExitMin := 30
	enteredAt := time.Date(2021, 10, 1, 0, 0, 0, 0, timeutil.JapanLocation())
	expectedUntil := enteredAt.Add(time.Duration(expectedUntilExitMin) * time.Minute)

	ctx := context.Background()

	client, clientErr := firestore.NewClient(ctx, firestore.DetectProjectID)
	if clientErr != nil {
		t.Fatal(clientErr)
	}
	app := WorkspaceApp{
		Repository:    &repository.FirestoreControllerImplements{firestoreClient: client},
		alertOwnerBot: moderatorbot.DummyMessageBot{},
	}
	t.Cleanup(func() {
		app.CloseFirestoreClient()
	})

	// ユーザーデータを作成しておく
	userErr := app.Repository.CreateUser(ctx, nil, userID, repository.UserDoc{})
	if userErr != nil {
		t.Fatal(userErr)
	}
	t.Cleanup(func() {
		userRef := app.Repository.FirestoreClient.Collection(repository.USERS).Doc(userID)
		if err := app.Repository.DeleteDocRef(ctx, nil, userRef); err != nil {
			t.Fatal(err)
		}
	})

	var resultUntilExitMin int
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		untilExitMin, err := app.enterRoom(
			ctx,
			tx,
			userID,
			userDisplayName,
			userProfileImageURL,
			inOption.SeatID,
			inOption.IsMemberSeat,
			inOption.MinWorkOrderOption.WorkName,
			"",
			inOption.MinWorkOrderOption.DurationMin,
			seatAppearance,
			repository.WorkState,
			true,
			time.Time{},
			time.Time{},
			enteredAt,
		)
		if err != nil {
			return err
		}
		resultUntilExitMin = untilExitMin
		return nil
	})
	if txErr != nil {
		t.Fatal(txErr)
	}
	t.Cleanup(func() {
		if err := app.Repository.DeleteSeat(ctx, nil, inOption.SeatID, inOption.IsMemberSeat); err != nil {
			t.Fatal(err)
		}
	})

	// 入室したことを確認
	seat, seatErr := app.Repository.ReadSeat(ctx, nil, inOption.SeatID, inOption.IsMemberSeat)
	if seatErr != nil {
		t.Fatal(seatErr)
	}
	assert.Equal(t, repository.SeatDoc{
		SeatID:                 inOption.SeatID,
		UserID:                 userID,
		UserDisplayName:        userDisplayName,
		WorkName:               inOption.MinWorkOrderOption.WorkName,
		BreakWorkName:          "",
		EnteredAt:              enteredAt.UTC(),
		Until:                  expectedUntil.UTC(),
		Appearance:             seatAppearance,
		State:                  repository.WorkState,
		CurrentStateStartedAt:  enteredAt.UTC(),
		CurrentStateUntil:      expectedUntil.UTC(),
		CumulativeWorkSec:      0,
		DailyCumulativeWorkSec: 0,
		UserProfileImageURL:    userProfileImageURL,
	}, seat)

	// 履歴が作成されたことを確認
	iter := app.Repository.FirestoreClient.Collection(repository.UserActivities).Where(repository.UserIDDocProperty, "==", userID).Documents(ctx)
	var userActivities []repository.UserActivityDoc
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		var userActivity repository.UserActivityDoc
		dataErr := doc.DataTo(&userActivity)
		if dataErr != nil {
			t.Fatal(dataErr)
		}
		t.Cleanup(func() {
			userActivityRef := app.Repository.FirestoreClient.Collection(repository.UserActivities).Doc(doc.Ref.ID)
			if err := app.Repository.DeleteDocRef(ctx, nil, userActivityRef); err != nil {
				t.Fatal(err)
			}
		})
		userActivities = append(userActivities, userActivity)
	}
	assert.Len(t, userActivities, 1)
	userActivity := userActivities[0]
	assert.Equal(t, repository.UserActivityDoc{
		UserID:       userID,
		ActivityType: repository.EnterRoomActivity,
		SeatID:       inOption.SeatID,
		IsMemberSeat: inOption.IsMemberSeat,
		TakenAt:      enteredAt.UTC(),
	}, userActivity)

	// 自動退室予定時刻が正しいことを確認
	assert.Equal(t, expectedUntilExitMin, resultUntilExitMin)
}
