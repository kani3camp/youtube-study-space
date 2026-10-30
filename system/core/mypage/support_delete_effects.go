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

// FirestoreDeletionEffectsはソースで確認できるFirestore許可リストを実装する。
// RestoreGuardは同じプロジェクト・世代・cutoffについて、全実行系の停止と完了待ち、一覧の完全性、
// 旧キュー・import・restoreの排除を独立して証明する必要がある。nilや不明な証拠では変更しない。
type FirestoreDeletionEffects struct {
	Store        *FirestoreDeletionStore
	RestoreGuard supportdelete.Effects
	Clock        func() time.Time
}
type deletionLookup struct{ collection, field, value, except string }

const historyBudgets = "mypage-history-budgets" // 廃止済み明細機能の残存データも削除対象。

func sourceDeletionLookups(op supportdelete.Operation) ([]deletionLookup, bool) {
	channel := op.Execution.Selector.Target.ChannelID
	switch op.Step.Scope {
	case "seat-state":
		return []deletionLookup{{repository.SEATS, repository.UserIDDocProperty, channel, ""}, {repository.MemberSeats, repository.UserIDDocProperty, channel, ""}}, true
	case "firestore-primary":
		lookups := []deletionLookup{{repository.USERS, "", channel, ""}}
		for _, c := range []string{repository.WorkSegments, repository.WorkMonths, historyBudgets, repository.UserActivities, repository.OrderHistory, repository.SeatLimitsBlackList, repository.SeatLimitsWhiteList, repository.MemberSeatLimitsBlackList, repository.MemberSeatLimitsWhiteList} {
			lookups = append(lookups, deletionLookup{c, repository.UserIDDocProperty, channel, ""})
		}
		return append(lookups, deletionLookup{repository.LiveChatHistory, "author-channel-id", channel, ""}), true
	case "firestore-web":
		return []deletionLookup{{"web-accounts", "", channel, ""}}, true
	case "firestore-oauth":
		return []deletionLookup{{"oauth-transactions", "channel.channelId", channel, ""}}, true
	case "firestore-support-relations":
		// 現在の受付・証拠・確保情報はアトミックな完了処理まで保持する。
		// この依頼で、別の未完了受付を暗黙に完了させない。
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
			// Keep the current fresh proof until the atomic terminal scrub.
			for i := range lookups {
				lookups[i].except = receipt.OAuthTransactionID
			}
			lookups = append(lookups, deletionLookup{"oauth-transactions", "support.requestRef", op.Execution.Selector.RequestRef, receipt.OAuthTransactionID})
		} else {
			lookups = append(lookups, deletionLookup{"support-request-ids", "requestRef", op.Execution.Selector.RequestRef, digest(receipt.Environment + ":" + receipt.RequestID)})
		}
	}
	for _, lookup := range lookups {
		// 上限付き先頭ページを削除し、部分コミットやACK消失後の文書の読み飛ばしを避ける。
		// 各ページで、同じ所有者の実行保護状態をトランザクション内で読み直す。
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
