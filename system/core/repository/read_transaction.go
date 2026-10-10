package repository

import (
	"context"

	"cloud.google.com/go/firestore"
)

type readTransactionKey struct{}

// WithReadTransaction scopes legacy identity/history queries to the writer's
// transaction without changing their public interface or nontransactional use.
// Explicit document reads still receive their transaction argument as before.
func WithReadTransaction(ctx context.Context, tx *firestore.Transaction) context.Context {
	return context.WithValue(ctx, readTransactionKey{}, tx)
}

func queryDocuments(ctx context.Context, query firestore.Query) *firestore.DocumentIterator {
	if tx, ok := ctx.Value(readTransactionKey{}).(*firestore.Transaction); ok && tx != nil {
		return tx.Documents(query)
	}
	return query.Documents(ctx)
}
