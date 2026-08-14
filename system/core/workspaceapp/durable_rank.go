package workspaceapp

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/firestore"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/repository"
	"app.modules/core/utils"
)

// buildDurableRankReplyTx toggles RankVisible inside the caller-owned message
// transaction and returns the legacy reply without posting externally. For a
// first-use user, the caller has only prepared an in-memory UserDoc, so this
// function mutates that document and lets the caller's later CreateUser write
// persist the toggled value atomically with Inbox Processed.
func (app *WorkspaceApp) buildDurableRankReplyTx(
	ctx context.Context,
	tx *firestore.Transaction,
	userDoc *repository.UserDoc,
	userExists bool,
) (string, error) {
	if userDoc == nil {
		return "", errors.New("durable !rank user doc is nil")
	}

	isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
	if err != nil {
		return "", fmt.Errorf("check room state for durable !rank: %w", err)
	}
	isInRoom := isInMemberRoom || isInGeneralRoom
	if isInRoom && !userExists {
		return "", errors.New("unregistered durable !rank user unexpectedly has an active seat")
	}

	var currentSeat repository.SeatDoc
	var realtimeTotalStudySec int
	if isInRoom {
		currentSeat, err = app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
		if err != nil {
			return "", fmt.Errorf("read current seat for durable !rank: %w", err)
		}
		realtimeTotalStudyDuration, _, durationErr := app.GetUserRealtimeTotalStudyDurations(
			ctx,
			tx,
			app.ProcessedUserID,
		)
		if durationErr != nil {
			return "", fmt.Errorf("read realtime study duration for durable !rank: %w", durationErr)
		}
		realtimeTotalStudySec = int(realtimeTotalStudyDuration.Seconds())
	}

	newRankVisible := !userDoc.RankVisible
	if userExists {
		if err := app.Repository.UpdateUserRankVisible(tx, app.ProcessedUserID, newRankVisible); err != nil {
			return "", fmt.Errorf("update rank visibility for durable !rank: %w", err)
		}
	} else {
		userDoc.RankVisible = newRankVisible
	}

	if isInRoom {
		seatAppearance, err := utils.GetSeatAppearance(
			realtimeTotalStudySec,
			newRankVisible,
			userDoc.RankPoint,
			userDoc.FavoriteColor,
		)
		if err != nil {
			return "", fmt.Errorf("calculate seat appearance for durable !rank: %w", err)
		}
		currentSeat.Appearance = seatAppearance
		if err := app.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
			return "", fmt.Errorf("update seat appearance for durable !rank: %w", err)
		}
	}

	newValueStr := i18nmsg.CommonOff()
	if newRankVisible {
		newValueStr = i18nmsg.CommonOn()
	}
	return i18nmsg.CommandRank(app.ProcessedUserDisplayName, newValueStr), nil
}
