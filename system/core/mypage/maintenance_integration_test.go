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
	previous := MaintenanceActive
	if revision == 0 {
		previous = "MISSING"
	} else if state == MaintenanceActive {
		previous = MaintenanceNormal
	}
	return MaintenanceTransition{Authority: MaintenanceAuthorityGitHub, Environment: "development", State: state, ExpectedRevision: revision, ExpectedPreviousState: previous, ReasonCategory: reason, ActionRef: action, RepositoryID: "340900071", RepositoryOwnerID: "54093651", ActorID: "54093651", TriggeringActorID: "54093651", RunID: "123", RunAttempt: 1, GitRef: "refs/heads/integration/mypage-canon-20261006", CommitSHA: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", WorkflowRef: "kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@refs/heads/integration/mypage-canon-20261006", CloudRunExecution: "mypage-maintenance-operator-abcde"}
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
	eventRef := client.Collection(maintenanceEvents).Doc(request.ActionRef)
	if _, err := eventRef.Update(ctx, []firestore.Update{{Path: "previousState", Value: MaintenanceActive}}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyMaintenanceTransition(ctx, client, request, now.Add(time.Second)); err == nil {
		t.Fatal("verify accepted an inconsistent previous state")
	}
	if _, err := eventRef.Update(ctx, []firestore.Update{{Path: "previousState", Value: "MISSING"}, {Path: "privateNote", Value: "must-not-be-accepted"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyMaintenanceTransition(ctx, client, request, now.Add(time.Second)); err == nil {
		t.Fatal("verify accepted an unknown audit field")
	}
	request.ActionRef = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if _, err := VerifyMaintenanceTransition(ctx, client, request, now.Add(time.Second)); err == nil {
		t.Fatal("verify accepted a missing audit receipt")
	}
}

func TestMaintenanceApplyNormalizesFirestoreTimestampAndLateVerify(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 27, 2, 0, 0, 123456789, time.UTC)
	request := maintenanceRequest(MaintenanceNormal, 0, "abababababababababababababababababababababababababababababababab", "INITIALIZATION")
	result, err := ApplyMaintenanceTransition(ctx, client, request, now)
	expected := now.Truncate(time.Microsecond)
	if err != nil || !result.Current.UpdatedAt.Equal(expected) {
		t.Fatalf("result=%+v expected=%v err=%v", result, expected, err)
	}

	rawEnd := expected.Add(time.Second + 789*time.Nanosecond)
	end := rawEnd.Truncate(time.Microsecond)
	activate := maintenanceRequest(MaintenanceActive, 1, "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd", "PLANNED_MAINTENANCE")
	activate.EstimatedEndAt = &rawEnd
	result, err = ApplyMaintenanceTransition(ctx, client, activate, expected.Add(time.Microsecond))
	if err != nil || !sameOptionalTime(result.Current.EstimatedEndAt, &end) {
		t.Fatal(err)
	}
	if _, err := VerifyMaintenanceTransition(ctx, client, activate, end.Add(time.Hour)); err != nil {
		t.Fatalf("late verify rejected committed estimate: %v", err)
	}
}

func TestMaintenanceBootstrapRequiresValidInitializedControlAndAllowsActiveRecovery(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	if RequireMaintenanceInitialized(ctx, client, "development") == nil {
		t.Fatal("missing bootstrap control accepted")
	}
	seedNormalMaintenance(t, client, "development", now)
	if err := RequireMaintenanceInitialized(ctx, client, "development"); err != nil {
		t.Fatal(err)
	}
	_, err := maintenanceRef(client).Set(ctx, MaintenanceControl{Environment: "development", State: MaintenanceActive, Revision: 2, UpdatedAt: now.Add(time.Second), MessageTemplate: MaintenanceMessageTemplate, ReasonCategory: "PLANNED_MAINTENANCE"})
	if err != nil {
		t.Fatal(err)
	}
	if err := RequireMaintenanceInitialized(ctx, client, "development"); err != nil {
		t.Fatalf("ACTIVE must permit a recovery deploy or rollback: %v", err)
	}
}

func TestMaintenanceBreakGlassAuditDoesNotInventGitHubProvenance(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := ApplyMaintenanceTransition(ctx, client, maintenanceRequest(MaintenanceNormal, 0, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "INITIALIZATION"), now); err != nil {
		t.Fatal(err)
	}
	request := MaintenanceTransition{Authority: MaintenanceAuthorityBreakGlass, Environment: "development", State: MaintenanceActive, ExpectedRevision: 1, ExpectedPreviousState: MaintenanceNormal, ReasonCategory: "SECURITY_MAINTENANCE", ActionRef: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", GitRef: "refs/heads/integration/mypage-canon-20261006", CommitSHA: "cccccccccccccccccccccccccccccccccccccccc", CloudRunExecution: "mypage-maintenance-breakglass-abcde", BreakGlassApprovalRef: "INCIDENT_123", ClaimedOperator: "operator@example.com"}
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

func TestMaintenanceActiveEstimateUpdateUsesCASAndTransactionalAudit(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := ApplyMaintenanceTransition(ctx, client, maintenanceRequest(MaintenanceNormal, 0, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "INITIALIZATION"), now); err != nil {
		t.Fatal(err)
	}
	firstEnd := now.Add(2 * time.Hour)
	activate := maintenanceRequest(MaintenanceActive, 1, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "PLANNED_MAINTENANCE")
	activate.EstimatedEndAt = &firstEnd
	if _, err := ApplyMaintenanceTransition(ctx, client, activate, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	updatedEnd := now.Add(4 * time.Hour)
	update := maintenanceRequest(MaintenanceActive, 2, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", "PLANNED_MAINTENANCE")
	update.ExpectedPreviousState = MaintenanceActive
	update.ExpectedPreviousEndAt = &firstEnd
	update.EstimatedEndAt = &updatedEnd
	result, err := ApplyMaintenanceTransition(ctx, client, update, now.Add(2*time.Minute))
	if err != nil || result.Previous == nil || result.Previous.State != MaintenanceActive || result.Current.Revision != 3 || !sameOptionalTime(result.Current.EstimatedEndAt, &updatedEnd) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := VerifyMaintenanceTransition(ctx, client, update, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	doc, err := client.Collection(maintenanceEvents).Doc(update.ActionRef).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var event maintenanceAuditEvent
	if err := doc.DataTo(&event); err != nil || event.PreviousState != MaintenanceActive || !sameOptionalTime(event.PreviousEstimatedEndAt, &firstEnd) || !sameOptionalTime(event.EstimatedEndAt, &updatedEnd) {
		t.Fatalf("event=%+v err=%v", event, err)
	}
	tamperedEnd := firstEnd.Add(time.Minute)
	if _, err := client.Collection(maintenanceEvents).Doc(update.ActionRef).Update(ctx, []firestore.Update{{Path: "previousEstimatedEndAt", Value: tamperedEnd}}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyMaintenanceTransition(ctx, client, update, now.Add(3*time.Minute)); err == nil {
		t.Fatal("verify accepted a tampered previous estimate")
	}
	stale := maintenanceRequest(MaintenanceActive, 2, "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", "PLANNED_MAINTENANCE")
	stale.ExpectedPreviousState = MaintenanceActive
	stale.ExpectedPreviousEndAt = &firstEnd
	staleEnd := now.Add(5 * time.Hour)
	stale.EstimatedEndAt = &staleEnd
	if _, err := ApplyMaintenanceTransition(ctx, client, stale, now.Add(4*time.Minute)); errorCode(err) != "MAINTENANCE_REVISION_MISMATCH" {
		t.Fatalf("stale update code=%s", errorCode(err))
	}
	noOp := maintenanceRequest(MaintenanceActive, 3, "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", "PLANNED_MAINTENANCE")
	noOp.ExpectedPreviousState = MaintenanceActive
	noOp.ExpectedPreviousEndAt = &updatedEnd
	noOp.EstimatedEndAt = &updatedEnd
	if _, err := ApplyMaintenanceTransition(ctx, client, noOp, now.Add(4*time.Minute)); errorCode(err) != "INVALID_REQUEST" {
		t.Fatalf("no-op update code=%s", errorCode(err))
	}
	reasonChange := maintenanceRequest(MaintenanceActive, 3, "1212121212121212121212121212121212121212121212121212121212121212", "DATA_MIGRATION")
	reasonChange.ExpectedPreviousState = MaintenanceActive
	reasonChange.ExpectedPreviousEndAt = &updatedEnd
	reasonChangeEnd := now.Add(6 * time.Hour)
	reasonChange.EstimatedEndAt = &reasonChangeEnd
	if _, err := ApplyMaintenanceTransition(ctx, client, reasonChange, now.Add(4*time.Minute)); errorCode(err) != "INVALID_REQUEST" {
		t.Fatalf("reason-changing update code=%s", errorCode(err))
	}
	for _, action := range []string{stale.ActionRef, noOp.ActionRef, reasonChange.ActionRef} {
		if _, err := client.Collection(maintenanceEvents).Doc(action).Get(ctx); status.Code(err) != codes.NotFound {
			t.Fatalf("rejected update wrote audit %s: %v", action, err)
		}
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
	request := maintenanceRequest(MaintenanceNormal, 1, action, "RECOVERY")
	request.ExpectedPreviousState = MaintenanceNormal
	_, err := ApplyMaintenanceTransition(ctx, client, request, now.Add(time.Minute))
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
	for _, state := range []string{MaintenanceNormal, MaintenanceActive} {
		if _, err := maintenanceRef(client).Set(ctx, map[string]any{"environment": "development", "state": state, "revision": 1, "updatedAt": time.Now(), "estimatedEndAt": nil, "messageTemplate": MaintenanceMessageTemplate, "reasonCategory": "RECOVERY"}); err != nil {
			t.Fatal(err)
		}
		if errorCode(gate.CheckAuthoritative(ctx)) != "TEMPORARY_UNAVAILABLE" {
			t.Fatalf("%s control accepted explicit null estimatedEndAt", state)
		}
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
	value := OAuthTransaction{Purpose: "login", StateHash: digest("state"), Status: "pending", PrivacyPolicyVersion: "p1", TermsVersion: "t1", PrivacyConsentedAt: now, CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute), MaintenanceRevision: 1}
	store := &FirestoreAuthStore{Client: client, Environment: "development"}
	if code := errorCode(store.Create(ctx, id, "", value, now)); code != "MAINTENANCE_ACTIVE" {
		t.Fatalf("write barrier code=%s", code)
	}
	if _, err := client.Collection("oauth-transactions").Doc(id).Get(ctx); status.Code(err) != codes.NotFound {
		t.Fatalf("maintenance write committed: %v", err)
	}
}

func TestNormalOAuthTransactionCannotCrossACompleteMaintenanceInterval(t *testing.T) {
	client := maintenanceIntegrationClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedNormalMaintenance(t, client, "development", now)
	store := &FirestoreAuthStore{Client: client, Environment: "development"}
	id := "abababababababababababababababababababababababababababababababab"
	value := OAuthTransaction{Purpose: "login", StateHash: digest("state"), Status: "pending", PrivacyPolicyVersion: "p1", TermsVersion: "t1", PrivacyConsentedAt: now, CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute), MaintenanceRevision: 1}
	if err := store.Create(ctx, id, "", value, now); err != nil {
		t.Fatal(err)
	}
	if _, err := maintenanceRef(client).Set(ctx, MaintenanceControl{Environment: "development", State: MaintenanceNormal, Revision: 3, UpdatedAt: now.Add(2 * time.Minute), MessageTemplate: MaintenanceMessageTemplate, ReasonCategory: "RECOVERY"}); err != nil {
		t.Fatal(err)
	}
	if code := errorCode(store.Claim(ctx, id, digest("state"), now.Add(3*time.Minute))); code != "TEMPORARY_UNAVAILABLE" {
		t.Fatalf("revision fence code=%s", code)
	}
	doc, err := client.Collection("oauth-transactions").Doc(id).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var unchanged OAuthTransaction
	if err := doc.DataTo(&unchanged); err != nil || unchanged.Status != "pending" {
		t.Fatalf("stale OAuth transaction changed: %+v err=%v", unchanged, err)
	}
}
