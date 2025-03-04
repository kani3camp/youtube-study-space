package workspaceapp

import (
	"app.modules/core/guardians"
	"app.modules/core/i18n"
	"app.modules/core/repository"
	"app.modules/core/studyspaceerror"
	"app.modules/core/utils"
	"app.modules/core/youtubebot"
	"cloud.google.com/go/firestore"
	"context"
	"fmt"
	"github.com/kr/pretty"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/youtube/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log/slog"
	"math/rand"
	"strconv"
	"time"
)

// IsSeatExist 席番号1～max-seatsの席かどうかを判定。
func (s *WorkspaceApp) IsSeatExist(ctx context.Context, seatId int, isMemberSeat bool) (bool, error) {
	realtimeConstants, err := s.Repository.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("in ReadSystemConstantsConfig: %w", err)
	}
	if isMemberSeat {
		return 1 <= seatId && seatId <= realtimeConstants.MemberMaxSeats, nil
	} else {
		return 1 <= seatId && seatId <= realtimeConstants.MaxSeats, nil
	}
}

// IfSeatVacant 席番号がseatIdの席が空いているかどうか。
func (s *WorkspaceApp) IfSeatVacant(ctx context.Context, tx *firestore.Transaction, seatId int, isMemberSeat bool) (bool, error) {
	_, err := s.Repository.ReadSeat(ctx, tx, seatId, isMemberSeat)
	if err != nil {
		if status.Code(err) == codes.NotFound { // その座席のドキュメントは存在しない
			// maxSeats以内かどうか
			isExist, err := s.IsSeatExist(ctx, seatId, isMemberSeat)
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

func (s *WorkspaceApp) IfUserRegistered(ctx context.Context, tx *firestore.Transaction) (bool, error) {
	_, err := s.Repository.ReadUser(ctx, tx, s.ProcessedUserId)
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
func (s *WorkspaceApp) IsUserInRoom(ctx context.Context, userId string) (isInMemberRoom bool, isInGeneralRoom bool, returnErr error) {
	isInMemberRoom = true
	isInGeneralRoom = true
	if _, err := s.Repository.ReadSeatWithUserId(ctx, userId, true); err != nil {
		if status.Code(err) == codes.NotFound {
			isInMemberRoom = false
		} else {
			return false, false, fmt.Errorf("in ReadSeatWithUserId: %w", err)
		}
	}
	if _, err := s.Repository.ReadSeatWithUserId(ctx, userId, false); err != nil {
		if status.Code(err) == codes.NotFound {
			isInGeneralRoom = false
		} else {
			return false, false, fmt.Errorf("in ReadSeatWithUserId: %w", err)
		}
	}
	if isInGeneralRoom && isInMemberRoom {
		return false, false, errors.New("isInGeneralRoom && isInMemberRoom")
	}
	return isInMemberRoom, isInGeneralRoom, nil
}

func (s *WorkspaceApp) CreateUser(ctx context.Context, tx *firestore.Transaction) error {
	slog.Info(utils.NameOf(s.CreateUser))
	userData := repository.UserDoc{
		DailyTotalStudySec: 0,
		TotalStudySec:      0,
		RegistrationDate:   utils.JstNow(),
	}
	return s.Repository.CreateUser(ctx, tx, s.ProcessedUserId, userData)
}

func (s *WorkspaceApp) GetNextPageToken(ctx context.Context, tx *firestore.Transaction) (string, error) {
	return s.Repository.ReadNextPageToken(ctx, tx)
}

func (s *WorkspaceApp) SaveNextPageToken(ctx context.Context, nextPageToken string) error {
	return s.Repository.UpdateNextPageToken(ctx, nextPageToken)
}

func (s *WorkspaceApp) CurrentSeat(ctx context.Context, userId string, isMemberSeat bool) (repository.SeatDoc, error) {
	seat, err := s.Repository.ReadSeatWithUserId(ctx, userId, isMemberSeat)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return repository.SeatDoc{}, studyspaceerror.ErrUserNotInTheRoom
		}
		return repository.SeatDoc{}, fmt.Errorf("in ReadSeatWithUserId: %w", err)
	}
	return seat, nil
}

func (s *WorkspaceApp) UpdateTotalWorkTime(tx *firestore.Transaction, userId string, previousUserDoc *repository.UserDoc, newWorkedTimeSec int, newDailyWorkedTimeSec int) error {
	// 更新前の値
	previousTotalSec := previousUserDoc.TotalStudySec
	previousDailyTotalSec := previousUserDoc.DailyTotalStudySec
	// 更新後の値
	newTotalSec := previousTotalSec + newWorkedTimeSec
	newDailyTotalSec := previousDailyTotalSec + newDailyWorkedTimeSec

	// 累計作業時間が減るなんてことがないか確認
	if newTotalSec < previousTotalSec {
		return errors.New(fmt.Sprintf("newTotalSec < previousTotalSec ??!! 処理を中断します。userId: %s,newTotalSec: %d, previousTotalSec: %d", userId, newTotalSec, previousTotalSec))
	}

	if err := s.Repository.UpdateUserTotalTime(tx, userId, newTotalSec, newDailyTotalSec); err != nil {
		return fmt.Errorf("in UpdateUserTotalTime: %w", err)
	}
	return nil
}

// GetUserRealtimeTotalStudyDurations リアルタイムの累積作業時間・当日累積作業時間を返す。
func (s *WorkspaceApp) GetUserRealtimeTotalStudyDurations(ctx context.Context, tx *firestore.Transaction, userId string) (time.Duration, time.Duration, error) {
	// 入室中ならばリアルタイムの作業時間も加算する
	realtimeDuration := time.Duration(0)
	realtimeDailyDuration := time.Duration(0)
	isInMemberRoom, isInGeneralRoom, err := s.IsUserInRoom(ctx, s.ProcessedUserId)
	if err != nil {
		return 0, 0, fmt.Errorf("failed IsUserInRoom: %w", err)
	}
	if isInMemberRoom || isInGeneralRoom {
		// 作業時間を計算
		currentSeat, err := s.CurrentSeat(ctx, userId, isInMemberRoom)
		if err != nil {
			return 0, 0, fmt.Errorf("failed s.CurrentSeat(): %w", err)
		}

		realtimeDuration, err = utils.RealTimeTotalStudyDurationOfSeat(currentSeat, utils.JstNow())
		if err != nil {
			return 0, 0, fmt.Errorf("in RealTimeTotalStudyDurationOfSeat: %w", err)
		}
		realtimeDailyDuration, err = utils.RealTimeDailyTotalStudyDurationOfSeat(currentSeat, utils.JstNow())
		if err != nil {
			return 0, 0, fmt.Errorf("in RealTimeDailyTotalStudyDurationOfSeat: %w", err)
		}
	}

	userData, err := s.Repository.ReadUser(ctx, tx, userId)
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
func (s *WorkspaceApp) ExitAllUsersInRoom(ctx context.Context, isMemberRoom bool) error {
	for {
		var seats []repository.SeatDoc
		var err error
		if isMemberRoom {
			seats, err = s.Repository.ReadMemberSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadMemberSeats: %w", err)
			}
		} else {
			seats, err = s.Repository.ReadGeneralSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadGeneralSeats: %w", err)
			}
		}
		if len(seats) == 0 {
			break
		}
		for _, seatCandidate := range seats {
			var message string
			txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
				seat, err := s.Repository.ReadSeat(ctx, tx, seatCandidate.SeatId, isMemberRoom)
				if err != nil {
					return fmt.Errorf("in ReadSeat: %w", err)
				}
				s.SetProcessedUser(seat.UserId, seat.UserDisplayName, seatCandidate.UserProfileImageUrl, false, false, isMemberRoom)
				userDoc, err := s.Repository.ReadUser(ctx, tx, s.ProcessedUserId)
				if err != nil {
					return fmt.Errorf("in ReadUser: %w", err)
				}
				// 退室処理
				workedTimeSec, addedRP, err := s.exitRoom(ctx, tx, isMemberRoom, seat, &userDoc)
				if err != nil {
					return fmt.Errorf("failed to exitRoom for %s: %w", s.ProcessedUserId, err)
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
				message = i18n.T("command:exit", s.ProcessedUserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)
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

func (s *WorkspaceApp) ListLiveChatMessages(ctx context.Context, pageToken string) ([]*youtube.LiveChatMessage, string, int, error) {
	return s.LiveChatBot.ListMessages(ctx, pageToken)
}

func (s *WorkspaceApp) MessageToLiveChat(ctx context.Context, message string) {
	if err := s.LiveChatBot.PostMessage(ctx, message); err != nil {
		s.MessageToOwnerWithError(ctx, "failed to send live chat message \""+message+"\"\n", err)
	}
}

func (s *WorkspaceApp) MessageToOwner(ctx context.Context, message string) {
	if err := s.alertOwnerBot.SendMessage(ctx, message); err != nil {
		slog.ErrorContext(ctx, "failed to send message to owner", "error", err)
	}
	// これが最終連絡手段のため、エラーは返さずログのみ。
}

func (s *WorkspaceApp) MessageToOwnerWithError(ctx context.Context, message string, argErr error) {
	if err := s.alertOwnerBot.SendMessageWithError(ctx, message, argErr); err != nil {
		slog.ErrorContext(ctx, "failed to send message to owner", "error", err)
	}
	// これが最終連絡手段のため、エラーは返さずログのみ。
}

func (s *WorkspaceApp) MessageToModerators(ctx context.Context, message string) error {
	return s.alertModeratorsBot.SendMessage(ctx, message)
}

func (s *WorkspaceApp) LogToModerators(ctx context.Context, logMessage string) error {
	return s.logModeratorsBot.SendMessage(ctx, logMessage)
}

// CheckLongTimeSitting 長時間入室しているユーザーを席移動させる。
func (s *WorkspaceApp) CheckLongTimeSitting(ctx context.Context, isMemberRoom bool) error {
	// 全座席のスナップショットをとる（トランザクションなし）
	var seatsSnapshot []repository.SeatDoc
	var err error
	if isMemberRoom {
		seatsSnapshot, err = s.Repository.ReadMemberSeats(ctx)
	} else {
		seatsSnapshot, err = s.Repository.ReadGeneralSeats(ctx)
	}
	if err != nil {
		return fmt.Errorf("failed to read seats: %w", err)
	}

	return s.processLongTimeSitting(ctx, seatsSnapshot, isMemberRoom)
}

// processLongTimeSitting 長時間入室しているユーザーを席移動させる処理を実行する
func (s *WorkspaceApp) processLongTimeSitting(ctx context.Context, seatsSnapshot []repository.SeatDoc, isMemberRoom bool) error {
	if err := s.OrganizeDBForceMove(ctx, seatsSnapshot, isMemberRoom); err != nil {
		return fmt.Errorf("in OrganizeDBForceMove: %w", err)
	}

	return nil
}

func (s *WorkspaceApp) CheckLiveStreamStatus(ctx context.Context) error {
	checker := guardians.NewLiveStreamChecker(s.Repository, s.LiveChatBot, s.alertOwnerBot)
	return checker.Check(ctx)
}

func (s *WorkspaceApp) GetUserIdsToProcessRP(ctx context.Context) ([]string, error) {
	slog.Info(utils.NameOf(s.GetUserIdsToProcessRP))
	jstNow := utils.JstNow()
	// 過去31日以内に入室したことのあるユーザーをクエリ（本当は退室したことのある人も取得したいが、クエリはORに対応してないため無視）
	_31daysAgo := jstNow.AddDate(0, 0, -31)
	iter := s.Repository.GetUsersActiveAfterDate(ctx, _31daysAgo)

	var userIds []string
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return []string{}, fmt.Errorf("in iter.Next(): %w", err)
		}
		userId := doc.Ref.ID
		userIds = append(userIds, userId)
	}
	return userIds, nil
}

func (s *WorkspaceApp) GetAllUsersTotalStudySecList(ctx context.Context) ([]utils.UserIdTotalStudySecSet, error) {
	var set []utils.UserIdTotalStudySecSet

	userDocRefs, err := s.Repository.GetAllUserDocRefs(ctx)
	if err != nil {
		return set, fmt.Errorf("in GetAllUserDocRefs: %w", err)
	}
	for _, userDocRef := range userDocRefs {
		userDoc, err := s.Repository.ReadUser(ctx, nil, userDocRef.ID)
		if err != nil {
			return set, fmt.Errorf("in ReadUser: %w", err)
		}
		set = append(set, utils.UserIdTotalStudySecSet{
			UserId:        userDocRef.ID,
			TotalStudySec: userDoc.TotalStudySec,
		})
	}
	return set, nil
}

// MinAvailableSeatIdForUser 空いている最小の番号の席番号を求める。該当ユーザーの入室上限にかからない範囲に限定。
func (s *WorkspaceApp) MinAvailableSeatIdForUser(ctx context.Context, tx *firestore.Transaction, userId string, isMemberSeat bool) (int, error) {
	var seats []repository.SeatDoc
	var err error
	if isMemberSeat {
		seats, err = s.Repository.ReadMemberSeats(ctx)
		if err != nil {
			return -1, fmt.Errorf("in ReadMemberSeats(): %w", err)
		}
	} else {
		seats, err = s.Repository.ReadGeneralSeats(ctx)
		if err != nil {
			return -1, fmt.Errorf("in ReadGeneralSeats(): %w", err)
		}
	}

	constants, err := s.Repository.ReadSystemConstantsConfig(ctx, tx)
	if err != nil {
		return -1, fmt.Errorf("in ReadSystemConstantsConfig(): %w", err)
	}

	// 使用されている座席番号リストを取得
	var usedSeatIds []int
	for _, seat := range seats {
		usedSeatIds = append(usedSeatIds, seat.SeatId)
	}

	// 使用されていない最小の席番号を求める。1から順に探索
	searchingSeatId := 1
	for searchingSeatId <= constants.MaxSeats {
		// searchingSeatIdがusedSeatIdsに含まれているか
		isUsed := false
		for _, usedSeatId := range usedSeatIds {
			if usedSeatId == searchingSeatId {
				isUsed = true
			}
		}
		if !isUsed { // 使われていない
			// 且つ、該当ユーザーが入室制限にかからなければその席番号を返す
			ifSittingTooMuch, err := s.CheckIfUserSittingTooMuchForSeat(ctx, userId, searchingSeatId, isMemberSeat)
			if err != nil {
				return -1, fmt.Errorf("in CheckIfUserSittingTooMuchForSeat(): %w", err)
			}
			if !ifSittingTooMuch {
				return searchingSeatId, nil
			}
		}
		searchingSeatId += 1
	}
	return -1, studyspaceerror.ErrNoSeatAvailable
}

func (s *WorkspaceApp) AddLiveChatHistoryDoc(ctx context.Context, chatMessage *youtube.LiveChatMessage) error {
	// example of publishedAt: "2021-11-13T07:21:30.486982+00:00"
	publishedAt, err := time.Parse(time.RFC3339Nano, chatMessage.Snippet.PublishedAt)
	if err != nil {
		return fmt.Errorf("failed to Parse publishedAt: %w", err)
	}
	publishedAt = publishedAt.In(utils.JapanLocation())

	liveChatHistoryDoc := repository.LiveChatHistoryDoc{
		AuthorChannelId:       chatMessage.AuthorDetails.ChannelId,
		AuthorDisplayName:     chatMessage.AuthorDetails.DisplayName,
		AuthorProfileImageUrl: chatMessage.AuthorDetails.ProfileImageUrl,
		AuthorIsChatModerator: chatMessage.AuthorDetails.IsChatModerator,
		Id:                    chatMessage.Id,
		LiveChatId:            chatMessage.Snippet.LiveChatId,
		MessageText:           youtubebot.ExtractTextMessageByAuthor(chatMessage),
		PublishedAt:           publishedAt,
		Type:                  chatMessage.Snippet.Type,
	}
	return s.Repository.CreateLiveChatHistoryDoc(ctx, nil, liveChatHistoryDoc)
}

func (s *WorkspaceApp) DeleteCollectionHistoryBeforeDate(ctx context.Context, date time.Time) (int, int, int, error) {
	// Firestoreでは1回のトランザクションで500件までしか削除できないため、500件ずつ回す
	var numRowsLiveChat, numRowsUserActivity, numRowsOrderHistory int

	// date以前の全てのlive chat history docsをクエリで取得
	for {
		iter := s.Repository.Get500LiveChatHistoryDocIdsBeforeDate(ctx, date)
		count, err := s.DeleteIteratorDocs(ctx, iter)
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
		iter := s.Repository.Get500UserActivityDocIdsBeforeDate(ctx, date)
		count, err := s.DeleteIteratorDocs(ctx, iter)
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
		iter := s.Repository.Get500OrderHistoryDocIdsBeforeDate(ctx, date)
		count, err := s.DeleteIteratorDocs(ctx, iter)
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
func (s *WorkspaceApp) DeleteIteratorDocs(ctx context.Context, iter *firestore.DocumentIterator) (int, error) {
	count := 0 // iterのアイテムの件数
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
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
				if err := s.Repository.DeleteDocRef(ctx, tx, doc.Ref); err != nil {
					return fmt.Errorf("in DeleteDocRef(): %w", err)
				}
			}
		}
		return nil
	})
	return count, txErr
}

func (s *WorkspaceApp) CheckIfUserSittingTooMuchForSeat(ctx context.Context, userId string, seatId int, isMemberSeat bool) (bool, error) {
	jstNow := utils.JstNow()

	// ホワイトリスト・ブラックリストを検索
	whiteListForUserAndSeat, err := s.Repository.ReadSeatLimitsWHITEListWithSeatIdAndUserId(ctx, seatId, userId, isMemberSeat)
	if err != nil {
		return false, fmt.Errorf("in ReadSeatLimitsWHITEListWithSeatIdAndUserId(): %w", err)
	}
	blackListForUserAndSeat, err := s.Repository.ReadSeatLimitsBLACKListWithSeatIdAndUserId(ctx, seatId, userId, isMemberSeat)
	if err != nil {
		return false, fmt.Errorf("in ReadSeatLimitsBLACKListWithSeatIdAndUserId(): %w", err)
	}

	// もし両方あったら矛盾なのでエラー
	if len(whiteListForUserAndSeat) > 0 && len(blackListForUserAndSeat) > 0 {
		return false, errors.New("len(whiteListForUserAndSeat) > 0 && len(blackListForUserAndSeat) > 0")
	}

	// 片方しかなければチェックは不要
	if len(whiteListForUserAndSeat) > 1 {
		return false, errors.New(fmt.Sprintf("len(whiteListForUserAndSeat) > 1, seatId=%d, userId=%s", seatId, userId))
	} else if len(whiteListForUserAndSeat) == 1 {
		if whiteListForUserAndSeat[0].Until.After(jstNow) {
			slog.Info("[seat " + strconv.Itoa(seatId) + ": " + userId + "] found in white list. skipping.")
			return false, nil
		}
		// ホワイトリストに入っているが、期限切れのためチェックを続行
	}
	if len(blackListForUserAndSeat) > 1 {
		return false, errors.New(fmt.Sprintf("len(blackListForUserAndSeat) > 1, seatId=%d, userId=%s", seatId, userId))
	} else if len(blackListForUserAndSeat) == 1 {
		if blackListForUserAndSeat[0].Until.After(jstNow) {
			slog.Info("[seat " + strconv.Itoa(seatId) + ": " + userId + "] found in black list. skipping.")
			return true, nil
		}
		// ブラックリストに入っているが、期限切れのためチェックを続行
	}

	totalEntryDuration, err := s.GetRecentUserSittingTimeForSeat(ctx, userId, seatId, isMemberSeat)
	if err != nil {
		return false, fmt.Errorf("in GetRecentUserSittingTimeForSeat(): %w", err)
	}

	slog.Info("",
		"userId", userId,
		"seatId", seatId,
		"過去何分", s.Configs.Constants.RecentRangeMin,
		"合計何分", int(totalEntryDuration.Minutes()))

	// 制限値と比較
	ifSittingTooMuch := int(totalEntryDuration.Minutes()) > s.Configs.Constants.RecentThresholdMin

	if !ifSittingTooMuch {
		until := jstNow.Add(time.Duration(s.Configs.Constants.RecentThresholdMin)*time.Minute - totalEntryDuration)
		if until.Sub(jstNow) > time.Duration(s.Configs.Constants.MinimumCheckLongTimeSittingIntervalMinutes)*time.Minute {
			// ホワイトリストに登録
			if err := s.Repository.CreateSeatLimitInWHITEList(ctx, seatId, userId, jstNow, until, isMemberSeat); err != nil {
				return false, fmt.Errorf("in CreateSeatLimitInWHITEList(): %w", err)
			}
			slog.Info("[seat " + strconv.Itoa(seatId) + ": " + userId + "] saved to white list.")
		}
	} else {
		// ブラックリストに登録
		until := jstNow.Add(time.Duration(s.Configs.Constants.LongTimeSittingPenaltyMinutes) * time.Minute)
		if err := s.Repository.CreateSeatLimitInBLACKList(ctx, seatId, userId, jstNow, until, isMemberSeat); err != nil {
			return false, fmt.Errorf("in CreateSeatLimitInBLACKList(): %w", err)
		}
		slog.Info("[seat " + strconv.Itoa(seatId) + ": " + userId + "] saved to black list.")
	}

	return ifSittingTooMuch, nil
}

func (s *WorkspaceApp) GetRecentUserSittingTimeForSeat(ctx context.Context, userId string, seatId int, isMemberSeat bool) (time.Duration, error) {
	checkDurationFrom := utils.JstNow().Add(-time.Duration(s.Configs.Constants.RecentRangeMin) * time.Minute)

	// 指定期間の該当ユーザーの該当座席への入退室ドキュメントを取得する
	enterRoomActivities, err := s.Repository.GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx, checkDurationFrom, userId, seatId, isMemberSeat)
	if err != nil {
		return 0, fmt.Errorf("in "+utils.NameOf(s.Repository.GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat)+": %w", err)
	}
	exitRoomActivities, err := s.Repository.GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx, checkDurationFrom, userId, seatId, isMemberSeat)
	if err != nil {
		return 0, fmt.Errorf("in "+utils.NameOf(s.Repository.GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat)+": %w", err)
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
				totalEntryDuration += utils.NoNegativeDuration(utils.JstNow().Sub(activity.TakenAt))
			}
			continue
		} else if activity.ActivityType == repository.ExitRoomActivity {
			entryCount += 1
			totalEntryDuration += utils.NoNegativeDuration(activity.TakenAt.Sub(lastEnteredTimestamp))
		}
	}
	return totalEntryDuration, nil
}

