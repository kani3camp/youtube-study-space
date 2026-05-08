package workspaceapp

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"time"

	"errors"

	"app.modules/core/guardians"
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/repository"
	"app.modules/core/studyspaceerror"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp/presenter"
	"app.modules/core/youtubebot"
	"cloud.google.com/go/firestore"
	"github.com/kr/pretty"
	"google.golang.org/api/iterator"
	"google.golang.org/api/youtube/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsSeatExist 席番号1～max-seatsの席かどうかを判定。
func (app *WorkspaceApp) IsSeatExist(ctx context.Context, seatID int, isMemberSeat bool) (bool, error) {
	realtimeConstants, err := app.Repository.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("in ReadSystemConstantsConfig: %w", err)
	}
	if isMemberSeat {
		return 1 <= seatID && seatID <= realtimeConstants.MemberMaxSeats, nil
	} else {
		return 1 <= seatID && seatID <= realtimeConstants.MaxSeats, nil
	}
}

// IfSeatVacant 席番号がseatIDの席が空いているかどうか。
func (app *WorkspaceApp) IfSeatVacant(ctx context.Context, tx *firestore.Transaction, seatID int, isMemberSeat bool) (bool, error) {
	_, err := app.Repository.ReadSeat(ctx, tx, seatID, isMemberSeat)
	if err != nil {
		if status.Code(err) == codes.NotFound { // その座席のドキュメントは存在しない
			// maxSeats以内かどうか
			isExist, err := app.IsSeatExist(ctx, seatID, isMemberSeat)
			if err != nil {
				return false, fmt.Errorf("in IsSeatExist: %w", err)
			}
			return isExist, nil
		}
		return false, fmt.Errorf("in ReadSeat: %w", err)
	}
	// ここまで来ると指定された番号の席が使われてるということ
	return false, nil
}

func (app *WorkspaceApp) IfUserRegistered(ctx context.Context, tx *firestore.Transaction) (bool, error) {
	_, err := app.Repository.ReadUser(ctx, tx, app.ProcessedUserID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		} else {
			return false, fmt.Errorf("in ReadUser: %w", err)
		}
	}
	return true, nil
}

// IsUserInRoom そのユーザーがルーム内にいるか？登録済みかに関わらず。
func (app *WorkspaceApp) IsUserInRoom(ctx context.Context, userID string) (isInMemberRoom bool, isInGeneralRoom bool, returnErr error) {
	isInMemberRoom = true
	isInGeneralRoom = true
	if _, err := app.Repository.ReadSeatWithUserID(ctx, userID, true); err != nil {
		if status.Code(err) == codes.NotFound {
			isInMemberRoom = false
		} else {
			return false, false, fmt.Errorf("in ReadSeatWithUserID: %w", err)
		}
	}
	if _, err := app.Repository.ReadSeatWithUserID(ctx, userID, false); err != nil {
		if status.Code(err) == codes.NotFound {
			isInGeneralRoom = false
		} else {
			return false, false, fmt.Errorf("in ReadSeatWithUserID: %w", err)
		}
	}
	if isInGeneralRoom && isInMemberRoom {
		return false, false, errors.New("isInGeneralRoom && isInMemberRoom")
	}
	return isInMemberRoom, isInGeneralRoom, nil
}

func (app *WorkspaceApp) CreateUser(ctx context.Context, tx *firestore.Transaction) error {
	slog.Info(utils.NameOf(app.CreateUser))
	userData := repository.UserDoc{
		DailyTotalStudySec: 0,
		TotalStudySec:      0,
		RegistrationDate:   app.currentTime(),
	}
	return app.Repository.CreateUser(ctx, tx, app.ProcessedUserID, userData)
}

func (app *WorkspaceApp) GetNextPageToken(ctx context.Context, tx *firestore.Transaction) (string, error) {
	return app.Repository.ReadNextPageToken(ctx, tx)
}

func (app *WorkspaceApp) SaveNextPageToken(ctx context.Context, nextPageToken string) error {
	return app.Repository.UpdateNextPageToken(ctx, nextPageToken)
}

func (app *WorkspaceApp) CurrentSeat(ctx context.Context, userID string, isMemberSeat bool) (repository.SeatDoc, error) {
	seat, err := app.Repository.ReadSeatWithUserID(ctx, userID, isMemberSeat)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return repository.SeatDoc{}, studyspaceerror.ErrUserNotInTheRoom
		}
		return repository.SeatDoc{}, fmt.Errorf("in ReadSeatWithUserID: %w", err)
	}
	return seat, nil
}

func (app *WorkspaceApp) UpdateTotalWorkTime(tx *firestore.Transaction, userID string, previousUserDoc *repository.UserDoc, newWorkedTimeSec int, newDailyWorkedTimeSec int) error {
	// 更新前の値
	previousTotalSec := previousUserDoc.TotalStudySec
	previousDailyTotalSec := previousUserDoc.DailyTotalStudySec
	// 更新後の値
	newTotalSec := previousTotalSec + newWorkedTimeSec
	newDailyTotalSec := previousDailyTotalSec + newDailyWorkedTimeSec

	// 累計作業時間が減るなんてことがないか確認
	if newTotalSec < previousTotalSec {
		return fmt.Errorf("newTotalSec < previousTotalSec ??!! 処理を中断します。userID: %s,newTotalSec: %d, previousTotalSec: %d", userID, newTotalSec, previousTotalSec)
	}

	if err := app.Repository.UpdateUserTotalTime(tx, userID, newTotalSec, newDailyTotalSec); err != nil {
		return fmt.Errorf("in UpdateUserTotalTime: %w", err)
	}
	return nil
}

