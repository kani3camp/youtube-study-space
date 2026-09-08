package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"app.modules/core/repository"
	"app.modules/core/workspaceapp"

	"google.golang.org/api/option"
)

type youtubeBotPreflightOutput struct {
	Environment       string `json:"environment"`
	ProjectID         string `json:"project_id"`
	CredentialsRead   bool   `json:"credentials_read"`
	SystemConstants   bool   `json:"system_constants_read"`
	MenuDocuments     int    `json:"menu_documents"`
	NGWordConfigCount int    `json:"ng_word_config_count"`
}

func Preflight(ctx context.Context, clientOption option.ClientOption, stdout io.Writer) error {
	environment := strings.TrimSpace(os.Getenv(youtubeBotEnvironmentEnv))
	projectID := strings.TrimSpace(os.Getenv(googleCloudProjectEnvName))
	if err := validateYoutubeBotTarget(environment, projectID); err != nil {
		return fmt.Errorf("validate youtube-bot target: %w", err)
	}

	repo, err := repository.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("initialize Firestore: %w", err)
	}
	defer func() {
		if err := repo.FirestoreClient().Close(); err != nil {
			fmt.Fprintln(os.Stderr, "youtube-bot preflight: close Firestore:", err)
		}
	}()

	if _, err := repo.ReadCredentialsConfig(ctx, nil); err != nil {
		return fmt.Errorf("read credentials config: %w", err)
	}
	constants, err := repo.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return fmt.Errorf("read system constants: %w", err)
	}
	uninitializedFields := workspaceapp.UninitializedConstantsFields(constants)
	if len(uninitializedFields) > 0 {
		return fmt.Errorf(
			"system constants contain zero values: %s",
			strings.Join(uninitializedFields, ", "),
		)
	}
	menuDocs, err := repo.ReadAllMenuDocsOrderByCode(ctx)
	if err != nil {
		return fmt.Errorf("read menu docs: %w", err)
	}
	ngWordConfig, err := loadNGWordConfig(ctx, clientOption, constants.BotConfigSpreadsheetID)
	if err != nil {
		return fmt.Errorf("read NG word config: %w", err)
	}

	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(youtubeBotPreflightOutput{
		Environment:       environment,
		ProjectID:         projectID,
		CredentialsRead:   true,
		SystemConstants:   true,
		MenuDocuments:     len(menuDocs),
		NGWordConfigCount: ngWordConfig.Count(),
	}); err != nil {
		return fmt.Errorf("encode youtube-bot preflight output: %w", err)
	}
	return nil
}
