package workspaceapp

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"

	"app.modules/core/repository"
)

func (app *WorkspaceApp) prepareClosedSegment(ctx context.Context, tx *firestore.Transaction, segment repository.WorkSegmentDoc) (context.Context, error) {
	// Persistence adapters prepare their own write plan. Pure command fakes do
	// not implement persistence; the real adapter rejects every unprepared write.
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