// GetUserRealtimeTotalStudyDurations リアルタイムの累積作業時間・当日累積作業時間を返す。
func (app *WorkspaceApp) GetUserRealtimeTotalStudyDurations(ctx context.Context, tx *firestore.Transaction, userID string) (time.Duration, time.Duration, error) {
	jstNow := app.currentTime()
	// 入室中ならばリアルタイムの作業時間も加算する
	realtimeDuration := time.Duration(0)
	realtimeDailyDuration := time.Duration(0)
	isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, userID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed IsUserInRoom: %w", err)
	}
	if isInMemberRoom || isInGeneralRoom {
		// 作業時間を計算
		currentSeat, err := app.CurrentSeat(ctx, userID, isInMemberRoom)
		if err != nil {
			return 0, 0, fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}

		realtimeDuration, err = utils.RealTimeTotalStudyDurationOfSeat(currentSeat, jstNow)
		if err != nil {
			return 0, 0, fmt.Errorf("in RealTimeTotalStudyDurationOfSeat: %w", err)
		}
		realtimeDailyDuration, err = utils.RealTimeDailyTotalStudyDurationOfSeat(currentSeat, jstNow)
		if err != nil {
			return 0, 0, fmt.Errorf("in RealTimeDailyTotalStudyDurationOfSeat: %w", err)
		}
	}

	userData, err := app.Repository.ReadUser(ctx, tx, userID)
	if err != nil {
		return 0, 0, fmt.Errorf("in ReadUser: %w", err)
	}

	// 累計
	totalDuration := realtimeDuration + time.Duration(userData.TotalStudySec)*time.Second

	// 当日の累計
	dailyTotalDuration := realtimeDailyDuration + time.Duration(userData.DailyTotalStudySec)*time.Second

	return totalDuration, dailyTotalDuration, nil
}

// ExitAllUsersInRoom roomの全てのユーザーを退室させる。
func (app *WorkspaceApp) ExitAllUsersInRoom(ctx context.Context, isMemberRoom bool) error {
	for {
		var seats []repository.SeatDoc
		var err error
		if isMemberRoom {
			seats, err = app.Repository.ReadMemberSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadMemberSeats: %w", err)
			}
		} else {
			seats, err = app.Repository.ReadGeneralSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadGeneralSeats: %w", err)
			}
		}
		if len(seats) == 0 {
			break
		}
		for _, seatCandidate := range seats {
			var message string
			txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
				seat, err := app.Repository.ReadSeat(ctx, tx, seatCandidate.SeatID, isMemberRoom)
				if err != nil {
					return fmt.Errorf("in ReadSeat: %w", err)
				}
				app.SetProcessedUser(seat.UserID, seat.UserDisplayName, seatCandidate.UserProfileImageURL, false, false, isMemberRoom)
				userDoc, err := app.Repository.ReadUser(ctx, tx, app.ProcessedUserID)
				if err != nil {
					return fmt.Errorf("in ReadUser: %w", err)
				}

				workSegments, err := app.Repository.ReadWorkStateSegmentsBySessionID(ctx, seat.SessionID)
				if err != nil {
					return fmt.Errorf("in ReadWorkStateSegmentsBySessionID: %w", err)
				}

				// 退室処理
				workedTimeSec, addedRP, err := app.exitRoom(ctx, tx, isMemberRoom, seat, &userDoc, workSegments)
				if err != nil {
					return fmt.Errorf("failed to exitRoom for %s: %w", app.ProcessedUserID, err)
				}
				var rpEarned string
				var seatIDStr string
				if userDoc.RankVisible {
					rpEarned = i18nmsg.CommandRpEarned(addedRP)
				}
				seatIDStr = presenter.SeatIDStr(seat.SeatID, isMemberRoom)
				message = i18nmsg.CommandExit(app.ProcessedUserDisplayName, workedTimeSec/60, seatIDStr, rpEarned)
				return nil
			})
			if txErr != nil { // log txErr but continues
				slog.Error("error in transaction", "txErr", txErr)
			}
			slog.Info(message)
		}
	}
	return nil
}

func (app *WorkspaceApp) ListLiveChatMessages(ctx context.Context, pageToken string) ([]*youtube.LiveChatMessage, string, int, error) {
	return app.LiveChatBot.ListMessages(ctx, pageToken)
}

func (app *WorkspaceApp) MessageToLiveChat(ctx context.Context, message string) {
	if err := app.LiveChatBot.PostMessage(ctx, message); err != nil {
		app.MessageToOwnerWithError(ctx, "failed to send live chat message \""+message+"\"\n", err)
	}
}

func (app *WorkspaceApp) MessageToOwner(ctx context.Context, message string) {
	if err := app.alertOwnerBot.SendMessage(ctx, message); err != nil {
		slog.ErrorContext(ctx, "failed to send message to owner", "error", err)
	}
	// これが最終連絡手段のため、エラーは返さずログのみ。
}

// MessageToOwnerOrError はOwner通知を行い、通知失敗を呼び出し元へ返す。
// 通知Lambdaのように、配送失敗自体をCloudWatch Alarmで検知したい用途で使う。
func (app *WorkspaceApp) MessageToOwnerOrError(ctx context.Context, message string) error {
	return app.alertOwnerBot.SendMessage(ctx, message)
}

