package repository

import (
	"context"

	"cloud.google.com/go/firestore"
)

type readTransactionKey struct{}

// WithReadTransactionは、既存の本人情報・履歴クエリを書き込み元のトランザクションに参加させる。
// 公開インターフェースやトランザクション外の利用は変更しない。
// 明示的な文書読み取りは、従来どおりトランザクション引数を受け取る。
func WithReadTransaction(ctx context.Context, tx *firestore.Transaction) context.Context {
	return context.WithValue(ctx, readTransactionKey{}, tx)
}

func queryDocuments(ctx context.Context, query firestore.Query) *firestore.DocumentIterator {
	if tx, ok := ctx.Value(readTransactionKey{}).(*firestore.Transaction); ok && tx != nil {
		return tx.Documents(query)
	}
	return query.Documents(ctx)
}
