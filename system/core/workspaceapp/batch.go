package workspaceapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"time"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/mybigquery"
	"app.modules/core/mystorage"
	"app.modules/core/repository"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp/presenter"
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
func (app *WorkspaceApp) OrganizeDB(ctx context.Context, isMemberRoom bool) error {
	slog.Info(utils.NameOf(app.OrganizeDB), "isMemberRoom", isMemberRoom)

	slog.Info("自動退室")
	// 全座席のスナップショットをとる（トランザクションなし）
	if err := app.OrganizeDBAutoExit(ctx, isMemberRoom); err != nil {
		return fmt.Errorf("in OrganizeDBAutoExit(): %w", err)
	}

	slog.Info("作業再開")
	if err := app.OrganizeDBResume(ctx, isMemberRoom); err != nil {
		return fmt.Errorf("in OrganizeDBResume(): %w", err)
	}

	slog.Info("一時着席制限ブラックリスト・ホワイトリストのクリーニング")
	if err := app.OrganizeDBDeleteExpiredSeatLimits(ctx, isMemberRoom); err != nil {
		return fmt.Errorf("in OrganizeDBDeleteExpiredSeatLimits(): %w", err)
	}

	return nil
}

func (app *WorkspaceApp) OrganizeDBAutoExit(ctx context.Context, isMemberRoom bool) error {
	jstNow := utils.JstNow()
	candidateSeatsSnapshot, err := app.Repository.ReadSeatsExpiredUntil(ctx, jstNow, isMemberRoom)
	if err != nil {
		return fmt.Errorf("in ReadSeatsExpiredUntil(): %w", err)
	}
	slog.Info("自動退室候補" + strconv.Itoa(len(candidateSeatsSnapshot)) + "人")

	for _, seatSnapshot := range candidateSeatsSnapshot {
		liveChatMessage := ""
		txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			app.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, seatSnapshot.UserProfileImageUrl, false, false, isMemberRoom)

			// 現在も存在しているか
			seat, err := app.Repository.ReadSeat(ctx, tx, seatSnapshot.SeatId, isMemberRoom)
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

			userDoc, err := app.Repository.ReadUser(ctx, tx, app.ProcessedUserId)
			if err != nil {
				return fmt.Errorf("in ReadUser(): %w", err)
			}

			autoExit := seat.Until.Before(utils.JstNow()) // 自動退室時刻を過ぎていたら自動退室

			// 以下書き込みのみ

			// 自動退室時刻による退室処理
			if autoExit {
				workedTimeSec, addedRP, err := app.exitRoom(ctx, tx, isMemberRoom, seat, &userDoc)
				if err != nil {
					return fmt.Errorf("%sさん（%s）の退室処理中にエラーが発生しました: %w", app.ProcessedUserDisplayName, app.ProcessedUserId, err)
				}
				var rpEarned string
				if userDoc.RankVisible {
					rpEarned = i18nmsg.CommandRpEarned(addedRP)
				}
				seatIdStr := presenter.SeatIDStr(seat.SeatId, isMemberRoom)
				liveChatMessage = i18nmsg.CommandExit(app.ProcessedUserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)
			}

			return nil
		})
		if txErr != nil {
			app.MessageToOwnerWithError(ctx, "failed transaction", txErr)
			continue // txErr != nil でもreturnではなく次に進む
		}
		if liveChatMessage != "" {
			app.MessageToLiveChat(ctx, liveChatMessage)
		}
	}
	return nil
}

