package workspaceapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"time"

	"app.modules/core/i18n"
	"app.modules/core/mybigquery"
	"app.modules/core/mystorage"
	"app.modules/core/repository"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OrganizeDB 1分ごとに処理を行う。
// - 自動退室予定時刻(until)を過ぎているルーム内のユーザーを退室させる。
// - CurrentStateUntilを過ぎている休憩中のユーザーを作業再開させる。
// - 一時着席制限ブラックリスト・ホワイトリストのuntilを過ぎているドキュメントを削除する。
func (s *WorkspaceApp) OrganizeDB(ctx context.Context, isMemberRoom bool) error {
	slog.Info(utils.NameOf(s.OrganizeDB), "isMemberRoom", isMemberRoom)

	slog.Info("自動退室")
	// 全座席のスナップショットをとる（トランザクションなし）
	if err := s.OrganizeDBAutoExit(ctx, isMemberRoom); err != nil {
		return fmt.Errorf("in OrganizeDBAutoExit(): %w", err)
	}

	slog.Info("作業再開")
	if err := s.OrganizeDBResume(ctx, isMemberRoom); err != nil {
		return fmt.Errorf("in OrganizeDBResume(): %w", err)
	}

	slog.Info("一時着席制限ブラックリスト・ホワイトリストのクリーニング")
	if err := s.OrganizeDBDeleteExpiredSeatLimits(ctx, isMemberRoom); err != nil {
		return fmt.Errorf("in OrganizeDBDeleteExpiredSeatLimits(): %w", err)
	}

	return nil
}

func (s *WorkspaceApp) OrganizeDBAutoExit(ctx context.Context, isMemberRoom bool) error {
	jstNow := utils.JstNow()
	candidateSeatsSnapshot, err := s.Repository.ReadSeatsExpiredUntil(ctx, jstNow, isMemberRoom)
	if err != nil {
		return fmt.Errorf("in ReadSeatsExpiredUntil(): %w", err)
	}
	slog.Info("自動退室候補" + strconv.Itoa(len(candidateSeatsSnapshot)) + "人")

	for _, seatSnapshot := range candidateSeatsSnapshot {
		liveChatMessage := ""
		txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			s.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, seatSnapshot.UserProfileImageUrl, false, false, isMemberRoom)

			// 現在も存在しているか
			seat, err := s.Repository.ReadSeat(ctx, tx, seatSnapshot.SeatId, isMemberRoom)
			if err != nil {
				if status.Code(err) == codes.NotFound {
					slog.Info("すぐ前に退室したということなのでスルー")
					return nil
				}
				return fmt.Errorf("in ReadSeat(): %w", err)
			}
			if !reflect.DeepEqual(seat, seatSnapshot) {
				slog.Info("その座席に少しでも変更が加えられているのでスルー")
				return nil
			}

			userDoc, err := s.Repository.ReadUser(ctx, tx, s.ProcessedUserId)
			if err != nil {
				return fmt.Errorf("in ReadUser(): %w", err)
			}

			autoExit := seat.Until.Before(utils.JstNow()) // 自動退室時刻を過ぎていたら自動退室

			// 以下書き込みのみ

			// 自動退室時刻による退室処理
			if autoExit {
				workedTimeSec, addedRP, err := s.exitRoom(ctx, tx, isMemberRoom, seat, &userDoc)
				if err != nil {
					return fmt.Errorf("%sさん（%s）の退室処理中にエラーが発生しました: %w", s.ProcessedUserDisplayName, s.ProcessedUserId, err)
				}
				var rpEarned string
				var seatIdStr string
				if userDoc.RankVisible {
					rpEarned = i18n.T("command:rp-earned", addedRP)
				}
				if isMemberRoom {
					seatIdStr = i18n.T("common:vip-seat-id", seat.SeatId)
				} else {
					seatIdStr = strconv.Itoa(seat.SeatId)
				}
				liveChatMessage = i18n.T("command:exit", s.ProcessedUserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)
			}

			return nil
		})
		if txErr != nil {
			s.MessageToOwnerWithError(ctx, "failed transaction", txErr)
			continue // txErr != nil でもreturnではなく次に進む
		}
		if liveChatMessage != "" {
			s.MessageToLiveChat(ctx, liveChatMessage)
		}
	}
	return nil
}

