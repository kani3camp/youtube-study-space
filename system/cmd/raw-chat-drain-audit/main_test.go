package main

import (
	"context"
	"testing"
)

func TestRunRejectsUnknownEnvironmentBeforeCredentialAccess(t *testing.T) {
	t.Setenv("CREDENTIAL_FILE_LOCATION", "")

	err := run(context.Background(), []string{"raw-chat-drain-audit", "staging", "project-id"})
	if err == nil {
		t.Fatal("run() error = nil, want environment validation error")
	}
}

func TestRunRequiresExplicitTarget(t *testing.T) {
	err := run(context.Background(), []string{"raw-chat-drain-audit"})
	if err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
}
