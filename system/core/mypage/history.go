package mypage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/repository"
	"app.modules/core/serviceaccess"
)

type HistoryEntry struct {
	Key       string    `json:"key"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
	WorkSec   int64     `json:"workSec"`
	WorkName  string    `json:"workName"`
}

type HistoryPage struct {
	SourceConsistent bool           `json:"sourceConsistent"`
	AsOf             time.Time      `json:"asOf"`
	Timezone         string         `json:"timezone"`
	Selection        string         `json:"selection"`
	Coverage         string         `json:"coverage"`
	From             string         `json:"from"`
	To               string         `json:"to"`
	Items            []HistoryEntry `json:"items"`
	NextCursor       *string        `json:"nextCursor"`
}

// TierFor is an injected authoritative server entitlement reader. The default
// is Standard, including existing YouTube members; client plan/claims are not
// accepted as a Premium grant. Revision must change when entitlement changes.
type HistoryTier struct {
	Premium  bool
	Revision string
}

type HistoryService struct {
	Client      *firestore.Client
	Access      serviceaccess.Reader
	Environment string
	Cipher      *historyCipher
	TierFor     func(context.Context, string, WebAccount) (HistoryTier, error)
	Now         func() time.Time
}

func (s *HistoryService) authorization(ctx context.Context, uid string, account WebAccount, p historyPolicy) (HistoryTier, string, error) {
	if _, err := readAccess(ctx, s.Access, uid, account.AccessCheckpoint); err != nil {
		return HistoryTier{}, "", err
	}
	tier := HistoryTier{Revision: "standard-v1"}
	if s.TierFor != nil {
		var err error
		tier, err = s.TierFor(ctx, uid, account)
		if err != nil {
			return tier, "", fmt.Errorf("read authoritative history tier: %w", err)
		}
	}
	if tier.Revision == "" || len(tier.Revision) > 128 {
		return tier, "", apiError("TEMPORARY_UNAVAILABLE")
	}
	binding := digest(p.Revision + "\x00" + account.AccessCheckpoint + "\x00" + account.PrivacyPolicyVersion + "\x00" + account.TermsVersion + "\x00" + fmt.Sprint(tier.Premium) + "\x00" + tier.Revision)
	return tier, binding, nil
}

func (s *HistoryService) loadPolicy(ctx context.Context) (historyPolicy, error) {
	doc, err := s.Client.Collection(historyPolicyCollection).Doc(historyPolicyID).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return historyPolicy{}, apiError("FEATURE_UNAVAILABLE")
	}
	if err != nil {
		return historyPolicy{}, fmt.Errorf("read history policy: %w", err)
	}
	return decodeHistoryPolicy(doc)
}

func (s *HistoryService) preflight(ctx context.Context, uid string, q HistoryQuery, asOf time.Time, maxEntries int64) (time.Time, bool, error) {
	consistent := true
	var captured time.Time
	opts := []firestore.TransactionOption{firestore.ReadOnly}
	if !asOf.IsZero() {
		opts = append(opts, firestore.TransactionReadTime(asOf))
	}
	err := s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		docs, err := tx.GetAll([]*firestore.DocumentRef{s.Client.Collection(repository.USERS).Doc(uid)})
		if err != nil || len(docs) != 1 || !docs[0].Exists() {
			return apiError("HISTORY_NOT_READY")
		}
		captured = docs[0].ReadTime.UTC()
		var user repository.UserDoc
		if docs[0].DataTo(&user) != nil || !repository.ValidWorkHistoryWatermark(user, captured) {
			return apiError("HISTORY_NOT_READY")
		}
		if q.To.After(StatisticsWindows(captured).Today.AddDate(0, 0, 1)) {
			return apiError("INVALID_REQUEST")
		}
		keys := []string{}
		refs := []*firestore.DocumentRef{}
		first := time.Date(q.From.Year(), q.From.Month(), 1, 0, 0, 0, 0, q.From.Location())
		for at := first; at.Before(q.To); at = at.AddDate(0, 1, 0) {
			key := at.Format("2006-01")
			keys = append(keys, key)
			refs = append(refs, s.Client.Collection(repository.WorkMonths).Doc(repository.WorkMonthID(uid, key)))
		}
		if len(refs) > 3 {
			return apiError("INVALID_REQUEST")
		}
		months, err := tx.GetAll(refs)
		if err != nil || len(months) != len(refs) {
			return apiError("TEMPORARY_UNAVAILABLE")
		}
		values := map[string]repository.WorkMonthDoc{}
		for i, doc := range months {
			if !doc.Exists() {
				continue
			}
			value, err := repository.DecodeWorkMonth(doc, uid, keys[i])
			if err != nil {
				return apiError("TEMPORARY_UNAVAILABLE")
			}
			values[keys[i]] = value
		}
		var count int64
		for day := q.From; day.Before(q.To); day = day.AddDate(0, 0, 1) {
			month, exists := values[day.Format("2006-01")]
			if !exists && !day.Before(user.WorkHistoryActivatedAt) && user.WorkHistoryClosedThrough.Before(day) {
				continue
			}
			if !exists || !month.BaselineComplete || month.WriterEpoch != repository.MonthlyWorkEpoch {
				return apiError("HISTORY_NOT_READY")
			}
			value := month.Days[day.Format("2006-01-02")]
			if value.Invalid {
				consistent = false
			}
			if value.SourceEnd.After(captured) {
				return apiError("HISTORY_NOT_READY")
			}
			if value.CompletedWorkSegments > maxEntries-count {
				return apiError("HISTORY_LIMIT_EXCEEDED")
			}
			count += value.CompletedWorkSegments
		}
		return nil
	}, opts...)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("preflight bounded history: %w", err)
	}
	return captured, consistent, nil
}

func (s *HistoryService) Page(ctx context.Context, uid string, account WebAccount, q HistoryQuery, key string) (result HistoryPage, resultErr error) {
	if s == nil || s.Client == nil || s.Cipher == nil || s.Environment != "development" || s.Now == nil {
		return result, apiError("FEATURE_UNAVAILABLE")
	}
	if q.PageSize < 1 || q.PageSize > 100 || q.From.IsZero() || !q.To.After(q.From) || q.To.Sub(q.From) > 31*24*time.Hour {
		return result, apiError("INVALID_REQUEST")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	p, err := s.loadPolicy(ctx)
	if err != nil {
		return result, err
	}
	tier, authorization, err := s.authorization(ctx, uid, account, p)
	if err != nil {
		return result, err
	}
	now := p.ReadTime
	var cursor historyCursor
	if q.Cursor != "" {
		cursor, err = s.Cipher.open(q.Cursor, uid, authorization, q, now)
		if err != nil {
			return result, err
		}
	}
	limit := p.StandardDailyUnits
	if tier.Premium {
		limit = p.PremiumDailyUnits
	}
	// A fixed conservative reservation bounds document/index work and quota
	// management itself. Failure/cancellation is not a free retry. Overview,
	// logout and Privacy never consume these units.
	units := int64(q.PageSize+1) + (p.MaxIndexEntries+999)/1000 + 32
	fingerprint := digest(fmt.Sprintf("%s\x00%s\x00%d\x00%s\x00%s", q.From.Format("2006-01-02"), q.To.Format("2006-01-02"), q.PageSize, q.Cursor, authorization))
	claim, err := s.reserve(ctx, uid, account.AccessCheckpoint, key, fingerprint, p, limit, units)
	if err != nil {
		return result, err
	}
	defer func() {
		if err := s.release(claim); err != nil && resultErr == nil {
			result = HistoryPage{}
			resultErr = apiError("TEMPORARY_UNAVAILABLE")
		}
	}()
	asOf, consistent, err := s.preflight(ctx, uid, q, cursor.AsOf, p.MaxIndexEntries)
	if err != nil {
		return result, err
	}
	query := s.Client.Collection(repository.WorkSegments).Where(repository.UserIDDocProperty, "==", uid).Where("segment-type", "==", repository.WorkState).
		Where("ended-at", ">=", q.From).Where("ended-at", "<", q.To).OrderBy("ended-at", firestore.Asc).OrderBy(firestore.DocumentID, firestore.Asc).Limit(q.PageSize + 1)
	if q.Cursor != "" {
		query = query.StartAfter(cursor.AfterEnd, s.Client.Collection(repository.WorkSegments).Doc(cursor.AfterID))
	}
	iter := query.WithReadOptions(firestore.ReadTime(asOf)).Documents(ctx)
	defer iter.Stop()
	result = HistoryPage{AsOf: asOf, Timezone: "Asia/Tokyo", Selection: "completed_work", Coverage: "recorded_only", From: q.From.Format("2006-01-02"), To: q.To.Format("2006-01-02"), Items: []HistoryEntry{}, SourceConsistent: consistent}
	var lastID string
	var lastEnd time.Time
	encodedBytes := 0
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return HistoryPage{}, fmt.Errorf("read bounded history page: %w", err)
		}
		if len(result.Items) == q.PageSize {
			expires := asOf.Add(historyCursorTTL)
			value := historyCursor{Owner: s.Cipher.owner(uid), Authorization: authorization, From: result.From, To: result.To, PageSize: q.PageSize, AsOf: asOf, ExpiresAt: expires, AfterEnd: lastEnd, AfterID: lastID}
			token, err := s.Cipher.seal(value)
			if err != nil {
				return HistoryPage{}, err
			}
			result.NextCursor = &token
			break
		}
		segment, err := repository.DecodeWorkSegment(doc, uid)
		if err != nil || segment.SegmentType != repository.WorkState || segment.StartedAt.IsZero() || !segment.EndedAt.After(segment.StartedAt) || segment.EndedAt.After(asOf) || segment.DurationSec < 0 || int64(segment.DurationSec) != int64(segment.EndedAt.Sub(segment.StartedAt)/time.Second) {
			return HistoryPage{}, apiError("TEMPORARY_UNAVAILABLE")
		}
		entry := HistoryEntry{Key: digest(s.Cipher.owner(uid) + "\x00" + doc.Ref.ID), StartedAt: segment.StartedAt.UTC(), EndedAt: segment.EndedAt.UTC(), WorkSec: int64(segment.DurationSec), WorkName: segment.WorkName}
		body, err := json.Marshal(entry)
		if err != nil {
			return HistoryPage{}, fmt.Errorf("encode history entry: %w", err)
		}
		encodedBytes += len(body)
		if encodedBytes > 120*1024 {
			return HistoryPage{}, apiError("PAYLOAD_TOO_LARGE")
		}
		result.Items = append(result.Items, entry)
		lastID = doc.Ref.ID
		lastEnd = segment.EndedAt
	}
	current, err := s.loadPolicy(ctx)
	if err != nil {
		return HistoryPage{}, err
	}
	_, binding, err := s.authorization(ctx, uid, account, current)
	if err != nil {
		return HistoryPage{}, err
	}
	if binding != authorization {
		return HistoryPage{}, apiError("SERVICE_ACCESS_RESTRICTED")
	}
	return result, nil
}