func (s *WorkspaceApp) OrganizeDBResume(ctx context.Context, isMemberRoom bool) error {
	jstNow := utils.JstNow()
	candidateSeatsSnapshot, err := s.Repository.ReadSeatsExpiredBreakUntil(ctx, jstNow, isMemberRoom)
	if err != nil {
		return fmt.Errorf("in ReadSeatsExpiredBreakUntil(): %w", err)
	}
	slog.Info("作業再開候補" + strconv.Itoa(len(candidateSeatsSnapshot)) + "人")

	for _, seatSnapshot := range candidateSeatsSnapshot {
		liveChatMessage := ""
		txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			s.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, seatSnapshot.UserProfileImageUrl, false, false, isMemberRoom)

			// 現在も存在しているか
			seat, err := s.Repository.ReadSeat(ctx, tx, seatSnapshot.SeatId, isMemberRoom)
			if err != nil {
				if status.Code(err) == codes.NotFound {
					slog.Info("すぐ前に退室したということなのでスルー")
					return nil
				}
				return fmt.Errorf("in ReadSeat(): %w", err)
			}
			if !reflect.DeepEqual(seat, seatSnapshot) {
				slog.Info("その座席に少しでも変更が加えられているのでスルー")
				return nil
			}

			resume := seat.State == repository.BreakState && seat.CurrentStateUntil.Before(utils.JstNow())

			// 以下書き込みのみ

			if resume { // 作業再開処理
				jstNow := utils.JstNow()
				until := seat.Until
				breakSec := int(utils.NoNegativeDuration(jstNow.Sub(seat.CurrentStateStartedAt)).Seconds())
				// もし日付を跨いで休憩してたら、daily-cumulative-work-secは0にリセットする
				var dailyCumulativeWorkSec = seat.DailyCumulativeWorkSec
				if breakSec > utils.SecondsOfDay(jstNow) {
					dailyCumulativeWorkSec = 0
				}

				seat.State = repository.WorkState
				seat.CurrentStateStartedAt = jstNow
				seat.CurrentStateUntil = until
				seat.DailyCumulativeWorkSec = dailyCumulativeWorkSec
				if err := s.Repository.UpdateSeat(ctx, tx, seat, isMemberRoom); err != nil {
					return fmt.Errorf("in UpdateSeat(): %w", err)
				}
				// activityログ記録
				endBreakActivity := repository.UserActivityDoc{
					UserId:       s.ProcessedUserId,
					ActivityType: repository.EndBreakActivity,
					SeatId:       seat.SeatId,
					IsMemberSeat: isMemberRoom,
					TakenAt:      utils.JstNow(),
				}
				if err := s.Repository.CreateUserActivityDoc(ctx, tx, endBreakActivity); err != nil {
					return fmt.Errorf("in CreateUserActivityDoc(): %w", err)
				}
				var seatIdStr string
				if isMemberRoom {
					seatIdStr = i18n.T("common:vip-seat-id", seat.SeatId)
				} else {
					seatIdStr = strconv.Itoa(seat.SeatId)
				}

				liveChatMessage = i18n.T("command-resume:work", s.ProcessedUserDisplayName, seatIdStr, int(utils.NoNegativeDuration(until.Sub(jstNow)).Minutes()))
			}
			return nil
		})
		if txErr != nil {
			s.MessageToOwnerWithError(ctx, "failed transaction", txErr)
			continue // txErr != nil でもreturnではなく次に進む
		}
		if liveChatMessage != "" {
			s.MessageToLiveChat(ctx, liveChatMessage)
		}
	}
	return nil
}

func (s *WorkspaceApp) OrganizeDBDeleteExpiredSeatLimits(ctx context.Context, isMemberRoom bool) error {
	jstNow := utils.JstNow()
	// white list
	for {
		iter := s.Repository.Get500SeatLimitsAfterUntilInWHITEList(ctx, jstNow, isMemberRoom)
		count, err := s.DeleteIteratorDocs(ctx, iter)
		if err != nil {
			return fmt.Errorf("in DeleteIteratorDocs(): %w", err)
		}
		if count == 0 {
			break
		}
	}

	// black list
	for {
		iter := s.Repository.Get500SeatLimitsAfterUntilInBLACKList(ctx, jstNow, isMemberRoom)
		count, err := s.DeleteIteratorDocs(ctx, iter)
		if err != nil {
			return fmt.Errorf("in DeleteIteratorDocs(): %w", err)
		}
		if count == 0 {
			break
		}
	}
	return nil
}

