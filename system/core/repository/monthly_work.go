package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"strings"
	"time"

	"cloud.google.com/go/firestore"

	"app.modules/core/timeutil"
)

const (
	WorkMonths       = "work-months"
	MonthlyWorkEpoch = "monthly-v1"
)

func ValidWorkHistoryWatermark(user UserDoc, asOf time.Time) bool {
	return user.WorkHistoryEpoch == MonthlyWorkEpoch && !user.WorkHistoryActivatedAt.IsZero() &&
		!user.WorkHistoryClosedThrough.Before(user.WorkHistoryActivatedAt) &&
		!user.WorkHistoryClosedThrough.After(asOf) && !user.WorkHistoryActivatedAt.After(asOf)
}

// 1か月には最大31日の小さな値を保持する。名前と元区間はWorkSegmentsだけに置く。
// 文書の存在・作成時刻・新書き込み処理の初回イベントからBaselineCompleteを推測しない。
type WorkMonthDoc struct {
	UserID           string                  `firestore:"user-id"`
	Month            string                  `firestore:"month"`
	SchemaVersion    int                     `firestore:"schema-version"`
	BaselineComplete bool                    `firestore:"baseline-complete"`
	WriterEpoch      string                  `firestore:"writer-epoch"`
	Days             map[string]WorkMonthDay `firestore:"days"`
}

type WorkMonthDay struct {
	WorkSec int64 `firestore:"work-sec"`
	// 日跨ぎでも、原記録の週集計と同じく区間ごとに一度だけ秒を切り捨てる。
	CrossDayCarrySec      int64     `firestore:"cross-day-carry-sec"`
	CompletedWorkSegments int64     `firestore:"completed-work-segments"`
	SourceEnd             time.Time `firestore:"source-ended-at"`
	First                 time.Time `firestore:"first"`
	Last                  time.Time `firestore:"last"`
	Invalid               bool      `firestore:"invalid"`
}

var ErrWorkHistoryConflict = errors.New("work history conflict")