func (s *WorkspaceApp) BanUser(ctx context.Context, userId string) error {
	if err := s.LiveChatBot.BanUser(ctx, userId); err != nil {
		return fmt.Errorf("in BanUser: %w", err)
	}
	return nil
}

// GetMenuItemByNumber メニュー番号からメニューアイテムを取得する。
func (s *WorkspaceApp) GetMenuItemByNumber(number int) (repository.MenuDoc, error) {
	if len(s.SortedMenuItems) < number {
		return repository.MenuDoc{}, errors.Errorf("invalid menu number: %d, menuItems length = %d.", number, len(s.SortedMenuItems))
	}
	return s.SortedMenuItems[number-1], nil
}

// GetUserRealtimeSeatAppearance リアルタイムの現在のランクを求める
func (s *WorkspaceApp) GetUserRealtimeSeatAppearance(ctx context.Context, tx *firestore.Transaction, userId string) (repository.SeatAppearance, error) {
	userDoc, err := s.Repository.ReadUser(ctx, tx, userId)
	if err != nil {
		return repository.SeatAppearance{}, fmt.Errorf("in ReadUser(): %w", err)
	}
	totalStudyDuration, _, err := s.GetUserRealtimeTotalStudyDurations(ctx, tx, userId)
	if err != nil {
		return repository.SeatAppearance{}, fmt.Errorf("in GetUserRealtimeTotalStudyDurations(): %w", err)
	}
	seatAppearance, err := utils.GetSeatAppearance(int(totalStudyDuration.Seconds()), userDoc.RankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
	if err != nil {
		return repository.SeatAppearance{}, fmt.Errorf("in GetSeatAppearance(): %w", err)
	}
	return seatAppearance, nil
}

// RandomAvailableSeatIdForUser
// ルームの席が空いているならその中からランダムな席番号（該当ユーザーの入室上限にかからない範囲に限定）を、
// 空いていないならmax-seatsを増やし、最小の空席番号を返す。
func (s *WorkspaceApp) RandomAvailableSeatIdForUser(ctx context.Context, tx *firestore.Transaction, userId string, isMemberSeat bool) (int,
	error) {
	var seats []repository.SeatDoc
	var err error
	if isMemberSeat {
		seats, err = s.Repository.ReadMemberSeats(ctx)
		if err != nil {
			return 0, fmt.Errorf("in ReadMemberSeats: %w", err)
		}
	} else {
		seats, err = s.Repository.ReadGeneralSeats(ctx)
		if err != nil {
			return 0, fmt.Errorf("in ReadGeneralSeats: %w", err)
		}
	}

	constants, err := s.Repository.ReadSystemConstantsConfig(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("in ReadSystemConstantsConfig: %w", err)
	}
	var maxSeats int
	if isMemberSeat {
		maxSeats = constants.MemberMaxSeats
	} else {
		maxSeats = constants.MaxSeats
	}

	var vacantSeatIdList []int
	for id := 1; id <= maxSeats; id++ {
		isUsed := false
		for _, seatInUse := range seats {
			if id == seatInUse.SeatId {
				isUsed = true
				break
			}
		}
		if !isUsed {
			vacantSeatIdList = append(vacantSeatIdList, id)
		}
	}

	if len(vacantSeatIdList) > 0 {
		// 入室制限にかからない席を選ぶ
		r := rand.New(rand.NewSource(utils.JstNow().UnixNano()))
		r.Shuffle(len(vacantSeatIdList), func(i, j int) { vacantSeatIdList[i], vacantSeatIdList[j] = vacantSeatIdList[j], vacantSeatIdList[i] })
		for _, seatId := range vacantSeatIdList {
			ifSittingTooMuch, err := s.CheckIfUserSittingTooMuchForSeat(ctx, userId, seatId, isMemberSeat)
			if err != nil {
				return -1, fmt.Errorf("in CheckIfUserSittingTooMuchForSeat: %w", err)
			}
			if !ifSittingTooMuch {
				return seatId, nil
			}
		}
	}
	return 0, studyspaceerror.ErrNoSeatAvailable
}

// enterRoom ユーザーを入室させる。
func (s *WorkspaceApp) enterRoom(
	ctx context.Context,
	tx *firestore.Transaction,
	userId string,
	userDisplayName string,
	userProfileImageUrl string,
	seatId int,
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
		SeatId:                 seatId,
		UserId:                 userId,
		UserDisplayName:        userDisplayName,
		UserProfileImageUrl:    userProfileImageUrl,
		WorkName:               workName,
		BreakWorkName:          breakWorkName,
		EnteredAt:              enterDate,
		Until:                  exitDate,
		Appearance:             seatAppearance,
		MenuCode:               menuCode,
		State:                  state,
		CurrentStateStartedAt:  currentStateStartedAt,
		CurrentStateUntil:      currentStateUntil,
		CumulativeWorkSec:      0,
		DailyCumulativeWorkSec: 0,
	}
	if err := s.Repository.CreateSeat(tx, newSeat, isMemberSeat); err != nil {
		return 0, fmt.Errorf("in CreateSeat: %w", err)
	}

	// 入室時刻を記録
	if err := s.Repository.UpdateUserLastEnteredDate(tx, userId, enterDate); err != nil {
		return 0, fmt.Errorf("in UpdateUserLastEnteredDate: %w", err)
	}
	// activityログ記録
	enterActivity := repository.UserActivityDoc{
		UserId:       userId,
		ActivityType: repository.EnterRoomActivity,
		SeatId:       seatId,
		IsMemberSeat: isMemberSeat,
		TakenAt:      enterDate,
	}
	if err := s.Repository.CreateUserActivityDoc(ctx, tx, enterActivity); err != nil {
		return 0, fmt.Errorf("in CreateUserActivityDoc: %w", err)
	}
	// 久しぶりの入室であれば、isContinuousActiveをtrueに、lastPenaltyImposedDaysを0に更新
	if !isContinuousActive {
		if err := s.Repository.UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(ctx, tx, userId, true, enterDate); err != nil {
			return 0, fmt.Errorf("in UpdateUserIsContinuousActiveAndCurrentActivityStateStarted: %w", err)
		}
		if err := s.Repository.UpdateUserLastPenaltyImposedDays(ctx, tx, userId, 0); err != nil {
			return 0, fmt.Errorf("in UpdateUserLastPenaltyImposedDays: %w", err)
		}
	}

	// 入室から自動退室予定時刻までの時間（分）
	untilExitMin := int(exitDate.Sub(enterDate).Minutes())

	return untilExitMin, nil
}

