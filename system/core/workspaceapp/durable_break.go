package workspaceapp

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/firestore"

	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp/presenter"
	"app.modules/core/workspaceapp/usecase"
)

// buildDurableBreakReplyTx applies !break in the caller-owned message
// transaction and returns the legacy presenter message without posting it.
func (app *WorkspaceApp) buildDurableBreakReplyTx(
	ctx context.Context,
	tx *firestore.Transaction,
	breakOption *utils.MinWorkOrderOption,
	userExists bool,
) (string, error) {
	if breakOption == nil {
		return "", errors.New("durable !break option is nil")
	}

	now := app.currentTime()
	var result usecase.Result
	isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
	if err != nil {
		return "", fmt.Errorf("check room state for durable !break: %w", err)
	}
	isInRoom := isInMemberRoom || isInGeneralRoom
	if !isInRoom {
		result.Add(usecase.BreakEnterOnly{})
		return presenter.BuildBreakMessage(result, app.ProcessedUserDisplayName), nil
	}
	if !userExists {
		return "", errors.New("unregistered durable !break user unexpectedly has an active seat")
	}

	seat, err := app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
	if err != nil {
		return "", fmt.Errorf("read current seat for durable !break: %w", err)
	}
	if seat.State != repository.WorkState {
		result.Add(usecase.BreakWorkOnly{})
		return presenter.BuildBreakMessage(result, app.ProcessedUserDisplayName), nil
	}

	currentWorkedMin := int(timeutil.NoNegativeDuration(now.Sub(seat.CurrentStateStartedAt)).Minutes())
	if currentWorkedMin < app.Configs.Constants.MinBreakIntervalMin {
		result.Add(usecase.BreakWarn{
			MinBreakIntervalMin: app.Configs.Constants.MinBreakIntervalMin,
			CurrentWorkedMin:    currentWorkedMin,
		})
		return presenter.BuildBreakMessage(result, app.ProcessedUserDisplayName), nil
	}

	option := *breakOption
	if !option.IsDurationMinSet {
		option.DurationMin = app.Configs.Constants.DefaultBreakDurationMin
	}
	if !option.IsWorkNameSet {
		option.WorkName = seat.BreakWorkName
	}

	// All reads are complete. The previous Work segment, Seat state transition,
	// deprecated activity log, reply intent and Processed marker will commit in
	// the same Firestore transaction.
	workSegment, err := seat.GenerateWorkSegment(now, isInMemberRoom)
	if err != nil {
		return "", fmt.Errorf("generate work segment for durable !break: %w", err)
	}
	if err := app.Repository.CreateWorkSegmentDoc(ctx, tx, workSegment); err != nil {
		return "", fmt.Errorf("create work segment for durable !break: %w", err)
	}
	if err := seat.StartBreak(now, option.WorkName, option.DurationMin); err != nil {
		return "", fmt.Errorf("start break for durable !break: %w", err)
	}
	if err := app.Repository.UpdateSeat(ctx, tx, seat, isInMemberRoom); err != nil {
		return "", fmt.Errorf("update seat for durable !break: %w", err)
	}
	activity := repository.UserActivityDoc{
		UserID:       app.ProcessedUserID,
		ActivityType: repository.StartBreakActivity,
		SeatID:       seat.SeatID,
		IsMemberSeat: isInMemberRoom,
		TakenAt:      now,
	}
	if err := app.Repository.CreateUserActivityDoc(ctx, tx, activity); err != nil {
		return "", fmt.Errorf("create start-break activity for durable !break: %w", err)
	}

	result.Add(usecase.BreakStarted{
		SeatID:       seat.SeatID,
		IsMemberSeat: isInMemberRoom,
		WorkName:     option.WorkName,
		DurationMin:  option.DurationMin,
	})
	return presenter.BuildBreakMessage(result, app.ProcessedUserDisplayName), nil
}
