package main

import (
	"context"
	"strings"
	"testing"
)

func TestRunRejectsUnknownEnvironmentBeforeCredentialAccess(t *testing.T) {
	err := run(context.Background(), []string{"raw-chat-drain-audit", "staging", "project-id"})
	if err == nil {
		t.Fatal("run() error = nil, want environment validation error")
	}
}

func TestRunRejectsEnvironmentProjectMismatchBeforeCredentialAccess(t *testing.T) {
	err := run(context.Background(), []string{"raw-chat-drain-audit", "development", "youtube-study-space"})
	if err == nil || !strings.Contains(err.Error(), "environment/project mismatch") {
		t.Fatalf("run() error = %v, want environment/project mismatch", err)
	}
}

func TestRunRequiresExplicitTarget(t *testing.T) {
	err := run(context.Background(), []string{"raw-chat-drain-audit"})
	if err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
}
