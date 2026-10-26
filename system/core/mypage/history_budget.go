package mypage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/serviceaccess"
	"app.modules/core/timeutil"
)

const (
	historyBudgets          = "mypage-history-budgets"
	historyPolicyCollection = "mypage-maintenance"
	historyPolicyID         = "history-policy"
)

type historyPolicy struct {
	Enabled            bool      `firestore:"enabled"`
	StandardDailyUnits int64     `firestore:"standardDailyUnits"`
	PremiumDailyUnits  int64     `firestore:"premiumDailyUnits"`
	MaxConcurrent      int64     `firestore:"maxConcurrent"`
	MaxIndexEntries    int64     `firestore:"maxIndexEntries"`
	Revision           string    `firestore:"-"`
	ReadTime           time.Time `firestore:"-"`
}

func decodeHistoryPolicy(doc *firestore.DocumentSnapshot) (historyPolicy, error) {
	var p historyPolicy
	if doc == nil || !doc.Exists() {
		return p, apiError("FEATURE_UNAVAILABLE")
	}
	data := doc.Data()
	if len(data) != 5 {
		return p, apiError("TEMPORARY_UNAVAILABLE")
	}
	if _, ok := data["enabled"].(bool); !ok {
		return p, apiError("TEMPORARY_UNAVAILABLE")
	}
	for _, key := range []string{"standardDailyUnits", "premiumDailyUnits", "maxConcurrent", "maxIndexEntries"} {
		if _, ok := data[key].(int64); !ok {
			return p, apiError("TEMPORARY_UNAVAILABLE")
		}
	}
	if doc.DataTo(&p) != nil || p.StandardDailyUnits < 1 || p.StandardDailyUnits > 10000 || p.PremiumDailyUnits < 1 || p.PremiumDailyUnits > 10000 || p.MaxConcurrent < 1 || p.MaxConcurrent > 4 || p.MaxIndexEntries < 1 || p.MaxIndexEntries > 100000 {
		return p, apiError("TEMPORARY_UNAVAILABLE")
	}
	if !p.Enabled {
		return p, apiError("FEATURE_UNAVAILABLE")
	}
	p.Revision = digest(fmt.Sprintf("%t:%d:%d:%d:%d:%s", p.Enabled, p.StandardDailyUnits, p.PremiumDailyUnits, p.MaxConcurrent, p.MaxIndexEntries, doc.UpdateTime.UTC().Format(time.RFC3339Nano)))
	p.ReadTime = doc.ReadTime.UTC()
	if p.ReadTime.IsZero() {
		return p, apiError("TEMPORARY_UNAVAILABLE")
	}
	return p, nil
}

type (
	historyOperation   struct{ Fingerprint, Day string }
	historyBudgetState struct {
		UID        string                      `firestore:"user-id"`
		Day        string                      `firestore:"day"`
		Spent      int64                       `firestore:"spent"`
		Operations map[string]historyOperation `firestore:"operations"`
		Leases     map[string]time.Time        `firestore:"leases"`
	}
)

type HistoryWaitError struct {
	ResetAt time.Time
	Daily   bool
}

func (e *HistoryWaitError) Error() string { return "history budget unavailable" }

type historyClaim struct {
	UID, Key string
	Until    time.Time
}

func decodeHistoryBudget(doc *firestore.DocumentSnapshot, uid string) (historyBudgetState, error) {
	state := historyBudgetState{UID: uid, Operations: map[string]historyOperation{}, Leases: map[string]time.Time{}}
	if doc == nil || !doc.Exists() {
		return state, nil
	}
	data := doc.Data()
	if len(data) != 5 {
		return state, apiError("TEMPORARY_UNAVAILABLE")
	}
	if _, ok := data["user-id"].(string); !ok {
		return state, apiError("TEMPORARY_UNAVAILABLE")
	}
	if _, ok := data["day"].(string); !ok {
		return state, apiError("TEMPORARY_UNAVAILABLE")
	}
	if _, ok := data["spent"].(int64); !ok {
		return state, apiError("TEMPORARY_UNAVAILABLE")
	}
	ops, ok := data["operations"].(map[string]any)
	if !ok || len(ops) > 2048 {
		return state, apiError("TEMPORARY_UNAVAILABLE")
	}
	for key, raw := range ops {
		op, ok := raw.(map[string]any)
		if !ok || len(op) != 2 || len(key) != 64 {
			return state, apiError("TEMPORARY_UNAVAILABLE")
		}
		if _, ok := op["Fingerprint"].(string); !ok {
			return state, apiError("TEMPORARY_UNAVAILABLE")
		}
		if _, ok := op["Day"].(string); !ok {
			return state, apiError("TEMPORARY_UNAVAILABLE")
		}
	}
	leases, ok := data["leases"].(map[string]any)
	if !ok || len(leases) > 4 {
		return state, apiError("TEMPORARY_UNAVAILABLE")
	}
	for key, raw := range leases {
		if _, ok := raw.(time.Time); !ok || len(key) != 64 {
			return state, apiError("TEMPORARY_UNAVAILABLE")
		}
	}
	if doc.DataTo(&state) != nil || state.UID != uid || state.Spent < 0 || state.Spent > 10000 {
		return state, apiError("TEMPORARY_UNAVAILABLE")
	}
	if _, err := time.Parse("2006-01-02", state.Day); err != nil {
		return state, apiError("TEMPORARY_UNAVAILABLE")
	}
	for _, op := range state.Operations {
		if len(op.Fingerprint) != 64 {
			return state, apiError("TEMPORARY_UNAVAILABLE")
		}
		if _, err := time.Parse("2006-01-02", op.Day); err != nil {
			return state, apiError("TEMPORARY_UNAVAILABLE")
		}
	}
	return state, nil
}

