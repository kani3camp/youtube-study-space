//go:build integration

package mypage

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"app.modules/internal/integrationtest"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func maintenanceIntegrationClient(t *testing.T) *firestore.Client {
	t.Helper()
	integrationtest.RequireFirestoreEmulator(t)
	integrationtest.ResetFirestore(t)
	client, err := firestore.NewClient(context.Background(), "demo-youtube-study-space-ci", option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func seedNormalMaintenance(t *testing.T, client *firestore.Client, environment string, now time.Time) {
	t.Helper()
	ref := maintenanceRef(client)
	_, err := ref.Set(context.Background(), MaintenanceControl{
		Environment: environment, State: MaintenanceNormal, Revision: 1,
		UpdatedAt: now.UTC(), MessageTemplate: MaintenanceMessageTemplate,
		ReasonCategory: "INITIALIZATION",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, deleteErr := ref.Delete(context.Background())
		if deleteErr != nil && status.Code(deleteErr) != codes.NotFound {
			t.Error(deleteErr)
		}
	})
}

func maintenanceRequest(state string, revision int64, action, reason string) MaintenanceTransition {
	return MaintenanceTransition{Authority: MaintenanceAuthorityGitHub, Environment: "development", State: state, ExpectedRevision: revision, ReasonCategory: reason, ActionRef: action, RepositoryID: "340900071", RepositoryOwnerID: "54093651", ActorID: "54093651", TriggeringActorID: "54093651", RunID: "123", RunAttempt: 1, GitRef: "refs/heads/integration/mypage-canon-20261006", CommitSHA: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", WorkflowRef: "kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@refs/heads/integration/mypage-canon-20261006", CloudRunExecution: "mypage-maintenance-operator-abcde"}
}

func TestMaintenancePreviewDoesNotMutateAndVerifyChecksReceipt(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	request := maintenanceRequest(MaintenanceNormal, 0, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "INITIALIZATION")
	preview, err := PreviewMaintenanceTransition(ctx, client, request, now)
	if err != nil || preview.Current.Revision != 1 || preview.Current.State != MaintenanceNormal {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	if _, err := maintenanceRef(client).Get(ctx); status.Code(err) != codes.NotFound {
		t.Fatalf("preview wrote control: %v", err)
	}
	if _, err := client.Collection(maintenanceEvents).Doc(request.ActionRef).Get(ctx); status.Code(err) != codes.NotFound {
		t.Fatalf("preview wrote audit: %v", err)
	}
	if _, err := ApplyMaintenanceTransition(ctx, client, request, now); err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyMaintenanceTransition(ctx, client, request, now.Add(time.Second))
	if err != nil || verified.Current.Revision != 1 {
		t.Fatalf("verified=%+v err=%v", verified, err)
	}
	request.ActionRef = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if _, err := VerifyMaintenanceTransition(ctx, client, request, now.Add(time.Second)); err == nil {
		t.Fatal("verify accepted a missing audit receipt")
	}
}

func TestMaintenanceBootstrapRequiresInitializedNormal(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	if RequireMaintenanceBootstrapNormal(ctx, client, "development") == nil {
		t.Fatal("missing bootstrap control accepted")
	}
	seedNormalMaintenance(t, client, "development", now)
	if err := RequireMaintenanceBootstrapNormal(ctx, client, "development"); err != nil {
		t.Fatal(err)
	}
	_, err := maintenanceRef(client).Set(ctx, MaintenanceControl{Environment: "development", State: MaintenanceActive, Revision: 2, UpdatedAt: now.Add(time.Second), MessageTemplate: MaintenanceMessageTemplate, ReasonCategory: "PLANNED_MAINTENANCE"})
	if err != nil {
		t.Fatal(err)
	}
	if RequireMaintenanceBootstrapNormal(ctx, client, "development") == nil {
		t.Fatal("ACTIVE bootstrap control accepted")
	}
}

func TestMaintenanceBreakGlassAuditDoesNotInventGitHubProvenance(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := ApplyMaintenanceTransition(ctx, client, maintenanceRequest(MaintenanceNormal, 0, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "INITIALIZATION"), now); err != nil {
		t.Fatal(err)
	}
	request := MaintenanceTransition{Authority: MaintenanceAuthorityBreakGlass, Environment: "development", State: MaintenanceActive, ExpectedRevision: 1, ReasonCategory: "SECURITY_MAINTENANCE", ActionRef: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", GitRef: "refs/heads/integration/mypage-canon-20261006", CommitSHA: "cccccccccccccccccccccccccccccccccccccccc", CloudRunExecution: "mypage-maintenance-operator-breakglass1", BreakGlassApprovalRef: "INCIDENT_123", ClaimedOperator: "operator@example.com"}
	if _, err := ApplyMaintenanceTransition(ctx, client, request, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyMaintenanceTransition(ctx, client, request, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	doc, err := client.Collection(maintenanceEvents).Doc(request.ActionRef).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var event maintenanceAuditEvent
	if doc.DataTo(&event) != nil || event.Authority != MaintenanceAuthorityBreakGlass || event.BreakGlassApprovalRef != "INCIDENT_123" || event.ClaimedOperator != "operator@example.com" || event.RepositoryID != "" || event.RunID != "" || event.WorkflowRef != "" {
		t.Fatalf("invalid break-glass audit: %+v", event)
	}
}

func TestMaintenanceTransitionCASAndAuditAreAtomic(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	initial, err := ApplyMaintenanceTransition(ctx, client, maintenanceRequest(MaintenanceNormal, 0, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "INITIALIZATION"), now)
	if err != nil || initial.Current.Revision != 1 || initial.Previous != nil {
		t.Fatalf("initial=%+v err=%v", initial, err)
	}
	var successes atomic.Int32
	var wait sync.WaitGroup
	for i, action := range []string{"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"} {
		wait.Add(1)
		go func(i int, action string) {
			defer wait.Done()
			request := maintenanceRequest(MaintenanceActive, 1, action, "PLANNED_MAINTENANCE")
			end := now.Add(time.Duration(i+1) * time.Hour)
			request.EstimatedEndAt = &end
			if _, err := ApplyMaintenanceTransition(ctx, client, request, now.Add(time.Minute)); err == nil {
				successes.Add(1)
			}
		}(i, action)
	}
	wait.Wait()
	if successes.Load() != 1 {
		t.Fatalf("CAS successes=%d", successes.Load())
	}
	control, err := (&MaintenanceGate{Client: client, Environment: "development"}).read(ctx)
	if err != nil || control.Revision != 2 || control.State != MaintenanceActive {
		t.Fatalf("control=%+v err=%v", control, err)
	}
	iter := client.Collection(maintenanceEvents).Documents(ctx)
	defer iter.Stop()
	count := 0
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		var event maintenanceAuditEvent
		if err := doc.DataTo(&event); err != nil || event.Authority != MaintenanceAuthorityGitHub || event.Result != "COMMITTED" || event.RepositoryID != "340900071" || event.ActorID != "54093651" || event.CloudRunExecution == "" {
			t.Fatalf("invalid audit event=%+v err=%v", event, err)
		}
		count++
	}
	if count != 2 {
		t.Fatalf("audit events=%d", count)
	}
}

func TestMaintenanceAuditCreateFailureRollsBackControl(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	action := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := ApplyMaintenanceTransition(ctx, client, maintenanceRequest(MaintenanceNormal, 0, action, "INITIALIZATION"), now); err != nil {
		t.Fatal(err)
	}
	request := maintenanceRequest(MaintenanceActive, 1, action, "PLANNED_MAINTENANCE")
	if _, err := ApplyMaintenanceTransition(ctx, client, request, now.Add(time.Minute)); err == nil {
		t.Fatal("duplicate audit action unexpectedly committed")
	}
	control, err := (&MaintenanceGate{Client: client, Environment: "development"}).read(ctx)
	if err != nil || control.State != MaintenanceNormal || control.Revision != 1 {
		t.Fatalf("rollback lost: %+v err=%v", control, err)
	}
}

func TestMaintenanceRejectsNoOpTransitionWithoutAudit(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedNormalMaintenance(t, client, "development", now)
	action := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	_, err := ApplyMaintenanceTransition(ctx, client, maintenanceRequest(MaintenanceNormal, 1, action, "RECOVERY"), now.Add(time.Minute))
	if errorCode(err) != "INVALID_REQUEST" {
		t.Fatalf("no-op transition code=%s", errorCode(err))
	}
	if _, err := client.Collection(maintenanceEvents).Doc(action).Get(ctx); status.Code(err) != codes.NotFound {
		t.Fatalf("rejected transition wrote audit: %v", err)
	}
}

func TestMaintenanceGateFailsClosedForMissingAndMalformedControl(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	gate := &MaintenanceGate{Client: client, Environment: "development"}
	if errorCode(gate.CheckAuthoritative(ctx)) != "TEMPORARY_UNAVAILABLE" {
		t.Fatal("missing control did not fail closed")
	}
	if _, err := maintenanceRef(client).Set(ctx, map[string]any{"environment": "development", "state": "NOTICE", "revision": 1, "updatedAt": time.Now(), "messageTemplate": MaintenanceMessageTemplate, "reasonCategory": "RECOVERY"}); err != nil {
		t.Fatal(err)
	}
	if errorCode(gate.CheckAuthoritative(ctx)) != "TEMPORARY_UNAVAILABLE" {
		t.Fatal("malformed control did not fail closed")
	}
}

func TestMaintenanceCacheExpiresWithinBoundAndNeverReusesStaleNormalOnFailure(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedNormalMaintenance(t, client, "development", now)
	clock := now
	gate := &MaintenanceGate{Client: client, Environment: "development", Now: func() time.Time { return clock }, TTL: 5 * time.Second}
	if err := gate.Check(ctx); err != nil {
		t.Fatal(err)
	}
	_, err := maintenanceRef(client).Set(ctx, MaintenanceControl{Environment: "development", State: MaintenanceActive, Revision: 2, UpdatedAt: now.Add(time.Second), MessageTemplate: MaintenanceMessageTemplate, ReasonCategory: "PLANNED_MAINTENANCE"})
	if err != nil {
		t.Fatal(err)
	}
	clock = now.Add(4 * time.Second)
	if err := gate.Check(ctx); err != nil {
		t.Fatalf("cache expired too early: %v", err)
	}
	clock = now.Add(6 * time.Second)
	if code := errorCode(gate.Check(ctx)); code != "MAINTENANCE_ACTIVE" {
		t.Fatalf("expired cache code=%s", code)
	}
	_, err = maintenanceRef(client).Set(ctx, map[string]any{"state": MaintenanceNormal})
	if err != nil {
		t.Fatal(err)
	}
	clock = now.Add(12 * time.Second)
	if code := errorCode(gate.Check(ctx)); code != "TEMPORARY_UNAVAILABLE" {
		t.Fatalf("malformed state reused stale cache: %s", code)
	}
}

func TestMaintenanceTransactionBarrierRejectsNormalOAuthWrite(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err := maintenanceRef(client).Set(ctx, MaintenanceControl{
		Environment: "development", State: MaintenanceActive, Revision: 2,
		UpdatedAt: now, MessageTemplate: MaintenanceMessageTemplate,
		ReasonCategory: "PLANNED_MAINTENANCE",
	})
	if err != nil {
		t.Fatal(err)
	}
	id := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	value := OAuthTransaction{Purpose: "login", StateHash: digest("state"), Status: "pending", PrivacyPolicyVersion: "p1", TermsVersion: "t1", PrivacyConsentedAt: now, CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute)}
	store := &FirestoreAuthStore{Client: client, Environment: "development"}
	if code := errorCode(store.Create(ctx, id, "", value, now)); code != "MAINTENANCE_ACTIVE" {
		t.Fatalf("write barrier code=%s", code)
	}
	if _, err := client.Collection("oauth-transactions").Doc(id).Get(ctx); status.Code(err) != codes.NotFound {
		t.Fatalf("maintenance write committed: %v", err)
	}
}
