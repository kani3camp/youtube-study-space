package workspaceapp

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/repository"
	"app.modules/core/studyspaceerror"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp/presenter"
	"app.modules/core/workspaceapp/usecase"
	"cloud.google.com/go/firestore"
	"github.com/pkg/errors"
)

func (app *WorkspaceApp) In(ctx context.Context, inOption *utils.InOption) error {
	jstNow := utils.JstNow()
	var replyMessage string
	result := usecase.Result{}
	// no-op
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
		// order系イベントは最後に追加してメッセージ順を旧実装と合わせる
		var orderEvents []usecase.Event
		// 席が指定されているか？
		if inOption.IsSeatIdSet {
			// 0番席だったら最小番号の空席に決定
			if inOption.SeatId == 0 {
				seatId, err := app.MinAvailableSeatIdForUser(ctx, tx, app.ProcessedUserId, isTargetMemberSeat)
				if err != nil {
					return fmt.Errorf("in app.MinAvailableSeatIdForUser(): %w", err)
				}
				inOption.SeatId = seatId
			} else {
				// その席が空いているか？
				{
					isVacant, err := app.IfSeatVacant(ctx, tx, inOption.SeatId, isTargetMemberSeat)
					if err != nil {
						return fmt.Errorf("in app.IfSeatVacant(): %w", err)
					}
					if !isVacant {
						replyMessage = i18nmsg.CommandInNoSeat(app.ProcessedUserDisplayName, utils.InCommand)
						return nil
					}
				}
				// ユーザーはその席に対して入室制限を受けてないか？
				{
					isTooMuch, err := app.CheckIfUserSittingTooMuchForSeat(ctx, app.ProcessedUserId, inOption.SeatId, isTargetMemberSeat)
					if err != nil {
						return fmt.Errorf("in app.CheckIfUserSittingTooMuchForSeat(): %w", err)
					}
					if isTooMuch {
						replyMessage = i18nmsg.CommandInNoAvailability(app.ProcessedUserDisplayName, utils.InCommand)
						return nil
					}
				}
			}
		} else { // 席の指定なし
			seatId, err := app.RandomAvailableSeatIdForUser(ctx, tx, app.ProcessedUserId, isTargetMemberSeat)
			if err != nil {
				if errors.Is(err, studyspaceerror.ErrNoSeatAvailable) {
					return fmt.Errorf("席数がmax seatに達していて、ユーザーが入室できない事象が発生: %w", err)
				}
				return err
			}
			inOption.SeatId = seatId
		}

		userDoc, err := app.Repository.ReadUser(ctx, tx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in ReadUser(): %w", err)
		}

		// 作業時間が指定されているか？
		if !inOption.MinWorkOrderOption.IsDurationMinSet {
			if userDoc.DefaultStudyMin == 0 {
				inOption.MinWorkOrderOption.DurationMin = app.Configs.Constants.DefaultWorkTimeMin
			} else {
				inOption.MinWorkOrderOption.DurationMin = userDoc.DefaultStudyMin
			}
		}

		// ランクから席の色を決定
		seatAppearance, err := app.GetUserRealtimeSeatAppearance(ctx, tx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in GetUserRealtimeSeatAppearance(): %w", err)
		}

		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in IsUserInRoom(): %w", err)
		}
		isInRoom := isInGeneralRoom || isInMemberRoom
		var currentSeat repository.SeatDoc
		if isInRoom { // 現在座っている席を取得
			var err error
			currentSeat, err = app.CurrentSeat(ctx, app.ProcessedUserId, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in CurrentSeat(): %w", err)
			}
		}

		var totalOrderCount int64
		var targetMenuItem repository.MenuDoc
		var orderLimitExceeded bool
		if inOption.MinWorkOrderOption.IsOrderSet {
			// メンバーでない場合は、本日の注文回数をチェック
			totalOrderCount, err = app.Repository.CountUserOrdersOfTheDay(ctx, app.ProcessedUserId, jstNow)
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
					currentSeat.MenuCode = targetMenuItem.Code
				}
			}
		}

		// =========== 以降は書き込み処理のみ ===========

		// メニュー注文されている場合は、メニューコードをセット（イベント化）
		if inOption.MinWorkOrderOption.IsOrderSet {
			if orderLimitExceeded {
				orderEvents = append(orderEvents, usecase.OrderLimitExceeded{MaxDailyOrderCount: app.Configs.Constants.MaxDailyOrderCount})
			} else {
				if isInRoom {
					currentSeat.MenuCode = targetMenuItem.Code
				}

				// 注文履歴を作成
				orderHistoryDoc := repository.OrderHistoryDoc{
					UserId:       app.ProcessedUserId,
					MenuCode:     targetMenuItem.Code,
					SeatId:       inOption.SeatId,
					IsMemberSeat: isTargetMemberSeat,
					OrderedAt:    jstNow,
				}
				if err := app.Repository.CreateOrderHistoryDoc(ctx, tx, orderHistoryDoc); err != nil {
					return fmt.Errorf("in CreateOrderHistoryDoc: %w", err)
				}

				orderEvents = append(orderEvents, usecase.MenuOrdered{MenuName: targetMenuItem.Name, CountAfter: totalOrderCount + 1})
			}
		}

		if isInRoom && inOption.IsSeatIdSet { // 入室中で、席指定があれば、席移動処理
			workedTimeSec, addedRP, untilExitMin, err := app.moveSeat(
				ctx,
				tx,
				inOption.SeatId,
				app.ProcessedUserProfileImageUrl,
				isInMemberRoom,
				isTargetMemberSeat,
				*inOption.MinWorkOrderOption,
				currentSeat,
				&userDoc)
			if err != nil {
				return fmt.Errorf("failed to moveSeat for %s (%s): %w", app.ProcessedUserDisplayName, app.ProcessedUserId, err)
			}

			var workName string
			if inOption.MinWorkOrderOption.IsWorkNameSet {
				workName = inOption.MinWorkOrderOption.WorkName
			} else {
				workName = currentSeat.WorkName
			}
			result.Add(usecase.SeatMoved{
				FromSeatID:       currentSeat.SeatId,
				FromIsMemberSeat: isInMemberRoom,
				ToSeatID:         inOption.SeatId,
				ToIsMemberSeat:   isTargetMemberSeat,
				WorkName:         workName,
				WorkedTimeSec:    workedTimeSec,
				AddedRP:          addedRP,
				RankVisible:      userDoc.RankVisible,
				UntilExitMin:     untilExitMin,
			})
		} else if isInRoom && !inOption.IsSeatIdSet { // 入室中で、席指定がない場合は、指定があったオプションのみ更新処理（席移動なし）
			var seatIdStr string
			if isInMemberRoom {
				seatIdStr = i18nmsg.CommonVipSeatId(currentSeat.SeatId)
			} else {
				seatIdStr = strconv.Itoa(currentSeat.SeatId)
			}
			replyMessage += i18nmsg.CommandInAlreadySeat(app.ProcessedUserDisplayName, seatIdStr)

			if inOption.MinWorkOrderOption.IsWorkNameSet {
				switch currentSeat.State {
				case repository.WorkState:
					currentSeat.WorkName = inOption.MinWorkOrderOption.WorkName
					replyMessage += i18nmsg.CommandChangeUpdateWork(inOption.MinWorkOrderOption.WorkName, seatIdStr)
				case repository.BreakState:
					currentSeat.BreakWorkName = inOption.MinWorkOrderOption.WorkName
					replyMessage += i18nmsg.CommandChangeUpdateBreak(inOption.MinWorkOrderOption.WorkName, seatIdStr)
				}
			}

			if inOption.MinWorkOrderOption.IsDurationMinSet {
				switch currentSeat.State {
				case repository.WorkState:
					// 作業時間を（入室時間から自動退室までの時間）を変更
					realtimeEntryDurationMin := int(utils.NoNegativeDuration(currentSeat.RealtimeEntryDurationMin(jstNow)).Minutes())
					requestedUntil := currentSeat.EnteredAt.Add(time.Duration(inOption.MinWorkOrderOption.DurationMin) * time.Minute)

					if requestedUntil.Before(jstNow) {
						// もし現在時刻が指定時間を経過していたら却下
						remainingWorkMin := int(currentSeat.Until.Sub(jstNow).Minutes())
						replyMessage += i18nmsg.CommandChangeWorkDurationBefore(inOption.MinWorkOrderOption.DurationMin, realtimeEntryDurationMin, remainingWorkMin)
					} else if requestedUntil.After(jstNow.Add(time.Duration(app.Configs.Constants.MaxWorkTimeMin) * time.Minute)) {
						// もし現在時刻より最大延長可能時間以上後なら却下
						remainingWorkMin := int(currentSeat.Until.Sub(jstNow).Minutes())
						replyMessage += i18nmsg.CommandChangeWorkDurationAfter(app.Configs.Constants.MaxWorkTimeMin, realtimeEntryDurationMin, remainingWorkMin)
					} else { // それ以外なら延長
						currentSeat.Until = requestedUntil
						currentSeat.CurrentStateUntil = requestedUntil
						remainingWorkMin := int(utils.NoNegativeDuration(requestedUntil.Sub(jstNow)).Minutes())
						replyMessage += i18nmsg.CommandChangeWorkDuration(inOption.MinWorkOrderOption.DurationMin, realtimeEntryDurationMin, remainingWorkMin)
					}
				case repository.BreakState:
					// 休憩時間を変更
					realtimeBreakDuration := utils.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt))
					requestedUntil := currentSeat.CurrentStateStartedAt.Add(time.Duration(inOption.MinWorkOrderOption.DurationMin) * time.Minute)

					if requestedUntil.Before(jstNow) {
						// もし現在時刻が指定時間を経過していたら却下
						remainingBreakDuration := currentSeat.CurrentStateUntil.Sub(jstNow)
						replyMessage += i18nmsg.CommandChangeBreakDurationBefore(inOption.MinWorkOrderOption.DurationMin, int(realtimeBreakDuration.Minutes()), int(remainingBreakDuration.Minutes()))
					} else { // それ以外ならuntilを変更
						currentSeat.CurrentStateUntil = requestedUntil
						remainingBreakDuration := requestedUntil.Sub(jstNow)
						replyMessage += i18nmsg.CommandChangeBreakDuration(inOption.MinWorkOrderOption.DurationMin, int(realtimeBreakDuration.Minutes()), int(remainingBreakDuration.Minutes()))
					}
				}
			}

			if err := app.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
				return fmt.Errorf("in UpdateSeat(): %w", err)
			}
		} else { // 入室のみ
			untilExitMin, err := app.enterRoom(
				ctx,
				tx,
				app.ProcessedUserId,
				app.ProcessedUserDisplayName,
				app.ProcessedUserProfileImageUrl,
				inOption.SeatId,
				isTargetMemberSeat,
				inOption.MinWorkOrderOption.WorkName,
				"",
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
			// イベント積む（入室）
			result.Add(usecase.SeatEntered{
				SeatID:       inOption.SeatId,
				IsMemberSeat: isTargetMemberSeat,
				WorkName:     inOption.MinWorkOrderOption.WorkName,
				UntilExitMin: untilExitMin,
			})
		}
		// 旧実装の順序に合わせて最後にorderイベントを追加
		for _, event := range orderEvents {
			result.Add(event)
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in In()", "txErr", txErr)
		replyMessage = i18nmsg.CommandError(app.ProcessedUserDisplayName)
	}
	if txErr == nil {
		// イベントから返信文をTx外で組み立てる
		replyMessage += presenter.BuildInMessage(result, app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (app *WorkspaceApp) Out(ctx context.Context) error {
	// no-op
	var replyMessage string
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := app.Repository.ReadUser(ctx, tx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in ReadUser(): %w", err)
		}

		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in IsUserInRoom(): %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			if userDoc.LastExited.IsZero() {
				replyMessage = i18nmsg.CommandOutAlreadyExit(app.ProcessedUserDisplayName)
			} else {
				lastExited := userDoc.LastExited.In(utils.JapanLocation())
				replyMessage = i18nmsg.CommandOutAlreadyExitWithLastExitTime(app.ProcessedUserDisplayName, lastExited.Hour(), lastExited.Minute())
			}
			return nil
		}

		// 現在座っている席を特定
		seat, err := app.CurrentSeat(ctx, app.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("in CurrentSeat(): %w", err)
		}

		// 退室処理
		workedTimeSec, addedRP, err := app.exitRoom(ctx, tx, isInMemberRoom, seat, &userDoc)
		if err != nil {
			return fmt.Errorf("in exitRoom(): %w", err)
		}
		var rpEarned string
		var seatIdStr string
		if userDoc.RankVisible {
			rpEarned = i18nmsg.CommandRpEarned(addedRP)
		}
		if isInMemberRoom {
			seatIdStr = i18nmsg.CommonVipSeatId(seat.SeatId)
		} else {
			seatIdStr = strconv.Itoa(seat.SeatId)
		}
		replyMessage = i18nmsg.CommandExit(app.ProcessedUserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)
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
	// no-op
	showDetails := seatOption.ShowDetails
	var replyMessage string
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// そのユーザーは入室しているか？
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in IsUserInRoom(): %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if isInRoom {
			currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserId, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in app.CurrentSeat(): %w", err)
			}

			realtimeSittingDurationMin := int(utils.NoNegativeDuration(utils.JstNow().Sub(currentSeat.EnteredAt)).Minutes())
			realtimeTotalStudyDurationOfSeat, err := utils.RealTimeTotalStudyDurationOfSeat(currentSeat, utils.JstNow())
			if err != nil {
				return fmt.Errorf("in RealTimeTotalStudyDurationOfSeat(): %w", err)
			}
			remainingMinutes := int(utils.NoNegativeDuration(currentSeat.Until.Sub(utils.JstNow())).Minutes())
			var stateStr string
			var breakUntilStr string
			switch currentSeat.State {
			case repository.WorkState:
				stateStr = i18nmsg.CommonWork()
				breakUntilStr = ""
			case repository.BreakState:
				stateStr = i18nmsg.CommonBreak()
				breakUntilDuration := utils.NoNegativeDuration(currentSeat.CurrentStateUntil.Sub(utils.JstNow()))
				breakUntilStr = i18nmsg.CommandSeatInfoBreakUntil(int(breakUntilDuration.Minutes()))
			}
			var seatIdStr string
			if isInMemberRoom {
				seatIdStr = i18nmsg.CommonVipSeatId(currentSeat.SeatId)
			} else {
				seatIdStr = strconv.Itoa(currentSeat.SeatId)
			}
			replyMessage = i18nmsg.CommandSeatInfoBase(app.ProcessedUserDisplayName, seatIdStr, stateStr, realtimeSittingDurationMin, int(realtimeTotalStudyDurationOfSeat.Minutes()), remainingMinutes, breakUntilStr)

			if showDetails {
				recentTotalEntryDuration, err := app.GetRecentUserSittingTimeForSeat(ctx, app.ProcessedUserId, currentSeat.SeatId, isInMemberRoom)
				if err != nil {
					return fmt.Errorf("in GetRecentUserSittingTimeForSeat(): %w", err)
				}
				replyMessage += i18nmsg.CommandSeatInfoDetails(app.Configs.Constants.RecentRangeMin, seatIdStr, int(recentTotalEntryDuration.Minutes()))
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
	jstNow := utils.JstNow()
	replyMessage := ""
	var result usecase.Result
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// そのユーザーは入室中か？
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			result.Add(usecase.ChangeValidationError{
				Message: i18nmsg.CommandEnterOnly(),
			})
			return nil
		}

		currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed app.CurrentSeat(): %w", err)
		}

		// validation
		if err := app.ValidateChange(*changeOption, currentSeat.State); err != nil {
			result.Add(usecase.ChangeValidationError{Message: err.Error()})
			return nil
		}

		// これ以降は書き込みのみ可。イベントを積む

		newSeat := &currentSeat
		if changeOption.IsWorkNameSet { // 作業名もしくは休憩作業名を書きかえ
			switch currentSeat.State {
			case repository.WorkState:
				newSeat.WorkName = changeOption.WorkName
				result.Add(usecase.ChangeUpdatedWork{
					WorkName:     changeOption.WorkName,
					SeatID:       currentSeat.SeatId,
					IsMemberSeat: isInMemberRoom,
				})
			case repository.BreakState:
				newSeat.BreakWorkName = changeOption.WorkName
				result.Add(usecase.ChangeUpdatedBreak{
					WorkName:     changeOption.WorkName,
					SeatID:       currentSeat.SeatId,
					IsMemberSeat: isInMemberRoom,
				})
			}
		}
		if changeOption.IsDurationMinSet {
			switch currentSeat.State {
			case repository.WorkState:
				// 作業時間（入室時間から自動退室までの時間）を変更
				realtimeEntryDurationMin := int(utils.NoNegativeDuration(jstNow.Sub(currentSeat.EnteredAt)).Minutes())
				requestedUntil := currentSeat.EnteredAt.Add(time.Duration(changeOption.DurationMin) * time.Minute)

				if requestedUntil.Before(jstNow) {
					// もし現在時刻が指定時間を経過していたら却下
					remainingWorkMin := int(currentSeat.Until.Sub(jstNow).Minutes())
					result.Add(usecase.ChangeWorkDurationRejectedBefore{
						RequestedMin:             changeOption.DurationMin,
						RealtimeEntryDurationMin: realtimeEntryDurationMin,
						RemainingWorkMin:         remainingWorkMin,
					})
				} else if requestedUntil.After(jstNow.Add(time.Duration(app.Configs.Constants.MaxWorkTimeMin) * time.Minute)) {
					// もし現在時刻より最大延長可能時間以上後なら却下
					remainingWorkMin := int(currentSeat.Until.Sub(jstNow).Minutes())
					result.Add(usecase.ChangeWorkDurationRejectedAfter{
						MaxWorkTimeMin:           app.Configs.Constants.MaxWorkTimeMin,
						RealtimeEntryDurationMin: realtimeEntryDurationMin,
						RemainingWorkMin:         remainingWorkMin,
					})
				} else { // それ以外なら延長
					newSeat.Until = requestedUntil
					newSeat.CurrentStateUntil = requestedUntil
					remainingWorkMin := int(utils.NoNegativeDuration(requestedUntil.Sub(jstNow)).Minutes())
					result.Add(usecase.ChangeWorkDurationUpdated{
						RequestedMin:             changeOption.DurationMin,
						RealtimeEntryDurationMin: realtimeEntryDurationMin,
						RemainingWorkMin:         remainingWorkMin,
					})
				}
			case repository.BreakState:
				// 休憩時間を変更
				realtimeBreakDuration := utils.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt))
				requestedUntil := currentSeat.CurrentStateStartedAt.Add(time.Duration(changeOption.DurationMin) * time.Minute)

				if requestedUntil.Before(jstNow) {
					// もし現在時刻が指定時間を経過していたら却下
					remainingBreakDuration := currentSeat.CurrentStateUntil.Sub(jstNow)
					result.Add(usecase.ChangeBreakDurationRejectedBefore{
						RequestedMin:             changeOption.DurationMin,
						RealtimeBreakDurationMin: int(realtimeBreakDuration.Minutes()),
						RemainingBreakMin:        int(remainingBreakDuration.Minutes()),
					})
				} else { // それ以外ならuntilを変更
					newSeat.CurrentStateUntil = requestedUntil
					remainingBreakDuration := requestedUntil.Sub(jstNow)
					result.Add(usecase.ChangeBreakDurationUpdated{
						RequestedMin:             changeOption.DurationMin,
						RealtimeBreakDurationMin: int(realtimeBreakDuration.Minutes()),
						RemainingBreakMin:        int(remainingBreakDuration.Minutes()),
					})
				}
			}
		}
		if err := app.Repository.UpdateSeat(ctx, tx, *newSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in UpdateSeats: %w", err)
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
		jstNow := utils.JstNow()

		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			result.Add(usecase.MoreEnterOnly{})
			return nil
		}

		currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed app.CurrentSeat(): %w", err)
		}

		// 以降書き込みのみ
		newSeat := &currentSeat

		var addedMin int              // 最終的な延長時間（分）
		var remainingUntilExitMin int // 最終的な自動退室予定時刻までの残り時間（分）

		switch currentSeat.State {
		case repository.WorkState:
			// オーバーフロー対策。延長時間が最大作業時間を超えていたら、最大作業時間で上書き。
			if moreOption.IsDurationMinSet && moreOption.DurationMin > app.Configs.Constants.MaxWorkTimeMin {
				moreOption.DurationMin = app.Configs.Constants.MaxWorkTimeMin
			}

			// 延長時間が指定されていなかったら、最大延長。
			if !moreOption.IsDurationMinSet {
				moreOption.DurationMin = app.Configs.Constants.MaxWorkTimeMin
			}

			// 作業時間を指定分延長する
			newUntil := currentSeat.Until.Add(time.Duration(moreOption.DurationMin) * time.Minute)
			// もし延長後の時間が最大作業時間を超えていたら、最大作業時間まで延長
			remainingUntilExitMin = int(utils.NoNegativeDuration(newUntil.Sub(jstNow)).Minutes())
			if remainingUntilExitMin > app.Configs.Constants.MaxWorkTimeMin {
				newUntil = jstNow.Add(time.Duration(app.Configs.Constants.MaxWorkTimeMin) * time.Minute)
				result.Add(usecase.MoreMaxWork{
					MaxWorkTimeMin: app.Configs.Constants.MaxWorkTimeMin,
				})
			}
			addedMin = int(utils.NoNegativeDuration(newUntil.Sub(currentSeat.Until)).Minutes())
			newSeat.Until = newUntil
			newSeat.CurrentStateUntil = newUntil
			remainingUntilExitMin = int(utils.NoNegativeDuration(newUntil.Sub(jstNow)).Minutes())
		case repository.BreakState:
			// オーバーフロー対策。延長時間が最大休憩時間を超えていたら、最大休憩時間で上書き。
			if moreOption.IsDurationMinSet && moreOption.DurationMin > app.Configs.Constants.MaxBreakDurationMin {
				moreOption.DurationMin = app.Configs.Constants.MaxBreakDurationMin
			}

			// 延長時間が指定されていなかったら、最大延長。
			if !moreOption.IsDurationMinSet {
				moreOption.DurationMin = app.Configs.Constants.MaxBreakDurationMin
			}

			// 休憩時間を指定分延長する
			newBreakUntil := currentSeat.CurrentStateUntil.Add(time.Duration(moreOption.DurationMin) * time.Minute)
			// もし延長後の休憩時間が最大休憩時間を超えていたら、最大休憩時間まで延長
			newBreakDuration := utils.NoNegativeDuration(newBreakUntil.Sub(currentSeat.CurrentStateStartedAt))
			if int(newBreakDuration.Minutes()) > app.Configs.Constants.MaxBreakDurationMin {
				newBreakUntil = currentSeat.CurrentStateStartedAt.Add(time.Duration(app.Configs.Constants.MaxBreakDurationMin) * time.Minute)
				result.Add(usecase.MoreMaxBreak{
					MaxBreakDurationMin: app.Configs.Constants.MaxBreakDurationMin,
				})
			}
			addedMin = int(utils.NoNegativeDuration(newBreakUntil.Sub(currentSeat.CurrentStateUntil)).Minutes())
			newSeat.CurrentStateUntil = newBreakUntil
			// もし延長後の休憩時間がUntilを超えていたらUntilもそれに合わせる
			if newBreakUntil.After(currentSeat.Until) {
				newUntil := newBreakUntil
				newSeat.Until = newUntil
				remainingUntilExitMin = int(utils.NoNegativeDuration(newUntil.Sub(jstNow)).Minutes())
			} else {
				remainingUntilExitMin = int(utils.NoNegativeDuration(currentSeat.Until.Sub(jstNow)).Minutes())
			}
		}

		if err := app.Repository.UpdateSeat(ctx, tx, *newSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in app.Repository.UpdateSeats: %w", err)
		}

		switch currentSeat.State {
		case repository.WorkState:
			result.Add(usecase.MoreWorkExtended{AddedMin: addedMin})
		case repository.BreakState:
			remainingBreakDuration := utils.NoNegativeDuration(newSeat.CurrentStateUntil.Sub(jstNow))
			result.Add(usecase.MoreBreakExtended{
				AddedMin:          addedMin,
				RemainingBreakMin: int(remainingBreakDuration.Minutes()),
			})
		}
		realtimeEnteredTimeMin := int(utils.NoNegativeDuration(jstNow.Sub(currentSeat.EnteredAt)).Minutes())
		result.Add(usecase.MoreSummary{
			RealtimeEnteredMin:    realtimeEnteredTimeMin,
			RemainingUntilExitMin: remainingUntilExitMin,
		})

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
		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			result.Add(usecase.BreakEnterOnly{})
			return nil
		}

		// stateを確認
		currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed app.CurrentSeat(): %w", err)
		}
		if currentSeat.State != repository.WorkState {
			result.Add(usecase.BreakWorkOnly{})
			return nil
		}

		// 前回の入室または再開から、最低休憩間隔経っているか？
		currentWorkedMin := int(utils.NoNegativeDuration(utils.JstNow().Sub(currentSeat.CurrentStateStartedAt)).Minutes())
		if currentWorkedMin < app.Configs.Constants.MinBreakIntervalMin {
			result.Add(usecase.BreakWarn{
				MinBreakIntervalMin: app.Configs.Constants.MinBreakIntervalMin,
				CurrentWorkedMin:    currentWorkedMin,
			})
			return nil
		}

		// オプション確認
		if !breakOption.IsDurationMinSet {
			breakOption.DurationMin = app.Configs.Constants.DefaultBreakDurationMin
		}
		if !breakOption.IsWorkNameSet {
			breakOption.WorkName = currentSeat.BreakWorkName
		}

		// 休憩処理
		jstNow := utils.JstNow()
		breakUntil := jstNow.Add(time.Duration(breakOption.DurationMin) * time.Minute)
		workedSec := int(utils.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt)).Seconds())
		cumulativeWorkSec := currentSeat.CumulativeWorkSec + workedSec
		// もし日付を跨いで作業してたら、daily-cumulative-work-secは日付変更からの時間にする
		var dailyCumulativeWorkSec int
		if workedSec > utils.SecondsOfDay(jstNow) {
			dailyCumulativeWorkSec = utils.SecondsOfDay(jstNow)
		} else {
			dailyCumulativeWorkSec = currentSeat.DailyCumulativeWorkSec + workedSec
		}
		currentSeat.State = repository.BreakState
		currentSeat.CurrentStateStartedAt = jstNow
		currentSeat.CurrentStateUntil = breakUntil
		currentSeat.CumulativeWorkSec = cumulativeWorkSec
		currentSeat.DailyCumulativeWorkSec = dailyCumulativeWorkSec
		currentSeat.BreakWorkName = breakOption.WorkName

		if err := app.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in app.Repository.UpdateSeats: %w", err)
		}
		// activityログ記録
		startBreakActivity := repository.UserActivityDoc{
			UserId:       app.ProcessedUserId,
			ActivityType: repository.StartBreakActivity,
			SeatId:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			TakenAt:      utils.JstNow(),
		}
		if err := app.Repository.CreateUserActivityDoc(ctx, tx, startBreakActivity); err != nil {
			return fmt.Errorf("in CreateUserActivityDoc: %w", err)
		}

		result.Add(usecase.BreakStarted{
			SeatID:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			WorkName:     breakOption.WorkName,
			DurationMin:  breakOption.DurationMin,
		})
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
		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			result.Add(usecase.ResumeEnterOnly{})
			return nil
		}

		// stateを確認
		currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed app.CurrentSeat(): %w", err)
		}
		if currentSeat.State != repository.BreakState {
			result.Add(usecase.ResumeBreakOnly{})
			return nil
		}

		// 再開処理
		jstNow := utils.JstNow()
		until := currentSeat.Until
		breakSec := int(utils.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt)).Seconds())
		// もし日付を跨いで休憩してたら、daily-cumulative-work-secは0にリセットする
		var dailyCumulativeWorkSec = currentSeat.DailyCumulativeWorkSec
		if breakSec > utils.SecondsOfDay(jstNow) {
			dailyCumulativeWorkSec = 0
		}
		// 作業名が指定されていなかったら、既存の作業名を引継ぎ
		var workName = resumeOption.WorkName
		if !resumeOption.IsWorkNameSet {
			workName = currentSeat.WorkName
		}

		currentSeat.State = repository.WorkState
		currentSeat.CurrentStateStartedAt = jstNow
		currentSeat.CurrentStateUntil = until
		currentSeat.DailyCumulativeWorkSec = dailyCumulativeWorkSec
		currentSeat.WorkName = workName

		if err := app.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in app.Repository.UpdateSeats: %w", err)
		}
		// activityログ記録
		endBreakActivity := repository.UserActivityDoc{
			UserId:       app.ProcessedUserId,
			ActivityType: repository.EndBreakActivity,
			SeatId:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			TakenAt:      utils.JstNow(),
		}
		if err := app.Repository.CreateUserActivityDoc(ctx, tx, endBreakActivity); err != nil {
			return fmt.Errorf("in CreateUserActivityDoc: %w", err)
		}

		untilExitDuration := utils.NoNegativeDuration(until.Sub(jstNow))
		result.Add(usecase.ResumeStarted{
			SeatID:                currentSeat.SeatId,
			IsMemberSeat:          isInMemberRoom,
			RemainingUntilExitMin: int(untilExitDuration.Minutes()),
		})
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
		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			result.Add(usecase.OrderEnterOnly{})
			return nil
		}

		// メンバーでないなら本日の注文回数をチェック
		todayOrderCount, err := app.Repository.CountUserOrdersOfTheDay(ctx, app.ProcessedUserId, utils.JstNow())
		if err != nil {
			return fmt.Errorf("in CountUserOrdersOfTheDay: %w", err)
		}
		if !app.ProcessedUserIsMember && !orderOption.ClearFlag { // 下膳の場合はスキップ
			if todayOrderCount >= int64(app.Configs.Constants.MaxDailyOrderCount) {
				result.Add(usecase.OrderTooMany{
					MaxDailyOrderCount: app.Configs.Constants.MaxDailyOrderCount,
				})
				return nil
			}
		}

		currentSeat, err := app.CurrentSeat(ctx, app.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed app.CurrentSeat(): %w", err)
		}

		// これ以降は書き込みのみ

		if orderOption.ClearFlag {
			// 食器を下げる（注文履歴は削除しない）
			currentSeat.MenuCode = ""
			err := app.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in UpdateSeat: %w", err)
			}
			result.Add(usecase.OrderCleared{})
			return nil
		}

		targetMenuItem, err := app.GetMenuItemByNumber(orderOption.IntValue)
		if err != nil {
			return fmt.Errorf("in GetMenuItemByNumber: %w", err)
		}

		// 注文履歴を作成
		orderHistoryDoc := repository.OrderHistoryDoc{
			UserId:       app.ProcessedUserId,
			MenuCode:     targetMenuItem.Code,
			SeatId:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			OrderedAt:    utils.JstNow(),
		}
		if err := app.Repository.CreateOrderHistoryDoc(ctx, tx, orderHistoryDoc); err != nil {
			return fmt.Errorf("in CreateOrderHistoryDoc: %w", err)
		}

		// 座席ドキュメントを更新
		currentSeat.MenuCode = targetMenuItem.Code
		err = app.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("in UpdateSeat: %w", err)
		}

		result.Add(usecase.OrderOrdered{
			MenuName:   targetMenuItem.Name,
			CountAfter: todayOrderCount + 1,
		})
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
	replyMessage := ""
	var result usecase.Result
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			result.Add(usecase.ClearEnterOnly{})
			return nil
		}

		seat, err := app.CurrentSeat(ctx, app.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed app.CurrentSeat(): %w", err)
		}

		// これ以降は書き込みのみ

		// 作業内容をクリアする
		switch seat.State {
		case repository.WorkState:
			seat.WorkName = ""
			result.Add(usecase.ClearWork{SeatID: seat.SeatId, IsMemberSeat: isInMemberRoom})
		case repository.BreakState:
			seat.BreakWorkName = ""
			result.Add(usecase.ClearBreak{SeatID: seat.SeatId, IsMemberSeat: isInMemberRoom})
		}

		err = app.Repository.UpdateSeat(ctx, tx, seat, isInMemberRoom)
		if err != nil {
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
