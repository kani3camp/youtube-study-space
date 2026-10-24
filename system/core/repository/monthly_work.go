package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/serviceaccess"
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

// A month contains at most 31 small day values. Names and original intervals
// remain exclusively in WorkSegments. BaselineComplete is never inferred from
// existence, creation time, or the first new writer event.
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
	// Preserve the raw week's once-per-interval second floor across midnight.
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

func normalizeSegment(s WorkSegmentDoc) WorkSegmentDoc {
	s.StartedAt = s.StartedAt.UTC().Truncate(time.Microsecond)
	s.EndedAt = s.EndedAt.UTC().Truncate(time.Microsecond)
	return s
}

// One source event has one immutable identity, independent of content edits.
// A different payload with the same identity conflicts instead of double adding.
func segmentIdentity(s WorkSegmentDoc) string {
	return workHash(fmt.Sprintf("%s\x00%s\x00%d\x00%t\x00%s\x00%s\x00%s", s.UserID, s.SessionID, s.SeatID, s.IsMemberSeat, s.SegmentType, s.StartedAt.Format(time.RFC3339Nano), s.EndedAt.Format(time.RFC3339Nano)))
}

func segmentFingerprint(s WorkSegmentDoc) string {
	return workHash(fmt.Sprintf("%s\x00%s\x00%d", segmentIdentity(s), s.WorkName, s.DurationSec))
}

func monthStart(t time.Time) time.Time {
	jst := t.In(timeutil.JapanLocation())
	return time.Date(jst.Year(), jst.Month(), 1, 0, 0, 0, 0, jst.Location())
}

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

// Append is exact for the source writer's ordered, non-overlapping intervals.
// Late/overlapping or corrupt records invalidate affected days; callers never
// present the resulting sum as exact. Explicit bounded rebuild can repair them.
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

// DecodeWorkMonth rejects missing day fields; absence must never become a
// trusted zero completion count or a missing source-consistency bound.
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

type (
	segmentPlanKey      struct{}
	historyAdmissionKey struct{}
)

// WithWorkHistoryAdmission binds all retry attempts of one business operation
// to the first privacy checkpoint read for each target. Recreate-after-delete
// cannot turn a stale admitted operation into a fresh legitimate source write.
func WithWorkHistoryAdmission(ctx context.Context) context.Context {
	return context.WithValue(ctx, historyAdmissionKey{}, map[string]string{})
}

func (c *FirestoreControllerImplements) PrepareWorkSegment(ctx context.Context, tx *firestore.Transaction, segment WorkSegmentDoc) (context.Context, error) {
	return PrepareWorkSegmentWrites(ctx, tx, c.firestoreClient, segment)
}

type segmentWritePlan struct {
	tx          *firestore.Transaction
	client      DBClient
	segment     WorkSegmentDoc
	fingerprint string
	ref         *firestore.DocumentRef
	duplicate   bool
	months      []WorkMonthDoc
	user        *UserDoc
}

