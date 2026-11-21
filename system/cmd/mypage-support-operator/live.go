package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// Owner-controlled GCP IAM is the operator boundary; channel ownership remains a separate fresh OAuth proof.
func liveIntakeFactory(ctx context.Context, p proposal) (*intakeRuntime, error) {
	expected := map[string]string{"development": "test-youtube-study-space", "production": "youtube-study-space"}
	if expected[p.Environment] != p.Project || p.Handoff == "" || (p.Environment == "production" && !p.AllowProduction) {
		return nil, errIntake
	}
	credentials, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/datastore")
	if err != nil || (credentials.ProjectID != "" && credentials.ProjectID != p.Project) {
		return nil, errIntake
	}
	client, err := firestore.NewClient(ctx, p.Project, option.WithCredentials(credentials))
	if err != nil {
		return nil, errIntake
	}
	runtime, err := newFirestoreIntakeRuntime(p, client, func(_ context.Context, intent intakeIntent) (intakeGrant, error) {
		return intakeGrant{Actor: "owner-controlled-gcp-iam", Intent: intent}, nil
	}, func(_ context.Context, _ intakeIntent, receipt intakeReceipt) error {
		return writePrivateHandoff(p.Handoff, receipt)
	}, time.Now)
	if err != nil {
		client.Close() //nolint:errcheck // Shutdown diagnostics must not expose credentials.
		return nil, errIntake
	}
	runtime.close = func() {
		client.Close() //nolint:errcheck // Shutdown diagnostics must not expose credentials.
	}
	return runtime, nil
}

func writePrivateHandoff(path string, receipt intakeReceipt) error {
	// Each handoff is a new private file. Lost ACK uses inspect/recover; it never resets AcceptedAt.
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errIntake
	}
	encodeErr := json.NewEncoder(output).Encode(receipt)
	syncErr := output.Sync()
	closeErr := output.Close()
	if encodeErr != nil || syncErr != nil || closeErr != nil {
		return errIntake
	}
	return nil
}