func (app *WorkspaceApp) OrganizeDBResume(ctx context.Context, isMemberRoom bool) error {
	jstNow := utils.JstNow()
	candidateSeatsSnapshot, err := app.Repository.ReadSeatsExpiredBreakUntil(ctx, jstNow, isMemberRoom)
	if err != nil {
		return fmt.Errorf("in ReadSeatsExpiredBreakUntil(): %w", err)
	}
	slog.Info("作業再開候補" + strconv.Itoa(len(candidateSeatsSnapshot)) + "人")

	for _, seatSnapshot := range candidateSeatsSnapshot {
		liveChatMessage := ""
		txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			app.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, seatSnapshot.UserProfileImageUrl, false, false, isMemberRoom)

			// 現在も存在しているか
			seat, err := app.Repository.ReadSeat(ctx, tx, seatSnapshot.SeatId, isMemberRoom)
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
				if err := app.Repository.UpdateSeat(ctx, tx, seat, isMemberRoom); err != nil {
					return fmt.Errorf("in UpdateSeat(): %w", err)
				}
				// activityログ記録
				endBreakActivity := repository.UserActivityDoc{
					UserId:       app.ProcessedUserId,
					ActivityType: repository.EndBreakActivity,
					SeatId:       seat.SeatId,
					IsMemberSeat: isMemberRoom,
					TakenAt:      utils.JstNow(),
				}
				if err := app.Repository.CreateUserActivityDoc(ctx, tx, endBreakActivity); err != nil {
					return fmt.Errorf("in CreateUserActivityDoc(): %w", err)
				}
				seatIdStr := presenter.SeatIDStr(seat.SeatId, isMemberRoom)

				liveChatMessage = i18nmsg.CommandResumeWork(app.ProcessedUserDisplayName, seatIdStr, int(utils.NoNegativeDuration(until.Sub(jstNow)).Minutes()))
			}
			return nil
		})
		if txErr != nil {
			app.MessageToOwnerWithError(ctx, "failed transaction", txErr)
			continue // txErr != nil でもreturnではなく次に進む
		}
		if liveChatMessage != "" {
			app.MessageToLiveChat(ctx, liveChatMessage)
		}
	}
	return nil
}

func (app *WorkspaceApp) OrganizeDBDeleteExpiredSeatLimits(ctx context.Context, isMemberRoom bool) error {
	jstNow := utils.JstNow()
	// white list
	for {
		iter := app.Repository.Get500SeatLimitsAfterUntilInWHITEList(ctx, jstNow, isMemberRoom)
		count, err := app.DeleteIteratorDocs(ctx, iter)
		if err != nil {
			return fmt.Errorf("in DeleteIteratorDocs(): %w", err)
		}
		if count == 0 {
			break
		}
	}

	// black list
	for {
		iter := app.Repository.Get500SeatLimitsAfterUntilInBLACKList(ctx, jstNow, isMemberRoom)
		count, err := app.DeleteIteratorDocs(ctx, iter)
		if err != nil {
			return fmt.Errorf("in DeleteIteratorDocs(): %w", err)
		}
		if count == 0 {
			break
		}
	}
	return nil
}

func (app *WorkspaceApp) OrganizeDBForceMove(ctx context.Context, seatsSnapshot []repository.SeatDoc, isMemberSeat bool) error {
	slog.Info(utils.NameOf(app.OrganizeDBForceMove), "isMemberSeat", isMemberSeat, "len(seatsSnapshot)", len(seatsSnapshot))
	for _, seatSnapshot := range seatsSnapshot {
		var forcedMove bool // 長時間入室制限による強制席移動
		txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			app.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, seatSnapshot.UserProfileImageUrl, false, false, isMemberSeat)

			// 現在も存在しているか
			seat, err := app.Repository.ReadSeat(ctx, tx, seatSnapshot.SeatId, isMemberSeat)
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
				ifSittingTooMuch, err := app.CheckIfUserSittingTooMuchForSeat(ctx, app.ProcessedUserId, seat.SeatId, isMemberSeat)
				if err != nil {
					return fmt.Errorf("%sさん（%s）の席移動処理中にエラーが発生しました: %w", app.ProcessedUserDisplayName, app.ProcessedUserId, err)
				}
				if ifSittingTooMuch {
					forcedMove = true
				}
			}

			return nil
		})
		if txErr != nil {
			app.MessageToOwnerWithError(ctx, "failed transaction in OrganizeDBForceMove", txErr)
			continue
		}
		if forcedMove { // 長時間入室制限による強制席移動。nested transactionとならないよう、RunTransactionの外側で実行
			seatIdStr := presenter.SeatIDStr(seatSnapshot.SeatId, isMemberSeat)
			app.MessageToLiveChat(ctx, i18nmsg.OthersForceMove(app.ProcessedUserDisplayName, seatIdStr))

			var isOrderSet bool
			var menuNum int
			if seatSnapshot.MenuCode != "" {
				var err error
				menuNum, err = app.GetMenuNumByCode(seatSnapshot.MenuCode)
				if err != nil {
					return fmt.Errorf("in GetMenuNumByCode(): %w", err)
				}
			}

			inCommandDetails := &utils.CommandDetails{
				CommandType: utils.In,
				InOption: utils.InOption{
					IsSeatIdSet: true,
					SeatId:      0,
					MinWorkOrderOption: &utils.MinWorkOrderOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						IsOrderSet:       isOrderSet,
						WorkName:         seatSnapshot.WorkName,
						DurationMin:      int(utils.NoNegativeDuration(seatSnapshot.Until.Sub(utils.JstNow())).Minutes()),
						OrderNum:         menuNum,
					},
					IsMemberSeat: isMemberSeat,
				},
			}
			if err := app.In(ctx, &inCommandDetails.InOption); err != nil {
				return fmt.Errorf("%sさん（%s）の自動席移動処理中にエラーが発生しました: %w", app.ProcessedUserDisplayName, app.ProcessedUserId, err)
			}
		}
	}
	return nil
}