func (app *WorkspaceApp) MessageToOwnerWithError(ctx context.Context, message string, argErr error) {
	if err := app.alertOwnerBot.SendMessageWithError(ctx, message, argErr); err != nil {
		slog.ErrorContext(ctx, "failed to send message to owner", "error", err)
	}
	// これが最終連絡手段のため、エラーは返さずログのみ。
}

func (app *WorkspaceApp) MessageToModerators(ctx context.Context, message string) error {
	return app.alertModeratorsBot.SendMessage(ctx, message)
}

func (app *WorkspaceApp) LogToModerators(ctx context.Context, logMessage string) error {
	return app.logModeratorsBot.SendMessage(ctx, logMessage)
}

// CheckLongTimeSitting 長時間入室しているユーザーを席移動させる。
func (app *WorkspaceApp) CheckLongTimeSitting(ctx context.Context, isMemberRoom bool) error {
	// 全座席のスナップショットをとる（トランザクションなし）
	var seatsSnapshot []repository.SeatDoc
	var err error
	if isMemberRoom {
		seatsSnapshot, err = app.Repository.ReadMemberSeats(ctx)
	} else {
		seatsSnapshot, err = app.Repository.ReadGeneralSeats(ctx)
	}
	if err != nil {
		return fmt.Errorf("failed to read seats: %w", err)
	}

	return app.OrganizeDBForceMove(ctx, seatsSnapshot, isMemberRoom)
}

func (app *WorkspaceApp) CheckLiveStreamStatus(ctx context.Context) error {
	checker := guardians.NewLiveStreamChecker(app.Repository, app.LiveChatBot, app.alertOwnerBot)
	return checker.Check(ctx)
}

func (app *WorkspaceApp) GetUserIDsToProcessRP(ctx context.Context) ([]string, error) {
	slog.Info(utils.NameOf(app.GetUserIDsToProcessRP))
	jstNow := app.currentTime()
	// 過去31日以内に入室したことのあるユーザーをクエリ（本当は退室したことのある人も取得したいが、クエリはORに対応してないため無視）
	_31daysAgo := jstNow.AddDate(0, 0, -31)
	iter := app.Repository.GetUsersActiveAfterDate(ctx, _31daysAgo)

	var userIDs []string
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return []string{}, fmt.Errorf("in iter.Next(): %w", err)
		}
		userID := doc.Ref.ID
		userIDs = append(userIDs, userID)
	}
	return userIDs, nil
}

func (app *WorkspaceApp) GetAllUsersTotalStudySecList(ctx context.Context) ([]utils.UserIDTotalStudySecSet, error) {
	var set []utils.UserIDTotalStudySecSet

	userDocRefs, err := app.Repository.GetAllUserDocRefs(ctx)
	if err != nil {
		return set, fmt.Errorf("in GetAllUserDocRefs: %w", err)
	}
	for _, userDocRef := range userDocRefs {
		userDoc, err := app.Repository.ReadUser(ctx, nil, userDocRef.ID)
		if err != nil {
			return set, fmt.Errorf("in ReadUser: %w", err)
		}
		set = append(set, utils.UserIDTotalStudySecSet{
			UserID:        userDocRef.ID,
			TotalStudySec: userDoc.TotalStudySec,
		})
	}
	return set, nil
}

// MinAvailableSeatIDForUser 空いている最小の番号の席番号を求める。該当ユーザーの入室上限にかからない範囲に限定。
func (app *WorkspaceApp) MinAvailableSeatIDForUser(ctx context.Context, tx *firestore.Transaction, userID string, isMemberSeat bool) (int, error) {
	var seats []repository.SeatDoc
	var err error
	if isMemberSeat {
		seats, err = app.Repository.ReadMemberSeats(ctx)
		if err != nil {
			return -1, fmt.Errorf("in ReadMemberSeats(): %w", err)
		}
	} else {
		seats, err = app.Repository.ReadGeneralSeats(ctx)
		if err != nil {
			return -1, fmt.Errorf("in ReadGeneralSeats(): %w", err)
		}
	}

	constants, err := app.Repository.ReadSystemConstantsConfig(ctx, tx)
	if err != nil {
		return -1, fmt.Errorf("in ReadSystemConstantsConfig(): %w", err)
	}

	// 使用されている座席番号リストを取得
	var usedSeatIDs []int
	for _, seat := range seats {
		usedSeatIDs = append(usedSeatIDs, seat.SeatID)
	}

	// 使用されていない最小の席番号を求める。1から順に探索
	searchingSeatID := 1
	for searchingSeatID <= constants.MaxSeats {
		// searchingSeatIDがusedSeatIDsに含まれているか
		isUsed := false
		for _, usedSeatID := range usedSeatIDs {
			if usedSeatID == searchingSeatID {
				isUsed = true
			}
		}
		if !isUsed { // 使われていない
			// 且つ、該当ユーザーが入室制限にかからなければその席番号を返す
			ifSittingTooMuch, err := app.CheckIfUserSittingTooMuchForSeat(ctx, userID, searchingSeatID, isMemberSeat)
			if err != nil {
				return -1, fmt.Errorf("in CheckIfUserSittingTooMuchForSeat(): %w", err)
			}
			if !ifSittingTooMuch {
				return searchingSeatID, nil
			}
		}
		searchingSeatID += 1
	}
	return -1, studyspaceerror.ErrNoSeatAvailable
}