// exitRoom ユーザーを退室させる。
func (s *WorkspaceApp) exitRoom(
	ctx context.Context,
	tx *firestore.Transaction,
	isMemberSeat bool,
	previousSeat repository.SeatDoc,
	previousUserDoc *repository.UserDoc,
) (int, int, error) {
	// 作業時間を計算
	exitDate := utils.JstNow()
	var addedWorkedTimeSec int
	var addedDailyWorkedTimeSec int
	switch previousSeat.State {
	case repository.BreakState:
		addedWorkedTimeSec = previousSeat.CumulativeWorkSec
		// もし直前の休憩で日付を跨いでたら
		justBreakTimeSec := int(utils.NoNegativeDuration(exitDate.Sub(previousSeat.CurrentStateStartedAt)).Seconds())
		if justBreakTimeSec > utils.SecondsOfDay(exitDate) {
			addedDailyWorkedTimeSec = 0
		} else {
			addedDailyWorkedTimeSec = previousSeat.DailyCumulativeWorkSec
		}
	case repository.WorkState:
		justWorkedTimeSec := int(utils.NoNegativeDuration(exitDate.Sub(previousSeat.CurrentStateStartedAt)).Seconds())
		addedWorkedTimeSec = previousSeat.CumulativeWorkSec + justWorkedTimeSec
		// もし日付変更を跨いで入室してたら、当日の累計時間は日付変更からの時間にする
		if justWorkedTimeSec > utils.SecondsOfDay(exitDate) {
			addedDailyWorkedTimeSec = utils.SecondsOfDay(exitDate)
		} else {
			addedDailyWorkedTimeSec = previousSeat.DailyCumulativeWorkSec + justWorkedTimeSec
		}
	}

	// 退室処理
	if err := s.Repository.DeleteSeat(ctx, tx, previousSeat.SeatId, isMemberSeat); err != nil {
		return 0, 0, fmt.Errorf("in DeleteSeat: %w", err)
	}

	// ログ記録
	exitActivity := repository.UserActivityDoc{
		UserId:       previousSeat.UserId,
		ActivityType: repository.ExitRoomActivity,
		SeatId:       previousSeat.SeatId,
		IsMemberSeat: isMemberSeat,
		TakenAt:      exitDate,
	}
	if err := s.Repository.CreateUserActivityDoc(ctx, tx, exitActivity); err != nil {
		return 0, 0, fmt.Errorf("in CreateUserActivityDoc: %w", err)
	}
	// 退室時刻を記録
	if err := s.Repository.UpdateUserLastExitedDate(tx, previousSeat.UserId, exitDate); err != nil {
		return 0, 0, fmt.Errorf("in UpdateUserLastExitedDate: %w", err)
	}
	// 累計作業時間を更新
	if err := s.UpdateTotalWorkTime(tx, previousSeat.UserId, previousUserDoc, addedWorkedTimeSec, addedDailyWorkedTimeSec); err != nil {
		return 0, 0, fmt.Errorf("in UpdateTotalWorkTime: %w", err)
	}
	// RP更新
	netStudyDuration := time.Duration(addedWorkedTimeSec) * time.Second
	newRP, err := utils.CalcNewRPExitRoom(netStudyDuration, previousSeat.WorkName != "", previousUserDoc.IsContinuousActive, previousUserDoc.CurrentActivityStateStarted, exitDate, previousUserDoc.RankPoint)
	if err != nil {
		return 0, 0, fmt.Errorf("in CalcNewRPExitRoom: %w", err)
	}
	if err := s.Repository.UpdateUserRankPoint(tx, previousSeat.UserId, newRP); err != nil {
		return 0, 0, fmt.Errorf("in UpdateUserRP: %w", err)
	}
	addedRP := newRP - previousUserDoc.RankPoint

	slog.Info("user exited the room.",
		"userId", previousSeat.UserId,
		"seatId", previousSeat.SeatId,
		"addedWorkedTimeSec", addedWorkedTimeSec,
		"addedRP", addedRP,
		"newRP", newRP,
		"previous RP", previousUserDoc.RankPoint)
	return addedWorkedTimeSec, addedRP, nil
}

