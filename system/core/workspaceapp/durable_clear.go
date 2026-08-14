package workspaceapp

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/firestore"

	"app.modules/core/repository"
	"app.modules/core/workspaceapp/presenter"
	"app.modules/core/workspaceapp/usecase"
)

// buildDurableClearReplyTx applies !clear inside the caller-owned message
// transaction and returns the legacy presenter reply without posting it. The
// generated WorkSegment and Seat update can therefore commit together with the
// reply intent and Inbox Processed marker.
func (app *WorkspaceApp) buildDurableClearReplyTx(
	ctx context.Context,
	tx *firestore.Transaction,
	userExists bool,
) (string, error) {
	now := app.currentTime()
	var result usecase.Result

	isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
	if err != nil {
		return "", fmt.Errorf("check room state for durable !clear: %w", err)
	}
	isInRoom := isInMemberRoom || isInGeneralRoom
	if !isInRoom {
		result.Add(usecase.ClearEnterOnly{})
		return presenter.BuildClearMessage(result, app.ProcessedUserDisplayName), nil
	}
	if !userExists {
		return "", errors.New("unregistered durable !clear user unexpectedly has an active seat")
	}

	seat, err := app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
	if err != nil {
		return "", fmt.Errorf("read current seat for durable !clear: %w", err)
	}

	// All reads are complete. Segment log + Seat mutation are staged in the
	// caller-owned transaction from this point forward.
	workSegment, err := seat.GenerateWorkSegment(now, isInMemberRoom)
	if err != nil {
		return "", fmt.Errorf("generate work segment for durable !clear: %w", err)
	}
	if err := app.Repository.CreateWorkSegmentDoc(ctx, tx, workSegment); err != nil {
		return "", fmt.Errorf("create work segment for durable !clear: %w", err)
	}

	switch seat.State {
	case repository.WorkState:
		seat.ClearWorkName()
		result.Add(usecase.ClearWork{SeatID: seat.SeatID, IsMemberSeat: isInMemberRoom})
	case repository.BreakState:
		seat.ClearBreakWorkName()
		result.Add(usecase.ClearBreak{SeatID: seat.SeatID, IsMemberSeat: isInMemberRoom})
	default:
		return "", fmt.Errorf("unknown seat state for durable !clear: %q", seat.State)
	}
	seat.SetCurrentSegmentStartedAt(now)

	if err := app.Repository.UpdateSeat(ctx, tx, seat, isInMemberRoom); err != nil {
		return "", fmt.Errorf("update seat for durable !clear: %w", err)
	}
	return presenter.BuildClearMessage(result, app.ProcessedUserDisplayName), nil
}
