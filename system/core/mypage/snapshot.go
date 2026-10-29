package mypage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"app.modules/core/repository"
	"app.modules/core/timeutil"

	"cloud.google.com/go/firestore"
)

// SnapshotReadObservationには上限付き件数とクエリエラーフラグだけを含める。
// チャンネル識別子・文書パス・データ・エラーメッセージは保存しない。
// 返却文書数は課金読み取り数とは異なる。空クエリの最低課金・インデックス・再試行を含む
// 全体費用はQuery Explain / Cloud Billingで確認する。クエリ失敗を正常な空結果として扱わない。
type SnapshotReadObservation struct {
	UserLookups         int
	SeatQueries         int
	SeatsReturned       int
	SeatQueryErrors     int
	SegmentQueries      int
	SegmentsReturned    int
	SegmentQueryError   bool
	MonthLookups        int
	MonthsReturned      int
	MonthReadErrors     int
	ElapsedMilliseconds int64
}

type SnapshotReader interface {
	Read(context.Context, string) (WorkSnapshot, error)
}

type FirestoreSnapshotReader struct {
	Client *firestore.Client
	// 完全性を確認した履歴期間だけを設定できる。既定のepochは設けない。
	Coverage []Interval
	// 空なら従来の原記録読み取りを使い、明示的な導入・ロールバックに対応する。
	// epochは旧書き込み処理の完了を待った後の、検証済み上限付き移行に一致する必要がある。
	MonthlyEpoch string
	// 任意のフックで、スナップショット読み取り成功後に一度だけ呼ぶ。
	Observe func(SnapshotReadObservation)
}

