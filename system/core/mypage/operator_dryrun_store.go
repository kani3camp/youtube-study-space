package mypage

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/firestore"

	"app.modules/core/repository"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type dryRunLookup struct {
	collection, field, scope string
	document                 bool
}

var dryRunLookups = map[string]dryRunLookup{
	"web-accounts":                  {"web-accounts", "", "channel", true},
	"users":                         {repository.USERS, "", "channel", true},
	"seats":                         {repository.SEATS, repository.UserIDDocProperty, "channel", false},
	"member-seats":                  {repository.MemberSeats, repository.UserIDDocProperty, "channel", false},
	"work-segments":                 {repository.WorkSegments, repository.UserIDDocProperty, "channel", false},
	"user-activities":               {repository.UserActivities, repository.UserIDDocProperty, "channel", false},
	"order-history":                 {repository.OrderHistory, repository.UserIDDocProperty, "channel", false},
	"seat-limits-black-list":        {repository.SeatLimitsBlackList, repository.UserIDDocProperty, "channel", false},
	"seat-limits-white-list":        {repository.SeatLimitsWhiteList, repository.UserIDDocProperty, "channel", false},
	"member-seat-limits-black-list": {repository.MemberSeatLimitsBlackList, repository.UserIDDocProperty, "channel", false},
	"member-seat-limits-white-list": {repository.MemberSeatLimitsWhiteList, repository.UserIDDocProperty, "channel", false},
	"live-chat-history":             {repository.LiveChatHistory, "author-channel-id", "channel", false},
	"channel-oauth-transactions":    {"oauth-transactions", "channel.channelId", "channel", false},
	"channel-support-requests":      {"support-requests", "targetChannel", "channel", false},
	"receipt-oauth-transactions":    {"oauth-transactions", "support.requestRef", "receipt", false},
	"receipt-challenge-indexes":     {"support-challenges", "requestRef", "receipt", false},
	"receipt-request-indexes":       {"support-request-ids", "requestRef", "receipt", false},
}

// Separate ownership/retention evidence is needed for these stores. Missing
// adapters never imply absence of data or permission to perform cleanup.
var uninspectedDryRunStores = []string{"firebase-auth", "cache-and-inflight", "backups-and-exports", "bigquery", "platform-logs", "legacy-mypage-mappings", "other-support-oauth-bindings"}

type FirestoreDryRunReader struct{ Client *firestore.Client }

func (r *FirestoreDryRunReader) Inspect(ctx context.Context, s DryRunSelector) (DryRunSnapshot, error) {
	if err := validateDryRunSelector(s); err != nil || r.Client == nil {
		return DryRunSnapshot{}, ErrDryRunUnavailable
	}
	ref := r.Client.Collection("support-requests").Doc(s.RequestRef)
	if !strings.HasPrefix(ref.Path, "projects/"+s.ExpectedProject+"/databases/(default)/documents/") {
		return DryRunSnapshot{}, ErrDryRunUnavailable
	}
	var result DryRunSnapshot
	err := r.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(ref)
		if err != nil {
			return fmt.Errorf("read dryrun receipt: %w", err)
		}
		var receipt SupportRequest
		if err := doc.DataTo(&receipt); err != nil {
			return fmt.Errorf("decode dryrun receipt: %w", err)
		}
		if err := validateDryRunReceipt(receipt, s); err != nil {
			return err
		}
		proof, err := tx.Get(r.Client.Collection("oauth-transactions").Doc(receipt.OAuthTransactionID))
		missing := status.Code(err) == codes.NotFound
		var oauth OAuthTransaction
		if err != nil && !missing {
			return fmt.Errorf("read dryrun proof: %w", err)
		}
		if !missing {
			if err := proof.DataTo(&oauth); err != nil {
				return fmt.Errorf("decode dryrun proof: %w", err)
			}
			if err := validateDryRunProof(receipt, oauth, s); err != nil {
				return err
			}
		}
		index, err := tx.Get(r.Client.Collection("support-request-ids").Doc(digest(s.Environment + ":" + receipt.RequestID)))
		if err != nil {
			return fmt.Errorf("read dryrun receipt index: %w", err)
		}
		var binding supportChallengeIndex
		if err := index.DataTo(&binding); err != nil {
			return fmt.Errorf("decode dryrun receipt index: %w", err)
		}
		if binding.Environment != s.Environment || binding.RequestRef != s.RequestRef {
			return ErrDryRunUnavailable
		}
		result = DryRunSnapshot{Project: s.ExpectedProject, AsOf: doc.ReadTime, Receipt: receipt, OAuth: oauth, OAuthMissing: missing, Counts: make(map[string]InventoryCount, len(dryRunLookups))}
		for name, lookup := range dryRunLookups {
			value := receipt.TargetChannel
			if lookup.scope == "receipt" {
				value = s.RequestRef
			}
			count := InventoryCount{State: "unknown", Scope: lookup.scope}
			if lookup.document {
				_, err := tx.Get(r.Client.Collection(lookup.collection).Doc(value))
				if err == nil {
					n := 1
					count = InventoryCount{State: "known", Count: &n, Scope: lookup.scope}
				} else if status.Code(err) == codes.NotFound {
					n := 0
					count = InventoryCount{State: "known", Count: &n, Scope: lookup.scope}
				}
			} else {
				docs, err := tx.Documents(r.Client.Collection(lookup.collection).Where(lookup.field, "==", value).Select().Limit(1001)).GetAll()
				if err == nil {
					n := len(docs)
					state := "known"
					if n == 1001 {
						state = "at_least"
					}
					count = InventoryCount{State: state, Count: &n, Scope: lookup.scope}
				}
			}
			result.Counts[name] = count
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
		return nil
	}, firestore.ReadOnly)
	if err != nil {
		return DryRunSnapshot{}, fmt.Errorf("inspect read-only operator snapshot: %w", err)
	}
	return result, nil
}
