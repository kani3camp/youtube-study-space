package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/firestore/apiv1/firestorepb"

	"app.modules/core/repository"
	"app.modules/internal/operatorauth"
)

const firestoreCountAlias = "row_count"

type auditTarget struct {
	Environment string `json:"environment"`
	ProjectID   string `json:"project_id"`
	AuthMode    string `json:"auth_mode"`
	Collection  string `json:"collection"`
}

type auditOutput struct {
	GeneratedAt time.Time   `json:"generated_at"`
	Target      auditTarget `json:"target"`
	TotalRows   int64       `json:"total_rows"`
	Drained     bool        `json:"drained"`
}

func main() {
	if err := run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "raw-chat-drain-audit:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return usageError()
	}
	environment := strings.TrimSpace(args[1])
	expectedProjectID := strings.TrimSpace(args[2])
	target, err := operatorauth.ResolveTarget(environment, expectedProjectID)
	if err != nil {
		return err
	}

	credentials, err := operatorauth.NewGoogleCredentials(ctx, target)
	if err != nil {
		return fmt.Errorf("initialize Google credentials: %w", err)
	}

	repo, err := repository.NewFirestoreController(ctx, credentials.ClientOption)
	if err != nil {
		return fmt.Errorf("initialize Firestore: %w", err)
	}
	defer func() {
		if err := repo.FirestoreClient().Close(); err != nil {
			fmt.Fprintln(os.Stderr, "raw-chat-drain-audit: close Firestore:", err)
		}
	}()

	totalRows, err := countRawLiveChatRows(ctx, repo.FirestoreClient())
	if err != nil {
		return err
	}

	output := auditOutput{
		GeneratedAt: time.Now().UTC(),
		Target: auditTarget{
			Environment: target.Environment,
			ProjectID:   credentials.ProjectID,
			AuthMode:    credentials.AuthMode,
			Collection:  repository.LiveChatHistory,
		},
		TotalRows: totalRows,
		Drained:   totalRows == 0,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("encode audit output: %w", err)
	}
	return nil
}

func countRawLiveChatRows(ctx context.Context, client repository.DBClient) (int64, error) {
	query := client.Collection(repository.LiveChatHistory).Query
	result, err := query.NewAggregationQuery().WithCount(firestoreCountAlias).Get(ctx)
	if err != nil {
		return 0, fmt.Errorf("count Firestore raw live chat rows: %w", err)
	}
	rawCount, ok := result[firestoreCountAlias]
	if !ok {
		return 0, fmt.Errorf("count aggregation missing alias %q", firestoreCountAlias)
	}
	countValue, ok := rawCount.(*firestorepb.Value)
	if !ok {
		return 0, fmt.Errorf("count aggregation has unexpected type %T", rawCount)
	}
	return countValue.GetIntegerValue(), nil
}

func usageError() error {
	return errors.New("usage: raw-chat-drain-audit <development|production> <expected-project-id>")
}