// PrepareWorkSegmentWrites performs ALL source, control and aggregate reads.
// It must run before any transaction write, including order/activity/seat writes.
// The sealed plan is bound to this exact attempt and normalized source payload.
func PrepareWorkSegmentWrites(ctx context.Context, tx *firestore.Transaction, client DBClient, segment WorkSegmentDoc) (context.Context, error) {
	if tx == nil || client == nil {
		return nil, ErrWorkHistoryConflict
	}
	segment = normalizeSegment(segment)
	if plan, ok := ctx.Value(segmentPlanKey{}).(*segmentWritePlan); ok && plan.tx == tx && plan.client == client && plan.fingerprint == segmentFingerprint(segment) {
		return ctx, nil
	}
	months, err := WorkSegmentMonths(segment)
	if err != nil {
		return nil, err
	}
	completionMonth := segment.EndedAt.In(timeutil.JapanLocation()).Format("2006-01")
	if len(months) == 0 || months[len(months)-1] != completionMonth {
		months = append(months, completionMonth)
	}
	doc, readErr := tx.Get(client.Collection(serviceaccess.Collection).Doc(segment.UserID))
	control, err := serviceaccess.DecodeSnapshot(doc, readErr)
	if err != nil {
		return nil, fmt.Errorf("read history privacy fence: %w", err)
	}
	// Bot moderation remains independent. Only the destructive privacy window
	// fences these new writes; changing the control forces transaction retry.
	if control.Control.PrivacyDeletion.Active {
		return nil, serviceaccess.ErrDeletion
	}
	admission, ok := ctx.Value(historyAdmissionKey{}).(map[string]string)
	if !ok {
		return nil, ErrWorkHistoryConflict
	}
	if checkpoint, exists := admission[segment.UserID]; exists && checkpoint != control.Checkpoint() {
		return nil, ErrWorkHistoryConflict
	}
	admission[segment.UserID] = control.Checkpoint()
	plan := &segmentWritePlan{tx: tx, client: client, segment: segment, fingerprint: segmentFingerprint(segment), ref: client.Collection(WorkSegments).Doc(segmentIdentity(segment))}
	doc, err = tx.Get(plan.ref)
	if err == nil {
		old, err := DecodeWorkSegment(doc, segment.UserID)
		if err != nil || segmentFingerprint(normalizeSegment(old)) != plan.fingerprint {
			return nil, ErrWorkHistoryConflict
		}
		plan.duplicate = true
		return context.WithValue(ctx, segmentPlanKey{}, plan), nil
	}
	if status.Code(err) != codes.NotFound {
		return nil, fmt.Errorf("read history idempotency: %w", err)
	}
	doc, err = tx.Get(client.Collection(USERS).Doc(segment.UserID))
	if err == nil {
		var user UserDoc
		if doc.DataTo(&user) != nil {
			return nil, ErrWorkHistoryConflict
		}
		if user.WorkHistoryEpoch != "" && !ValidWorkHistoryWatermark(user, doc.ReadTime) {
			return nil, ErrWorkHistoryConflict
		}
		plan.user = &user
	} else {
		if status.Code(err) != codes.NotFound {
			return nil, fmt.Errorf("read history user: %w", err)
		}
		return nil, fmt.Errorf("source close requires existing user: %w", ErrWorkHistoryConflict)
	}
	seats := SEATS
	if segment.IsMemberSeat {
		seats = MemberSeats
	}
	seatDoc, err := tx.Get(client.Collection(seats).Doc(fmt.Sprint(segment.SeatID)))
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return nil, fmt.Errorf("read history source seat: %w", err)
		}
		return nil, fmt.Errorf("source close requires existing seat: %w", ErrWorkHistoryConflict)
	}
	var seat SeatDoc
	if seatDoc.DataTo(&seat) != nil || seat.UserID != segment.UserID || seat.SessionID != segment.SessionID || seat.State != segment.SegmentType || !seat.CurrentSegmentStartedAt.Equal(segment.StartedAt) {
		return nil, ErrWorkHistoryConflict
	}
	for _, month := range months {
		value := WorkMonthDoc{UserID: segment.UserID, Month: month, SchemaVersion: 1, Days: map[string]WorkMonthDay{}}
		doc, err = tx.Get(client.Collection(WorkMonths).Doc(WorkMonthID(segment.UserID, month)))
		if err == nil {
			value, err = DecodeWorkMonth(doc, segment.UserID, month)
			if err != nil {
				return nil, ErrWorkHistoryConflict
			}
		} else if status.Code(err) != codes.NotFound {
			return nil, fmt.Errorf("read history month: %w", err)
		} else if plan.user != nil && plan.user.WorkHistoryEpoch == MonthlyWorkEpoch && !plan.user.WorkHistoryActivatedAt.IsZero() {
			start, err := time.ParseInLocation("2006-01", month, timeutil.JapanLocation())
			if err != nil {
				return nil, fmt.Errorf("parse prepared month: %w", err)
			}
			if !start.Before(plan.user.WorkHistoryActivatedAt) && plan.user.WorkHistoryClosedThrough.Before(start) {
				value.BaselineComplete = true
				value.WriterEpoch = MonthlyWorkEpoch
			}
		}
		if err := value.Append(segment); err != nil {
			return nil, err
		}
		plan.months = append(plan.months, value)
	}
	return context.WithValue(ctx, segmentPlanKey{}, plan), nil
}

func (c *FirestoreControllerImplements) createWorkSegmentWithMonths(ctx context.Context, tx *firestore.Transaction, segment WorkSegmentDoc) error {
	segment = normalizeSegment(segment)
	if tx == nil {
		return fmt.Errorf("source close requires an admitted seat transaction: %w", ErrWorkHistoryConflict)
	}
	plan, ok := ctx.Value(segmentPlanKey{}).(*segmentWritePlan)
	if !ok || plan.tx != tx || plan.client != c.firestoreClient || plan.fingerprint != segmentFingerprint(segment) {
		return fmt.Errorf("missing transaction-bound history preparation: %w", ErrWorkHistoryConflict)
	}
	if plan.duplicate {
		return nil
	}
	if err := tx.Create(plan.ref, segment); err != nil {
		return fmt.Errorf("create history source: %w", err)
	}
	for _, month := range plan.months {
		if err := tx.Set(c.firestoreClient.Collection(WorkMonths).Doc(WorkMonthID(segment.UserID, month.Month)), month); err != nil {
			return fmt.Errorf("write history month: %w", err)
		}
	}
	if plan.user != nil && plan.user.WorkHistoryEpoch == MonthlyWorkEpoch && segment.EndedAt.After(plan.user.WorkHistoryClosedThrough) {
		if err := tx.Update(c.usersCollection().Doc(segment.UserID), []firestore.Update{{Path: "work-history-closed-through", Value: segment.EndedAt}}); err != nil {
			return fmt.Errorf("advance history source checkpoint: %w", err)
		}
	}
	return nil
}