func (app *WorkspaceApp) DailyOrganizeDB(ctx context.Context) ([]string, error) {
	slog.Info(utils.NameOf(app.DailyOrganizeDB))
	var ownerMessage string

	slog.Info("一時的累計作業時間をリセット")
	dailyResetCount, err := app.ResetDailyTotalStudyTime(ctx)
	if err != nil {
		return []string{}, fmt.Errorf("in ResetDailyTotalStudyTime(): %w", err)
	}
	ownerMessage += "\nsuccessfully reset daily total study time. (" + strconv.Itoa(dailyResetCount) + " users)"

	slog.Info("RP関連の情報更新・ペナルティ処理を行うユーザーのIDのリストを取得")
	userIdsToProcessRP, err := app.GetUserIdsToProcessRP(ctx)
	if err != nil {
		return []string{}, fmt.Errorf("in GetUserIdsToProcessRP(): %w", err)
	}

	ownerMessage += "\n過去31日以内に入室した人数（RP処理対象）: " + strconv.Itoa(len(userIdsToProcessRP))
	ownerMessage += "\n本日のDailyOrganizeDB()処理が完了しました（RP更新処理以外）。"
	app.MessageToOwner(ctx, ownerMessage)
	slog.Info("finished " + utils.NameOf(app.DailyOrganizeDB))
	return userIdsToProcessRP, nil
}

func (app *WorkspaceApp) ResetDailyTotalStudyTime(ctx context.Context) (int, error) {
	slog.Info(utils.NameOf(app.ResetDailyTotalStudyTime))
	// 時間がかかる処理なのでトランザクションはなし
	previousDate := app.Configs.Constants.LastResetDailyTotalStudySec.In(utils.JapanLocation())
	now := utils.JstNow()
	isDifferentDay := now.Year() != previousDate.Year() || now.Month() != previousDate.Month() || now.Day() != previousDate.Day() // TODO: isDifferentDay := !utils.DateEqualJST(now, previousDate)
	if isDifferentDay && now.After(previousDate) {
		userIter := app.Repository.GetAllNonDailyZeroUserDocs(ctx)
		count := 0
		for {
			doc, err := userIter.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return 0, fmt.Errorf("in userIter.Next(): %w", err)
			}
			if err := app.Repository.ResetDailyTotalStudyTime(ctx, doc.Ref); err != nil {
				return 0, fmt.Errorf("in ResetDailyTotalStudyTime(): %w", err)
			}
			count += 1
		}
		if err := app.Repository.UpdateLastResetDailyTotalStudyTime(ctx, now); err != nil {
			return 0, fmt.Errorf("in UpdateLastResetDailyTotalStudyTime(): %w", err)
		}
		return count, nil
	} else {
		app.MessageToOwner(ctx, "all user's daily total study times are already reset today.")
		return 0, nil
	}
}

func (app *WorkspaceApp) UpdateUserRPBatch(ctx context.Context, userIds []string, timeLimitSeconds int) []string {
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
		if err := app.UpdateUserRP(ctx, userId, jstNow); err != nil {
			app.MessageToOwnerWithError(ctx, "failed to UpdateUserRP, while processing "+userId, err)
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

func (app *WorkspaceApp) UpdateUserRP(ctx context.Context, userId string, jstNow time.Time) error {
	slog.Info("processing RP.", "userId", userId)
	return app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := app.Repository.ReadUser(ctx, tx, userId)
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
			if err := app.Repository.UpdateUserLastPenaltyImposedDays(ctx, tx, userId, lastPenaltyImposedDays); err != nil {
				return fmt.Errorf("in UpdateUserLastPenaltyImposedDays(): %w", err)
			}
		}
		if isContinuousActive != userDoc.IsContinuousActive || !currentActivityStateStarted.Equal(userDoc.CurrentActivityStateStarted) {
			if err := app.Repository.UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(ctx, tx, userId, isContinuousActive, currentActivityStateStarted); err != nil {
				return fmt.Errorf("in UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(): %w", err)
			}
		}
		if rankPoint != userDoc.RankPoint {
			if err := app.Repository.UpdateUserRankPoint(tx, userId, rankPoint); err != nil {
				return fmt.Errorf("in UpdateUserRankPoint(): %w", err)
			}
		}

		if err := app.Repository.UpdateUserLastRPProcessed(tx, userId, jstNow); err != nil {
			return fmt.Errorf("in UpdateUserLastRPProcessed(): %w", err)
		}

		return nil
	})
}

