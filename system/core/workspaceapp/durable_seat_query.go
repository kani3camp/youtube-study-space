package workspaceapp

import (
	"context"
	"fmt"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp/presenter"
)

// buildDurableSeatInfoReply reproduces the read-only !seat response without
// posting externally. It intentionally performs no domain write so the caller
// can persist the resulting reply intent together with Inbox Processed.
func (app *WorkspaceApp) buildDurableSeatInfoReply(
	ctx context.Context,
	seatOption *utils.SeatOption,
) (string, error) {
	now := app.currentTime()
	isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
	if err != nil {
		return "", fmt.Errorf("check room state for durable !seat: %w", err)
	}
	if !isInMemberRoom && !isInGeneralRoom {
		return i18nmsg.CommandNotEnter(app.ProcessedUserDisplayName, utils.InCommand), nil
	}

	currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
	if err != nil {
		return "", fmt.Errorf("read current seat for durable !seat: %w", err)
	}

	realtimeSittingDurationMin := int(timeutil.NoNegativeDuration(now.Sub(currentSeat.EnteredAt)).Minutes())
	realtimeTotalStudyDurationOfSeat, err := utils.RealTimeTotalStudyDurationOfSeat(currentSeat, now)
	if err != nil {
		return "", fmt.Errorf("calculate realtime seat study duration for durable !seat: %w", err)
	}
	remainingMinutes := currentSeat.RemainingWorkMin(now)

	var stateStr string
	var breakUntilStr string
	switch currentSeat.State {
	case repository.WorkState:
		stateStr = i18nmsg.CommonWork()
	case repository.BreakState:
		stateStr = i18nmsg.CommonBreak()
		breakUntilDuration := timeutil.NoNegativeDuration(currentSeat.CurrentStateUntil.Sub(now))
		breakUntilStr = i18nmsg.CommandSeatInfoBreakUntil(int(breakUntilDuration.Minutes()))
	default:
		return "", fmt.Errorf("unknown seat state for durable !seat: %q", currentSeat.State)
	}

	seatIDStr := presenter.SeatIDStr(currentSeat.SeatID, isInMemberRoom)
	reply := i18nmsg.CommandSeatInfoBase(
		app.ProcessedUserDisplayName,
		seatIDStr,
		stateStr,
		realtimeSittingDurationMin,
		int(realtimeTotalStudyDurationOfSeat.Minutes()),
		remainingMinutes,
		breakUntilStr,
	)

	if seatOption.ShowDetails {
		recentTotalEntryDuration, err := app.GetRecentUserSittingTimeForSeat(
			ctx,
			app.ProcessedUserID,
			currentSeat.SeatID,
			isInMemberRoom,
		)
		if err != nil {
			return "", fmt.Errorf("read recent seat sitting time for durable !seat: %w", err)
		}
		reply += i18nmsg.CommandSeatInfoDetails(
			app.Configs.Constants.RecentRangeMin,
			seatIDStr,
			int(recentTotalEntryDuration.Minutes()),
		)
	}
	return reply, nil
}