func workHash(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func WorkMonthID(uid, month string) string { return workHash(uid + "\x00" + month) }

func WorkSegmentMonths(s WorkSegmentDoc) ([]string, error) {
	if s.UserID == "" || strings.Contains(s.UserID, "/") || s.StartedAt.IsZero() || s.EndedAt.IsZero() || s.EndedAt.Before(s.StartedAt) || s.EndedAt.Sub(s.StartedAt) > 366*24*time.Hour {
		return nil, ErrWorkHistoryConflict
	}
	end := s.EndedAt
	if end.After(s.StartedAt) {
		end = end.Add(-time.Nanosecond)
	}
	var months []string
	for at := monthStart(s.StartedAt); !at.After(end); at = at.AddDate(0, 1, 0) {
		months = append(months, at.Format("2006-01"))
	}
	return months, nil
}

func (m WorkMonthDoc) Validate(uid, month string) error {
	start, err := time.ParseInLocation("2006-01", month, timeutil.JapanLocation())
	if err != nil || start.Format("2006-01") != month || m.UserID != uid || m.Month != month || m.SchemaVersion != 1 || len(m.Days) > 31 || (m.BaselineComplete && m.WriterEpoch == "") {
		return ErrWorkHistoryConflict
	}
	for key, day := range m.Days {
		date, err := time.ParseInLocation("2006-01-02", key, timeutil.JapanLocation())
		if err != nil || date.Format("2006-01-02") != key || date.Format("2006-01") != month || day.WorkSec < 0 || day.WorkSec > 86400 || day.CrossDayCarrySec < 0 || day.CrossDayCarrySec > 86400 || day.CompletedWorkSegments < 0 || day.First.IsZero() || day.Last.Before(day.First) || day.SourceEnd.Before(day.Last) || day.First.Before(date) || day.Last.After(date.AddDate(0, 0, 1)) {
			return ErrWorkHistoryConflict
		}
	}
	return nil
}

// Appendは、書き込み元の順序付き・重複なし区間に対して正確に集計する。
// 遅延・重複・破損記録では対象日を無効化し、得られた合計を正確な値として表示しない。
// 明示的な上限付き再集計で修復できる。
func (m *WorkMonthDoc) Append(s WorkSegmentDoc) error {
	if m.Days == nil {
		m.Days = map[string]WorkMonthDay{}
	}
	start, err := time.ParseInLocation("2006-01", m.Month, timeutil.JapanLocation())
	if err != nil {
		return ErrWorkHistoryConflict
	}
	monthEnd := start.AddDate(0, 1, 0)
	valid := s.EndedAt.After(s.StartedAt) && s.DurationSec >= 0 && int64(s.DurationSec) == int64(s.EndedAt.Sub(s.StartedAt)/time.Second) && (s.SegmentType == WorkState || s.SegmentType == BreakState)
	startDay := s.StartedAt.In(timeutil.JapanLocation()).Format("2006-01-02")
	endDay := s.EndedAt.Add(-time.Nanosecond).In(timeutil.JapanLocation()).Format("2006-01-02")
	carry := valid && s.SegmentType == WorkState && startDay != endDay && s.StartedAt.Nanosecond() > 0 && s.EndedAt.Nanosecond() >= s.StartedAt.Nanosecond()
	for date := start; date.Before(monthEnd); date = date.AddDate(0, 0, 1) {
		a, b := s.StartedAt, s.EndedAt
		if a.Before(date) {
			a = date
		}
		if b.After(date.AddDate(0, 0, 1)) {
			b = date.AddDate(0, 0, 1)
		}
		if b.Before(a) || (b.Equal(a) && !s.StartedAt.Equal(s.EndedAt)) {
			continue
		}
		if a.Before(date) || !a.Before(date.AddDate(0, 0, 1)) {
			continue
		}
		key := date.Format("2006-01-02")
		day, exists := m.Days[key]
		day.Invalid = day.Invalid || !valid || (exists && a.Before(day.Last))
		if !exists || a.Before(day.First) {
			day.First = a
		}
		if b.After(day.Last) {
			day.Last = b
		}
		if s.EndedAt.After(day.SourceEnd) {
			day.SourceEnd = s.EndedAt
		}
		if valid && s.SegmentType == WorkState {
			seconds := int64(b.Sub(a) / time.Second)
			if seconds < 0 || day.WorkSec > math.MaxInt64-seconds {
				return ErrWorkHistoryConflict
			}
			day.WorkSec += seconds
			if carry && key == startDay {
				day.CrossDayCarrySec++
			}
			if day.WorkSec > 86400 {
				day.Invalid = true
				day.WorkSec = 86400
			}
		}
		m.Days[key] = day
	}
	if s.EndedAt.In(timeutil.JapanLocation()).Format("2006-01") == m.Month {
		key := s.EndedAt.In(timeutil.JapanLocation()).Format("2006-01-02")
		day, exists := m.Days[key]
		if !exists {
			day.First = s.EndedAt
			day.Last = s.EndedAt
			day.SourceEnd = s.EndedAt
		}
		if s.SegmentType == WorkState {
			if day.CompletedWorkSegments == math.MaxInt64 {
				return ErrWorkHistoryConflict
			}
			day.CompletedWorkSegments++
		}
		m.Days[key] = day
	}
	return nil
}

// DecodeWorkMonthは日別フィールドの欠落を拒否する。
// 欠落を信頼できる終了件数0や、原記録の整合性境界の欠如として扱わない。
func DecodeWorkMonth(doc *firestore.DocumentSnapshot, uid, month string) (WorkMonthDoc, error) {
	var value WorkMonthDoc
	if doc == nil || !doc.Exists() {
		return value, ErrWorkHistoryConflict
	}
	data := doc.Data()
	if len(data) != 6 {
		return value, ErrWorkHistoryConflict
	}
	for _, key := range []string{"user-id", "month", "schema-version", "baseline-complete", "writer-epoch", "days"} {
		if _, ok := data[key]; !ok {
			return value, ErrWorkHistoryConflict
		}
	}
	for _, key := range []string{"user-id", "month", "writer-epoch"} {
		if _, ok := data[key].(string); !ok {
			return value, ErrWorkHistoryConflict
		}
	}
	if _, ok := data["schema-version"].(int64); !ok {
		return value, ErrWorkHistoryConflict
	}
	if _, ok := data["baseline-complete"].(bool); !ok {
		return value, ErrWorkHistoryConflict
	}
	days, ok := data["days"].(map[string]any)
	if !ok {
		return value, ErrWorkHistoryConflict
	}
	for _, raw := range days {
		day, ok := raw.(map[string]any)
		if !ok || len(day) != 7 {
			return value, ErrWorkHistoryConflict
		}
		for _, key := range []string{"work-sec", "cross-day-carry-sec", "completed-work-segments", "source-ended-at", "first", "last", "invalid"} {
			if _, ok := day[key]; !ok {
				return value, ErrWorkHistoryConflict
			}
		}
		for _, key := range []string{"work-sec", "cross-day-carry-sec", "completed-work-segments"} {
			if _, ok := day[key].(int64); !ok {
				return value, ErrWorkHistoryConflict
			}
		}
		for _, key := range []string{"source-ended-at", "first", "last"} {
			if _, ok := day[key].(time.Time); !ok {
				return value, ErrWorkHistoryConflict
			}
		}
		if _, ok := day["invalid"].(bool); !ok {
			return value, ErrWorkHistoryConflict
		}
	}
	if doc.DataTo(&value) != nil || value.Validate(uid, month) != nil {
		return value, ErrWorkHistoryConflict
	}
	return value, nil
}

func DecodeWorkSegment(doc *firestore.DocumentSnapshot, uid string) (WorkSegmentDoc, error) {
	var value WorkSegmentDoc
	if doc == nil || !doc.Exists() {
		return value, ErrWorkHistoryConflict
	}
	data := doc.Data()
	for _, key := range []string{"user-id", "segment-type"} {
		if _, ok := data[key].(string); !ok {
			return value, ErrWorkHistoryConflict
		}
	}
	for _, key := range []string{"started-at", "ended-at"} {
		if _, ok := data[key].(time.Time); !ok {
			return value, ErrWorkHistoryConflict
		}
	}
	if _, ok := data["duration-sec"].(int64); !ok {
		return value, ErrWorkHistoryConflict
	}
	for _, key := range []string{"work-name", "session-id"} {
		if raw, exists := data[key]; exists {
			if _, ok := raw.(string); !ok {
				return value, ErrWorkHistoryConflict
			}
		}
	}
	if raw, exists := data["seat-id"]; exists {
		if _, ok := raw.(int64); !ok {
			return value, ErrWorkHistoryConflict
		}
	}
	if raw, exists := data["is-member-seat"]; exists {
		if _, ok := raw.(bool); !ok {
			return value, ErrWorkHistoryConflict
		}
	}
	if doc.DataTo(&value) != nil || value.UserID != uid {
		return value, ErrWorkHistoryConflict
	}
	return value, nil
}

func monthStart(t time.Time) time.Time {
	jst := t.In(timeutil.JapanLocation())
	return time.Date(jst.Year(), jst.Month(), 1, 0, 0, 0, 0, jst.Location())
}
