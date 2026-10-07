package mypage

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
)

// ReadRuntimeTransaction supplies durable admission provenance before a
// callback knows its channel. Claim still performs the atomic state/hash check;
// this read does not replace that check or introduce an external SDK call.
func (s *FirestoreAuthStore) ReadRuntimeTransaction(ctx context.Context, id string) (OAuthTransaction, error) {
	ref, err := s.transactionRef(id)
	if err != nil {
		return OAuthTransaction{}, err
	}
	var result OAuthTransaction
	err = s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		var readErr error
		result, readErr = s.readTransaction(tx, ref)
		return readErr
	}, firestore.ReadOnly)
	if err != nil {
		return OAuthTransaction{}, fmt.Errorf("read runtime transaction provenance: %w", err)
	}
	return result, nil
}
