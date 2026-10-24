//go:build integration

package mypage

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSupportRecordCreationReissueAndEnvironmentIsolation(t *testing.T) {
	_, authStore, _, _, now := authTestService(t)
	store := &FirestoreSupportStore{Client: authStore.Client, Environment: "development"}
	ctx := context.Background()
	requestID, err := opaque()
	if err != nil {
		t.Fatal(err)
	}
	accepted := now.Add(-time.Hour)
	ref, challenge, err := store.Create(ctx, requestID, "UCsynthetic-support", SupportDelete, accepted, now)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := store.Resolve(ctx, challenge, now)
	if err != nil || binding.RequestRef != ref || binding.RequestID != requestID || binding.Purpose != SupportDelete {
		t.Fatal("challenge did not bind to operator request")
	}
	prod := &FirestoreSupportStore{Client: authStore.Client, Environment: "production"}
	if _, err := prod.Resolve(ctx, challenge, now); errorCode(err) != "SUPPORT_CHALLENGE_INVALID" {
		t.Fatal("cross-environment support challenge accepted")
	}
	if _, err := store.Resolve(ctx, challenge, now.Add(24*time.Hour)); errorCode(err) != "SUPPORT_CHALLENGE_INVALID" {
		t.Fatal("challenge expiry boundary not enforced")
	}
	next, err := store.Reissue(ctx, ref, now.Add(25*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, challenge, now.Add(25*time.Hour)); errorCode(err) != "SUPPORT_CHALLENGE_INVALID" {
		t.Fatal("old challenge survived reissue")
	}
	if _, err := store.Resolve(ctx, next, now.Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	doc, err := store.Client.Collection("support-requests").Doc(ref).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var value SupportRequest
	if err := doc.DataTo(&value); err != nil {
		t.Fatal(err)
	}
	if !value.AcceptedAt.Equal(accepted) || value.DeleteBy == nil || !value.DeleteBy.Equal(accepted.Add(7*24*time.Hour)) || value.ChallengeHash == next {
		t.Fatal("reissue reset receipt/SLA or stored raw challenge")
	}
	if _, err := store.Client.Collection("support-challenges").Doc(digest(challenge)).Get(ctx); status.Code(err) != codes.NotFound {
		t.Fatal("superseded challenge index retained")
	}
}

func TestSupportDuplicateReceiptRollsBackAllCreatedDocuments(t *testing.T) {
	_, authStore, _, _, now := authTestService(t)
	store := &FirestoreSupportStore{Client: authStore.Client, Environment: "development"}
	ctx := context.Background()
	requestID, err := opaque()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Create(ctx, requestID, "UCsynthetic-support", SupportRevoke, now, now); err != nil {
		t.Fatal(err)
	}
	before, err := store.Client.Collection("support-requests").Where("requestId", "==", requestID).Documents(ctx).GetAll()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Create(ctx, requestID, "UCsynthetic-other", SupportDelete, now, now); err == nil {
		t.Fatal("duplicate receipt overwrote original purpose/channel")
	}
	after, err := store.Client.Collection("support-requests").Where("requestId", "==", requestID).Documents(ctx).GetAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || len(after) != 1 || before[0].Ref.ID != after[0].Ref.ID {
		t.Fatal("failed duplicate batch left another record")
	}
}

func TestSupportConcurrentReceiptCreationHasOneWinner(t *testing.T) {
	_, authStore, _, _, now := authTestService(t)
	store := &FirestoreSupportStore{Client: authStore.Client, Environment: "development"}
	ctx := context.Background()
	requestID, err := opaque()
	if err != nil {
		t.Fatal(err)
	}
	if got := concurrently(t, func() error {
		_, _, err := store.Create(ctx, requestID, "UCsynthetic-support", SupportDisclosure, now, now)
		return err
	}); got != 1 {
		t.Fatalf("concurrent receipt creation winners=%d", got)
	}
	docs, err := store.Client.Collection("support-requests").Where("requestId", "==", requestID).Documents(ctx).GetAll()
	if err != nil || len(docs) != 1 {
		t.Fatal("concurrent receipt writes left duplicates")
	}
}
