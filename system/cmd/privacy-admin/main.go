package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"app.modules/core/mybigquery"
	"app.modules/core/repository"
	"app.modules/core/utils"
	"app.modules/internal/privacyops"
	"google.golang.org/api/option"
)

type inspectOutput struct {
	YouTubeChannelID string                       `json:"youtube_channel_id"`
	Firestore        privacyops.FirestoreInventory `json:"firestore"`
	BigQuery         mybigquery.UserDataInventory `json:"bigquery"`
	Notes            []string                     `json:"notes"`
}

func main() {
	if err := run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "privacy-admin:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) != 3 || args[1] != "inspect" {
		return errors.New("usage: privacy-admin inspect <youtube-channel-id>")
	}
	youtubeChannelID := strings.TrimSpace(args[2])
	if youtubeChannelID == "" {
		return errors.New("youtube channel id is empty")
	}

	utils.LoadEnv(".env")
	credentialFilePath := strings.TrimSpace(os.Getenv("CREDENTIAL_FILE_LOCATION"))
	if credentialFilePath == "" {
		return errors.New("CREDENTIAL_FILE_LOCATION is required")
	}
	clientOption := option.WithCredentialsFile(credentialFilePath)

	repo, err := repository.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("initialize Firestore: %w", err)
	}
	defer func() {
		_ = repo.FirestoreClient().Close()
	}()

	firestoreInventory, err := privacyops.InspectFirestore(ctx, repo.FirestoreClient(), youtubeChannelID)
	if err != nil {
		return fmt.Errorf("inspect Firestore: %w", err)
	}

	projectID, err := utils.GetGcpProjectID(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("resolve GCP project ID: %w", err)
	}
	bqClient, err := mybigquery.NewBigqueryClient(ctx, projectID, clientOption, "US")
	if err != nil {
		return fmt.Errorf("initialize BigQuery: %w", err)
	}
	defer bqClient.CloseClient()

	bigQueryInventory, err := bqClient.InspectUserData(ctx, youtubeChannelID)
	if err != nil {
		return fmt.Errorf("inspect BigQuery: %w", err)
	}

	output := inspectOutput{
		YouTubeChannelID: youtubeChannelID,
		Firestore:        firestoreInventory,
		BigQuery:         bigQueryInventory,
		Notes: []string{
			"read-only: this command does not delete data",
			"GCS Firestore export snapshots are not inspectable per user by this command",
			"Firebase Auth user existence is not checked; firebase_uid is reported when a MyPage mapping exists",
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("encode inventory: %w", err)
	}
	return nil
}
