package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp"

	"google.golang.org/api/option"
)

func main() {
	ctx := context.Background()

	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		slog.Error("failed to get Firestore client option", "err", err)
		os.Exit(1)
	}

	app, err := workspaceapp.NewWorkspaceApp(ctx, false, clientOption)
	if err != nil {
		slog.Error("failed to init WorkspaceApp", "err", err)
		os.Exit(1)
	}
	defer app.CloseFirestoreClient()

	job := os.Getenv("JOB")
	if job == "" {
		job = "all"
	}
	app.MessageToOwner(ctx, "daily-batch started. JOB="+job)

	var runErr error
	switch job {
	case "all":
		runErr = runAll(ctx, app, clientOption)
	case "reset":
		if err := doResetDailyTotal(ctx, app); err != nil {
			runErr = fmt.Errorf("reset: %w", err)
		}
	case "update-rp":
		if err := doUpdateRP(ctx, app); err != nil {
			runErr = fmt.Errorf("update-rp: %w", err)
		}
	case "transfer-bq":
		if err := doTransferBQ(ctx, app, clientOption); err != nil {
			runErr = fmt.Errorf("transfer-bq: %w", err)
		}
	default:
		runErr = fmt.Errorf("unknown job: %s", job)
	}

	if runErr != nil {
		app.MessageToOwnerWithError(ctx, "daily-batch failed", runErr)
		os.Exit(1)
	}

	app.MessageToOwner(ctx, "daily-batch finished. JOB="+job)
}

func runAll(ctx context.Context, app *workspaceapp.WorkspaceApp, clientOption option.ClientOption) error {
	if err := doResetDailyTotal(ctx, app); err != nil {
		return fmt.Errorf("reset: %w", err)
	}
	if err := doUpdateRP(ctx, app); err != nil {
		return fmt.Errorf("update-rp: %w", err)
	}
	if err := doTransferBQ(ctx, app, clientOption); err != nil {
		return fmt.Errorf("transfer-bq: %w", err)
	}
	return nil
}

func doResetDailyTotal(ctx context.Context, app *workspaceapp.WorkspaceApp) error {
	count, err := app.ResetDailyTotalStudyTime(ctx)
	if err != nil {
		return fmt.Errorf("ResetDailyTotalStudyTime: %w", err)
	}
	app.MessageToOwner(ctx, "reset-daily-total finished. reset_count="+strconv.Itoa(count))
	return nil
}

func doUpdateRP(ctx context.Context, app *workspaceapp.WorkspaceApp) error {
	userIds, err := app.GetUserIdsToProcessRP(ctx)
	if err != nil {
		return fmt.Errorf("GetUserIdsToProcessRP: %w", err)
	}
	jstNow := utils.JstNow()

	var success, failed int
	for _, uid := range userIds {
		if err := app.UpdateUserRP(ctx, uid, jstNow); err != nil {
			failed++
			app.MessageToOwnerWithError(ctx, "failed UpdateUserRP: "+uid, err)
			continue
		}
		success++
	}
	app.MessageToOwner(ctx, "update-rp finished. success="+strconv.Itoa(success)+", failed="+strconv.Itoa(failed)+", total="+strconv.Itoa(len(userIds)))
	if failed > 0 {
		return fmt.Errorf("UpdateUserRP failed for %d out of %d users", failed, len(userIds))
	}
	return nil
}

func doTransferBQ(ctx context.Context, app *workspaceapp.WorkspaceApp, clientOption option.ClientOption) error {
	if err := app.BackupCollectionHistoryFromGcsToBigquery(ctx, clientOption); err != nil {
		return fmt.Errorf("BackupCollectionHistoryFromGcsToBigquery: %w", err)
	}
	app.MessageToOwner(ctx, "transfer-bq finished.")
	return nil
}
