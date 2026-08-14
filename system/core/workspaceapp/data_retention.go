package workspaceapp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"app.modules/core/mybigquery"
	"app.modules/core/utils"
	"google.golang.org/api/option"
)

// rawYouTubeDataRetentionDays is a hard upper bound for raw YouTube API data.
// Study-space-owned activity data has a separate lifecycle and is not affected
// by this limit.
const rawYouTubeDataRetentionDays = 30

func rawYouTubeDataCutoff(now time.Time) time.Time {
	return now.AddDate(0, 0, -rawYouTubeDataRetentionDays)
}

// DeleteLiveChatHistoryBeforeDate removes only raw live-chat documents.
// It intentionally does not delete user-activities, order-history, work-segments,
// or other study-space-owned data.
func (app *WorkspaceApp) DeleteLiveChatHistoryBeforeDate(ctx context.Context, date time.Time) (int, error) {
	var deleted int

	for {
		iter := app.Repository.Get500LiveChatHistoryDocIDsBeforeDate(ctx, date)
		count, err := app.DeleteIteratorDocs(ctx, iter)
		deleted += count
		if err != nil {
			return 0, fmt.Errorf("delete live chat history batch: %w", err)
		}
		if count == 0 {
			break
		}
	}

	return deleted, nil
}

// EnforceRawYouTubeDataRetention keeps raw YouTube live-chat API data for no
// longer than 30 days in the stores controlled by this application.
//
// GCS Firestore export retention is intentionally not changed here because the
// export bucket can contain collections unrelated to live-chat-history. Its
// lifecycle must be audited/configured separately before deleting snapshots.
func (app *WorkspaceApp) EnforceRawYouTubeDataRetention(ctx context.Context, clientOption option.ClientOption) error {
	cutoff := rawYouTubeDataCutoff(app.currentTime())

	deletedFirestoreRows, err := app.DeleteLiveChatHistoryBeforeDate(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("enforce Firestore live chat retention: %w", err)
	}

	projectID, err := utils.GetGcpProjectID(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("get GCP project ID for BigQuery retention: %w", err)
	}
	bqClient, err := mybigquery.NewBigqueryClient(
		ctx,
		projectID,
		clientOption,
		app.Configs.Constants.GcpRegion,
	)
	if err != nil {
		return fmt.Errorf("create BigQuery client for retention: %w", err)
	}
	defer bqClient.CloseClient()

	if err := bqClient.DeleteLiveChatHistoryBefore(ctx, cutoff); err != nil {
		return fmt.Errorf("enforce BigQuery live chat retention: %w", err)
	}

	slog.InfoContext(
		ctx,
		"raw YouTube live chat retention enforced",
		"retention_days", rawYouTubeDataRetentionDays,
		"cutoff", cutoff,
		"firestore_deleted", deletedFirestoreRows,
	)

	return nil
}