func (r *FirestoreSnapshotReader) Read(ctx context.Context, uid string) (WorkSnapshot, error) {
	if uid == "" || strings.Contains(uid, "/") {
		return WorkSnapshot{}, apiError("AUTH_REQUIRED")
	}
	var result WorkSnapshot
	var observation SnapshotReadObservation
	started := time.Now()
	err := r.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// トランザクションの試行ごとに新しい件数を使い、成功後に一度だけ報告する。
		// 再試行で発生する追加の課金読み取りは、この件数に含めない。
		attempt := SnapshotReadObservation{UserLookups: 1}
		defer func() { observation = attempt }()
		s := WorkSnapshot{UID: uid, Coverage: r.Coverage}
		// GetAllは利用者がいなくてもreadTimeを返す。スナップショット確定後にこれを使って
		// JSTの期間を決め、リクエスト時刻の境界を使わない。
		docs, err := tx.GetAll([]*firestore.DocumentRef{r.Client.Collection("users").Doc(uid)})
		if err != nil {
			return fmt.Errorf("read snapshot user: %w", err)
		}
		if len(docs) != 1 || docs[0].ReadTime.IsZero() {
			return fmt.Errorf("snapshot user read time missing")
		}
		s.AsOf = docs[0].ReadTime.UTC()
		if !docs[0].Exists() {
			result = s
			return nil
		}
		var user repository.UserDoc
		if err := docs[0].DataTo(&user); err != nil {
			return fmt.Errorf("decode snapshot user: %w", err)
		}
		s.User = &user
		epoch := r.MonthlyEpoch
		if epoch == "" {
			epoch = user.WorkHistoryEpoch
		}
		userFields := docs[0].Data()
		if _, ok := userFields["total-study-sec"].(int64); !ok {
			s.UserTotalInconsistent = true
		}
		rawEpoch, hasEpoch := userFields["work-history-epoch"]
		_, hasActivated := userFields["work-history-activated-at"]
		_, hasThrough := userFields["work-history-closed-through"]
		if hasEpoch {
			if value, ok := rawEpoch.(string); !ok || value != repository.MonthlyWorkEpoch {
				s.HistoryInconsistent = true
				epoch = repository.MonthlyWorkEpoch
			}
		} else if hasActivated || hasThrough {
			s.HistoryInconsistent = true
			epoch = repository.MonthlyWorkEpoch
		}
		if user.WorkHistoryEpoch != "" && user.WorkHistoryEpoch != repository.MonthlyWorkEpoch {
			s.HistoryInconsistent = true
		}
		if user.WorkHistoryEpoch == repository.MonthlyWorkEpoch {
			_, hasWatermark := userFields["work-history-closed-through"]
			if !hasWatermark || !repository.ValidWorkHistoryWatermark(user, s.AsOf) {
				s.HistoryInconsistent = true
			}
		}
		for _, name := range []string{"seats", "member-seats"} {
			attempt.SeatQueries++
			seats, err := tx.Documents(r.Client.Collection(name).Where("user-id", "==", uid).Limit(2)).GetAll()
			if err != nil {
				attempt.SeatQueryErrors++
				s.SeatUnavailable = true
				continue
			}
			attempt.SeatsReturned += len(seats)
			if len(seats) > 1 || (s.Seat != nil && len(seats) > 0) {
				s.SeatInconsistent = true
			}
			if len(seats) > 0 {
				var seat repository.SeatDoc
				if err := seats[0].DataTo(&seat); err != nil {
					s.SeatInconsistent = true
					continue
				}
				s.Seat = &seat
				s.MemberSeat = name == "member-seats"
			}
		}
		window := StatisticsWindows(s.AsOf)
		if epoch != "" {
			s.MonthlyEpoch = epoch
			s.Months = map[string]repository.WorkMonthDoc{}
			first := window.QueryStart().In(timeutil.JapanLocation())
			month := time.Date(first.Year(), first.Month(), 1, 0, 0, 0, 0, first.Location())
			var keys []string
			var refs []*firestore.DocumentRef
			for !month.After(s.AsOf) {
				key := month.Format("2006-01")
				keys = append(keys, key)
				refs = append(refs, r.Client.Collection(repository.WorkMonths).Doc(repository.WorkMonthID(uid, key)))
				month = month.AddDate(0, 1, 0)
			}
			attempt.MonthLookups = len(refs)
			docs, err := tx.GetAll(refs)
			if err != nil || len(docs) != len(refs) {
				attempt.MonthReadErrors = len(refs)
				s.HistoryUnavailable = true
			} else {
				for i, doc := range docs {
					if doc.Exists() {
						attempt.MonthsReturned++
						value, err := repository.DecodeWorkMonth(doc, uid, keys[i])
						if err != nil {
							s.HistoryInconsistent = true
						} else {
							s.Months[keys[i]] = value
						}
					}
				}
			}
			result = s
			return nil
		}
		query := r.Client.Collection("work-segments").Where("user-id", "==", uid).
			Where("ended-at", ">", window.QueryStart()).Where("started-at", "<", s.AsOf).
			OrderBy("ended-at", firestore.Asc).OrderBy("started-at", firestore.Asc).Limit(HistoryReadLimit)
		attempt.SegmentQueries++
		segments, err := tx.Documents(query).GetAll()
		if err != nil {
			attempt.SegmentQueryError = true
			s.HistoryUnavailable = true
		} else {
			attempt.SegmentsReturned = len(segments)
			for _, doc := range segments {
				segment, err := repository.DecodeWorkSegment(doc, uid)
				if err != nil {
					// 読めない区間は、検索期間のどの日にも影響し得る。
					// その区間を捨てて、正確に見える合計を報告しない。
					s.HistoryUnavailable = true
					break
				}
				s.Segments = append(s.Segments, segment)
			}
		}
		result = s
		return nil
	}, firestore.ReadOnly)
	if err != nil {
		return WorkSnapshot{}, fmt.Errorf("read work snapshot: %w", err)
	}
	if r.Observe != nil {
		observation.ElapsedMilliseconds = time.Since(started).Milliseconds()
		r.Observe(observation)
	}
	return result, nil
}
