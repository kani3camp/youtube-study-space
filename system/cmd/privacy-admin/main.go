package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/api/option"

	"app.modules/core/mybigquery"
	"app.modules/core/repository"
	"app.modules/core/utils"
	"app.modules/internal/privacyops"
)

type inspectOutput struct {
	YouTubeChannelID string                        `json:"youtube_channel_id"`
	Firestore        privacyops.FirestoreInventory `json:"firestore"`
	BigQuery         mybigquery.UserDataInventory  `json:"bigquery"`
	Notes            []string                      `json:"notes"`
}

type deletePrimaryOutput struct {
	YouTubeChannelID string                           `json:"youtube_channel_id"`
	Before           inspectOutput                    `json:"before"`
	FirestoreDeleted privacyops.FirestoreDeleteResult `json:"firestore_deleted"`
	After            inspectOutput                    `json:"after"`
	Notes            []string                         `json:"notes"`
}

type clients struct {
	repository *repository.FirestoreControllerImplements
	bigquery   *mybigquery.BigqueryController
}

func main() {
	if err := run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "privacy-admin:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return usageError()
	}

	switch args[1] {
	case "inspect":
		if len(args) != 3 {
			return usageError()
		}
		return runInspect(ctx, strings.TrimSpace(args[2]))
	case "delete-primary":
		if len(args) != 5 || args[3] != "--confirm" {
			return usageError()
		}
		return runDeletePrimary(
			ctx,
			strings.TrimSpace(args[2]),
			strings.TrimSpace(args[4]),
		)
	default:
		return usageError()
	}
}

func runInspect(ctx context.Context, youtubeChannelID string) error {
	if youtubeChannelID == "" {
		return errors.New("youtube channel id is empty")
	}

	appClients, err := newClients(ctx)
	if err != nil {
		return err
	}
	defer appClients.close()

	inventory, err := inspect(ctx, appClients, youtubeChannelID)
	if err != nil {
		return err
	}
	return encodeJSON(inventory)
}

func runDeletePrimary(
	ctx context.Context,
	youtubeChannelID string,
	confirmation string,
) error {
	if err := validateDeleteConfirmation(youtubeChannelID, confirmation); err != nil {
		return err
	}

	appClients, err := newClients(ctx)
	if err != nil {
		return err
	}
	defer appClients.close()

	before, err := inspect(ctx, appClients, youtubeChannelID)
	if err != nil {
		return fmt.Errorf("inspect before deletion: %w", err)
	}

	firestoreDeleted, err := privacyops.DeleteFirestoreUserData(
		ctx,
		appClients.repository.FirestoreClient(),
		youtubeChannelID,
	)
	if err != nil {
		return fmt.Errorf("delete primary Firestore data: %w", err)
	}

	if err := appClients.bigquery.DeleteUserData(ctx, youtubeChannelID); err != nil {
		return fmt.Errorf("delete primary BigQuery data: %w", err)
	}

	after, err := inspect(ctx, appClients, youtubeChannelID)
	if err != nil {
		return fmt.Errorf("inspect after deletion: %w", err)
	}

	output := deletePrimaryOutput{
		YouTubeChannelID: youtubeChannelID,
		Before:           before,
		FirestoreDeleted: firestoreDeleted,
		After:            after,
		Notes: []string{
			"DESTRUCTIVE: primary Firestore and BigQuery data were deleted",
			"GCS Firestore export snapshots are NOT deleted by this command",
			"Firebase Authentication user records are NOT deleted by this command",
			"Discord moderation/log copies are NOT deleted by this command",
			"using Study Space again after deletion may create new data",
		},
	}

	return encodeJSON(output)
}

func validateDeleteConfirmation(youtubeChannelID string, confirmation string) error {
	if youtubeChannelID == "" {
		return errors.New("youtube channel id is empty")
	}
	if confirmation == "" {
		return errors.New("deletion confirmation is empty")
	}
	if confirmation != youtubeChannelID {
		return errors.New("deletion confirmation must exactly match the YouTube channel id")
	}
	return nil
}

func newClients(ctx context.Context) (*clients, error) {
	utils.LoadEnv(".env")
	credentialFilePath := strings.TrimSpace(os.Getenv("CREDENTIAL_FILE_LOCATION"))
	if credentialFilePath == "" {
		return nil, errors.New("CREDENTIAL_FILE_LOCATION is required")
	}
	//nolint:staticcheck // Credential file path is operator-controlled and restricted to the service-account JSON used for this admin command.
	clientOption := option.WithCredentialsFile(credentialFilePath)

	repo, err := repository.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return nil, fmt.Errorf("initialize Firestore: %w", err)
	}

	constants, err := repo.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		_ = repo.FirestoreClient().Close()
		return nil, fmt.Errorf("read system constants: %w", err)
	}
	projectID, err := utils.GetGcpProjectID(ctx, clientOption)
	if err != nil {
		_ = repo.FirestoreClient().Close()
		return nil, fmt.Errorf("resolve GCP project ID: %w", err)
	}
	bqClient, err := mybigquery.NewBigqueryClient(
		ctx,
		projectID,
		clientOption,
		constants.GcpRegion,
	)
	if err != nil {
		_ = repo.FirestoreClient().Close()
		return nil, fmt.Errorf("initialize BigQuery: %w", err)
	}

	return &clients{repository: repo, bigquery: bqClient}, nil
}

func (c *clients) close() {
	if err := c.repository.FirestoreClient().Close(); err != nil {
		fmt.Fprintln(os.Stderr, "privacy-admin: close Firestore:", err)
	}
	c.bigquery.CloseClient()
}

func inspect(
	ctx context.Context,
	appClients *clients,
	youtubeChannelID string,
) (inspectOutput, error) {
	firestoreInventory, err := privacyops.InspectFirestore(
		ctx,
		appClients.repository.FirestoreClient(),
		youtubeChannelID,
	)
	if err != nil {
		return inspectOutput{}, fmt.Errorf("inspect Firestore: %w", err)
	}

	bigQueryInventory, err := appClients.bigquery.InspectUserData(ctx, youtubeChannelID)
	if err != nil {
		return inspectOutput{}, fmt.Errorf("inspect BigQuery: %w", err)
	}

	return inspectOutput{
		YouTubeChannelID: youtubeChannelID,
		Firestore:        firestoreInventory,
		BigQuery:         bigQueryInventory,
		Notes: []string{
			"read-only: this inventory does not delete data",
			"GCS Firestore export snapshots are not inspectable per user by this command",
			"Firebase Auth user existence is not checked; firebase_uid is reported when a MyPage mapping exists",
		},
	}, nil
}

func encodeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	return nil
}

func usageError() error {
	return errors.New(
		"usage: privacy-admin inspect <youtube-channel-id> OR privacy-admin delete-primary <youtube-channel-id> --confirm <same-youtube-channel-id>",
	)
}
