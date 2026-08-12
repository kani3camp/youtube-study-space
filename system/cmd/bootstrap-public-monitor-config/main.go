package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"app.modules/core/repository"

	"google.golang.org/api/option"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap public monitor config: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("bootstrap-public-monitor-config", flag.ContinueOnError)
	projectID := flags.String("project-id", "", "required Google Cloud project ID")
	credentialsFile := flags.String("credentials-file", "", "optional service account credential file; defaults to ADC")
	apply := flags.Bool("apply", false, "create public-config/monitor after an interactive project confirmation")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	if *projectID == "" {
		return errors.New("--project-id is required")
	}

	clientOptions := make([]option.ClientOption, 0, 1)
	if *credentialsFile != "" {
		//nolint:staticcheck // This operator-only migration command accepts an explicit credential file by design.
		clientOptions = append(clientOptions, option.WithCredentialsFile(*credentialsFile))
	}
	controller, err := repository.NewFirestoreControllerForProject(ctx, *projectID, clientOptions...)
	if err != nil {
		return fmt.Errorf("create Firestore client for project %q: %w", *projectID, err)
	}
	defer func() {
		if closeErr := controller.FirestoreClient().Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "close Firestore client: %v\n", closeErr)
		}
	}()

	monitorConfig, err := controller.PrepareMonitorPublicConfigBootstrap(ctx)
	if err != nil {
		return fmt.Errorf("prepare migration preview: %w", err)
	}
	preview, err := json.MarshalIndent(monitorConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("encode migration preview: %w", err)
	}
	fmt.Printf("Project: %s\nTarget: public-config/monitor\n%s\n", *projectID, preview)

	if !*apply {
		fmt.Println("Preview only. Re-run with --apply to create the document.")
		return nil
	}

	fmt.Printf("Type the project ID %q to confirm: ", *projectID)
	var confirmation string
	if _, err := fmt.Scanln(&confirmation); err != nil {
		return fmt.Errorf("read project confirmation: %w", err)
	}
	if confirmation != *projectID {
		return errors.New("project confirmation did not match; no data was written")
	}
	if err := controller.CreateMonitorPublicConfig(ctx, monitorConfig); err != nil {
		return fmt.Errorf("persist migration: %w", err)
	}
	fmt.Println("Created public-config/monitor. Existing documents are never overwritten by this command.")
	return nil
}