func (app *WorkspaceApp) BackupCollectionHistoryFromGcsToBigquery(ctx context.Context, clientOption option.ClientOption) error {
	slog.Info(utils.NameOf(app.BackupCollectionHistoryFromGcsToBigquery))
	// 時間がかかる処理なのでトランザクションはなし
	previousDate := app.Configs.Constants.LastTransferCollectionHistoryBigquery.In(utils.JapanLocation())
	now := utils.JstNow()
	isDifferentDay := now.Year() != previousDate.Year() || now.Month() != previousDate.Month() || now.Day() != previousDate.Day()
	if isDifferentDay && now.After(previousDate) {
		gcsClient, err := mystorage.NewStorageClient(ctx, clientOption, app.Configs.Constants.GcpRegion)
		if err != nil {
			return fmt.Errorf("in NewStorageClient(): %w", err)
		}
		defer gcsClient.CloseClient()

		projectId, err := utils.GetGcpProjectId(ctx, clientOption)
		if err != nil {
			return fmt.Errorf("in GetGcpProjectId(): %w", err)
		}
		bqClient, err := mybigquery.NewBigqueryClient(ctx, projectId, clientOption, app.Configs.Constants.GcpRegion)
		if err != nil {
			return fmt.Errorf("in NewBigqueryClient(): %w", err)
		}
		defer bqClient.CloseClient()

		gcsTargetFolderName, err := gcsClient.GetGcsYesterdayExportFolderName(ctx, app.Configs.Constants.GcsFirestoreExportBucketName)
		if err != nil {
			return fmt.Errorf("in GetGcsYesterdayExportFolderName(): %w", err)
		}
		slog.Info("GCS folder name: " + gcsTargetFolderName)

		if err := bqClient.ReadCollectionsFromGcs(
			ctx,
			gcsTargetFolderName,
			app.Configs.Constants.GcsFirestoreExportBucketName,
			[]string{repository.LiveChatHistory, repository.UserActivities, repository.OrderHistory},
		); err != nil {
			return fmt.Errorf("in ReadCollectionsFromGcs(): %w", err)
		}
		slog.Info("successfully transfer yesterday's live chat history to bigquery.")

		// 一定期間前のライブチャットおよびユーザー行動ログを削除
		// 何日以降分を保持するか求める
		retentionFromDate := utils.JstNow().Add(-time.Duration(app.Configs.Constants.CollectionHistoryRetentionDays*24) * time.
			Hour)
		retentionFromDate = time.Date(retentionFromDate.Year(), retentionFromDate.Month(), retentionFromDate.Day(),
			0, 0, 0, 0, retentionFromDate.Location())

		// ライブチャット・ユーザー行動ログ削除
		numRowsLiveChat, numRowsUserActivity, numRowsOrderHistory, err := app.DeleteCollectionHistoryBeforeDate(ctx, retentionFromDate)
		if err != nil {
			return fmt.Errorf("in DeleteCollectionHistoryBeforeDate(): %w", err)
		}
		slog.Info(strconv.Itoa(int(retentionFromDate.Month()))+"月"+strconv.Itoa(retentionFromDate.Day())+
			"日より前の日付のライブチャット履歴およびユーザー行動ログをFirestoreから削除しました。",
			"削除したライブチャット件数", numRowsLiveChat,
			"削除したユーザー行動ログ件数", numRowsUserActivity,
			"削除した注文履歴件数", numRowsOrderHistory)

		if err := app.Repository.UpdateLastTransferCollectionHistoryBigquery(ctx, now); err != nil {
			return fmt.Errorf("in UpdateLastTransferCollectionHistoryBigquery(): %w", err)
		}
	} else {
		app.MessageToOwner(ctx, "yesterday's collection histories are already reset today.")
	}
	return nil
}
