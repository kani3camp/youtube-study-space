package core

import (
	"app.modules/core/i18n"
	"app.modules/core/mybigquery"
	"app.modules/core/myfirestore"
	"app.modules/core/mystorage"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"context"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"reflect"
	"strconv"
	"time"
)

// OrganizeDB 1分ごとに処理を行う。
// - 自動退室予定時刻(until)を過ぎているルーム内のユーザーを退室させる。
// - CurrentStateUntilを過ぎている休憩中のユーザーを作業再開させる。
// - 一時着席制限ブラックリスト・ホワイトリストのuntilを過ぎているドキュメントを削除する。
func (s *System) OrganizeDB(ctx context.Context, isMemberRoom bool) error {
	log.Println(utils.FuncNameOf(s.OrganizeDB), "isMemberRoom:", isMemberRoom)
	var err error

	log.Println("自動退室")
	// 全座席のスナップショットをとる（トランザクションなし）
	err = s.OrganizeDBAutoExit(ctx, isMemberRoom)
	if err != nil {
		s.MessageToOwnerWithError("failed to OrganizeDBAutoExit", err)
		return err
	}

	log.Println("作業再開")
	err = s.OrganizeDBResume(ctx, isMemberRoom)
	if err != nil {
		s.MessageToOwnerWithError("failed to OrganizeDBResume", err)
		return err
	}

	log.Println("一時着席制限ブラックリスト・ホワイトリストのクリーニング")
	err = s.OrganizeDBDeleteExpiredSeatLimits(ctx, isMemberRoom)
	if err != nil {
		s.MessageToOwnerWithError("failed to OrganizeDBDeleteExpiredSeatLimits", err)
		return err
	}

	return nil
}