func historyControlSnapshot(doc *firestore.DocumentSnapshot) (serviceaccess.Snapshot, error) {
	if !doc.Exists() {
		return serviceaccess.Snapshot{}, nil
	}
	control, err := serviceaccess.DecodeSnapshot(doc, nil)
	if err != nil {
		return control, apiError("TEMPORARY_UNAVAILABLE")
	}
	return control, nil
}

func historyAccessAllowed(control serviceaccess.Snapshot, checkpoint string) error {
	if control.Checkpoint() != checkpoint {
		return apiError("SERVICE_ACCESS_RESTRICTED")
	}
	if err := control.Allowed(); err != nil {
		if errors.Is(err, serviceaccess.ErrDeletion) {
			return apiError("DATA_DELETION_IN_PROGRESS")
		}
		return apiError("SERVICE_ACCESS_RESTRICTED")
	}
	return nil
}

func (s *HistoryService) reserve(ctx context.Context, uid, checkpoint, key, fingerprint string, p historyPolicy, limit, units int64) (historyClaim, error) {
	var claim historyClaim
	if len(key) < 16 || len(key) > 128 || strings.ContainsAny(key, "/\x00\r\n ,") || limit < 1 || limit > 10000 || units < 1 {
		return claim, apiError("INVALID_REQUEST")
	}
	opKey := digest(key)
	err := s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := transactionMaintenance(tx, s.Client, s.Environment, maintenanceRevisionFromContext(ctx)); err != nil {
			return err
		}
		clocks, err := tx.GetAll([]*firestore.DocumentRef{s.Client.Collection(serviceaccess.Collection).Doc(uid)})
		if err != nil || len(clocks) != 1 || clocks[0].ReadTime.IsZero() {
			return apiError("TEMPORARY_UNAVAILABLE")
		}
		control, err := historyControlSnapshot(clocks[0])
		if err != nil {
			return err
		}
		if err := historyAccessAllowed(control, checkpoint); err != nil {
			return err
		}
		at := clocks[0].ReadTime.In(timeutil.JapanLocation())
		day := at.Format("2006-01-02")
		tomorrow := time.Date(at.Year(), at.Month(), at.Day()+1, 0, 0, 0, 0, at.Location())
		policyDoc, err := tx.Get(s.Client.Collection(historyPolicyCollection).Doc(historyPolicyID))
		if err != nil {
			return apiError("FEATURE_UNAVAILABLE")
		}
		current, err := decodeHistoryPolicy(policyDoc)
		if err != nil {
			return err
		}
		if current.Revision != p.Revision {
			return apiError("INVALID_CURSOR")
		}
		ref := s.Client.Collection(historyBudgets).Doc(digest(s.Environment + "\x00" + uid))
		docs, err := tx.GetAll([]*firestore.DocumentRef{ref})
		if err != nil || len(docs) != 1 {
			return apiError("TEMPORARY_UNAVAILABLE")
		}
		state, err := decodeHistoryBudget(docs[0], uid)
		if err != nil {
			return err
		}
		if state.Day > day {
			return apiError("TEMPORARY_UNAVAILABLE")
		}
		if state.Day != day {
			state.Day = day
			state.Spent = 0
		}
		yesterday := at.AddDate(0, 0, -1).Format("2006-01-02")
		for old, op := range state.Operations {
			if op.Day < yesterday {
				delete(state.Operations, old)
			}
		}
		if op, exists := state.Operations[opKey]; exists {
			if op.Fingerprint != fingerprint {
				return apiError("INVALID_REQUEST")
			}
			return apiError("OPERATION_ALREADY_CONSUMED")
		}
		for old, until := range state.Leases {
			if !until.After(at) {
				delete(state.Leases, old)
			}
		}
		if len(state.Leases) >= int(p.MaxConcurrent) {
			reset := tomorrow
			for _, until := range state.Leases {
				if until.Before(reset) {
					reset = until
				}
			}
			return &HistoryWaitError{ResetAt: reset}
		}
		if state.Spent+units > limit || len(state.Operations) >= 2048 {
			return &HistoryWaitError{ResetAt: tomorrow, Daily: true}
		}
		state.Spent += units
		state.Operations[opKey] = historyOperation{Fingerprint: fingerprint, Day: day}
		claim = historyClaim{UID: uid, Key: opKey, Until: at.Add(15 * time.Second).UTC().Truncate(time.Microsecond)}
		state.Leases[opKey] = claim.Until
		if s.beforeBudgetWrite != nil {
			s.beforeBudgetWrite()
		}
		if err := tx.Set(ref, state); err != nil {
			return fmt.Errorf("reserve bounded history work: %w", err)
		}
		return nil
	}, firestore.MaxAttempts(3))
	if err != nil {
		return historyClaim{}, fmt.Errorf("reserve history budget: %w", err)
	}
	return claim, nil
}

func (s *HistoryService) release(claim historyClaim) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		ref := s.Client.Collection(historyBudgets).Doc(digest(s.Environment + "\x00" + claim.UID))
		doc, err := tx.Get(ref)
		if status.Code(err) == codes.NotFound {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read history lease release: %w", err)
		}
		state, err := decodeHistoryBudget(doc, claim.UID)
		if err != nil {
			return err
		}
		if until, ok := state.Leases[claim.Key]; !ok || !until.Equal(claim.Until) {
			return nil
		}
		delete(state.Leases, claim.Key)
		if err := tx.Update(ref, []firestore.Update{{Path: "leases", Value: state.Leases}}); err != nil {
			return fmt.Errorf("release history lease: %w", err)
		}
		return nil
	}, firestore.MaxAttempts(1))
	if err != nil {
		return fmt.Errorf("finish history lease: %w", err)
	}
	return nil
}
