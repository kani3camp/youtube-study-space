package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/serviceaccess"
	"app.modules/core/timeutil"
)

// 明示した1ユーザー・1か月を再開単位にする。外から渡す運用操作であり、概要GETからは呼ばない。
// WriterEpochは運用担当者のレビュー済み切替IDであり、旧書き込み処理の完了宣言ではない。
// 全月・全再試行で元の制御チェックポイントを維持する必要がある。
// プライバシー削除後に新しく取得すると、データが復活してしまう。
type WorkMonthBackfill struct {
	UID, Month, WriterEpoch, ControlCheckpoint string
	ExpectedVersion                            string
	MaxDocuments                               int
	Apply                                      bool
}

type WorkMonthBackfillReport struct {
	Documents               int
	PreviousSec, RebuiltSec int64
	Changed, Applied        bool
	SourceVersion           string
}

func (c *FirestoreControllerImplements) RebuildWorkMonth(ctx context.Context, input WorkMonthBackfill) (WorkMonthBackfillReport, error) {
	var result WorkMonthBackfillReport
	start, err := time.ParseInLocation("2006-01", input.Month, timeutil.JapanLocation())
	if err != nil || start.Format("2006-01") != input.Month || input.UID == "" || strings.Contains(input.UID, "/") || input.WriterEpoch == "" || len(input.WriterEpoch) > 64 || strings.ContainsAny(input.WriterEpoch, "/\x00\r\n") || input.ControlCheckpoint == "" || input.MaxDocuments < 1 || input.MaxDocuments > 10000 || (input.Apply && len(input.ExpectedVersion) != 64) {
		return result, ErrWorkHistoryConflict
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err = c.firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, readErr := tx.Get(c.firestoreClient.Collection(serviceaccess.Collection).Doc(input.UID))
		control, err := serviceaccess.DecodeSnapshot(doc, readErr)
		if err != nil {
			return fmt.Errorf("read migration privacy fence: %w", err)
		}
		if control.Control.PrivacyDeletion.Active || control.Checkpoint() != input.ControlCheckpoint {
			return ErrWorkHistoryConflict
		}
		if _, err := tx.Get(c.usersCollection().Doc(input.UID)); err != nil {
			if status.Code(err) != codes.NotFound {
				return fmt.Errorf("read migration user: %w", err)
			}
			return fmt.Errorf("migration requires existing user: %w", ErrWorkHistoryConflict)
		}
		ref := c.firestoreClient.Collection(WorkMonths).Doc(WorkMonthID(input.UID, input.Month))
		version := input.UID + "\x00" + input.Month + "\x00" + input.WriterEpoch + "\x00" + input.ControlCheckpoint
		previous := ""
		doc, err = tx.Get(ref)
		if err == nil {
			old, err := DecodeWorkMonth(doc, input.UID, input.Month)
			if err != nil {
				return ErrWorkHistoryConflict
			}
			encoded, err := json.Marshal(old)
			if err != nil {
				return fmt.Errorf("encode previous month comparison: %w", err)
			}
			previous = string(encoded)
			version += "\x00" + doc.UpdateTime.Format(time.RFC3339Nano)
			for _, day := range old.Days {
				result.PreviousSec += day.WorkSec
			}
		} else if status.Code(err) != codes.NotFound {
			return fmt.Errorf("read migration month: %w", err)
		}
		// 上限付きでUIDの原記録を調べ、日付不等式やorderByクエリで除外される不正行も対象にする。
		// 上限超過時は部分合計を公開しない。大きな履歴には別途レビューした移行を必要とする。
		query := c.workSegmentsCollection().Where(UserIDDocProperty, "==", input.UID).
			OrderBy(firestore.DocumentID, firestore.Asc).Limit(input.MaxDocuments + 1)
		iter := tx.Documents(query)
		defer iter.Stop()
		var docs []*firestore.DocumentSnapshot
		encodedBytes := 0
		for {
			doc, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return fmt.Errorf("read bounded migration source: %w", err)
			}
			encoded, err := json.Marshal(doc.Data())
			if err != nil {
				return fmt.Errorf("measure migration payload: %w", err)
			}
			encodedBytes += len(encoded)
			if encodedBytes > 4*1024*1024 {
				return ErrWorkHistoryConflict
			}
			docs = append(docs, doc)
		}
		result.Documents = len(docs)
		if len(docs) > input.MaxDocuments {
			return ErrWorkHistoryConflict
		}
		segments := make([]WorkSegmentDoc, len(docs))
		for i, doc := range docs {
			segments[i], err = DecodeWorkSegment(doc, input.UID)
			if err != nil {
				return ErrWorkHistoryConflict
			}
			s := segments[i]
			if s.StartedAt.IsZero() || !s.EndedAt.After(s.StartedAt) || s.EndedAt.After(doc.ReadTime) || s.DurationSec < 0 || int64(s.DurationSec) != int64(s.EndedAt.Sub(s.StartedAt)/time.Second) || (s.SegmentType != WorkState && s.SegmentType != BreakState) {
				return ErrWorkHistoryConflict
			}
			version += "\x00" + doc.Ref.ID + "\x00" + doc.UpdateTime.Format(time.RFC3339Nano)
		}
		result.SourceVersion = workHash(version)
		if input.Apply && result.SourceVersion != input.ExpectedVersion {
			return ErrWorkHistoryConflict
		}
		slices.SortFunc(segments, func(a, b WorkSegmentDoc) int { return a.StartedAt.Compare(b.StartedAt) })
		value := WorkMonthDoc{UserID: input.UID, Month: input.Month, SchemaVersion: 1, WriterEpoch: input.WriterEpoch, BaselineComplete: true, Days: map[string]WorkMonthDay{}}
		for _, segment := range segments {
			if (!segment.StartedAt.Before(start.AddDate(0, 1, 0)) || !segment.EndedAt.After(start)) && segment.EndedAt.In(timeutil.JapanLocation()).Format("2006-01") != input.Month {
				continue
			}
			if err := value.Append(segment); err != nil {
				return err
			}
		}
		for _, day := range value.Days {
			if day.Invalid {
				return ErrWorkHistoryConflict
			}
			result.RebuiltSec += day.WorkSec
		}
		if err := value.Validate(input.UID, input.Month); err != nil {
			return err
		}
		// 報告には数値だけを含め、元の名前・識別子・パスは外に出さない。
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("encode rebuilt month comparison: %w", err)
		}
		result.Changed = previous != string(encoded)
		if input.Apply {
			if err := tx.Set(ref, value); err != nil {
				return fmt.Errorf("publish rebuilt month: %w", err)
			}
			result.Applied = true
		}
		return nil
	}, firestore.MaxAttempts(1)) // Conflict needs a new bounded operator decision.
	if err != nil {
		result.Applied = false
		return result, fmt.Errorf("bounded month rebuild: %w", err)
	}
	return result, nil
}
