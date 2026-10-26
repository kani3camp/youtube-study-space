package mypagemaintenanceoperator

import (
	"bytes"
	"context"
	"testing"
	"time"

	"app.modules/core/mypage"

	"cloud.google.com/go/firestore"
)

func TestBreakGlassBinaryAuthorityRejectsGitHubSpoofBeforeFirestore(t *testing.T) {
	base := []string{
		"--operation=apply", "--project=test-youtube-study-space", "--environment=development",
		"--state=ACTIVE", "--expected-revision=1", "--reason-category=SECURITY_MAINTENANCE",
		"--action-ref=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"--git-ref=refs/heads/integration/mypage-canon-20261006",
		"--commit-sha=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"--break-glass-approval-ref=INCIDENT_123", "--claimed-operator=operator@example.com",
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