func (s *WorkspaceApp) OrganizeDBForceMove(ctx context.Context, seatsSnapshot []repository.SeatDoc, isMemberSeat bool) error {
	slog.Info(utils.NameOf(s.OrganizeDBForceMove), "isMemberSeat", isMemberSeat, "len(seatsSnapshot)", len(seatsSnapshot))
	for _, seatSnapshot := range seatsSnapshot {
		var forcedMove bool // 長時間入室制限による強制席移動
		txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			s.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, seatSnapshot.UserProfileImageUrl, false, false, isMemberSeat)

			// 現在も存在しているか
			seat, err := s.Repository.ReadSeat(ctx, tx, seatSnapshot.SeatId, isMemberSeat)
			if err != nil {
				if status.Code(err) == codes.NotFound {
					slog.Info("すぐ前に退室したということなのでスルー")
					return nil
				}
				return fmt.Errorf("in ReadSeat(): %w", err)
			}
			if !reflect.DeepEqual(seat, seatSnapshot) {
				slog.Info("その座席に少しでも変更が加えられているのでスルー")
				return nil
			}

			{
				ifSittingTooMuch, err := s.CheckIfUserSittingTooMuchForSeat(ctx, s.ProcessedUserId, seat.SeatId, isMemberSeat)
				if err != nil {
					return fmt.Errorf("%sさん（%s）の席移動処理中にエラーが発生しました: %w", s.ProcessedUserDisplayName, s.ProcessedUserId, err)
				}
				if ifSittingTooMuch {
					forcedMove = true
				}
			}

			return nil
		})
		if txErr != nil {
			s.MessageToOwnerWithError(ctx, "failed transaction in OrganizeDBForceMove", txErr)
			continue
		}
		if forcedMove { // 長時間入室制限による強制席移動。nested transactionとならないよう、RunTransactionの外側で実行
			var seatIdStr string
			if isMemberSeat {
				seatIdStr = i18n.T("common:vip-seat-id", seatSnapshot.SeatId)
			} else {
				seatIdStr = strconv.Itoa(seatSnapshot.SeatId)
			}
			s.MessageToLiveChat(ctx, i18n.T("others:force-move", s.ProcessedUserDisplayName, seatIdStr))

			inCommandDetails := &utils.CommandDetails{
				CommandType: utils.In,
				InOption: utils.InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &utils.MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         seatSnapshot.WorkName,
						DurationMin:      int(utils.NoNegativeDuration(seatSnapshot.Until.Sub(utils.JstNow())).Minutes()),
					},
					IsMemberSeat: isMemberSeat,
				},
			}
			if err := s.In(ctx, inCommandDetails); err != nil {
				return fmt.Errorf("%sさん（%s）の自動席移動処理中にエラーが発生しました: %w", s.ProcessedUserDisplayName, s.ProcessedUserId, err)
			}
		}
	}
	return nil
}

func (s *WorkspaceApp) DailyOrganizeDB(ctx context.Context) ([]string, error) {
	slog.Info(utils.NameOf(s.DailyOrganizeDB))
	var ownerMessage string

	slog.Info("一時的累計作業時間をリセット")
	dailyResetCount, err := s.ResetDailyTotalStudyTime(ctx)
	if err != nil {
		return []string{}, fmt.Errorf("in ResetDailyTotalStudyTime(): %w", err)
	}
	ownerMessage += "\nsuccessfully reset daily total study time. (" + strconv.Itoa(dailyResetCount) + " users)"

	slog.Info("RP関連の情報更新・ペナルティ処理を行うユーザーのIDのリストを取得")
	userIdsToProcessRP, err := s.GetUserIdsToProcessRP(ctx)
	if err != nil {
		return []string{}, fmt.Errorf("in GetUserIdsToProcessRP(): %w", err)
	}

	ownerMessage += "\n過去31日以内に入室した人数（RP処理対象）: " + strconv.Itoa(len(userIdsToProcessRP))
	ownerMessage += "\n本日のDailyOrganizeDB()処理が完了しました（RP更新処理以外）。"
	s.MessageToOwner(ctx, ownerMessage)
	slog.Info("finished " + utils.NameOf(s.DailyOrganizeDB))
	return userIdsToProcessRP, nil
}