func (app *WorkspaceApp) AddLiveChatHistoryDoc(ctx context.Context, chatMessage *youtube.LiveChatMessage) error {
	// example of publishedAt: "2021-11-13T07:21:30.486982+00:00"
	publishedAt, err := time.Parse(time.RFC3339Nano, chatMessage.Snippet.PublishedAt)
	if err != nil {
		return fmt.Errorf("failed to Parse publishedAt: %w", err)
	}
	publishedAt = publishedAt.In(timeutil.JapanLocation())

	liveChatHistoryDoc := repository.LiveChatHistoryDoc{
		AuthorChannelID:       chatMessage.AuthorDetails.ChannelId,
		AuthorDisplayName:     chatMessage.AuthorDetails.DisplayName,
		AuthorProfileImageURL: chatMessage.AuthorDetails.ProfileImageUrl,
		AuthorIsChatModerator: chatMessage.AuthorDetails.IsChatModerator,
		ID:                    chatMessage.Id,
		LiveChatID:            chatMessage.Snippet.LiveChatId,
		MessageText:           youtubebot.ExtractTextMessageByAuthor(chatMessage),
		PublishedAt:           publishedAt,
		Type:                  chatMessage.Snippet.Type,
	}
	return app.Repository.CreateLiveChatHistoryDoc(ctx, nil, liveChatHistoryDoc)
}

func (app *WorkspaceApp) DeleteCollectionHistoryBeforeDate(ctx context.Context, date time.Time) (int, int, int, error) {
	// Firestoreでは1回のトランザクションで500件までしか削除できないため、500件ずつ回す
	var numRowsLiveChat, numRowsUserActivity, numRowsOrderHistory int

	// date以前の全てのlive chat history docsをクエリで取得
	for {
		iter := app.Repository.Get500LiveChatHistoryDocIDsBeforeDate(ctx, date)
		count, err := app.DeleteIteratorDocs(ctx, iter)
		numRowsLiveChat += count
		if err != nil {
			return 0, 0, 0, fmt.Errorf("in DeleteIteratorDocs(): %w", err)
		}
		if count == 0 {
			break
		}
	}

	// date以前の全てのuser activity docをクエリで取得
	for {
		iter := app.Repository.Get500UserActivityDocIDsBeforeDate(ctx, date)
		count, err := app.DeleteIteratorDocs(ctx, iter)
		numRowsUserActivity += count
		if err != nil {
			return 0, 0, 0, fmt.Errorf("in DeleteIteratorDocs(): %w", err)
		}
		if count == 0 {
			break
		}
	}

	// date以前の全てのorder history docをクエリで取得
	for {
		iter := app.Repository.Get500OrderHistoryDocIDsBeforeDate(ctx, date)
		count, err := app.DeleteIteratorDocs(ctx, iter)
		numRowsOrderHistory += count
		if err != nil {
			return 0, 0, 0, fmt.Errorf("in DeleteIteratorDocs(): %w", err)
		}
		if count == 0 {
			break
		}
	}

	return numRowsLiveChat, numRowsUserActivity, numRowsOrderHistory, nil
}

// DeleteIteratorDocs iterは最大500件とすること。
func (app *WorkspaceApp) DeleteIteratorDocs(ctx context.Context, iter *firestore.DocumentIterator) (int, error) {
	count := 0 // iterのアイテムの件数
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// forで各docをdeleteしていく
		for {
			doc, err := iter.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return fmt.Errorf("in iter.Next(): %w", err)
			}
			count++
			{
				if err := app.Repository.DeleteDocRef(ctx, tx, doc.Ref); err != nil {
					return fmt.Errorf("in DeleteDocRef(): %w", err)
				}
			}
		}
		return nil
	})
	return count, txErr
}