func (s *System) OrganizeDBAutoExit(ctx context.Context, isMemberRoom bool) error {
	jstNow := utils.JstNow()
	candidateSeatsSnapshot, err := s.FirestoreController.ReadSeatsExpiredUntil(ctx, jstNow, isMemberRoom)
	if err != nil {
		s.MessageToOwnerWithError("failed to ReadGeneralSeats", err)
		return err
	}
	log.Println("自動退室候補" + strconv.Itoa(len(candidateSeatsSnapshot)) + "人")

	for _, seatSnapshot := range candidateSeatsSnapshot {
		liveChatMessage := ""
		err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			s.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, seatSnapshot.UserProfileImageUrl, false, false, isMemberRoom)

			// 現在も存在しているか
			seat, err := s.FirestoreController.ReadSeat(ctx, tx, seatSnapshot.SeatId, isMemberRoom)
			if err != nil {
				if status.Code(err) == codes.NotFound {
					log.Println("すぐ前に退室したということなのでスルー")
					return nil
				}
				s.MessageToOwnerWithError("failed to ReadSeat", err)
				return err
			}
			if !reflect.DeepEqual(seat, seatSnapshot) {
				log.Println("その座席に少しでも変更が加えられているのでスルー")
				return nil
			}

			userDoc, err := s.FirestoreController.ReadUser(ctx, tx, s.ProcessedUserId)
			if err != nil {
				s.MessageToOwnerWithError("failed to ReadUser", err)
				return err
			}

			autoExit := seat.Until.Before(utils.JstNow()) // 自動退室時刻を過ぎていたら自動退室

			// 以下書き込みのみ

			// 自動退室時刻による退室処理
			if autoExit {
				workedTimeSec, addedRP, err := s.exitRoom(ctx, tx, isMemberRoom, seat, &userDoc)
				if err != nil {
					s.MessageToOwnerWithError(s.ProcessedUserDisplayName+"さん（"+s.ProcessedUserId+"）の退室処理中にエラーが発生しました", err)
					return err
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
		if err != nil {
			s.MessageToOwnerWithError("failed transaction", err)
			continue // err != nil でもreturnではなく次に進む
		}
		if liveChatMessage != "" {
			s.MessageToLiveChat(ctx, liveChatMessage)
		}
	}
	return nil
}

func (s *System) OrganizeDBResume(ctx context.Context, isMemberRoom bool) error {
	jstNow := utils.JstNow()
	candidateSeatsSnapshot, err := s.FirestoreController.ReadSeatsExpiredBreakUntil(ctx, jstNow, isMemberRoom)
	if err != nil {
		s.MessageToOwnerWithError("failed to ReadGeneralSeats", err)
		return err
	}
	log.Println("作業再開候補" + strconv.Itoa(len(candidateSeatsSnapshot)) + "人")

	for _, seatSnapshot := range candidateSeatsSnapshot {
		liveChatMessage := ""
		err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			s.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, seatSnapshot.UserProfileImageUrl, false, false, isMemberRoom)

			// 現在も存在しているか
			seat, err := s.FirestoreController.ReadSeat(ctx, tx, seatSnapshot.SeatId, isMemberRoom)
			if err != nil {
				if status.Code(err) == codes.NotFound {
					log.Println("すぐ前に退室したということなのでスルー")
					return nil
				}
				s.MessageToOwnerWithError("failed to ReadSeat", err)
				return err
			}
			if !reflect.DeepEqual(seat, seatSnapshot) {
				log.Println("その座席に少しでも変更が加えられているのでスルー")
				return nil
			}

			resume := seat.State == myfirestore.BreakState && seat.CurrentStateUntil.Before(utils.JstNow())

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

				seat.State = myfirestore.WorkState
				seat.CurrentStateStartedAt = jstNow
				seat.CurrentStateUntil = until
				seat.DailyCumulativeWorkSec = dailyCumulativeWorkSec
				err = s.FirestoreController.UpdateSeat(ctx, tx, seat, isMemberRoom)
				if err != nil {
					s.MessageToOwnerWithError("failed to s.FirestoreController.UpdateSeat", err)
					return err
				}
				// activityログ記録
				endBreakActivity := myfirestore.UserActivityDoc{
					UserId:       s.ProcessedUserId,
					ActivityType: myfirestore.EndBreakActivity,
					SeatId:       seat.SeatId,
					IsMemberSeat: isMemberRoom,
					TakenAt:      utils.JstNow(),
				}
				err = s.FirestoreController.CreateUserActivityDoc(ctx, tx, endBreakActivity)
				if err != nil {
					s.MessageToOwnerWithError("failed to add an user activity", err)
					return err
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
		if err != nil {
			s.MessageToOwnerWithError("failed transaction", err)
			continue // err != nil でもreturnではなく次に進む
		}
		if liveChatMessage != "" {
			s.MessageToLiveChat(ctx, liveChatMessage)
		}
	}
	return nil
}

func (s *System) OrganizeDBDeleteExpiredSeatLimits(ctx context.Context, isMemberRoom bool) error {
	jstNow := utils.JstNow()
	// white list
	for {
		iter := s.FirestoreController.Get500SeatLimitsAfterUntilInWHITEList(ctx, jstNow, isMemberRoom)
		count, err := s.DeleteIteratorDocs(ctx, iter)
		if err != nil {
			return err
		}
		if count == 0 {
			break
		}
	}

	// black list
	for {
		iter := s.FirestoreController.Get500SeatLimitsAfterUntilInBLACKList(ctx, jstNow, isMemberRoom)
		count, err := s.DeleteIteratorDocs(ctx, iter)
		if err != nil {
			return err
		}
		if count == 0 {
			break
		}
	}
	return nil
}

func (s *System) OrganizeDBForceMove(ctx context.Context, seatsSnapshot []myfirestore.SeatDoc, isMemberSeat bool) error {
	log.Println(strconv.Itoa(len(seatsSnapshot)) + "人")
	for _, seatSnapshot := range seatsSnapshot {
		var forcedMove bool // 長時間入室制限による強制席移動
		err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			s.SetProcessedUser(seatSnapshot.UserId, seatSnapshot.UserDisplayName, seatSnapshot.UserProfileImageUrl, false, false, isMemberSeat)

			// 現在も存在しているか
			seat, err := s.FirestoreController.ReadSeat(ctx, tx, seatSnapshot.SeatId, isMemberSeat)
			if err != nil {
				if status.Code(err) == codes.NotFound {
					log.Println("すぐ前に退室したということなのでスルー")
					return nil
				}
				s.MessageToOwnerWithError("failed to ReadSeat", err)
				return err
			}
			if !reflect.DeepEqual(seat, seatSnapshot) {
				log.Println("その座席に少しでも変更が加えられているのでスルー")
				return nil
			}

			ifSittingTooMuch, err := s.CheckIfUserSittingTooMuchForSeat(ctx, s.ProcessedUserId, seat.SeatId, isMemberSeat)
			if err != nil {
				s.MessageToOwnerWithError(s.ProcessedUserDisplayName+"さん（"+s.ProcessedUserId+"）の席移動処理中にエラーが発生しました", err)
				return err
			}
			if ifSittingTooMuch {
				forcedMove = true
			}

			return nil
		})
		if err != nil {
			s.MessageToOwnerWithError("failed transaction in OrganizeDBForceMove", err)
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
			err = s.In(ctx, inCommandDetails)
			if err != nil {
				s.MessageToOwnerWithError(s.ProcessedUserDisplayName+"さん（"+s.ProcessedUserId+"）の自動席移動処理中にエラーが発生しました", err)
				return err
			}
		}
	}
	return nil
}

