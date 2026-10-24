package repository

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"

	"app.modules/core/serviceaccess"
)

// SetWorkMonthReader is an explicit operator cutover/rollback, never a normal
// API side effect. The expected user revision and original privacy checkpoint
// must come from the reviewed dry run. A cutover attests that all old writers
// have drained; local/mock execution cannot supply that operational evidence.
func (c *FirestoreControllerImplements) SetWorkMonthReader(ctx context.Context, uid, checkpoint string, expectedUserRevision time.Time, enabled, apply bool) error {
	if uid == "" || checkpoint == "" || expectedUserRevision.IsZero() {
		return ErrWorkHistoryConflict
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := c.firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, readErr := tx.Get(c.firestoreClient.Collection(serviceaccess.Collection).Doc(uid))
		control, err := serviceaccess.DecodeSnapshot(doc, readErr)
		if err != nil || control.Control.PrivacyDeletion.Active || control.Checkpoint() != checkpoint {
			return ErrWorkHistoryConflict
		}
		ref := c.usersCollection().Doc(uid)
		doc, err = tx.Get(ref)
		if err != nil {
			return fmt.Errorf("read cutover user: %w", err)
		}
		if !doc.UpdateTime.Equal(expectedUserRevision) {
			return ErrWorkHistoryConflict
		}
		if !enabled {
			if !apply {
				return nil
			}
			return tx.Update(ref, []firestore.Update{{Path: "work-history-epoch", Value: firestore.Delete}, {Path: "work-history-activated-at", Value: firestore.Delete}, {Path: "work-history-closed-through", Value: firestore.Delete}})
		}
		var user UserDoc
		if doc.DataTo(&user) != nil {
			return ErrWorkHistoryConflict
		}
		asOf := doc.ReadTime
		if asOf.IsZero() {
			return ErrWorkHistoryConflict
		}
		// Seven days can touch at most two months. Validate both, including
		// empty days, before activating the overview path for this user.
		lastKey := ""
		for i := 6; i >= 0; i-- {
			month := monthStart(asOf.AddDate(0, 0, -i)).Format("2006-01")
			if month == lastKey {
				continue
			}
			lastKey = month
			valueDoc, err := tx.Get(c.firestoreClient.Collection(WorkMonths).Doc(WorkMonthID(uid, month)))
			if err != nil {
				return ErrWorkHistoryConflict
			}
			value, err := DecodeWorkMonth(valueDoc, uid, month)
			if err != nil || !value.BaselineComplete || value.WriterEpoch != MonthlyWorkEpoch {
				return ErrWorkHistoryConflict
			}
			for _, day := range value.Days {
				if day.Invalid || day.Last.After(asOf) {
					return ErrWorkHistoryConflict
				}
			}
		}
		future, err := tx.Documents(c.workSegmentsCollection().Where(UserIDDocProperty, "==", uid).Where("ended-at", ">", asOf).OrderBy("ended-at", firestore.Asc).OrderBy("started-at", firestore.Asc).Limit(1)).GetAll()
		if err != nil {
			return fmt.Errorf("read bounded cutover watermark: %w", err)
		}
		if len(future) != 0 {
			return ErrWorkHistoryConflict
		}
		if !apply {
			return nil
		}
		return tx.Update(ref, []firestore.Update{{Path: "work-history-epoch", Value: MonthlyWorkEpoch}, {Path: "work-history-activated-at", Value: asOf}, {Path: "work-history-closed-through", Value: asOf}})
	}, firestore.MaxAttempts(1))
	if err != nil {
		return fmt.Errorf("cut over month reader: %w", err)
	}
	return nil
}
