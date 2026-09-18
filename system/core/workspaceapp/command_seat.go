package workspaceapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/firestore"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/repository"
	"app.modules/core/studyspaceerror"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp/presenter"
	"app.modules/core/workspaceapp/usecase"
)

func (app *WorkspaceApp) In(ctx context.Context, inOption *utils.InOption) error {
	jstNow := app.currentTime()
	var replyMessage string
	result := usecase.Result{}
	isTargetMemberSeat := inOption.IsMemberSeat

	if isTargetMemberSeat && !app.ProcessedUserIsMember {
		if app.Configs.Constants.YoutubeMembershipEnabled {
			app.MessageToLiveChat(ctx, i18nmsg.CommandInMemberSeatForbidden(app.ProcessedUserDisplayName))
		} else {
			app.MessageToLiveChat(ctx, i18nmsg.CommandInMembershipDisabled(app.ProcessedUserDisplayName))
		}
		return nil
	}

	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if inOption.IsSeatIDSet {
			if inOption.SeatID == 0 {
				seatID, err := app.MinAvailableSeatIDForUser(ctx, tx, app.ProcessedUserID, isTargetMemberSeat)
				if err != nil {
					return fmt.Errorf("in app.MinAvailableSeatIDForUser(): %w", err)
				}
				inOption.SeatID = seatID
			} else {
				{
					isVacant, err := app.IfSeatVacant(ctx, tx, inOption.SeatID, isTargetMemberSeat)
					if err != nil {
						return fmt.Errorf("in app.IfSeatVacant(): %w", err)
					}
					if !isVacant {
						replyMessage = i18nmsg.CommandInNoSeat(app.ProcessedUserDisplayName, utils.InCommand)
						return nil
					}
				}
				{
					isTooMuch, err := app.CheckIfUserSittingTooMuchForSeat(ctx, app.ProcessedUserID, inOption.SeatID, isTargetMemberSeat)
					if err != nil {
						return fmt.Errorf("in app.CheckIfUserSittingTooMuchForSeat(): %w", err)
					}
					if isTooMuch {
						replyMessage = i18nmsg.CommandInNoAvailability(app.ProcessedUserDisplayName, utils.InCommand)
						return nil
					}
				}
			}
		} else {
			seatID, err := app.RandomAvailableSeatIDForUser(ctx, tx, app.ProcessedUserID, isTargetMemberSeat)
			if err != nil {
				if errors.Is(err, studyspaceerror.ErrNoSeatAvailable) {
					return fmt.Errorf("席数がmax seatに達していて、ユーザーが入室できない事象が発生: %w", err)
				}
				return err
			}
			inOption.SeatID = seatID
		}

		userDoc, err := app.Repository.ReadUser(ctx, tx, app.ProcessedUserID)
		if err != nil {
			return fmt.Errorf("in ReadUser(): %w", err)
		}

		if !inOption.MinWorkOrderOption.IsDurationMinSet {
			if userDoc.DefaultStudyMin == 0 {
				inOption.MinWorkOrderOption.DurationMin = app.Configs.Constants.DefaultWorkTimeMin
			} else {
				inOption.MinWorkOrderOption.DurationMin = userDoc.DefaultStudyMin
			}
		}

		seatAppearance, err := app.GetUserRealtimeSeatAppearance(ctx, tx, app.ProcessedUserID)
		if err != nil {
			return fmt.Errorf("in GetUserRealtimeSeatAppearance(): %w", err)
		}

		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
		if err != nil {
			return fmt.Errorf("in IsUserInRoom(): %w", err)
		}
		isInRoom := isInGeneralRoom || isInMemberRoom
		var currentSeat repository.SeatDoc
		if isInRoom {
			var err error
			currentSeat, err = app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in CurrentSeat(): %w", err)
			}
		}

		var totalOrderCount int64
		var targetMenuItem repository.MenuDoc
		var orderLimitExceeded bool
		if inOption.MinWorkOrderOption.IsOrderSet {
			totalOrderCount, err = app.Repository.CountUserOrdersOfTheDay(ctx, app.ProcessedUserID, jstNow)
			if err != nil {
				return fmt.Errorf("in CountUserOrdersOfTheDay(): %w", err)
			}
			orderLimitExceeded = !app.ProcessedUserIsMember && totalOrderCount >= int64(app.Configs.Constants.MaxDailyOrderCount)

			if !orderLimitExceeded {
				targetMenuItem, err = app.GetMenuItemByNumber(inOption.MinWorkOrderOption.OrderNum)
				if err != nil {
					return fmt.Errorf("in GetMenuItemByNumber(): %w", err)
				}
				if isInRoom {
					currentSeat.SetMenuCode(targetMenuItem.Code)
				}
			}
		}

		var workSegments []repository.WorkSegmentDoc
		if isInRoom && inOption.IsSeatIDSet {
			workSegments, err = app.Repository.ReadWorkStateSegmentsBySessionID(ctx, currentSeat.SessionID)
			if err != nil {
				return fmt.Errorf("in ReadWorkStateSegmentsBySessionID(): %w", err)
			}
		}

		if inOption.MinWorkOrderOption.IsOrderSet {
			if orderLimitExceeded {
				result.Add(usecase.OrderLimitExceeded{MaxDailyOrderCount: app.Configs.Constants.MaxDailyOrderCount})
			} else {
				if isInRoom {
					currentSeat.SetMenuCode(targetMenuItem.Code)
				}
				orderHistoryDoc := repository.OrderHistoryDoc{
					UserID:       app.ProcessedUserID,
					MenuCode:     targetMenuItem.Code,
					SeatID:       inOption.SeatID,
					IsMemberSeat: isTargetMemberSeat,
					OrderedAt:    jstNow,
				}
				if err := app.Repository.CreateOrderHistoryDoc(ctx, tx, orderHistoryDoc); err != nil {
					return fmt.Errorf("in CreateOrderHistoryDoc: %w", err)
				}
				result.Add(usecase.MenuOrdered{MenuName: targetMenuItem.Name, CountAfter: totalOrderCount + 1})
			}
		}

		if isInRoom && inOption.IsSeatIDSet {
			workedTimeSec, addedRP, untilExitMin, err := app.moveSeat(
				ctx,
				tx,
				inOption.SeatID,
				app.ProcessedUserProfileImageURL,
				isInMemberRoom,
				isTargetMemberSeat,
				*inOption.MinWorkOrderOption,
				currentSeat,
				&userDoc,
				workSegments)
			if err != nil {
				return fmt.Errorf("failed to moveSeat for %s (%s): %w", app.ProcessedUserDisplayName, app.ProcessedUserID, err)
			}

			var workName string
			if inOption.MinWorkOrderOption.IsWorkNameSet {
				workName = inOption.MinWorkOrderOption.WorkName
			} else {
				workName = currentSeat.WorkName
			}
			result.Add(usecase.SeatMoved{
				FromSeatID:       currentSeat.SeatID,
				FromIsMemberSeat: isInMemberRoom,
				ToSeatID:         inOption.SeatID,
				ToIsMemberSeat:   isTargetMemberSeat,
				WorkName:         workName,
				WorkedTimeSec:    workedTimeSec,
				AddedRP:          addedRP,
				RankVisible:      userDoc.RankVisible,
				UntilExitMin:     untilExitMin,
			})
		} else if isInRoom && !inOption.IsSeatIDSet {
			seatIDStr := presenter.SeatIDStr(currentSeat.SeatID, isInMemberRoom)
			replyMessage += i18nmsg.CommandInAlreadySeat(app.ProcessedUserDisplayName, seatIDStr)

			if inOption.MinWorkOrderOption.IsWorkNameSet {
				workSegment, err := currentSeat.GenerateWorkSegment(jstNow, isInMemberRoom)
				if err != nil {
					return fmt.Errorf("in GenerateWorkSegment: %w", err)
				}
				if err := app.Repository.CreateWorkSegmentDoc(ctx, tx, workSegment); err != nil {
					return fmt.Errorf("in CreateWorkSegmentDoc: %w", err)
				}
				currentSeat.SetWorkName(inOption.MinWorkOrderOption.WorkName)
				replyMessage += i18nmsg.CommandChangeUpdateWork(inOption.MinWorkOrderOption.WorkName, seatIDStr)
				currentSeat.SetCurrentSegmentStartedAt(jstNow)
			}

			if inOption.MinWorkOrderOption.IsDurationMinSet {
				switch currentSeat.State {
				case repository.WorkState:
					realtimeEntryDurationMin := int(timeutil.NoNegativeDuration(currentSeat.RealtimeEntryDurationMin(jstNow)).Minutes())
					requestedUntil := currentSeat.EnteredAt.Add(time.Duration(inOption.MinWorkOrderOption.DurationMin) * time.Minute)
					if requestedUntil.Before(jstNow) {
						remainingWorkMin := currentSeat.RemainingWorkMin(jstNow)
						replyMessage += i18nmsg.CommandChangeWorkDurationBefore(inOption.MinWorkOrderOption.DurationMin, realtimeEntryDurationMin, remainingWorkMin)
					} else if requestedUntil.After(jstNow.Add(time.Duration(app.Configs.Constants.MaxWorkTimeMin) * time.Minute)) {
						remainingWorkMin := currentSeat.RemainingWorkMin(jstNow)
						replyMessage += i18nmsg.CommandChangeWorkDurationAfter(app.Configs.Constants.MaxWorkTimeMin, realtimeEntryDurationMin, remainingWorkMin)
					} else {
						if err := currentSeat.SetWorkDuration(requestedUntil); err != nil {
							return fmt.Errorf("in SetWorkDuration: %w", err)
						}
						remainingWorkMin := currentSeat.RemainingWorkMin(jstNow)
						replyMessage += i18nmsg.CommandChangeWorkDuration(inOption.MinWorkOrderOption.DurationMin, realtimeEntryDurationMin, remainingWorkMin)
					}
				case repository.BreakState:
					realtimeBreakDuration := timeutil.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt))
					requestedUntil := currentSeat.CurrentStateStartedAt.Add(time.Duration(inOption.MinWorkOrderOption.DurationMin) * time.Minute)
					if requestedUntil.Before(jstNow) {
						remainingBreakDuration := currentSeat.CurrentStateUntil.Sub(jstNow)
						replyMessage += i18nmsg.CommandChangeBreakDurationBefore(inOption.MinWorkOrderOption.DurationMin, int(realtimeBreakDuration.Minutes()), int(remainingBreakDuration.Minutes()))
					} else {
						currentSeat.SetCurrentStateUntil(requestedUntil)
						remainingBreakDuration := requestedUntil.Sub(jstNow)
						replyMessage += i18nmsg.CommandChangeBreakDuration(inOption.MinWorkOrderOption.DurationMin, int(realtimeBreakDuration.Minutes()), int(remainingBreakDuration.Minutes()))
					}
				}
			}

			if err := app.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
				return fmt.Errorf("in UpdateSeat(): %w", err)
			}
		} else {
			untilExitMin, err := app.enterRoom(
				ctx,
				tx,
				app.ProcessedUserID,
				app.ProcessedUserDisplayName,
				app.ProcessedUserProfileImageURL,
				inOption.SeatID,
				isTargetMemberSeat,
				inOption.MinWorkOrderOption.WorkName,
				inOption.MinWorkOrderOption.DurationMin,
				seatAppearance,
				targetMenuItem.Code,
				repository.WorkState,
				userDoc.IsContinuousActive,
				time.Time{},
				time.Time{},
				jstNow)
			if err != nil {
				return fmt.Errorf("in enterRoom(): %w", err)
			}
			result.Add(usecase.SeatEntered{
				SeatID:       inOption.SeatID,
				IsMemberSeat: isTargetMemberSeat,
				WorkName:     inOption.MinWorkOrderOption.WorkName,
				UntilExitMin: untilExitMin,
			})
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in In()", "txErr", txErr)
		replyMessage = i18nmsg.CommandError(app.ProcessedUserDisplayName)
	} else {
		replyMessage += presenter.BuildInMessage(result, app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (app *WorkspaceApp) Out(ctx context.Context) error {
	var replyMessage string
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := app.Repository.ReadUser(ctx, tx, app.ProcessedUserID)
		if err != nil {
			return fmt.Errorf("in ReadUser(): %w", err)
		}
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
		if err != nil {
			return fmt.Errorf("in IsUserInRoom(): %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			if userDoc.LastExited.IsZero() {
				replyMessage = i18nmsg.CommandOutAlreadyExit(app.ProcessedUserDisplayName)
			} else {
				lastExited := userDoc.LastExited.In(timeutil.JapanLocation())
				replyMessage = i18nmsg.CommandOutAlreadyExitWithLastExitTime(app.ProcessedUserDisplayName, lastExited.Hour(), lastExited.Minute())
			}
			return nil
		}
		seat, err := app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("in CurrentSeat(): %w", err)
		}
		workSegments, err := app.Repository.ReadWorkStateSegmentsBySessionID(ctx, seat.SessionID)
		if err != nil {
			return fmt.Errorf("in ReadWorkStateSegmentsBySessionID(): %w", err)
		}
		workedTimeSec, addedRP, err := app.exitRoom(ctx, tx, isInMemberRoom, seat, &userDoc, workSegments)
		if err != nil {
			return fmt.Errorf("in exitRoom(): %w", err)
		}
		var rpEarned string
		if userDoc.RankVisible {
			rpEarned = i18nmsg.CommandRpEarned(addedRP)
		}
		seatIDStr := presenter.SeatIDStr(seat.SeatID, isInMemberRoom)
		replyMessage = i18nmsg.CommandExit(app.ProcessedUserDisplayName, workedTimeSec/60, seatIDStr, rpEarned)
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Out()", "txErr", txErr)
		replyMessage = i18nmsg.CommandError(app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (app *WorkspaceApp) ShowSeatInfo(ctx context.Context, seatOption *utils.SeatOption) error {
	jstNow := app.currentTime()
	showDetails := seatOption.ShowDetails
	var replyMessage string
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
		if err != nil {
			return fmt.Errorf("in IsUserInRoom(): %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if isInRoom {
			currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in app.CurrentSeat(): %w", err)
			}
			realtimeSittingDurationMin := int(timeutil.NoNegativeDuration(jstNow.Sub(currentSeat.EnteredAt)).Minutes())
			realtimeTotalStudyDurationOfSeat, err := utils.RealTimeTotalStudyDurationOfSeat(currentSeat, jstNow)
			if err != nil {
				return fmt.Errorf("in RealTimeTotalStudyDurationOfSeat(): %w", err)
			}
			remainingMinutes := currentSeat.RemainingWorkMin(jstNow)
			var stateStr string
			var breakUntilStr string
			switch currentSeat.State {
			case repository.WorkState:
				stateStr = i18nmsg.CommonWork()
				breakUntilStr = ""
			case repository.BreakState:
				stateStr = i18nmsg.CommonBreak()
				breakUntilDuration := timeutil.NoNegativeDuration(currentSeat.CurrentStateUntil.Sub(jstNow))
				breakUntilStr = i18nmsg.CommandSeatInfoBreakUntil(int(breakUntilDuration.Minutes()))
			}
			seatIDStr := presenter.SeatIDStr(currentSeat.SeatID, isInMemberRoom)
			replyMessage = i18nmsg.CommandSeatInfoBase(app.ProcessedUserDisplayName, seatIDStr, stateStr, realtimeSittingDurationMin, int(realtimeTotalStudyDurationOfSeat.Minutes()), remainingMinutes, breakUntilStr)
			if showDetails {
				recentTotalEntryDuration, err := app.GetRecentUserSittingTimeForSeat(ctx, app.ProcessedUserID, currentSeat.SeatID, isInMemberRoom)
				if err != nil {
					return fmt.Errorf("in GetRecentUserSittingTimeForSeat(): %w", err)
				}
				replyMessage += i18nmsg.CommandSeatInfoDetails(app.Configs.Constants.RecentRangeMin, seatIDStr, int(recentTotalEntryDuration.Minutes()))
			}
		} else {
			replyMessage = i18nmsg.CommandNotEnter(app.ProcessedUserDisplayName, utils.InCommand)
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in ShowSeatInfo()", "txErr", txErr)
		replyMessage = i18nmsg.CommandError(app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (app *WorkspaceApp) Change(ctx context.Context, changeOption *utils.MinWorkOrderOption) error {
	jstNow := app.currentTime()
	replyMessage := ""
	var result usecase.Result
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			result.Add(usecase.ChangeValidationError{Message: i18nmsg.CommandEnterOnly()})
			return nil
		}
		currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed app.CurrentSeat(): %w", err)
		}
		if err := app.ValidateChange(*changeOption, currentSeat.State); err != nil {
			result.Add(usecase.ChangeValidationError{Message: err.Error()})
			return nil
		}

		if changeOption.IsWorkNameSet {
			workSegment, err := currentSeat.GenerateWorkSegment(jstNow, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in GenerateWorkSegment: %w", err)
			}
			if err := app.Repository.CreateWorkSegmentDoc(ctx, tx, workSegment); err != nil {
				return fmt.Errorf("in CreateWorkSegmentDoc: %w", err)
			}
			currentSeat.SetCurrentSegmentStartedAt(jstNow)
			currentSeat.SetWorkName(changeOption.WorkName)
			result.Add(usecase.ChangeUpdatedWork{WorkName: changeOption.WorkName, SeatID: currentSeat.SeatID, IsMemberSeat: isInMemberRoom})
		}
		if changeOption.IsDurationMinSet {
			switch currentSeat.State {
			case repository.WorkState:
				realtimeEntryDurationMin := int(timeutil.NoNegativeDuration(jstNow.Sub(currentSeat.EnteredAt)).Minutes())
				requestedUntil := currentSeat.EnteredAt.Add(time.Duration(changeOption.DurationMin) * time.Minute)
				if requestedUntil.Before(jstNow) {
					remainingWorkMin := currentSeat.RemainingWorkMin(jstNow)
					result.Add(usecase.ChangeWorkDurationRejectedBefore{RequestedMin: changeOption.DurationMin, RealtimeEntryDurationMin: realtimeEntryDurationMin, RemainingWorkMin: remainingWorkMin})
				} else if requestedUntil.After(jstNow.Add(time.Duration(app.Configs.Constants.MaxWorkTimeMin) * time.Minute)) {
					remainingWorkMin := currentSeat.RemainingWorkMin(jstNow)
					result.Add(usecase.ChangeWorkDurationRejectedAfter{MaxWorkTimeMin: app.Configs.Constants.MaxWorkTimeMin, RealtimeEntryDurationMin: realtimeEntryDurationMin, RemainingWorkMin: remainingWorkMin})
				} else {
					if err := currentSeat.SetWorkDuration(requestedUntil); err != nil {
						return fmt.Errorf("in SetWorkDuration: %w", err)
					}
					remainingWorkMin := currentSeat.RemainingWorkMin(jstNow)
					result.Add(usecase.ChangeWorkDurationUpdated{RequestedMin: changeOption.DurationMin, RealtimeEntryDurationMin: realtimeEntryDurationMin, RemainingWorkMin: remainingWorkMin})
				}
			case repository.BreakState:
				realtimeBreakDuration := timeutil.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt))
				requestedUntil := currentSeat.CurrentStateStartedAt.Add(time.Duration(changeOption.DurationMin) * time.Minute)
				if requestedUntil.Before(jstNow) {
					remainingBreakDuration := currentSeat.CurrentStateUntil.Sub(jstNow)
					result.Add(usecase.ChangeBreakDurationRejectedBefore{RequestedMin: changeOption.DurationMin, RealtimeBreakDurationMin: int(realtimeBreakDuration.Minutes()), RemainingBreakMin: int(remainingBreakDuration.Minutes())})
				} else {
					currentSeat.SetCurrentStateUntil(requestedUntil)
					remainingBreakDuration := requestedUntil.Sub(jstNow)
					result.Add(usecase.ChangeBreakDurationUpdated{RequestedMin: changeOption.DurationMin, RealtimeBreakDurationMin: int(realtimeBreakDuration.Minutes()), RemainingBreakMin: int(remainingBreakDuration.Minutes())})
				}
			}
		}
		if err := app.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in UpdateSeat: %w", err)
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Change()", "txErr", txErr)
		replyMessage = i18nmsg.CommandError(app.ProcessedUserDisplayName)
	}
	if txErr == nil {
		replyMessage = presenter.BuildChangeMessage(result, app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (app *WorkspaceApp) More(ctx context.Context, moreOption *utils.MoreOption) error {
	replyMessage := ""
	var result usecase.Result
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		jstNow := app.currentTime()
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			result.Add(usecase.MoreEnterOnly{})
			return nil
		}
		currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed app.CurrentSeat(): %w", err)
		}
		newSeat := &currentSeat
		var addedMin int
		var remainingUntilExitMin int
		switch currentSeat.State {
		case repository.WorkState:
			if moreOption.IsDurationMinSet && moreOption.DurationMin > app.Configs.Constants.MaxWorkTimeMin {
				moreOption.DurationMin = app.Configs.Constants.MaxWorkTimeMin
			}
			if !moreOption.IsDurationMinSet {
				moreOption.DurationMin = app.Configs.Constants.MaxWorkTimeMin
			}
			expectedUntil := currentSeat.Until.Add(time.Duration(moreOption.DurationMin) * time.Minute)
			var err error
			addedMin, remainingUntilExitMin, err = newSeat.ExtendWorkDuration(jstNow, moreOption.DurationMin, app.Configs.Constants.MaxWorkTimeMin)
			if err != nil {
				return fmt.Errorf("in ExtendWorkDuration: %w", err)
			}
			if newSeat.Until.Before(expectedUntil) {
				result.Add(usecase.MoreMaxWork{MaxWorkTimeMin: app.Configs.Constants.MaxWorkTimeMin})
			}
		case repository.BreakState:
			if moreOption.IsDurationMinSet && moreOption.DurationMin > app.Configs.Constants.MaxBreakDurationMin {
				moreOption.DurationMin = app.Configs.Constants.MaxBreakDurationMin
			}
			if !moreOption.IsDurationMinSet {
				moreOption.DurationMin = app.Configs.Constants.MaxBreakDurationMin
			}
			expectedBreakUntil := currentSeat.CurrentStateUntil.Add(time.Duration(moreOption.DurationMin) * time.Minute)
			var err error
			addedMin, _, remainingUntilExitMin, err = newSeat.ExtendBreakDuration(jstNow, moreOption.DurationMin, app.Configs.Constants.MaxBreakDurationMin)
			if err != nil {
				return fmt.Errorf("in ExtendBreakDuration: %w", err)
			}
			if newSeat.CurrentStateUntil.Before(expectedBreakUntil) {
				result.Add(usecase.MoreMaxBreak{MaxBreakDurationMin: app.Configs.Constants.MaxBreakDurationMin})
			}
		}
		if err := app.Repository.UpdateSeat(ctx, tx, *newSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in app.Repository.UpdateSeat: %w", err)
		}
		switch currentSeat.State {
		case repository.WorkState:
			result.Add(usecase.MoreWorkExtended{AddedMin: addedMin})
		case repository.BreakState:
			remainingBreakDuration := timeutil.NoNegativeDuration(newSeat.CurrentStateUntil.Sub(jstNow))
			result.Add(usecase.MoreBreakExtended{AddedMin: addedMin, RemainingBreakMin: int(remainingBreakDuration.Minutes())})
		}
		realtimeEnteredTimeMin := int(timeutil.NoNegativeDuration(jstNow.Sub(currentSeat.EnteredAt)).Minutes())
		result.Add(usecase.MoreSummary{RealtimeEnteredMin: realtimeEnteredTimeMin, RemainingUntilExitMin: remainingUntilExitMin})
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in More()", "txErr", txErr)
		replyMessage = i18nmsg.CommandError(app.ProcessedUserDisplayName)
	}
	if txErr == nil {
		replyMessage = presenter.BuildMoreMessage(result, app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (app *WorkspaceApp) Break(ctx context.Context, breakOption *utils.MinWorkOrderOption) error {
	replyMessage := ""
	var result usecase.Result
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		jstNow := app.currentTime()
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			result.Add(usecase.BreakEnterOnly{})
			return nil
		}
		currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed app.CurrentSeat(): %w", err)
		}
		if currentSeat.State != repository.WorkState {
			result.Add(usecase.BreakWorkOnly{})
			return nil
		}
		currentWorkedMin := int(timeutil.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt)).Minutes())
		if currentWorkedMin < app.Configs.Constants.MinBreakIntervalMin {
			result.Add(usecase.BreakWarn{MinBreakIntervalMin: app.Configs.Constants.MinBreakIntervalMin, CurrentWorkedMin: currentWorkedMin})
			return nil
		}
		if !breakOption.IsDurationMinSet {
			breakOption.DurationMin = app.Configs.Constants.DefaultBreakDurationMin
		}
		{
			workSegment, err := currentSeat.GenerateWorkSegment(jstNow, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in GenerateWorkSegment: %w", err)
			}
			if err := app.Repository.CreateWorkSegmentDoc(ctx, tx, workSegment); err != nil {
				return fmt.Errorf("in CreateWorkSegmentDoc: %w", err)
			}
		}
		if err := currentSeat.StartBreak(jstNow, breakOption.DurationMin); err != nil {
			return fmt.Errorf("in StartBreak: %w", err)
		}
		if err := app.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in app.Repository.UpdateSeat: %w", err)
		}
		startBreakActivity := repository.UserActivityDoc{UserID: app.ProcessedUserID, ActivityType: repository.StartBreakActivity, SeatID: currentSeat.SeatID, IsMemberSeat: isInMemberRoom, TakenAt: jstNow}
		if err := app.Repository.CreateUserActivityDoc(ctx, tx, startBreakActivity); err != nil {
			return fmt.Errorf("in CreateUserActivityDoc: %w", err)
		}
		result.Add(usecase.BreakStarted{SeatID: currentSeat.SeatID, IsMemberSeat: isInMemberRoom, DurationMin: breakOption.DurationMin})
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Break()", "txErr", txErr)
		replyMessage = i18nmsg.CommandError(app.ProcessedUserDisplayName)
	}
	if txErr == nil {
		replyMessage = presenter.BuildBreakMessage(result, app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (app *WorkspaceApp) Resume(ctx context.Context, resumeOption *utils.WorkNameOption) error {
	replyMessage := ""
	var result usecase.Result
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			result.Add(usecase.ResumeEnterOnly{})
			return nil
		}
		currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed app.CurrentSeat(): %w", err)
		}
		if currentSeat.State != repository.BreakState {
			result.Add(usecase.ResumeBreakOnly{})
			return nil
		}
		jstNow := app.currentTime()
		until := currentSeat.Until
		{
			breakSegment, err := currentSeat.GenerateWorkSegment(jstNow, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in GenerateWorkSegment: %w", err)
			}
			if err := app.Repository.CreateWorkSegmentDoc(ctx, tx, breakSegment); err != nil {
				return fmt.Errorf("in CreateWorkSegmentDoc: %w", err)
			}
		}
		workName := resumeOption.WorkName
		if !resumeOption.IsWorkNameSet {
			workName = currentSeat.WorkName
		}
		if err := currentSeat.ResumeWork(jstNow, workName); err != nil {
			return fmt.Errorf("in ResumeWork: %w", err)
		}
		if err := app.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in app.Repository.UpdateSeat: %w", err)
		}
		endBreakActivity := repository.UserActivityDoc{UserID: app.ProcessedUserID, ActivityType: repository.EndBreakActivity, SeatID: currentSeat.SeatID, IsMemberSeat: isInMemberRoom, TakenAt: jstNow}
		if err := app.Repository.CreateUserActivityDoc(ctx, tx, endBreakActivity); err != nil {
			return fmt.Errorf("in CreateUserActivityDoc: %w", err)
		}
		untilExitDuration := timeutil.NoNegativeDuration(until.Sub(jstNow))
		result.Add(usecase.ResumeStarted{SeatID: currentSeat.SeatID, IsMemberSeat: isInMemberRoom, RemainingUntilExitMin: int(untilExitDuration.Minutes())})
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Resume()", "txErr", txErr)
		replyMessage = i18nmsg.CommandError(app.ProcessedUserDisplayName)
	}
	if txErr == nil {
		replyMessage = presenter.BuildResumeMessage(result, app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (app *WorkspaceApp) Order(ctx context.Context, orderOption *utils.OrderOption) error {
	replyMessage := ""
	var result usecase.Result
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		jstNow := app.currentTime()
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			result.Add(usecase.OrderEnterOnly{})
			return nil
		}
		todayOrderCount, err := app.Repository.CountUserOrdersOfTheDay(ctx, app.ProcessedUserID, jstNow)
		if err != nil {
			return fmt.Errorf("in CountUserOrdersOfTheDay: %w", err)
		}
		if !app.ProcessedUserIsMember && !orderOption.ClearFlag && todayOrderCount >= int64(app.Configs.Constants.MaxDailyOrderCount) {
			result.Add(usecase.OrderTooMany{MaxDailyOrderCount: app.Configs.Constants.MaxDailyOrderCount})
			return nil
		}
		currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed app.CurrentSeat(): %w", err)
		}
		if orderOption.ClearFlag {
			currentSeat.ClearMenuCode()
			if err := app.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
				return fmt.Errorf("in UpdateSeat: %w", err)
			}
			result.Add(usecase.OrderCleared{})
			return nil
		}
		targetMenuItem, err := app.GetMenuItemByNumber(orderOption.IntValue)
		if err != nil {
			return fmt.Errorf("in GetMenuItemByNumber: %w", err)
		}
		orderHistoryDoc := repository.OrderHistoryDoc{UserID: app.ProcessedUserID, MenuCode: targetMenuItem.Code, SeatID: currentSeat.SeatID, IsMemberSeat: isInMemberRoom, OrderedAt: jstNow}
		if err := app.Repository.CreateOrderHistoryDoc(ctx, tx, orderHistoryDoc); err != nil {
			return fmt.Errorf("in CreateOrderHistoryDoc: %w", err)
		}
		currentSeat.SetMenuCode(targetMenuItem.Code)
		if err := app.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in UpdateSeat: %w", err)
		}
		result.Add(usecase.OrderOrdered{MenuName: targetMenuItem.Name, CountAfter: todayOrderCount + 1})
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Order()", "txErr", txErr)
		replyMessage = i18nmsg.CommandError(app.ProcessedUserDisplayName)
	}
	if txErr == nil {
		replyMessage = presenter.BuildOrderMessage(result, app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (app *WorkspaceApp) Clear(ctx context.Context) error {
	jstNow := app.currentTime()
	replyMessage := ""
	var result usecase.Result
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserID)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			result.Add(usecase.ClearEnterOnly{})
			return nil
		}
		seat, err := app.CurrentSeat(ctx, app.ProcessedUserID, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed app.CurrentSeat(): %w", err)
		}
		workSegment, err := seat.GenerateWorkSegment(jstNow, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("in GenerateWorkSegment: %w", err)
		}
		if err := app.Repository.CreateWorkSegmentDoc(ctx, tx, workSegment); err != nil {
			return fmt.Errorf("in CreateWorkSegmentDoc: %w", err)
		}
		seat.ClearWorkName()
		result.Add(usecase.ClearWork{SeatID: seat.SeatID, IsMemberSeat: isInMemberRoom})
		seat.SetCurrentSegmentStartedAt(jstNow)
		if err := app.Repository.UpdateSeat(ctx, tx, seat, isInMemberRoom); err != nil {
			return fmt.Errorf("in UpdateSeat: %w", err)
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Clear()", "txErr", txErr)
		replyMessage = i18nmsg.CommandError(app.ProcessedUserDisplayName)
	}
	if txErr == nil {
		replyMessage = presenter.BuildClearMessage(result, app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}