func (s *System) DailyOrganizeDB(ctx context.Context) ([]string, error) {
	log.Println("DailyOrganizeDB()")
	var lineMessage string

	log.Println("一時的累計作業時間をリセット")
	dailyResetCount, err := s.ResetDailyTotalStudyTime(ctx)
	if err != nil {
		s.MessageToOwnerWithError("failed to ResetDailyTotalStudyTime", err)
		return []string{}, err
	}
	lineMessage += "\nsuccessfully reset daily total study time. (" + strconv.Itoa(dailyResetCount) + " users)"

	log.Println("RP関連の情報更新・ペナルティ処理を行うユーザーのIDのリストを取得")
	err, userIdsToProcessRP := s.GetUserIdsToProcessRP(ctx)
	if err != nil {
		s.MessageToOwnerWithError("failed to GetUserIdsToProcessRP", err)
		return []string{}, err
	}

	lineMessage += "\n過去31日以内に入室した人数（RP処理対象）: " + strconv.Itoa(len(userIdsToProcessRP))
	lineMessage += "\n本日のDailyOrganizeDatabase()処理が完了しました（RP更新処理以外）。"
	s.MessageToOwner(lineMessage)
	log.Println("finished DailyOrganizeDB().")
	return userIdsToProcessRP, nil
}

func (s *System) ResetDailyTotalStudyTime(ctx context.Context) (int, error) {
	log.Println("ResetDailyTotalStudyTime()")
	// 時間がかかる処理なのでトランザクションはなし
	previousDate := s.Configs.Constants.LastResetDailyTotalStudySec.In(utils.JapanLocation())
	now := utils.JstNow()
	isDifferentDay := now.Year() != previousDate.Year() || now.Month() != previousDate.Month() || now.Day() != previousDate.Day()
	if isDifferentDay && now.After(previousDate) {
		userIter := s.FirestoreController.GetAllNonDailyZeroUserDocs(ctx)
		count := 0
		for {
			doc, err := userIter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return 0, err
			}
			err = s.FirestoreController.ResetDailyTotalStudyTime(ctx, doc.Ref)
			if err != nil {
				return 0, err
			}
			count += 1
		}
		err := s.FirestoreController.UpdateLastResetDailyTotalStudyTime(ctx, now)
		if err != nil {
			s.MessageToOwnerWithError("failed to UpdateLastResetDailyTotalStudyTime", err)
			return 0, err
		}
		return count, nil
	} else {
		s.MessageToOwner("all user's daily total study times are already reset today.")
		return 0, nil
	}
}

func (s *System) UpdateUserRPBatch(ctx context.Context, userIds []string, timeLimitSeconds int) ([]string, error) {
	jstNow := utils.JstNow()
	startTime := jstNow
	var doneUserIds []string
	for _, userId := range userIds {
		// 時間チェック
		duration := utils.JstNow().Sub(startTime)
		if int(duration.Seconds()) > timeLimitSeconds {
			return userIds, nil
		}

		// 処理
		err := s.UpdateUserRP(ctx, userId, jstNow)
		if err != nil {
			s.MessageToOwnerWithError("failed to UpdateUserRP, while processing "+userId, err)
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
	return remainingUserIds, nil
}

func (s *System) UpdateUserRP(ctx context.Context, userId string, jstNow time.Time) error {
	log.Println("[userId: " + userId + "] processing RP.")
	return s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := s.FirestoreController.ReadUser(ctx, tx, userId)
		if err != nil {
			s.MessageToOwnerWithError("failed to ReadUser", err)
			return err
		}

		// 同日の重複処理防止チェック
		if utils.DateEqualJST(userDoc.LastRPProcessed, jstNow) {
			log.Println("user " + userId + " is already RP processed today, skipping.")
			return nil
		}

		lastPenaltyImposedDays, isContinuousActive, currentActivityStateStarted, rankPoint, err := utils.DailyUpdateRankPoint(
			userDoc.LastPenaltyImposedDays, userDoc.IsContinuousActive, userDoc.CurrentActivityStateStarted,
			userDoc.RankPoint, userDoc.LastEntered, userDoc.LastExited, jstNow)
		if err != nil {
			s.MessageToOwnerWithError("failed to DailyUpdateRankPoint", err)
			return err
		}

		// 変更項目がある場合のみ変更
		if lastPenaltyImposedDays != userDoc.LastPenaltyImposedDays {
			err := s.FirestoreController.UpdateUserLastPenaltyImposedDays(ctx, tx, userId, lastPenaltyImposedDays)
			if err != nil {
				s.MessageToOwnerWithError("failed to UpdateUserLastPenaltyImposedDays", err)
				return err
			}
		}
		if isContinuousActive != userDoc.IsContinuousActive || !currentActivityStateStarted.Equal(userDoc.CurrentActivityStateStarted) {
			err := s.FirestoreController.UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(ctx, tx, userId, isContinuousActive, currentActivityStateStarted)
			if err != nil {
				s.MessageToOwnerWithError("failed to UpdateUserIsContinuousActiveAndCurrentActivityStateStarted", err)
				return err
			}
		}
		if rankPoint != userDoc.RankPoint {
			err := s.FirestoreController.UpdateUserRankPoint(tx, userId, rankPoint)
			if err != nil {
				s.MessageToOwnerWithError("failed to UpdateUserRankPoint", err)
				return err
			}
		}

		err = s.FirestoreController.UpdateUserLastRPProcessed(tx, userId, jstNow)
		if err != nil {
			s.MessageToOwnerWithError("failed to UpdateUserLastRPProcessed", err)
			return err
		}

		return nil
	})
}

