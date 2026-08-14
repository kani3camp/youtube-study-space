package workspaceapp

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
)

// ensureProcessedUserRegisteredTx makes first-use user initialization reusable
// inside a caller-owned Firestore transaction. Legacy message preparation wraps
// it in its own transaction; the durable Inbox worker can compose the same
// operation after BeginLiveChatMessageTransaction and before finalization.
func (app *WorkspaceApp) ensureProcessedUserRegisteredTx(
	ctx context.Context,
	tx *firestore.Transaction,
) error {
	isRegistered, err := app.IfUserRegistered(ctx, tx)
	if err != nil {
		return fmt.Errorf("in IfUserRegistered(): %w", err)
	}
	if isRegistered {
		return nil
	}
	if err := app.CreateUser(ctx, tx); err != nil {
		return fmt.Errorf("in CreateUser(): %w", err)
	}
	return nil
}
