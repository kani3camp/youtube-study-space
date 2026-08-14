package workspaceapp

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"

	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp/presenter"
	"app.modules/core/workspaceapp/usecase"
)

// buildDurableMoreReplyTx applies !more inside the caller-owned transaction and
// returns the legacy presenter message without posting it externally. The Seat
// update can therefore commit atomically with reply intent + Inbox Processed.
func (app *WorkspaceApp) buildDurableMoreReplyTx(
	ctx context.Context,
	tx *firestore.Transaction,
	moreOption *utils.MoreOption,
) (string, error) {
	if moreOption == nil {
		return "", fmt.Errorf("durable !more option is nil")
	}

	jstNow := app.currentTime()
	var result usecase.Result

	isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
	if err != nil {
		return "", fmt.Errorf("check room state for durable !more: %w", err)
	}
	if !isInMemberRoom && !isInGeneralRoom {
		result.Add(usecase.MoreEnterOnly{})
		return presenter.BuildMoreMessage(result, app.ProcessedUserDisplayName), nil
	}

	currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
	if err != nil {
		return "", fmt.Errorf("read current seat for durable !more: %w", err)
	}
	newSeat := currentSeat
	option := *moreOption

	var addedMin int
	var remainingUntilExitMin int

	switch currentSeat.State {
	case repository.WorkState:
		if option.IsDurationMinSet && option.DurationMin > app.Configs.Constants.MaxWorkTimeMin {
			option.DurationMin = app.Configs.Constants.MaxWorkTimeMin
		}
		if !option.IsDurationMinSet {
			option.DurationMin = app.Configs.Constants.MaxWorkTimeMin
		}

		expectedUntil := currentSeat.Until.Add(time.Duration(option.DurationMin) * time.Minute)
		addedMin, remainingUntilExitMin, err = newSeat.ExtendWorkDuration(
			jstNow,
			option.DurationMin,
			app.Configs.Constants.MaxWorkTimeMin,
		)
		if err != nil {
			return "", fmt.Errorf("extend work duration for durable !more: %w", err)
		}
		if newSeat.Until.Before(expectedUntil) {
			result.Add(usecase.MoreMaxWork{MaxWorkTimeMin: app.Configs.Constants.MaxWorkTimeMin})
		}

	case repository.BreakState:
		if option.IsDurationMinSet && option.DurationMin > app.Configs.Constants.MaxBreakDurationMin {
			option.DurationMin = app.Configs.Constants.MaxBreakDurationMin
		}
		if !option.IsDurationMinSet {
			option.DurationMin = app.Configs.Constants.MaxBreakDurationMin
		}

		expectedBreakUntil := currentSeat.CurrentStateUntil.Add(time.Duration(option.DurationMin) * time.Minute)
		addedMin, _, remainingUntilExitMin, err = newSeat.ExtendBreakDuration(
			jstNow,
			option.DurationMin,
			app.Configs.Constants.MaxBreakDurationMin,
		)
		if err != nil {
			return "", fmt.Errorf("extend break duration for durable !more: %w", err)
		}
		if newSeat.CurrentStateUntil.Before(expectedBreakUntil) {
			result.Add(usecase.MoreMaxBreak{MaxBreakDurationMin: app.Configs.Constants.MaxBreakDurationMin})
		}

	default:
		return "", fmt.Errorf("unknown seat state for durable !more: %q", currentSeat.State)
	}

	if err := app.Repository.UpdateSeat(ctx, tx, newSeat, isInMemberRoom); err != nil {
		return "", fmt.Errorf("update seat for durable !more: %w", err)
	}

	switch currentSeat.State {
	case repository.WorkState:
		result.Add(usecase.MoreWorkExtended{AddedMin: addedMin})
	case repository.BreakState:
		remainingBreakDuration := timeutil.NoNegativeDuration(newSeat.CurrentStateUntil.Sub(jstNow))
		result.Add(usecase.MoreBreakExtended{
			AddedMin:          addedMin,
			RemainingBreakMin: int(remainingBreakDuration.Minutes()),
		})
	}
	realtimeEnteredTimeMin := int(timeutil.NoNegativeDuration(jstNow.Sub(currentSeat.EnteredAt)).Minutes())
	result.Add(usecase.MoreSummary{
		RealtimeEnteredMin:    realtimeEnteredTimeMin,
		RemainingUntilExitMin: remainingUntilExitMin,
	})

	return presenter.BuildMoreMessage(result, app.ProcessedUserDisplayName), nil
}