func (s *System) BackupCollectionHistoryFromGcsToBigquery(ctx context.Context, clientOption option.ClientOption) error {
	log.Println("BackupCollectionHistoryFromGcsToBigquery()")
	// 時間がかかる処理なのでトランザクションはなし
	previousDate := s.Configs.Constants.LastTransferCollectionHistoryBigquery.In(utils.JapanLocation())
	now := utils.JstNow()
	isDifferentDay := now.Year() != previousDate.Year() || now.Month() != previousDate.Month() || now.Day() != previousDate.Day()
	if isDifferentDay && now.After(previousDate) {
		gcsClient, err := mystorage.NewStorageClient(ctx, clientOption, s.Configs.Constants.GcpRegion)
		if err != nil {
			return err
		}
		defer gcsClient.CloseClient()

		projectId, err := utils.GetGcpProjectId(ctx, clientOption)
		if err != nil {
			return err
		}
		bqClient, err := mybigquery.NewBigqueryClient(ctx, projectId, clientOption, s.Configs.Constants.GcpRegion)
		if err != nil {
			return err
		}
		defer bqClient.CloseClient()

		gcsTargetFolderName, err := gcsClient.GetGcsYesterdayExportFolderName(ctx, s.Configs.Constants.GcsFirestoreExportBucketName)
		if err != nil {
			return err
		}

		err = bqClient.ReadCollectionsFromGcs(ctx, gcsTargetFolderName, s.Configs.Constants.GcsFirestoreExportBucketName,
			[]string{myfirestore.LiveChatHistory, myfirestore.UserActivities})
		if err != nil {
			return err
		}
		log.Println("successfully transfer yesterday's live chat history to bigquery.")

		// 一定期間前のライブチャットおよびユーザー行動ログを削除
		// 何日以降分を保持するか求める
		retentionFromDate := utils.JstNow().Add(-time.Duration(s.Configs.Constants.CollectionHistoryRetentionDays*24) * time.
			Hour)
		retentionFromDate = time.Date(retentionFromDate.Year(), retentionFromDate.Month(), retentionFromDate.Day(),
			0, 0, 0, 0, retentionFromDate.Location())

		// ライブチャット・ユーザー行動ログ削除
		numRowsLiveChat, numRowsUserActivity, err := s.DeleteCollectionHistoryBeforeDate(ctx, retentionFromDate)
		if err != nil {
			return err
		}
		log.Println(strconv.Itoa(int(retentionFromDate.Month())) + "月" + strconv.Itoa(retentionFromDate.Day()) +
			"日より前の日付のライブチャット履歴およびユーザー行動ログをFirestoreから削除しました。")
		log.Printf("削除したライブチャット件数: %d\n削除したユーザー行動ログ件数: %d\n", numRowsLiveChat, numRowsUserActivity)

		err = s.FirestoreController.UpdateLastTransferCollectionHistoryBigquery(ctx, now)
		if err != nil {
			return err
		}
	} else {
		s.MessageToOwner("yesterday's collection histories are already reset today.")
	}
	return nil
}