func (s *WorkspaceApp) moveSeat(
	ctx context.Context,
	tx *firestore.Transaction,
	targetSeatId int,
	latestUserProfileImage string,
	beforeIsMemberSeat,
	afterIsMemberSeat bool,
	option utils.MinWorkOrderOption,
	previousSeat repository.SeatDoc,
	previousUserDoc *repository.UserDoc,
) (int, int, int, error) {
	jstNow := utils.JstNow()

	// 値チェック
	if targetSeatId == previousSeat.SeatId && beforeIsMemberSeat == afterIsMemberSeat {
		return 0, 0, 0, errors.New("targetSeatId == previousSeat.SeatId && beforeIsMemberSeat == afterIsMemberSeat")
	}

	// 退室
	workedTimeSec, addedRP, err := s.exitRoom(ctx, tx, beforeIsMemberSeat, previousSeat, previousUserDoc)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("in exitRoom for %s: %w", s.ProcessedUserId, err)
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
		workMin = int(utils.NoNegativeDuration(previousSeat.Until.Sub(jstNow)).Minutes())
	}
	newTotalStudyDuration := time.Duration(previousUserDoc.TotalStudySec+workedTimeSec) * time.Second
	newRP := previousUserDoc.RankPoint + addedRP
	newSeatAppearance, err := utils.GetSeatAppearance(int(newTotalStudyDuration.Seconds()), previousUserDoc.RankVisible, newRP, previousUserDoc.FavoriteColor)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("in GetSeatAppearance: %w", err)
	}

	// 入室
	untilExitMin, err := s.enterRoom(
		ctx,
		tx,
		previousSeat.UserId,
		previousSeat.UserDisplayName,
		latestUserProfileImage,
		targetSeatId,
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
		utils.JstNow())
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to enterRoom for %s: %w", previousSeat.UserId, err)
	}

	return workedTimeSec, addedRP, untilExitMin, nil
}