func (app *WorkspaceApp) CheckIfUserSittingTooMuchForSeat(ctx context.Context, userID string, seatID int, isMemberSeat bool) (bool, error) {
	jstNow := app.currentTime()

	// ホワイトリスト・ブラックリストを検索
	whiteListForUserAndSeat, err := app.Repository.ReadSeatLimitsWHITEListWithSeatIDAndUserID(ctx, seatID, userID, isMemberSeat)
	if err != nil {
		return false, fmt.Errorf("in ReadSeatLimitsWHITEListWithSeatIDAndUserID(): %w", err)
	}
	blackListForUserAndSeat, err := app.Repository.ReadSeatLimitsBLACKListWithSeatIDAndUserID(ctx, seatID, userID, isMemberSeat)
	if err != nil {
		return false, fmt.Errorf("in ReadSeatLimitsBLACKListWithSeatIDAndUserID(): %w", err)
	}

	// もし両方あったら矛盾なのでエラー
	if len(whiteListForUserAndSeat) > 0 && len(blackListForUserAndSeat) > 0 {
		return false, errors.New("len(whiteListForUserAndSeat) > 0 && len(blackListForUserAndSeat) > 0")
	}

	// 片方しかなければチェックは不要
	if len(whiteListForUserAndSeat) > 1 {
		return false, fmt.Errorf("len(whiteListForUserAndSeat) > 1, seatID=%d, userID=%s", seatID, userID)
	} else if len(whiteListForUserAndSeat) == 1 {
		if whiteListForUserAndSeat[0].Until.After(jstNow) {
			slog.Info("[seat " + strconv.Itoa(seatID) + ": " + userID + "] found in white list. skipping.")
			return false, nil
		}
		// ホワイトリストに入っているが、期限切れのためチェックを続行
	}
	if len(blackListForUserAndSeat) > 1 {
		return false, fmt.Errorf("len(blackListForUserAndSeat) > 1, seatID=%d, userID=%s", seatID, userID)
	} else if len(blackListForUserAndSeat) == 1 {
		if blackListForUserAndSeat[0].Until.After(jstNow) {
			slog.Info("[seat " + strconv.Itoa(seatID) + ": " + userID + "] found in black list. skipping.")
			return true, nil
		}
		// ブラックリストに入っているが、期限切れのためチェックを続行
	}

	totalEntryDuration, err := app.GetRecentUserSittingTimeForSeat(ctx, userID, seatID, isMemberSeat)
	if err != nil {
		return false, fmt.Errorf("in GetRecentUserSittingTimeForSeat(): %w", err)
	}

	slog.Info("",
		"userID", userID,
		"seatID", seatID,
		"過去何分", app.Configs.Constants.RecentRangeMin,
		"合計何分", int(totalEntryDuration.Minutes()))

	// 制限値と比較
	ifSittingTooMuch := int(totalEntryDuration.Minutes()) > app.Configs.Constants.RecentThresholdMin

	if !ifSittingTooMuch {
		until := jstNow.Add(time.Duration(app.Configs.Constants.RecentThresholdMin)*time.Minute - totalEntryDuration)
		if until.Sub(jstNow) > time.Duration(app.Configs.Constants.MinimumCheckLongTimeSittingIntervalMinutes)*time.Minute {
			// ホワイトリストに登録
			if err := app.Repository.CreateSeatLimitInWHITEList(ctx, seatID, userID, jstNow, until, isMemberSeat); err != nil {
				return false, fmt.Errorf("in CreateSeatLimitInWHITEList(): %w", err)
			}
			slog.Info("[seat " + strconv.Itoa(seatID) + ": " + userID + "] saved to white list.")
		}
	} else {
		// ブラックリストに登録
		until := jstNow.Add(time.Duration(app.Configs.Constants.LongTimeSittingPenaltyMinutes) * time.Minute)
		if err := app.Repository.CreateSeatLimitInBLACKList(ctx, seatID, userID, jstNow, until, isMemberSeat); err != nil {
			return false, fmt.Errorf("in CreateSeatLimitInBLACKList(): %w", err)
		}
		slog.Info("[seat " + strconv.Itoa(seatID) + ": " + userID + "] saved to black list.")
	}

	return ifSittingTooMuch, nil
}

func (app *WorkspaceApp) GetRecentUserSittingTimeForSeat(ctx context.Context, userID string, seatID int, isMemberSeat bool) (time.Duration, error) {
	jstNow := app.currentTime()

	checkDurationFrom := jstNow.Add(-time.Duration(app.Configs.Constants.RecentRangeMin) * time.Minute)

	// 指定期間の該当ユーザーの該当座席への入退室ドキュメントを取得する
	enterRoomActivities, err := app.Repository.GetEnterRoomUserActivityDocIDsAfterDateForUserAndSeat(ctx, checkDurationFrom, userID, seatID, isMemberSeat)
	if err != nil {
		return 0, fmt.Errorf("in "+utils.NameOf(app.Repository.GetEnterRoomUserActivityDocIDsAfterDateForUserAndSeat)+": %w", err)
	}
	exitRoomActivities, err := app.Repository.GetExitRoomUserActivityDocIDsAfterDateForUserAndSeat(ctx, checkDurationFrom, userID, seatID, isMemberSeat)
	if err != nil {
		return 0, fmt.Errorf("in "+utils.NameOf(app.Repository.GetExitRoomUserActivityDocIDsAfterDateForUserAndSeat)+": %w", err)
	}
	activityOnlyEnterExitList := append(enterRoomActivities, exitRoomActivities...)

	// activityListは長さ0の可能性もあることに注意

	// 入室と退室が交互に並んでいるか確認
	utils.SortUserActivityByTakenAtAscending(activityOnlyEnterExitList)
	orderOK := utils.CheckEnterExitActivityOrder(activityOnlyEnterExitList)
	if !orderOK {
		return 0, errors.New("入室activityと退室activityが交互に並んでいない\n" + fmt.Sprintf("%v", pretty.Formatter(activityOnlyEnterExitList)))
	}

	slog.Info("入退室ドキュメント数：" + strconv.Itoa(len(activityOnlyEnterExitList)))

	// 入退室をセットで考え、合計入室時間を求める
	totalEntryDuration := time.Duration(0)
	entryCount := 0 // 退室時（もしくは現在日時）にentryCountをインクリメント。
	lastEnteredTimestamp := checkDurationFrom
	for i, activity := range activityOnlyEnterExitList {
		if activity.ActivityType == repository.EnterRoomActivity {
			lastEnteredTimestamp = activity.TakenAt
			if i+1 == len(activityOnlyEnterExitList) { // 最後のactivityであった場合、現在時刻までの時間を加算
				entryCount += 1
				totalEntryDuration += timeutil.NoNegativeDuration(jstNow.Sub(activity.TakenAt))
			}
			continue
		} else if activity.ActivityType == repository.ExitRoomActivity {
			entryCount += 1
			totalEntryDuration += timeutil.NoNegativeDuration(activity.TakenAt.Sub(lastEnteredTimestamp))
		}
	}
	return totalEntryDuration, nil
}

