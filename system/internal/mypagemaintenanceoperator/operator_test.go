package mypagemaintenanceoperator

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"app.modules/core/mypage"

	"cloud.google.com/go/firestore"
)

func TestVerifyAcceptsAnEstimateThatExpiredAfterCommit(t *testing.T) {
	base := []string{
		"--project=test-youtube-study-space", "--environment=development", "--state=ACTIVE",
		"--expected-revision=1", "--estimated-end-at=2026-10-27T02:01:00.1234Z", "--reason-category=PLANNED_MAINTENANCE",
		"--action-ref=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"--repository-id=340900071", "--repository-owner-id=54093651", "--actor-id=54093651", "--triggering-actor-id=54093651",
		"--run-id=123", "--run-attempt=1", "--git-ref=refs/heads/integration/mypage-canon-20261006",
		"--commit-sha=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"--workflow-ref=kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@refs/heads/integration/mypage-canon-20261006",
		"--expected-previous-state=NORMAL",
	}
	env := map[string]string{"GOOGLE_CLOUD_PROJECT": "test-youtube-study-space", "MYPAGE_ENVIRONMENT": "development", "CLOUD_RUN_EXECUTION": "mypage-maintenance-operator-abcde"}
	for _, operation := range []string{"preview", "verify"} {
		t.Run(operation, func(t *testing.T) {
			connected := false
			args := append([]string{"--operation=" + operation}, base...)
			err := Run(context.Background(), args, new(bytes.Buffer), func(key string) string { return env[key] }, func() time.Time { return time.Date(2026, 10, 27, 2, 2, 0, 0, time.UTC) }, func(context.Context, string) (*firestore.Client, error) {
				connected = true
				return nil, errors.New("stop after validation")
			}, mypage.MaintenanceAuthorityGitHub)
			if err == nil {
				t.Fatal("test connection stop unexpectedly succeeded")
			}
			if connected != (operation == "verify") {
				t.Fatalf("operation=%s connected=%v", operation, connected)
			}
		})
	}
}

func TestBreakGlassBinaryAuthorityRejectsGitHubSpoofBeforeFirestore(t *testing.T) {
	base := []string{
		"--operation=apply", "--project=test-youtube-study-space", "--environment=development",
		"--state=ACTIVE", "--expected-revision=1", "--reason-category=SECURITY_MAINTENANCE",
		"--action-ref=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"--git-ref=refs/heads/integration/mypage-canon-20261006",
		"--commit-sha=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"--break-glass-approval-ref=INCIDENT_123", "--claimed-operator=operator@example.com",
		"--expected-previous-state=NORMAL",
	}
	env := map[string]string{"GOOGLE_CLOUD_PROJECT": "test-youtube-study-space", "MYPAGE_ENVIRONMENT": "development", "CLOUD_RUN_EXECUTION": "mypage-maintenance-breakglass-abcde"}
	for name, extra := range map[string][]string{
		"authority flag":    {"--authority=GITHUB_ACTIONS"},
		"github provenance": {"--repository-id=340900071", "--repository-owner-id=54093651", "--actor-id=54093651", "--triggering-actor-id=54093651", "--run-id=123", "--run-attempt=1", "--workflow-ref=kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@refs/heads/integration/mypage-canon-20261006"},
	} {
		t.Run(name, func(t *testing.T) {
			connected := false
			err := Run(context.Background(), append(append([]string{}, base...), extra...), new(bytes.Buffer), func(key string) string { return env[key] }, func() time.Time { return time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC) }, func(context.Context, string) (*firestore.Client, error) {
				connected = true
				return nil, nil
			}, mypage.MaintenanceAuthorityBreakGlass)
			if err == nil || connected {
				t.Fatalf("err=%v connected=%v", err, connected)
			}
		})
	}
}