func (s *WorkspaceApp) ResetDailyTotalStudyTime(ctx context.Context) (int, error) {
	slog.Info(utils.NameOf(s.ResetDailyTotalStudyTime))
	// 時間がかかる処理なのでトランザクションはなし
	previousDate := s.Configs.Constants.LastResetDailyTotalStudySec.In(utils.JapanLocation())
	now := utils.JstNow()
	isDifferentDay := now.Year() != previousDate.Year() || now.Month() != previousDate.Month() || now.Day() != previousDate.Day() // TODO: isDifferentDay := !utils.DateEqualJST(now, previousDate)
	if isDifferentDay && now.After(previousDate) {
		userIter := s.Repository.GetAllNonDailyZeroUserDocs(ctx)
		count := 0
		for {
			doc, err := userIter.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return 0, fmt.Errorf("in userIter.Next(): %w", err)
			}
			if err := s.Repository.ResetDailyTotalStudyTime(ctx, doc.Ref); err != nil {
				return 0, fmt.Errorf("in ResetDailyTotalStudyTime(): %w", err)
			}
			count += 1
		}
		if err := s.Repository.UpdateLastResetDailyTotalStudyTime(ctx, now); err != nil {
			return 0, fmt.Errorf("in UpdateLastResetDailyTotalStudyTime(): %w", err)
		}
		return count, nil
	} else {
		s.MessageToOwner(ctx, "all user's daily total study times are already reset today.")
		return 0, nil
	}
}

func (s *WorkspaceApp) UpdateUserRPBatch(ctx context.Context, userIds []string, timeLimitSeconds int) []string {
	jstNow := utils.JstNow()
	startTime := jstNow
	var doneUserIds []string
	for _, userId := range userIds {
		// 時間チェック
		duration := utils.JstNow().Sub(startTime)
		if int(duration.Seconds()) > timeLimitSeconds {
			return userIds
		}

		// 処理
		if err := s.UpdateUserRP(ctx, userId, jstNow); err != nil {
			s.MessageToOwnerWithError(ctx, "failed to UpdateUserRP, while processing "+userId, err)
			// pass. mark user as done
		}
		doneUserIds = append(doneUserIds, userId)
	}

	var remainingUserIds []string
	for _, userId := range userIds {
		if utils.Contains(doneUserIds, userId) {
			continue
		} else {
			remainingUserIds = append(remainingUserIds, userId)
		}
	}
	return remainingUserIds
}

func (s *WorkspaceApp) UpdateUserRP(ctx context.Context, userId string, jstNow time.Time) error {
	slog.Info("processing RP.", "userId", userId)
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := s.Repository.ReadUser(ctx, tx, userId)
		if err != nil {
			return fmt.Errorf("in ReadUser(): %w", err)
		}

		// 同日の重複処理防止チェック
		if utils.DateEqualJST(userDoc.LastRPProcessed, jstNow) {
			slog.Warn("user " + userId + " is already RP processed today, skipping.")
			return nil
		}

		lastPenaltyImposedDays, isContinuousActive, currentActivityStateStarted, rankPoint, err := utils.DailyUpdateRankPoint(
			userDoc.LastPenaltyImposedDays, userDoc.IsContinuousActive, userDoc.CurrentActivityStateStarted,
			userDoc.RankPoint, userDoc.LastEntered, userDoc.LastExited, jstNow)
		if err != nil {
			return fmt.Errorf("in DailyUpdateRankPoint(): %w", err)
		}

		// 変更項目がある場合のみ変更
		if lastPenaltyImposedDays != userDoc.LastPenaltyImposedDays {
			if err := s.Repository.UpdateUserLastPenaltyImposedDays(ctx, tx, userId, lastPenaltyImposedDays); err != nil {
				return fmt.Errorf("in UpdateUserLastPenaltyImposedDays(): %w", err)
			}
		}
		if isContinuousActive != userDoc.IsContinuousActive || !currentActivityStateStarted.Equal(userDoc.CurrentActivityStateStarted) {
			if err := s.Repository.UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(ctx, tx, userId, isContinuousActive, currentActivityStateStarted); err != nil {
				return fmt.Errorf("in UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(): %w", err)
			}
		}
		if rankPoint != userDoc.RankPoint {
			if err := s.Repository.UpdateUserRankPoint(tx, userId, rankPoint); err != nil {
				return fmt.Errorf("in UpdateUserRankPoint(): %w", err)
			}
		}

		if err := s.Repository.UpdateUserLastRPProcessed(tx, userId, jstNow); err != nil {
			return fmt.Errorf("in UpdateUserLastRPProcessed(): %w", err)
		}

		return nil
	})
}

