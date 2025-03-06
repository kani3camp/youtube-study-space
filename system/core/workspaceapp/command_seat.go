package workspaceapp

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"app.modules/core/i18n"
	"app.modules/core/repository"
	"app.modules/core/studyspaceerror"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"github.com/pkg/errors"
)

func (s *WorkspaceApp) In(ctx context.Context, inOption *utils.InOption) error {
	jstNow := utils.JstNow()
	var replyMessage string
	t := i18n.GetTFunc("command-in")
	isTargetMemberSeat := inOption.IsMemberSeat

	if isTargetMemberSeat && !s.ProcessedUserIsMember {
		if s.Configs.Constants.YoutubeMembershipEnabled {
			s.MessageToLiveChat(ctx, t("member-seat-forbidden", s.ProcessedUserDisplayName))
		} else {
			s.MessageToLiveChat(ctx, t("membership-disabled", s.ProcessedUserDisplayName))
		}
		return nil
	}

	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 席が指定されているか？
		if inOption.IsSeatIdSet {
			// 0番席だったら最小番号の空席に決定
			if inOption.SeatId == 0 {
				seatId, err := s.MinAvailableSeatIdForUser(ctx, tx, s.ProcessedUserId, isTargetMemberSeat)
				if err != nil {
					return fmt.Errorf("in s.MinAvailableSeatIdForUser(): %w", err)
				}
				inOption.SeatId = seatId
			} else {
				// その席が空いているか？
				{
					isVacant, err := s.IfSeatVacant(ctx, tx, inOption.SeatId, isTargetMemberSeat)
					if err != nil {
						return fmt.Errorf("in s.IfSeatVacant(): %w", err)
					}
					if !isVacant {
						replyMessage = t("no-seat", s.ProcessedUserDisplayName, utils.InCommand)
						return nil
					}
				}
				// ユーザーはその席に対して入室制限を受けてないか？
				{
					isTooMuch, err := s.CheckIfUserSittingTooMuchForSeat(ctx, s.ProcessedUserId, inOption.SeatId, isTargetMemberSeat)
					if err != nil {
						return fmt.Errorf("in s.CheckIfUserSittingTooMuchForSeat(): %w", err)
					}
					if isTooMuch {
						replyMessage = t("no-availability", s.ProcessedUserDisplayName, utils.InCommand)
						return nil
					}
				}
			}
		} else { // 席の指定なし
			seatId, err := s.RandomAvailableSeatIdForUser(ctx, tx, s.ProcessedUserId, isTargetMemberSeat)
			if err != nil {
				if errors.Is(err, studyspaceerror.ErrNoSeatAvailable) {
					return fmt.Errorf("席数がmax seatに達していて、ユーザーが入室できない事象が発生: %w", err)
				}
				return err
			}
			inOption.SeatId = seatId
		}

		userDoc, err := s.Repository.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in ReadUser(): %w", err)
		}

		// 作業時間が指定されているか？
		if !inOption.MinutesAndWorkName.IsDurationMinSet {
			if userDoc.DefaultStudyMin == 0 {
				inOption.MinutesAndWorkName.DurationMin = s.Configs.Constants.DefaultWorkTimeMin
			} else {
				inOption.MinutesAndWorkName.DurationMin = userDoc.DefaultStudyMin
			}
		}

		// ランクから席の色を決定
		seatAppearance, err := s.GetUserRealtimeSeatAppearance(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in GetUserRealtimeSeatAppearance(): %w", err)
		}

		// 動作が決定

		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in IsUserInRoom(): %w", err)
		}
		isInRoom := isInGeneralRoom || isInMemberRoom
		var currentSeat repository.SeatDoc
		if isInRoom { // 現在座っている席を取得
			var err error
			currentSeat, err = s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in CurrentSeat(): %w", err)
			}
		}

		var totalOrderCount int64
		var targetMenuItem repository.MenuDoc
		var orderLimitExceeded bool
		if inOption.MinutesAndWorkName.IsOrderSet {
			// メンバーでない場合は、本日の注文回数をチェック
			totalOrderCount, err = s.Repository.CountUserOrdersOfTheDay(ctx, s.ProcessedUserId, jstNow)
			if err != nil {
				return fmt.Errorf("in CountUserOrdersOfTheDay(): %w", err)
			}
			orderLimitExceeded = !s.ProcessedUserIsMember && totalOrderCount >= int64(s.Configs.Constants.MaxDailyOrderCount)

			if !orderLimitExceeded {
				targetMenuItem, err = s.GetMenuItemByNumber(inOption.MinutesAndWorkName.OrderNum)
				if err != nil {
					return fmt.Errorf("in GetMenuItemByNumber(): %w", err)
				}
				if isInRoom {
					currentSeat.MenuCode = targetMenuItem.Code
				}
			}
		}

		// =========== 以降は書き込み処理のみ ===========

		if isInRoom && inOption.IsSeatIdSet { // 入室中で、席指定があれば、席移動処理
			workedTimeSec, addedRP, untilExitMin, err := s.moveSeat(
				ctx,
				tx,
				inOption.SeatId,
				s.ProcessedUserProfileImageUrl,
				isInMemberRoom,
				isTargetMemberSeat,
				*inOption.MinutesAndWorkName,
				currentSeat, &userDoc)
			if err != nil {
				return fmt.Errorf("failed to moveSeat for %s (%s): %w", s.ProcessedUserDisplayName, s.ProcessedUserId, err)
			}

			var rpEarned string
			if userDoc.RankVisible {
				rpEarned = i18n.T("command:rp-earned", addedRP)
			}
			previousSeatIdStr := utils.SeatIdStr(currentSeat.SeatId, isInMemberRoom)
			newSeatIdStr := utils.SeatIdStr(inOption.SeatId, isTargetMemberSeat)

			replyMessage += t("seat-move", s.ProcessedUserDisplayName, inOption.MinutesAndWorkName.WorkName, previousSeatIdStr, newSeatIdStr, workedTimeSec/60, rpEarned, untilExitMin)
		} else if isInRoom && !inOption.IsSeatIdSet { // 入室中で、席指定がない場合は、指定があったオプションのみ更新処理（席移動なし）
			var seatIdStr string
			if isInMemberRoom {
				seatIdStr = i18n.T("common:vip-seat-id", currentSeat.SeatId)
			} else {
				seatIdStr = strconv.Itoa(currentSeat.SeatId)
			}
			replyMessage += t("already-seat", s.ProcessedUserDisplayName, seatIdStr)

			if inOption.MinutesAndWorkName.IsWorkNameSet {
				switch currentSeat.State {
				case repository.WorkState:
					currentSeat.WorkName = inOption.MinutesAndWorkName.WorkName
					replyMessage += i18n.T("command-change:update-work", inOption.MinutesAndWorkName.WorkName, seatIdStr)
				case repository.BreakState:
					currentSeat.BreakWorkName = inOption.MinutesAndWorkName.WorkName
					replyMessage += i18n.T("command-change:update-break", inOption.MinutesAndWorkName.WorkName, seatIdStr)
				}
			}

			if inOption.MinutesAndWorkName.IsDurationMinSet {
				switch currentSeat.State {
				case repository.WorkState:
					// 作業時間を（入室時間から自動退室までの時間）を変更
					realtimeEntryDurationMin := int(utils.NoNegativeDuration(jstNow.Sub(currentSeat.EnteredAt)).Minutes())
					requestedUntil := currentSeat.EnteredAt.Add(time.Duration(inOption.MinutesAndWorkName.DurationMin) * time.Minute)

					if requestedUntil.Before(jstNow) {
						// もし現在時刻が指定時間を経過していたら却下
						remainingWorkMin := int(currentSeat.Until.Sub(jstNow).Minutes())
						replyMessage += i18n.T("command-change:work-duration-before", inOption.MinutesAndWorkName.DurationMin, realtimeEntryDurationMin, remainingWorkMin)
					} else if requestedUntil.After(jstNow.Add(time.Duration(s.Configs.Constants.MaxWorkTimeMin) * time.Minute)) {
						// もし現在時刻より最大延長可能時間以上後なら却下
						remainingWorkMin := int(currentSeat.Until.Sub(jstNow).Minutes())
						replyMessage += i18n.T("command-change:work-duration-after", s.Configs.Constants.MaxWorkTimeMin, realtimeEntryDurationMin, remainingWorkMin)
					} else { // それ以外なら延長
						currentSeat.Until = requestedUntil
						currentSeat.CurrentStateUntil = requestedUntil
						remainingWorkMin := int(utils.NoNegativeDuration(requestedUntil.Sub(jstNow)).Minutes())
						replyMessage += i18n.T("command-change:work-duration", inOption.MinutesAndWorkName.DurationMin, realtimeEntryDurationMin, remainingWorkMin)
					}
				case repository.BreakState:
					// 休憩時間を変更
					realtimeBreakDuration := utils.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt))
					requestedUntil := currentSeat.CurrentStateStartedAt.Add(time.Duration(inOption.MinutesAndWorkName.DurationMin) * time.Minute)

					if requestedUntil.Before(jstNow) {
						// もし現在時刻が指定時間を経過していたら却下
						remainingBreakDuration := currentSeat.CurrentStateUntil.Sub(jstNow)
						replyMessage += i18n.T("command-change:break-duration-before", inOption.MinutesAndWorkName.DurationMin, int(realtimeBreakDuration.Minutes()), int(remainingBreakDuration.Minutes()))
					} else { // それ以外ならuntilを変更
						currentSeat.CurrentStateUntil = requestedUntil
						remainingBreakDuration := requestedUntil.Sub(jstNow)
						replyMessage += i18n.T("command-change:break-duration", inOption.MinutesAndWorkName.DurationMin, int(realtimeBreakDuration.Minutes()), int(remainingBreakDuration.Minutes()))
					}
				}
			}
		} else { // 入室のみ
			untilExitMin, err := s.enterRoom(
				ctx,
				tx,
				s.ProcessedUserId,
				s.ProcessedUserDisplayName,
				s.ProcessedUserProfileImageUrl,
				inOption.SeatId,
				isTargetMemberSeat,
				inOption.MinutesAndWorkName.WorkName,
				"",
				inOption.MinutesAndWorkName.DurationMin,
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
			var newSeatId string
			if isTargetMemberSeat {
				newSeatId = i18n.T("common:vip-seat-id", inOption.SeatId)
			} else {
				newSeatId = strconv.Itoa(inOption.SeatId)
			}

			// 入室しましたのメッセージ
			replyMessage += t("start", s.ProcessedUserDisplayName, inOption.MinutesAndWorkName.WorkName, untilExitMin, newSeatId)
		}

		// メニュー注文されている場合は、メニューコードをセット
		if inOption.MinutesAndWorkName.IsOrderSet {
			if orderLimitExceeded {
				replyMessage += t("too-many-orders", s.Configs.Constants.MaxDailyOrderCount)
				return nil
			}

			// 注文履歴を作成
			orderHistoryDoc := repository.OrderHistoryDoc{
				UserId:       s.ProcessedUserId,
				MenuCode:     targetMenuItem.Code,
				SeatId:       inOption.SeatId,
				IsMemberSeat: isTargetMemberSeat,
				OrderedAt:    jstNow,
			}
			if err := s.Repository.CreateOrderHistoryDoc(ctx, tx, orderHistoryDoc); err != nil {
				return fmt.Errorf("in CreateOrderHistoryDoc: %w", err)
			}

			replyMessage += t("ordered", targetMenuItem.Name, totalOrderCount+1)
		}

		return nil
	})
	if txErr != nil {
		slog.Error("txErr in In()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Out(ctx context.Context) error {
	t := i18n.GetTFunc("command-out")
	var replyMessage string
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := s.Repository.ReadUser(ctx, tx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in ReadUser(): %w", err)
		}

		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in IsUserInRoom(): %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			if userDoc.LastExited.IsZero() {
				replyMessage = t("already-exit", s.ProcessedUserDisplayName)
			} else {
				lastExited := userDoc.LastExited.In(utils.JapanLocation())
				replyMessage = t("already-exit-with-last-exit-time", s.ProcessedUserDisplayName, lastExited.Hour(), lastExited.Minute())
			}
			return nil
		}

		// 現在座っている席を特定
		seat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("in CurrentSeat(): %w", err)
		}

		// 退室処理
		workedTimeSec, addedRP, err := s.exitRoom(ctx, tx, isInMemberRoom, seat, &userDoc)
		if err != nil {
			return fmt.Errorf("in exitRoom(): %w", err)
		}
		var rpEarned string
		var seatIdStr string
		if userDoc.RankVisible {
			rpEarned = i18n.T("command:rp-earned", addedRP)
		}
		if isInMemberRoom {
			seatIdStr = i18n.T("common:vip-seat-id", seat.SeatId)
		} else {
			seatIdStr = strconv.Itoa(seat.SeatId)
		}
		replyMessage = i18n.T("command:exit", s.ProcessedUserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Out()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) ShowSeatInfo(ctx context.Context, seatOption *utils.SeatOption) error {
	t := i18n.GetTFunc("command-seat-info")
	showDetails := seatOption.ShowDetails
	var replyMessage string
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// そのユーザーは入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in IsUserInRoom(): %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if isInRoom {
			currentSeat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in s.CurrentSeat(): %w", err)
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
				stateStr = i18n.T("common:work")
				breakUntilStr = ""
			case repository.BreakState:
				stateStr = i18n.T("common:break")
				breakUntilDuration := utils.NoNegativeDuration(currentSeat.CurrentStateUntil.Sub(utils.JstNow()))
				breakUntilStr = t("break-until", int(breakUntilDuration.Minutes()))
			}
			var seatIdStr string
			if isInMemberRoom {
				seatIdStr = i18n.T("common:vip-seat-id", currentSeat.SeatId)
			} else {
				seatIdStr = strconv.Itoa(currentSeat.SeatId)
			}
			replyMessage = t("base", s.ProcessedUserDisplayName, seatIdStr, stateStr, realtimeSittingDurationMin, int(realtimeTotalStudyDurationOfSeat.Minutes()), remainingMinutes, breakUntilStr)

			if showDetails {
				recentTotalEntryDuration, err := s.GetRecentUserSittingTimeForSeat(ctx, s.ProcessedUserId, currentSeat.SeatId, isInMemberRoom)
				if err != nil {
					return fmt.Errorf("in GetRecentUserSittingTimeForSeat(): %w", err)
				}
				replyMessage += t("details", s.Configs.Constants.RecentRangeMin, seatIdStr, int(recentTotalEntryDuration.Minutes()))
			}
		} else {
			replyMessage = i18n.T("command:not-enter", s.ProcessedUserDisplayName, utils.InCommand)
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in ShowSeatInfo()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Change(ctx context.Context, changeOption *utils.MinWorkOrderOption) error {
	jstNow := utils.JstNow()
	replyMessage := ""
	t := i18n.GetTFunc("command-change")
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// そのユーザーは入室中か？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		currentSeat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}

		// validation
		if err := s.ValidateChange(*changeOption, currentSeat.State); err != nil {
			replyMessage = fmt.Sprintf("%s%s", i18n.T("common:sir", s.ProcessedUserDisplayName), err) // TODO 動作確認
			return nil
		}

		// これ以降は書き込みのみ可。

		newSeat := &currentSeat
		replyMessage = i18n.T("common:sir", s.ProcessedUserDisplayName)
		if changeOption.IsWorkNameSet { // 作業名もしくは休憩作業名を書きかえ
			var seatIdStr string
			if isInMemberRoom {
				seatIdStr = i18n.T("common:vip-seat-id", currentSeat.SeatId)
			} else {
				seatIdStr = strconv.Itoa(currentSeat.SeatId)
			}

			switch currentSeat.State {
			case repository.WorkState:
				newSeat.WorkName = changeOption.WorkName
				replyMessage += t("update-work", changeOption.WorkName, seatIdStr)
			case repository.BreakState:
				newSeat.BreakWorkName = changeOption.WorkName
				replyMessage += t("update-break", changeOption.WorkName, seatIdStr)
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
					replyMessage += t("work-duration-before", changeOption.DurationMin, realtimeEntryDurationMin, remainingWorkMin)
				} else if requestedUntil.After(jstNow.Add(time.Duration(s.Configs.Constants.MaxWorkTimeMin) * time.Minute)) {
					// もし現在時刻より最大延長可能時間以上後なら却下
					remainingWorkMin := int(currentSeat.Until.Sub(jstNow).Minutes())
					replyMessage += t("work-duration-after", s.Configs.Constants.MaxWorkTimeMin, realtimeEntryDurationMin, remainingWorkMin)
				} else { // それ以外なら延長
					newSeat.Until = requestedUntil
					newSeat.CurrentStateUntil = requestedUntil
					remainingWorkMin := int(utils.NoNegativeDuration(requestedUntil.Sub(jstNow)).Minutes())
					replyMessage += t("work-duration", changeOption.DurationMin, realtimeEntryDurationMin, remainingWorkMin)
				}
			case repository.BreakState:
				// 休憩時間を変更
				realtimeBreakDuration := utils.NoNegativeDuration(jstNow.Sub(currentSeat.CurrentStateStartedAt))
				requestedUntil := currentSeat.CurrentStateStartedAt.Add(time.Duration(changeOption.DurationMin) * time.Minute)

				if requestedUntil.Before(jstNow) {
					// もし現在時刻が指定時間を経過していたら却下
					remainingBreakDuration := currentSeat.CurrentStateUntil.Sub(jstNow)
					replyMessage += t("break-duration-before", changeOption.DurationMin, int(realtimeBreakDuration.Minutes()), int(remainingBreakDuration.Minutes()))
				} else { // それ以外ならuntilを変更
					newSeat.CurrentStateUntil = requestedUntil
					remainingBreakDuration := requestedUntil.Sub(jstNow)
					replyMessage += t("break-duration", changeOption.DurationMin, int(realtimeBreakDuration.Minutes()), int(remainingBreakDuration.Minutes()))
				}
			}
		}
		if err := s.Repository.UpdateSeat(ctx, tx, *newSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in UpdateSeats: %w", err)
		}

		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Change()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) More(ctx context.Context, moreOption *utils.MoreOption) error {
	replyMessage := ""
	t := i18n.GetTFunc("command-more")
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		jstNow := utils.JstNow()

		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		currentSeat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}

		// 以降書き込みのみ
		newSeat := &currentSeat

		replyMessage = i18n.T("common:sir", s.ProcessedUserDisplayName)
		var addedMin int              // 最終的な延長時間（分）
		var remainingUntilExitMin int // 最終的な自動退室予定時刻までの残り時間（分）

		switch currentSeat.State {
		case repository.WorkState:
			// オーバーフロー対策。延長時間が最大作業時間を超えていたら、最大作業時間で上書き。
			if moreOption.IsDurationMinSet && moreOption.DurationMin > s.Configs.Constants.MaxWorkTimeMin {
				moreOption.DurationMin = s.Configs.Constants.MaxWorkTimeMin
			}

			// 延長時間が指定されていなかったら、最大延長。
			if !moreOption.IsDurationMinSet {
				moreOption.DurationMin = s.Configs.Constants.MaxWorkTimeMin
			}

			// 作業時間を指定分延長する
			newUntil := currentSeat.Until.Add(time.Duration(moreOption.DurationMin) * time.Minute)
			// もし延長後の時間が最大作業時間を超えていたら、最大作業時間まで延長
			remainingUntilExitMin = int(utils.NoNegativeDuration(newUntil.Sub(jstNow)).Minutes())
			if remainingUntilExitMin > s.Configs.Constants.MaxWorkTimeMin {
				newUntil = jstNow.Add(time.Duration(s.Configs.Constants.MaxWorkTimeMin) * time.Minute)
				replyMessage += t("max-work", s.Configs.Constants.MaxWorkTimeMin)
			}
			addedMin = int(utils.NoNegativeDuration(newUntil.Sub(currentSeat.Until)).Minutes())
			newSeat.Until = newUntil
			newSeat.CurrentStateUntil = newUntil
			remainingUntilExitMin = int(utils.NoNegativeDuration(newUntil.Sub(jstNow)).Minutes())
		case repository.BreakState:
			// オーバーフロー対策。延長時間が最大休憩時間を超えていたら、最大休憩時間で上書き。
			if moreOption.IsDurationMinSet && moreOption.DurationMin > s.Configs.Constants.MaxBreakDurationMin {
				moreOption.DurationMin = s.Configs.Constants.MaxBreakDurationMin
			}

			// 延長時間が指定されていなかったら、最大延長。
			if !moreOption.IsDurationMinSet {
				moreOption.DurationMin = s.Configs.Constants.MaxBreakDurationMin
			}

			// 休憩時間を指定分延長する
			newBreakUntil := currentSeat.CurrentStateUntil.Add(time.Duration(moreOption.DurationMin) * time.Minute)
			// もし延長後の休憩時間が最大休憩時間を超えていたら、最大休憩時間まで延長
			newBreakDuration := utils.NoNegativeDuration(newBreakUntil.Sub(currentSeat.CurrentStateStartedAt))
			if int(newBreakDuration.Minutes()) > s.Configs.Constants.MaxBreakDurationMin {
				newBreakUntil = currentSeat.CurrentStateStartedAt.Add(time.Duration(s.Configs.Constants.MaxBreakDurationMin) * time.Minute)
				replyMessage += t("max-break", strconv.Itoa(s.Configs.Constants.MaxBreakDurationMin))
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

		if err := s.Repository.UpdateSeat(ctx, tx, *newSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in s.Repository.UpdateSeats: %w", err)
		}

		switch currentSeat.State {
		case repository.WorkState:
			replyMessage += t("reply-work", addedMin)
		case repository.BreakState:
			remainingBreakDuration := utils.NoNegativeDuration(newSeat.CurrentStateUntil.Sub(jstNow))
			replyMessage += t("reply-break", addedMin, int(remainingBreakDuration.Minutes()))
		}
		realtimeEnteredTimeMin := int(utils.NoNegativeDuration(jstNow.Sub(currentSeat.EnteredAt)).Minutes())
		replyMessage += t("reply", realtimeEnteredTimeMin, remainingUntilExitMin)

		return nil
	})
	if txErr != nil {
		slog.Error("txErr in More()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Break(ctx context.Context, breakOption *utils.MinWorkOrderOption) error {
	replyMessage := ""
	t := i18n.GetTFunc("command-break")
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		// stateを確認
		currentSeat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}
		if currentSeat.State != repository.WorkState {
			replyMessage = t("work-only", s.ProcessedUserDisplayName)
			return nil
		}

		// 前回の入室または再開から、最低休憩間隔経っているか？
		currentWorkedMin := int(utils.NoNegativeDuration(utils.JstNow().Sub(currentSeat.CurrentStateStartedAt)).Minutes())
		if currentWorkedMin < s.Configs.Constants.MinBreakIntervalMin {
			replyMessage = t("warn", s.ProcessedUserDisplayName, s.Configs.Constants.MinBreakIntervalMin, currentWorkedMin)
			return nil
		}

		// オプション確認
		if !breakOption.IsDurationMinSet {
			breakOption.DurationMin = s.Configs.Constants.DefaultBreakDurationMin
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

		if err := s.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in s.Repository.UpdateSeats: %w", err)
		}
		// activityログ記録
		startBreakActivity := repository.UserActivityDoc{
			UserId:       s.ProcessedUserId,
			ActivityType: repository.StartBreakActivity,
			SeatId:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			TakenAt:      utils.JstNow(),
		}
		if err := s.Repository.CreateUserActivityDoc(ctx, tx, startBreakActivity); err != nil {
			return fmt.Errorf("in CreateUserActivityDoc: %w", err)
		}

		var seatIdStr string
		if isInMemberRoom {
			seatIdStr = i18n.T("common:vip-seat-id", currentSeat.SeatId)
		} else {
			seatIdStr = strconv.Itoa(currentSeat.SeatId)
		}

		replyMessage = t("break", s.ProcessedUserDisplayName, breakOption.DurationMin, seatIdStr)
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Break()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Resume(ctx context.Context, resumeOption *utils.WorkNameOption) error {
	replyMessage := ""
	t := i18n.GetTFunc("command-resume")
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		// stateを確認
		currentSeat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}
		if currentSeat.State != repository.BreakState {
			replyMessage = t("break-only", s.ProcessedUserDisplayName)
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

		if err := s.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
			return fmt.Errorf("in s.Repository.UpdateSeats: %w", err)
		}
		// activityログ記録
		endBreakActivity := repository.UserActivityDoc{
			UserId:       s.ProcessedUserId,
			ActivityType: repository.EndBreakActivity,
			SeatId:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			TakenAt:      utils.JstNow(),
		}
		if err := s.Repository.CreateUserActivityDoc(ctx, tx, endBreakActivity); err != nil {
			return fmt.Errorf("in CreateUserActivityDoc: %w", err)
		}

		var seatIdStr string
		if isInMemberRoom {
			seatIdStr = i18n.T("common:vip-seat-id", currentSeat.SeatId)
		} else {
			seatIdStr = strconv.Itoa(currentSeat.SeatId)
		}

		untilExitDuration := utils.NoNegativeDuration(until.Sub(jstNow))
		replyMessage = t("work", s.ProcessedUserDisplayName, seatIdStr, int(untilExitDuration.Minutes()))
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Resume()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Order(ctx context.Context, orderOption *utils.OrderOption) error {
	replyMessage := ""
	t := i18n.GetTFunc("command-order")
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		// メンバーでないなら本日の注文回数をチェック
		todayOrderCount, err := s.Repository.CountUserOrdersOfTheDay(ctx, s.ProcessedUserId, utils.JstNow())
		if err != nil {
			return fmt.Errorf("in CountUserOrdersOfTheDay: %w", err)
		}
		if !s.ProcessedUserIsMember && !orderOption.ClearFlag { // 下膳の場合はスキップ
			if todayOrderCount >= int64(s.Configs.Constants.MaxDailyOrderCount) {
				replyMessage = t("too-many-orders", s.ProcessedUserDisplayName, s.Configs.Constants.MaxDailyOrderCount)
				return nil
			}
		}

		currentSeat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}

		// これ以降は書き込みのみ

		if orderOption.ClearFlag {
			// 食器を下げる（注文履歴は削除しない）
			currentSeat.MenuCode = ""
			err := s.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("in UpdateSeat: %w", err)
			}
			replyMessage = t("cleared", s.ProcessedUserDisplayName)
			return nil
		}

		targetMenuItem, err := s.GetMenuItemByNumber(orderOption.IntValue)
		if err != nil {
			return fmt.Errorf("in GetMenuItemByNumber: %w", err)
		}

		// 注文履歴を作成
		orderHistoryDoc := repository.OrderHistoryDoc{
			UserId:       s.ProcessedUserId,
			MenuCode:     targetMenuItem.Code,
			SeatId:       currentSeat.SeatId,
			IsMemberSeat: isInMemberRoom,
			OrderedAt:    utils.JstNow(),
		}
		if err := s.Repository.CreateOrderHistoryDoc(ctx, tx, orderHistoryDoc); err != nil {
			return fmt.Errorf("in CreateOrderHistoryDoc: %w", err)
		}

		// 座席ドキュメントを更新
		currentSeat.MenuCode = targetMenuItem.Code
		err = s.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("in UpdateSeat: %w", err)
		}

		replyMessage = t("ordered", s.ProcessedUserDisplayName, targetMenuItem.Name, todayOrderCount+1)
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Order()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Clear(ctx context.Context) error {
	replyMessage := ""
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 入室しているか？
		isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom
		if !isInRoom {
			replyMessage = i18n.T("command:enter-only", s.ProcessedUserDisplayName)
			return nil
		}

		seat, err := s.CurrentSeat(ctx, s.ProcessedUserId, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}

		// これ以降は書き込みのみ

		// 作業内容をクリアする
		switch seat.State {
		case repository.WorkState:
			seat.WorkName = ""
			replyMessage = i18n.T("others:clear-work", s.ProcessedUserDisplayName, seat.SeatId)
		case repository.BreakState:
			seat.BreakWorkName = ""
			replyMessage = i18n.T("others:clear-break", s.ProcessedUserDisplayName, seat.SeatId)
		}

		err = s.Repository.UpdateSeat(ctx, tx, seat, isInMemberRoom)
		if err != nil {
			return fmt.Errorf("in UpdateSeat: %w", err)
		}

		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Clear()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}
