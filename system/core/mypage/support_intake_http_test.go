package mypage

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type fakePrivacyIntake struct {
	uid         string
	calls       int
	statusCalls int
}

func (f *fakePrivacyIntake) Create(_ context.Context, uid, _ string, purpose SupportPurpose, _ string, now time.Time) (PrivacyIntakeReceipt, error) {
	f.uid = uid
	f.calls++
	if purpose != SupportDelete {
		return PrivacyIntakeReceipt{}, apiError("INVALID_REQUEST")
	}
	return PrivacyIntakeReceipt{RequestRef: digest("synthetic-ref"), Purpose: purpose, Status: "awaiting_proof", AcceptedAt: now}, nil
}

func (f *fakePrivacyIntake) Status(_ context.Context, uid, _ string) (PrivacyRequestStatus, error) {
	f.uid = uid
	f.statusCalls++
	return PrivacyRequestStatus{RequestRef: digest("synthetic-ref"), Status: "verified"}, nil
}

func TestPrivacyHTTPIdentityRestrictionAndBoundaries(t *testing.T) {
	body := `{"submissionKey":"synthetic-key-0001","purpose":"delete","body":"Remove my data"}`
	t.Run("disabled", func(t *testing.T) {
		h, _, _, _ := boundaryFixture()
		if w := boundaryRequest(h, "POST", "/api/privacy/requests", body); w.Code != 503 {
			t.Fatal(w.Code)
		}
	})
	t.Run("identity and restriction", func(t *testing.T) {
		h, account, verifier, _ := boundaryFixture()
		fake := new(fakePrivacyIntake)
		h.Intake = fake
		h.Auth.Access = restrictedAccess(nowForAccessTest(), false)
		if w := boundaryRequest(h, "POST", "/api/privacy/requests", body); w.Code != 200 || fake.uid != "synthetic" || account.reads != 0 {
			t.Fatalf("blocked user's privacy request failed: %d", w.Code)
		}
		if w := boundaryRequest(h, "POST", "/api/privacy/requests/status", `{"requestRef":"`+digest("synthetic-ref")+`"}`); w.Code != 200 || fake.statusCalls != 1 {
			t.Fatal("restricted status refused")
		}
		verifier.tokenErr = fmt.Errorf("synthetic invalid token")
		if w := boundaryRequest(h, "POST", "/api/privacy/requests", body); w.Code != 401 || fake.calls != 1 {
			t.Fatal("no-token intake allowed")
		}
	})
	t.Run("forged fields and preverification", func(t *testing.T) {
		h, _, verifier, _ := boundaryFixture()
		fake := new(fakePrivacyIntake)
		h.Intake = fake
		for _, forged := range []string{
			`{"submissionKey":"synthetic-key-0001","purpose":"delete","body":"Remove my data","targetChannel":"UCvictim"}`,
			`{"submissionKey":"synthetic-key-0001","purpose":"delete","purpose":"revoke","body":"Remove my data"}`,
		} {
			if w := boundaryRequest(h, "POST", "/api/privacy/requests", forged); w.Code != 400 {
				t.Fatal("forged field accepted")
			}
		}
		if verifier.tokenCalls != 0 || fake.calls != 0 {
			t.Fatal("invalid input reached verification or store")
		}
		for i := 0; i < 4; i++ {
			w := boundaryRequest(h, "POST", "/api/privacy/requests", body)
			if i == 3 && (w.Code != 429 || w.Header().Get("Retry-After") == "") {
				t.Fatal("preverification limit missing")
			}
		}
	})
	t.Run("identity limit", func(t *testing.T) {
		h, _, _, _ := boundaryFixture()
		fake := new(fakePrivacyIntake)
		h.Intake = fake
		for range 2 {
			if w := boundaryRequest(h, "POST", "/api/privacy/requests", body); w.Code != 200 {
				t.Fatal("first identity-bound requests denied")
			}
		}
		w := boundaryRequest(h, "POST", "/api/privacy/requests", body)
		if w.Code != 429 || w.Header().Get("Retry-After") == "" || fake.calls != 2 {
			t.Fatal("identity-bound limit did not protect store")
		}
	})
}
