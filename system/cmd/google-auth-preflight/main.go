package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"app.modules/core/repository"
	"app.modules/internal/operatorauth"
)

type preflightOutput struct {
	Environment   string `json:"environment"`
	ProjectID     string `json:"project_id"`
	AWSProfile    string `json:"aws_profile"`
	FirestoreRead bool   `json:"firestore_read"`
}

func main() {
	if err := run(context.Background(), os.Args, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "google-auth-preflight:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) != 3 {
		return usageError()
	}
	environment := strings.TrimSpace(args[1])
	expectedProjectID := strings.TrimSpace(args[2])

	authConfig, err := operatorauth.GoogleConfigFromEnv(environment, expectedProjectID)
	if err != nil {
		return err
	}
	clientOption, err := authConfig.ClientOption(ctx)
	if err != nil {
		return err
	}

	repo, err := repository.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("initialize Firestore: %w", err)
	}
	defer func() {
		if err := repo.FirestoreClient().Close(); err != nil {
			fmt.Fprintln(os.Stderr, "google-auth-preflight: close Firestore:", err)
		}
	}()

	if _, err := repo.ReadSystemConstantsConfig(ctx, nil); err != nil {
		return fmt.Errorf("read system constants: %w", err)
	}

	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(preflightOutput{
		Environment:   authConfig.Environment,
		ProjectID:     authConfig.ProjectID,
		AWSProfile:    authConfig.AWSProfile,
		FirestoreRead: true,
	}); err != nil {
		return fmt.Errorf("encode preflight output: %w", err)
	}
	return nil
}

func usageError() error {
	return errors.New("usage: google-auth-preflight <development|production> <expected-project-id>")
}
