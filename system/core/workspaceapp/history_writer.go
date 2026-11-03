package workspaceapp

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"

	"app.modules/core/repository"
)

func (app *WorkspaceApp) prepareClosedSegment(ctx context.Context, tx *firestore.Transaction, segment repository.WorkSegmentDoc) (context.Context, error) {
	// 永続化アダプターは自分の書き込み計画を準備する。
	// 純粋なコマンド用の偽物には永続化を実装しない。実アダプターは未準備の書き込みをすべて拒否する。
	if preparer, ok := app.Repository.(interface {
		PrepareWorkSegment(context.Context, *firestore.Transaction, repository.WorkSegmentDoc) (context.Context, error)
	}); ok {
		prepared, err := preparer.PrepareWorkSegment(ctx, tx, segment)
		if err != nil {
			return nil, fmt.Errorf("prepare closed history: %w", err)
		}
		return prepared, nil
	}
	return ctx, nil
}
