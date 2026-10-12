package mypage

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/repository"
	"app.modules/core/supportdelete"
)

// FirestoreDeletionEffects implements the source-known Firestore allowlist.
// RestoreGuard must independently attest the paused/drained fleet, catalog
// completeness and exclusion of old queues/imports/restores for this exact
// project/generation/cutoff. Nil/unknown evidence performs no mutation.
type FirestoreDeletionEffects struct {
	Store        *FirestoreDeletionStore
	RestoreGuard supportdelete.Effects
	Clock        func() time.Time
}
type deletionLookup struct{ collection, field, value, except string }

func sourceDeletionLookups(op supportdelete.Operation) ([]deletionLookup, bool) {
	channel := op.Execution.Selector.Target.ChannelID
	switch op.Step.Scope {
	case "seat-state":
		return []deletionLookup{{repository.SEATS, repository.UserIDDocProperty, channel, ""}, {repository.MemberSeats, repository.UserIDDocProperty, channel, ""}}, true
	case "firestore-primary":
		lookups := []deletionLookup{{repository.USERS, "", channel, ""}}
		for _, c := range []string{repository.WorkSegments, repository.UserActivities, repository.OrderHistory, repository.SeatLimitsBlackList, repository.SeatLimitsWhiteList, repository.MemberSeatLimitsBlackList, repository.MemberSeatLimitsWhiteList} {
			lookups = append(lookups, deletionLookup{c, repository.UserIDDocProperty, channel, ""})
		}
		return append(lookups, deletionLookup{repository.LiveChatHistory, "author-channel-id", channel, ""}), true
	case "firestore-web":
		return []deletionLookup{{"web-accounts", "", channel, ""}}, true
	case "firestore-oauth":
		return []deletionLookup{{"oauth-transactions", "channel.channelId", channel, ""}}, true
	case "firestore-support-relations":
		// The current receipt/proof/claim remains until atomic finalization. An
		// unfinished other receipt cannot be silently completed by this case.
		return []deletionLookup{{"support-requests", "targetChannel", channel, op.Execution.Selector.RequestRef}, {"support-challenges", "requestRef", op.Execution.Selector.RequestRef, ""}, {supportDeleteExecutions, "selector.target.channelID", channel, op.Execution.Selector.ExecutionRef}, {supportDeleteClaims, "requestRef", op.Execution.Selector.RequestRef, op.Execution.Selector.ProofRef}}, true
	default:
		return nil, false
	}
}

func (a *FirestoreDeletionEffects) Apply(ctx context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
	if a == nil || a.Store == nil || a.Store.check(op.Execution.Selector) != nil || a.RestoreGuard == nil || a.Clock == nil || op.Execution.Validate() != nil || op.Execution.Cursor >= len(supportdelete.Steps()) || supportdelete.Steps()[op.Execution.Cursor] != op.Step || (op.Step.Action != "delete" && op.Step.Action != "inspect") {
		return supportdelete.Evidence{}, supportdelete.ErrUnavailable
	}
	lookups, known := sourceDeletionLookups(op)
	if !known {
		return supportdelete.Evidence{}, supportdelete.ErrUnavailable
	}
	v, err := a.RestoreGuard.Apply(ctx, op)
	if err != nil || v.Validate(op, a.Clock()) != nil {
		return supportdelete.Evidence{}, supportdelete.ErrEvidence
	}
	if op.Step.Scope == "firestore-oauth" || op.Step.Scope == "firestore-support-relations" {
		var receipt SupportRequest
		err = a.Store.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
			if _, _, err := a.Store.readFence(tx, op.Execution, true); err != nil {
				return err
			}
			doc, err := tx.Get(a.Store.Client.Collection("support-requests").Doc(op.Execution.Selector.RequestRef))
			if err != nil || doc.DataTo(&receipt) != nil || receipt.ProofRef != op.Execution.Selector.ProofRef || receipt.TargetChannel != op.Execution.Selector.Target.ChannelID || !validOpaque(receipt.OAuthTransactionID) {
				return supportdelete.ErrConflict
			}
			return nil
		})
		if err != nil {
			return supportdelete.Evidence{}, deletionStoreError(err)
		}
		if op.Step.Scope == "firestore-oauth" {
			lookups = append(lookups, deletionLookup{"oauth-transactions", "support.requestRef", op.Execution.Selector.RequestRef, receipt.OAuthTransactionID})
		} else {
			lookups = append(lookups, deletionLookup{"support-request-ids", "requestRef", op.Execution.Selector.RequestRef, digest(receipt.Environment + ":" + receipt.RequestID)})
		}
	}
	for _, lookup := range lookups {
		// Bounded first-page deletion avoids skipping documents after partial
		// commit/ack loss. Each page re-reads the same transactional owner fence.
		for page := 0; page < 10000; page++ {
			found := 0
			err = a.Store.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
				if _, _, err := a.Store.readFence(tx, op.Execution, true); err != nil {
					return err
				}
				var docs []*firestore.DocumentSnapshot
				if lookup.field == "" {
					doc, err := tx.Get(a.Store.Client.Collection(lookup.collection).Doc(lookup.value))
					if err == nil {
						docs = append(docs, doc)
					} else if status.Code(err) != codes.NotFound {
						return supportdelete.ErrUnavailable
					}
				} else {
					q := a.Store.Client.Collection(lookup.collection).Where(lookup.field, "==", lookup.value).Select().Limit(101)
					var err error
					docs, err = tx.Documents(q).GetAll()
					if err != nil {
						return supportdelete.ErrUnavailable
					}
				}
				for _, doc := range docs {
					if doc.Ref.ID == lookup.except {
						continue
					}
					found++
					if lookup.collection == "support-requests" || lookup.collection == supportDeleteExecutions || lookup.collection == supportDeleteClaims {
						return supportdelete.ErrEvidence
					}
					if op.Step.Action == "delete" && tx.Delete(doc.Ref) != nil {
						return supportdelete.ErrUnavailable
					}
				}
				return nil
			})
			if err != nil {
				return supportdelete.Evidence{}, deletionStoreError(err)
			}
			if found == 0 {
				break
			}
			if op.Step.Action == "inspect" || page == 9999 {
				return supportdelete.Evidence{}, supportdelete.ErrEvidence
			}
		}
	}
	v.ObservedAt = a.Clock()
	return v, nil
}
