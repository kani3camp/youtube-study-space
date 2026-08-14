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

// buildDurableResumeReplyTx applies !resume in the caller-owned message
// transaction and returns the legacy presenter message without posting it.
func (app *WorkspaceApp) buildDurableResumeReplyTx(
	ctx context.Context,
	tx *firestore.Transaction,
	resumeOption *utils.WorkNameOption,
	userExists bool,
) (string, error) {
	if resumeOption == nil {
		return "", errors.New("durable !resume option is nil")
	}

	now := app.currentTime()
	var result usecase.Result
	isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
	if err != nil {
		return "", fmt.Errorf("check room state for durable !resume: %w", err)
	}
	isInRoom := isInMemberRoom || isInGeneralRoom
	if !isInRoom {
		result.Add(usecase.ResumeEnterOnly{})
		return presenter.BuildResumeMessage(result, app.ProcessedUserDisplayName), nil
	}
	if !userExists {
		return "", errors.New("unregistered durable !resume user unexpectedly has an active seat")
	}

	seat, err := app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
	if err != nil {
		return "", fmt.Errorf("read current seat for durable !resume: %w", err)
	}
	if seat.State != repository.BreakState {
		result.Add(usecase.ResumeBreakOnly{})
		return presenter.BuildResumeMessage(result, app.ProcessedUserDisplayName), nil
	}

	until := seat.Until
	workName := resumeOption.WorkName
	if !resumeOption.IsWorkNameSet {
		workName = seat.WorkName
	}

	// All reads are complete. The Break segment, Seat transition, deprecated
	// activity log, reply intent and Processed marker are staged atomically.
	breakSegment, err := seat.GenerateWorkSegment(now, isInMemberRoom)
	if err != nil {
		return "", fmt.Errorf("generate work segment for durable !resume: %w", err)
	}
	if err := app.Repository.CreateWorkSegmentDoc(ctx, tx, breakSegment); err != nil {
		return "", fmt.Errorf("create work segment for durable !resume: %w", err)
	}
	if err := seat.ResumeWork(now, workName); err != nil {
		return "", fmt.Errorf("resume work for durable !resume: %w", err)
	}
	if err := app.Repository.UpdateSeat(ctx, tx, seat, isInMemberRoom); err != nil {
		return "", fmt.Errorf("update seat for durable !resume: %w", err)
	}
	activity := repository.UserActivityDoc{
		UserID:       app.ProcessedUserID,
		ActivityType: repository.EndBreakActivity,
		SeatID:       seat.SeatID,
		IsMemberSeat: isInMemberRoom,
		TakenAt:      now,
	}
	if err := app.Repository.CreateUserActivityDoc(ctx, tx, activity); err != nil {
		return "", fmt.Errorf("create end-break activity for durable !resume: %w", err)
	}

	untilExitDuration := timeutil.NoNegativeDuration(until.Sub(now))
	result.Add(usecase.ResumeStarted{
		SeatID:                seat.SeatID,
		IsMemberSeat:          isInMemberRoom,
		RemainingUntilExitMin: int(untilExitDuration.Minutes()),
	})
	return presenter.BuildResumeMessage(result, app.ProcessedUserDisplayName), nil
}