func (s *WorkspaceApp) BackupCollectionHistoryFromGcsToBigquery(ctx context.Context, clientOption option.ClientOption) error {
	slog.Info(utils.NameOf(s.BackupCollectionHistoryFromGcsToBigquery))
	// 時間がかかる処理なのでトランザクションはなし
	previousDate := s.Configs.Constants.LastTransferCollectionHistoryBigquery.In(utils.JapanLocation())
	now := utils.JstNow()
	isDifferentDay := now.Year() != previousDate.Year() || now.Month() != previousDate.Month() || now.Day() != previousDate.Day()
	if isDifferentDay && now.After(previousDate) {
		gcsClient, err := mystorage.NewStorageClient(ctx, clientOption, s.Configs.Constants.GcpRegion)
		if err != nil {
			return fmt.Errorf("in NewStorageClient(): %w", err)
		}
		defer gcsClient.CloseClient()

		projectId, err := utils.GetGcpProjectId(ctx, clientOption)
		if err != nil {
			return fmt.Errorf("in GetGcpProjectId(): %w", err)
		}
		bqClient, err := mybigquery.NewBigqueryClient(ctx, projectId, clientOption, s.Configs.Constants.GcpRegion)
		if err != nil {
			return fmt.Errorf("in NewBigqueryClient(): %w", err)
		}
		defer bqClient.CloseClient()

		gcsTargetFolderName, err := gcsClient.GetGcsYesterdayExportFolderName(ctx, s.Configs.Constants.GcsFirestoreExportBucketName)
		if err != nil {
			return fmt.Errorf("in GetGcsYesterdayExportFolderName(): %w", err)
		}
		slog.Info("GCS folder name: " + gcsTargetFolderName)

		if err := bqClient.ReadCollectionsFromGcs(
			ctx,
			gcsTargetFolderName,
			s.Configs.Constants.GcsFirestoreExportBucketName,
			[]string{repository.LiveChatHistory, repository.UserActivities, repository.OrderHistory},
		); err != nil {
			return fmt.Errorf("in ReadCollectionsFromGcs(): %w", err)
		}
		slog.Info("successfully transfer yesterday's live chat history to bigquery.")

		// 一定期間前のライブチャットおよびユーザー行動ログを削除
		// 何日以降分を保持するか求める
		retentionFromDate := utils.JstNow().Add(-time.Duration(s.Configs.Constants.CollectionHistoryRetentionDays*24) * time.
			Hour)
		retentionFromDate = time.Date(retentionFromDate.Year(), retentionFromDate.Month(), retentionFromDate.Day(),
			0, 0, 0, 0, retentionFromDate.Location())

		// ライブチャット・ユーザー行動ログ削除
		numRowsLiveChat, numRowsUserActivity, numRowsOrderHistory, err := s.DeleteCollectionHistoryBeforeDate(ctx, retentionFromDate)
		if err != nil {
			return fmt.Errorf("in DeleteCollectionHistoryBeforeDate(): %w", err)
		}
		slog.Info(strconv.Itoa(int(retentionFromDate.Month()))+"月"+strconv.Itoa(retentionFromDate.Day())+
			"日より前の日付のライブチャット履歴およびユーザー行動ログをFirestoreから削除しました。",
			"削除したライブチャット件数", numRowsLiveChat,
			"削除したユーザー行動ログ件数", numRowsUserActivity,
			"削除した注文履歴件数", numRowsOrderHistory)

		if err := s.Repository.UpdateLastTransferCollectionHistoryBigquery(ctx, now); err != nil {
			return fmt.Errorf("in UpdateLastTransferCollectionHistoryBigquery(): %w", err)
		}
	} else {
		s.MessageToOwner(ctx, "yesterday's collection histories are already reset today.")
	}
	return nil
}