func (app *WorkspaceApp) BanUser(ctx context.Context, userID string) error {
	if err := app.LiveChatBot.BanUser(ctx, userID); err != nil {
		return fmt.Errorf("in BanUser: %w", err)
	}
	return nil
}

// GetMenuItemByNumber メニュー番号からメニューアイテムを取得する。
func (app *WorkspaceApp) GetMenuItemByNumber(number int) (repository.MenuDoc, error) {
	if number < 1 || len(app.SortedMenuItems) < number {
		return repository.MenuDoc{}, fmt.Errorf("invalid menu number: %d, menuItems length = %d", number, len(app.SortedMenuItems))
	}
	return app.SortedMenuItems[number-1], nil
}

func (app *WorkspaceApp) GetMenuNumByCode(code string) (int, error) {
	for i, item := range app.SortedMenuItems {
		if item.Code == code {
			return i + 1, nil
		}
	}
	return -1, fmt.Errorf("menu code not found: %s", code)
}

// GetUserRealtimeSeatAppearance リアルタイムの現在のランクを求める
func (app *WorkspaceApp) GetUserRealtimeSeatAppearance(ctx context.Context, tx *firestore.Transaction, userID string) (repository.SeatAppearance, error) {
	userDoc, err := app.Repository.ReadUser(ctx, tx, userID)
	if err != nil {
		return repository.SeatAppearance{}, fmt.Errorf("in ReadUser(): %w", err)
	}
	totalStudyDuration, _, err := app.GetUserRealtimeTotalStudyDurations(ctx, tx, userID)
	if err != nil {
		return repository.SeatAppearance{}, fmt.Errorf("in GetUserRealtimeTotalStudyDurations(): %w", err)
	}
	seatAppearance, err := utils.GetSeatAppearance(int(totalStudyDuration.Seconds()), userDoc.RankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
	if err != nil {
		return repository.SeatAppearance{}, fmt.Errorf("in GetSeatAppearance(): %w", err)
	}
	return seatAppearance, nil
}

// RandomAvailableSeatIDForUser
// ルームの席が空いているならその中からランダムな席番号（該当ユーザーの入室上限にかからない範囲に限定）を、
// 空いていないならmax-seatsを増やし、最小の空席番号を返す。
func (app *WorkspaceApp) RandomAvailableSeatIDForUser(ctx context.Context, tx *firestore.Transaction, userID string, isMemberSeat bool) (int,
	error) {
	var seats []repository.SeatDoc
	var err error
	if isMemberSeat {
		seats, err = app.Repository.ReadMemberSeats(ctx)
		if err != nil {
			return 0, fmt.Errorf("in ReadMemberSeats: %w", err)
		}
	} else {
		seats, err = app.Repository.ReadGeneralSeats(ctx)
		if err != nil {
			return 0, fmt.Errorf("in ReadGeneralSeats: %w", err)
		}
	}

	constants, err := app.Repository.ReadSystemConstantsConfig(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("in ReadSystemConstantsConfig: %w", err)
	}
	var maxSeats int
	if isMemberSeat {
		maxSeats = constants.MemberMaxSeats
	} else {
		maxSeats = constants.MaxSeats
	}

	var vacantSeatIDList []int
	for id := 1; id <= maxSeats; id++ {
		isUsed := false
		for _, seatInUse := range seats {
			if id == seatInUse.SeatID {
				isUsed = true
				break
			}
		}
		if !isUsed {
			vacantSeatIDList = append(vacantSeatIDList, id)
		}
	}

	if len(vacantSeatIDList) > 0 {
		// 入室制限にかからない席を選ぶ
		r := rand.New(rand.NewSource(app.currentTime().UnixNano()))
		r.Shuffle(len(vacantSeatIDList), func(i, j int) { vacantSeatIDList[i], vacantSeatIDList[j] = vacantSeatIDList[j], vacantSeatIDList[i] })
		for _, seatID := range vacantSeatIDList {
			ifSittingTooMuch, err := app.CheckIfUserSittingTooMuchForSeat(ctx, userID, seatID, isMemberSeat)
			if err != nil {
				return -1, fmt.Errorf("in CheckIfUserSittingTooMuchForSeat: %w", err)
			}
			if !ifSittingTooMuch {
				return seatID, nil
			}
		}
	}
	return 0, studyspaceerror.ErrNoSeatAvailable
}

// enterRoom ユーザーを入室させる。
func (app *WorkspaceApp) enterRoom(
	ctx context.Context,
	tx *firestore.Transaction,
	userID string,
	userDisplayName string,
	userProfileImageURL string,
	seatID int,
	isMemberSeat bool,
	workName string,
	breakWorkName string,
	workMin int,
	seatAppearance repository.SeatAppearance,
	menuCode string,
	state repository.SeatState,
	isContinuousActive bool,
	breakStartedAt time.Time, // set when moving seat
	breakUntil time.Time, // set when moving seat
	enterDate time.Time,
) (int, error) {
	exitDate := enterDate.Add(time.Duration(workMin) * time.Minute)

	var currentStateStartedAt time.Time
	var currentStateUntil time.Time
	switch state {
	case repository.WorkState:
		currentStateStartedAt = enterDate
		currentStateUntil = exitDate
	case repository.BreakState:
		currentStateStartedAt = breakStartedAt
		currentStateUntil = breakUntil
	}

	newSeat := repository.SeatDoc{
		SeatID:                  seatID,
		UserID:                  userID,
		SessionID:               utils.GenerateSessionID(),
		UserDisplayName:         userDisplayName,
		UserProfileImageURL:     userProfileImageURL,
		WorkName:                workName,
		BreakWorkName:           breakWorkName,
		EnteredAt:               enterDate,
		Until:                   exitDate,
		Appearance:              seatAppearance,
		MenuCode:                menuCode,
		State:                   state,
		CurrentStateStartedAt:   currentStateStartedAt,
		CurrentStateUntil:       currentStateUntil,
		CurrentSegmentStartedAt: enterDate,
		CumulativeWorkSec:       0,
		DailyCumulativeWorkSec:  0,
	}
	if err := app.Repository.CreateSeat(tx, newSeat, isMemberSeat); err != nil {
		return 0, fmt.Errorf("in CreateSeat: %w", err)
	}

	// 入室時刻を記録
	if err := app.Repository.UpdateUserLastEnteredDate(tx, userID, enterDate); err != nil {
		return 0, fmt.Errorf("in UpdateUserLastEnteredDate: %w", err)
	}
	// activityログ記録
	enterActivity := repository.UserActivityDoc{
		UserID:       userID,
		ActivityType: repository.EnterRoomActivity,
		SeatID:       seatID,
		IsMemberSeat: isMemberSeat,
		TakenAt:      enterDate,
	}
	if err := app.Repository.CreateUserActivityDoc(ctx, tx, enterActivity); err != nil {
		return 0, fmt.Errorf("in CreateUserActivityDoc: %w", err)
	}
	// 久しぶりの入室であれば、isContinuousActiveをtrueに、lastPenaltyImposedDaysを0に更新
	if !isContinuousActive {
		if err := app.Repository.UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(ctx, tx, userID, true, enterDate); err != nil {
			return 0, fmt.Errorf("in UpdateUserIsContinuousActiveAndCurrentActivityStateStarted: %w", err)
		}
		if err := app.Repository.UpdateUserLastPenaltyImposedDays(ctx, tx, userID, 0); err != nil {
			return 0, fmt.Errorf("in UpdateUserLastPenaltyImposedDays: %w", err)
		}
	}

	// 入室から自動退室予定時刻までの時間（分）
	untilExitMin := int(exitDate.Sub(enterDate).Minutes())

	return untilExitMin, nil
}

// exitRoom ユーザーを退室させる。
func (app *WorkspaceApp) exitRoom(
	ctx context.Context,
	tx *firestore.Transaction,
	isMemberSeat bool,
	previousSeat repository.SeatDoc,
	previousUserDoc *repository.UserDoc,
	previousWorkSegments []repository.WorkSegmentDoc,
) (int, int, error) {
	// 作業時間を計算
	exitDate := app.currentTime()
	var addedWorkedTimeSec int
	var addedDailyWorkedTimeSec int
	switch previousSeat.State {
	case repository.BreakState:
		addedWorkedTimeSec = previousSeat.CumulativeWorkSec
		// もし直前の休憩で日付を跨いでたら
		justBreakTimeSec := int(timeutil.NoNegativeDuration(exitDate.Sub(previousSeat.CurrentStateStartedAt)).Seconds())
		if justBreakTimeSec > timeutil.SecondsOfDay(exitDate) {
			addedDailyWorkedTimeSec = 0
		} else {
			addedDailyWorkedTimeSec = previousSeat.DailyCumulativeWorkSec
		}
	case repository.WorkState:
		justWorkedTimeSec := int(timeutil.NoNegativeDuration(exitDate.Sub(previousSeat.CurrentStateStartedAt)).Seconds())
		addedWorkedTimeSec = previousSeat.CumulativeWorkSec + justWorkedTimeSec
		// もし日付変更を跨いで入室してたら、当日の累計時間は日付変更からの時間にする
		if justWorkedTimeSec > timeutil.SecondsOfDay(exitDate) {
			addedDailyWorkedTimeSec = timeutil.SecondsOfDay(exitDate)
		} else {
			addedDailyWorkedTimeSec = previousSeat.DailyCumulativeWorkSec + justWorkedTimeSec
		}
	}

	// 退室処理
	if err := app.Repository.DeleteSeat(ctx, tx, previousSeat.SeatID, isMemberSeat); err != nil {
		return 0, 0, fmt.Errorf("in DeleteSeat: %w", err)
	}

	// DEPRECATED: activityログ記録
	exitActivity := repository.UserActivityDoc{
		UserID:       previousSeat.UserID,
		ActivityType: repository.ExitRoomActivity,
		SeatID:       previousSeat.SeatID,
		IsMemberSeat: isMemberSeat,
		TakenAt:      exitDate,
	}
	if err := app.Repository.CreateUserActivityDoc(ctx, tx, exitActivity); err != nil {
		return 0, 0, fmt.Errorf("in CreateUserActivityDoc: %w", err)
	}
	// work segmentログ記録
	workSegment, err := previousSeat.GenerateWorkSegment(exitDate, isMemberSeat)
	if err != nil {
		return 0, 0, fmt.Errorf("in GenerateWorkSegment: %w", err)
	}
	if err := app.Repository.CreateWorkSegmentDoc(ctx, tx, workSegment); err != nil {
		return 0, 0, fmt.Errorf("in CreateWorkSegmentDoc: %w", err)
	}
	// 退室時刻を記録
	if err := app.Repository.UpdateUserLastExitedDate(tx, previousSeat.UserID, exitDate); err != nil {
		return 0, 0, fmt.Errorf("in UpdateUserLastExitedDate: %w", err)
	}

	// 検算
	{
		onlyWorkSegmentSec := 0
		if workSegment.SegmentType == repository.WorkState {
			onlyWorkSegmentSec += workSegment.DurationSec
		}
		for _, segment := range previousWorkSegments {
			if segment.SegmentType == repository.WorkState {
				onlyWorkSegmentSec += segment.DurationSec
			}
		}
		diffSec := onlyWorkSegmentSec - addedWorkedTimeSec
		if diffSec < 0 {
			diffSec = -diffSec
		}
		const allowedDiffSec = 10
		if diffSec > allowedDiffSec {
			app.MessageToOwner(ctx, fmt.Sprintf(
				"検算エラー: onlyWorkSegmentSec = %d, addedWorkedTimeSec = %d, diffSec = %d, allowedDiffSec = %d (userID=%s, seatID=%d)",
				onlyWorkSegmentSec, addedWorkedTimeSec, diffSec, allowedDiffSec, previousSeat.UserID, previousSeat.SeatID,
			))
		} else {
			slog.DebugContext(ctx,
				"検算成功: abs(onlyWorkSegmentSec-addedWorkedTimeSec) <= allowedDiffSec",
				"allowedDiffSec", allowedDiffSec,
				"userID", previousSeat.UserID,
				"seatID", previousSeat.SeatID,
				"onlyWorkSegmentSec", onlyWorkSegmentSec,
				"addedWorkedTimeSec", addedWorkedTimeSec,
				"diffSec", diffSec,
			)
		}
	}

	// 累計作業時間を更新
	if err := app.UpdateTotalWorkTime(tx, previousSeat.UserID, previousUserDoc, addedWorkedTimeSec, addedDailyWorkedTimeSec); err != nil {
		return 0, 0, fmt.Errorf("in UpdateTotalWorkTime: %w", err)
	}
	// RP更新
	netStudyDuration := time.Duration(addedWorkedTimeSec) * time.Second
	newRP, err := utils.CalcNewRPExitRoom(netStudyDuration, previousSeat.WorkName != "", previousUserDoc.IsContinuousActive, previousUserDoc.CurrentActivityStateStarted, exitDate, previousUserDoc.RankPoint)
	if err != nil {
		return 0, 0, fmt.Errorf("in CalcNewRPExitRoom: %w", err)
	}
	if err := app.Repository.UpdateUserRankPoint(tx, previousSeat.UserID, newRP); err != nil {
		return 0, 0, fmt.Errorf("in UpdateUserRP: %w", err)
	}
	addedRP := newRP - previousUserDoc.RankPoint

	slog.Info("user exited the room.",
		"userID", previousSeat.UserID,
		"seatID", previousSeat.SeatID,
		"addedWorkedTimeSec", addedWorkedTimeSec,
		"addedRP", addedRP,
		"newRP", newRP,
		"previous RP", previousUserDoc.RankPoint)
	return addedWorkedTimeSec, addedRP, nil
}

func (app *WorkspaceApp) moveSeat(
	ctx context.Context,
	tx *firestore.Transaction,
	targetSeatID int,
	latestUserProfileImage string,
	beforeIsMemberSeat,
	afterIsMemberSeat bool,
	option utils.MinWorkOrderOption,
	previousSeat repository.SeatDoc,
	previousUserDoc *repository.UserDoc,
	previousWorkSegments []repository.WorkSegmentDoc,
) (int, int, int, error) {
	jstNow := app.currentTime()

	// 値チェック
	if targetSeatID == previousSeat.SeatID && beforeIsMemberSeat == afterIsMemberSeat {
		return 0, 0, 0, errors.New("targetSeatID == previousSeat.SeatID && beforeIsMemberSeat == afterIsMemberSeat")
	}

	// 退室
	workedTimeSec, addedRP, err := app.exitRoom(ctx, tx, beforeIsMemberSeat, previousSeat, previousUserDoc, previousWorkSegments)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("in exitRoom for %s: %w", app.ProcessedUserID, err)
	}

	// 入室の準備
	var workName string
	var workMin int
	if option.IsWorkNameSet {
		workName = option.WorkName
	} else {
		workName = previousSeat.WorkName
	}
	if option.IsDurationMinSet {
		workMin = option.DurationMin
	} else {
		workMin = previousSeat.RemainingWorkMin(jstNow)
	}
	newTotalStudyDuration := time.Duration(previousUserDoc.TotalStudySec+workedTimeSec) * time.Second
	newRP := previousUserDoc.RankPoint + addedRP
	newSeatAppearance, err := utils.GetSeatAppearance(int(newTotalStudyDuration.Seconds()), previousUserDoc.RankVisible, newRP, previousUserDoc.FavoriteColor)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("in GetSeatAppearance: %w", err)
	}

	// 入室
	untilExitMin, err := app.enterRoom(
		ctx,
		tx,
		previousSeat.UserID,
		previousSeat.UserDisplayName,
		latestUserProfileImage,
		targetSeatID,
		afterIsMemberSeat,
		workName,
		previousSeat.BreakWorkName,
		workMin,
		newSeatAppearance,
		previousSeat.MenuCode,
		previousSeat.State,
		previousUserDoc.IsContinuousActive,
		previousSeat.CurrentStateStartedAt,
		previousSeat.CurrentStateUntil,
		jstNow)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to enterRoom for %s: %w", previousSeat.UserID, err)
	}

	return workedTimeSec, addedRP, untilExitMin, nil
}
