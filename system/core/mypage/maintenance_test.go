package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type maintenanceStub struct {
	mu             sync.Mutex
	checkErr       error
	authoritative  []error
	revisions      []int64
	authorityCalls int
	status         MaintenanceStatus
	statusErr      error
	checkCalls     int
}

type maintenanceFlowStore struct {
	AuthStore
	tx          OAuthTransaction
	channel     Channel
	creates     int
	failCalls   int
	failErr     error
	claimCalls  int
	verifyCalls int
	purposeErr  error
}

func (s *maintenanceFlowStore) Create(_ context.Context, _, _ string, value OAuthTransaction, _ time.Time) error {
	s.creates++
	s.tx = value
	return nil
}

func (s *maintenanceFlowStore) ReadPurpose(context.Context, string, time.Time) (string, int64, error) {
	return transactionPurpose(s.tx), s.tx.MaintenanceRevision, s.purposeErr
}

func (s *maintenanceFlowStore) Claim(context.Context, string, string, time.Time) error {
	s.claimCalls++
	s.tx.Status = "processing"
	return nil
}

func (s *maintenanceFlowStore) Verify(_ context.Context, _ string, channel Channel, confirmation string, now time.Time) error {
	s.verifyCalls++
	s.tx.Status = "channel_verified"
	s.tx.Channel = channel
	s.tx.ConfirmationRef = confirmation
	s.tx.VerifiedAt = now
	return nil
}

func (s *maintenanceFlowStore) Fail(context.Context, string) error {
	s.failCalls++
	return s.failErr
}

func (s *maintenanceFlowStore) ReadVerified(context.Context, string, time.Time) (OAuthTransaction, error) {
	return s.tx, nil
}

func (s *maintenanceFlowStore) Consume(context.Context, string, string, string, Policy, time.Time) (Channel, error) {
	return s.channel, nil
}

type maintenanceProvider struct {
	calls int
	err   error
}

func (*maintenanceProvider) AuthorizationURL(state string) string {
	return "https://example.test/?state=" + state
}

func (p *maintenanceProvider) Resolve(context.Context, string) ([]Channel, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return []Channel{{ID: "UCsynthetic", DisplayName: "Synthetic"}}, nil
}

type maintenanceMinter struct{ calls int }

func (m *maintenanceMinter) Mint(context.Context, string) (string, error) {
	m.calls++
	return "must-not-be-issued", nil
}

func (s *maintenanceStub) Check(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkCalls++
	return s.checkErr
}

func (s *maintenanceStub) CheckAuthoritative(ctx context.Context) error {
	_, err := s.CheckAuthoritativeRevision(ctx)
	return err
}

func (s *maintenanceStub) CheckAuthoritativeRevision(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.authorityCalls
	s.authorityCalls++
	revision := int64(1)
	if index < len(s.revisions) {
		revision = s.revisions[index]
	}
	if index < len(s.authoritative) {
		return revision, s.authoritative[index]
	}
	return revision, nil
}

func (s *maintenanceStub) PublicStatus(context.Context) (MaintenanceStatus, error) {
	return s.status, s.statusErr
}

func TestPublicMaintenanceStatusNeedsNoCredentialsAndLeaksNoOperatorFields(t *testing.T) {
	now := time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC)
	h := &HTTPHandler{PublicOrigin: "https://mypage.example.test", Maintenance: &maintenanceStub{status: MaintenanceStatus{State: MaintenanceActive, UpdatedAt: now, MessageTemplate: MaintenanceMessageTemplate}}}
	r := httptest.NewRequest(http.MethodGet, "https://mypage.example.test/api/public/mypage-status", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d headers=%v body=%s", w.Code, w.Header(), w.Body.String())
	}
	var body map[string]any
	if json.Unmarshal(w.Body.Bytes(), &body) != nil || len(body) != 3 || body["state"] != MaintenanceActive || body["messageTemplate"] != MaintenanceMessageTemplate {
		t.Fatalf("unexpected public body: %#v", body)
	}
	for _, forbidden := range []string{"revision", "reasonCategory", "environment", "actorId", "repositoryId"} {
		if _, exists := body[forbidden]; exists {
			t.Fatalf("public status exposed %s", forbidden)
		}
	}
}

func TestHTTPMaintenanceStopsPersonalDataBeforeAccountRead(t *testing.T) {
	h, store, verifier, reads := boundaryFixture()
	h.Maintenance = &maintenanceStub{checkErr: apiError("MAINTENANCE_ACTIVE")}
	w := boundaryRequest(h, http.MethodGet, "/api/mypage", "")
	if w.Code != http.StatusServiceUnavailable || responseCode(t, w) != "MAINTENANCE_ACTIVE" || store.reads != 0 || *reads != 0 || verifier.tokenCalls != 1 {
		t.Fatalf("status=%d code=%s account=%d data=%d token=%d", w.Code, responseCode(t, w), store.reads, *reads, verifier.tokenCalls)
	}
}

func TestMaintenancePermissionMatrixKeepsPrivacyPurposeAvailable(t *testing.T) {
	h, store, _, reads := boundaryFixture()
	h.Maintenance = &maintenanceStub{checkErr: apiError("MAINTENANCE_ACTIVE")}
	intake := new(fakePrivacyIntake)
	h.Intake = intake
	body := `{"submissionKey":"synthetic-key-0001","purpose":"delete","body":"Remove my data"}`
	w := boundaryRequest(h, http.MethodPost, "/api/privacy/requests", body)
	if w.Code != http.StatusOK || intake.calls != 1 || store.reads != 0 || *reads != 0 {
		t.Fatalf("status=%d intake=%d account=%d data=%d", w.Code, intake.calls, store.reads, *reads)
	}
	expected := map[string]bool{
		"/api/public/mypage-status": true,
		"/api/privacy/requests":     true, "/api/privacy/requests/status": true,
		"/api/auth/youtube/start": true, "/api/auth/youtube/callback": true,
		"/api/auth/youtube/channel": true, "/api/auth/youtube/confirm": true,
	}
	if len(maintenanceExemptRoutes) != len(expected) {
		t.Fatalf("maintenance exceptions=%v", maintenanceExemptRoutes)
	}
	for route := range routeMethods {
		if maintenanceExemptRoutes[route] != expected[route] {
			t.Fatalf("unexpected maintenance classification for %s", route)
		}
	}
}

func TestBFFDoesNotPublishInFlightDataAfterActivation(t *testing.T) {
	now := workFixture().AsOf
	gate := &maintenanceStub{authoritative: []error{nil, nil, apiError("MAINTENANCE_ACTIVE")}}
	reads := 0
	bff := &BFF{Access: allowedAccess(), Maintenance: gate, Environment: "development", Now: func() time.Time { return now }, Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) {
		reads++
		return workFixture(), nil
	})}
	_, err := bff.Get(context.Background(), "synthetic", healthyAccount(now))
	if errorCode(err) != "MAINTENANCE_ACTIVE" || reads != 1 {
		t.Fatalf("code=%s reads=%d", errorCode(err), reads)
	}
	if len(bff.cache) != 0 {
		t.Fatal("in-flight response entered cache after maintenance activation")
	}
}

func TestBFFCacheCannotCrossAnUnobservedActiveToNormalRevision(t *testing.T) {
	now := workFixture().AsOf
	gate := &maintenanceStub{revisions: []int64{1, 1, 1, 1, 3, 3, 3, 3}}
	total := 6000
	reads := 0
	bff := &BFF{Access: allowedAccess(), Maintenance: gate, Environment: "development", Now: func() time.Time { return now }, Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) {
		reads++
		snapshot := workFixture()
		snapshot.User.TotalStudySec = total
		return snapshot, nil
	})}
	first, err := bff.Get(context.Background(), "synthetic", healthyAccount(now))
	if err != nil {
		t.Fatal(err)
	}
	total = 9000 // Data changed while this instance received no ACTIVE request.
	second, err := bff.Get(context.Background(), "synthetic", healthyAccount(now))
	if err != nil {
		t.Fatal(err)
	}
	if reads != 2 || first.Summary.Data == nil || second.Summary.Data == nil ||
		first.Summary.Data.Lifetime.WorkSec == nil || second.Summary.Data.Lifetime.WorkSec == nil ||
		*first.Summary.Data.Lifetime.WorkSec == *second.Summary.Data.Lifetime.WorkSec {
		t.Fatalf("reads=%d first=%+v second=%+v", reads, first.Summary, second.Summary)
	}
}

func TestNormalOAuthChannelAndMintBarriersRejectActivation(t *testing.T) {
	now := time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC)
	store := &maintenanceFlowStore{
		tx:      OAuthTransaction{Purpose: "login", Channel: Channel{ID: "UCsynthetic", DisplayName: "Synthetic"}, MaintenanceRevision: 1},
		channel: Channel{ID: "UCsynthetic", DisplayName: "Synthetic"},
	}
	active := &maintenanceStub{authoritative: []error{apiError("MAINTENANCE_ACTIVE")}}
	service := &AuthService{Access: allowedAccess(), Store: store, Maintenance: active, Policy: Policy{Privacy: "p1", Terms: "t1"}, Now: func() time.Time { return now }}
	if _, err := service.Channel(context.Background(), "transaction"); errorCode(err) != "MAINTENANCE_ACTIVE" {
		t.Fatalf("channel code=%s", errorCode(err))
	}
	service.Maintenance = &maintenanceStub{revisions: []int64{3}}
	if _, err := service.Channel(context.Background(), "transaction"); errorCode(err) != "TEMPORARY_UNAVAILABLE" {
		t.Fatalf("channel revision code=%s", errorCode(err))
	}
	minter := new(maintenanceMinter)
	service.Minter = minter
	service.Maintenance = &maintenanceStub{authoritative: []error{nil, apiError("MAINTENANCE_ACTIVE")}}
	if _, err := service.ConfirmResult(context.Background(), "transaction", "confirmation"); errorCode(err) != "MAINTENANCE_ACTIVE" || minter.calls != 0 {
		t.Fatalf("confirm code=%s mint calls=%d", errorCode(err), minter.calls)
	}
}

func TestPersonalResponseFinalBarrierRequiresOriginatingRevisionAndHidesIt(t *testing.T) {
	h := &HTTPHandler{Maintenance: &maintenanceStub{revisions: []int64{3}}}
	w := httptest.NewRecorder()
	response := ChannelResponse{MaintenanceRevision: 2, Purpose: "login", DisplayName: "Synthetic"}
	if h.writePersonalJSON(w, context.Background(), http.StatusOK, response, "request", "", response.MaintenanceRevision) ||
		w.Code != http.StatusServiceUnavailable || responseCode(t, w) != "TEMPORARY_UNAVAILABLE" {
		t.Fatalf("status=%d code=%s", w.Code, responseCode(t, w))
	}

	h.Maintenance = &maintenanceStub{revisions: []int64{2}}
	w = httptest.NewRecorder()
	if !h.writePersonalJSON(w, context.Background(), http.StatusOK, response, "request", "", response.MaintenanceRevision) || w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "maintenanceRevision") {
		t.Fatalf("internal maintenance revision leaked: %s", w.Body.String())
	}
}

func TestNormalOAuthCallbackActivationFailsTransactionAndRedirectsWithoutProviderCall(t *testing.T) {
	now := time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC)
	store := &maintenanceFlowStore{tx: OAuthTransaction{Purpose: "login"}}
	provider := new(maintenanceProvider)
	gate := &maintenanceStub{authoritative: []error{apiError("MAINTENANCE_ACTIVE")}}
	h := &HTTPHandler{
		PublicOrigin: "https://mypage.example.test", Maintenance: gate,
		Verifier: &boundaryVerifier{},
		Auth:     &AuthService{Store: store, Provider: provider, Maintenance: gate, Now: func() time.Time { return now }},
	}
	r := httptest.NewRequest(http.MethodGet, "https://mypage.example.test/api/auth/youtube/callback?code=private-code&state=private-state", nil)
	r.Header.Set("Origin", h.PublicOrigin)
	r.AddCookie(&http.Cookie{Name: "__session", Value: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/maintenance" || store.failCalls != 1 || provider.calls != 0 {
		t.Fatalf("status=%d location=%s fails=%d provider=%d", w.Code, w.Header().Get("Location"), store.failCalls, provider.calls)
	}
	if body := w.Body.String(); strings.Contains(body, "private-code") || strings.Contains(body, "private-state") {
		t.Fatal("OAuth secret entered the redirect response")
	}
}

func TestSupportOAuthCallbackFailuresStayOnPublicSupportPathDuringMaintenance(t *testing.T) {
	now := time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, query, redirect   string
		purposeErr, providerErr error
	}{
		{name: "denied", query: "error=access_denied", redirect: "oauth_denied"},
		{name: "expired", query: "code=private-code", redirect: "oauth_transaction_expired", purposeErr: apiError("OAUTH_TRANSACTION_EXPIRED")},
		{name: "scope", query: "code=private-code", redirect: "oauth_scope_insufficient", providerErr: apiError("OAUTH_SCOPE_INSUFFICIENT")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &maintenanceFlowStore{tx: OAuthTransaction{Purpose: "support"}, purposeErr: tc.purposeErr}
			provider := &maintenanceProvider{err: tc.providerErr}
			gate := &maintenanceStub{authoritative: []error{apiError("MAINTENANCE_ACTIVE")}}
			h := &HTTPHandler{
				PublicOrigin: "https://mypage.example.test", Maintenance: gate,
				Verifier: &boundaryVerifier{},
				Auth:     &AuthService{Store: store, Provider: provider, Maintenance: gate, Now: func() time.Time { return now }},
			}
			r := httptest.NewRequest(http.MethodGet, "https://mypage.example.test/api/auth/youtube/callback?state=private-state&"+tc.query, nil)
			r.Header.Set("Origin", h.PublicOrigin)
			r.AddCookie(&http.Cookie{Name: "__session", Value: strings.Repeat("a", 64)})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusFound || w.Header().Get("Location") != "/contact?supportError="+tc.redirect {
				t.Fatalf("status=%d location=%s", w.Code, w.Header().Get("Location"))
			}
			cookies := w.Result().Cookies()
			if len(cookies) != 1 || cookies[0].Name != "__session" || cookies[0].MaxAge != -1 || cookies[0].Value != "" {
				t.Fatalf("support callback cookie not cleared: %+v", cookies)
			}
			if body := w.Body.String(); strings.Contains(body, "private-code") || strings.Contains(body, "private-state") {
				t.Fatal("OAuth input entered the redirect response")
			}
		})
	}
}

func TestNormalOAuthCallbackCannotCrossACompleteMaintenanceInterval(t *testing.T) {
	now := time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC)
	store := &maintenanceFlowStore{tx: OAuthTransaction{Purpose: "login", MaintenanceRevision: 1}}
	provider := new(maintenanceProvider)
	gate := &maintenanceStub{revisions: []int64{1, 3}}
	service := &AuthService{Store: store, Provider: provider, Maintenance: gate, Now: func() time.Time { return now }}
	err := service.Callback(context.Background(), "transaction", "state", "code", false)
	if errorCode(err) != "TEMPORARY_UNAVAILABLE" || store.failCalls != 1 || store.claimCalls != 1 || store.verifyCalls != 1 || provider.calls != 1 {
		t.Fatalf("code=%s fails=%d claims=%d verifies=%d provider=%d", errorCode(err), store.failCalls, store.claimCalls, store.verifyCalls, provider.calls)
	}
}

func TestNormalOAuthStartResponseCannotCrossACompleteMaintenanceInterval(t *testing.T) {
	now := time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC)
	store := new(maintenanceFlowStore)
	gate := &maintenanceStub{revisions: []int64{1, 3}}
	h := &HTTPHandler{
		PublicOrigin: "https://mypage.example.test", Maintenance: gate,
		Verifier: new(boundaryVerifier),
		Auth: &AuthService{
			Store: store, Provider: new(maintenanceProvider), Maintenance: gate,
			Policy: Policy{Privacy: "p1", Terms: "t1"}, Now: func() time.Time { return now },
		},
	}
	r := httptest.NewRequest(http.MethodPost, "https://mypage.example.test/api/auth/youtube/start", strings.NewReader(`{"privacyPolicyVersion":"p1","privacyAccepted":true,"termsVersion":"t1","termsAccepted":true}`))
	r.Header.Set("Origin", h.PublicOrigin)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Firebase-AppCheck", "synthetic-app-proof")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable || responseCode(t, w) != "TEMPORARY_UNAVAILABLE" || store.creates != 1 || store.failCalls != 1 {
		t.Fatalf("status=%d code=%s creates=%d fails=%d", w.Code, responseCode(t, w), store.creates, store.failCalls)
	}
	if strings.Contains(w.Header().Get("Set-Cookie"), "Max-Age=600") {
		t.Fatalf("stale OAuth transaction cookie published: %s", w.Header().Get("Set-Cookie"))
	}
}

func TestMaintenanceDefaultGuardCoversHistoryAndSessionCompletion(t *testing.T) {
	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/api/mypage/history?from=2026-10-01&to=2026-10-02"},
		{http.MethodPost, "/api/auth/session/complete"},
	} {
		h, store, verifier, reads := boundaryFixture()
		h.History = &HistoryService{}
		h.Maintenance = &maintenanceStub{checkErr: apiError("MAINTENANCE_ACTIVE")}
		w := boundaryRequest(h, request.method, request.path, "")
		if w.Code != http.StatusServiceUnavailable || responseCode(t, w) != "MAINTENANCE_ACTIVE" || store.reads != 0 || verifier.tokenCalls != 1 || *reads != 0 {
			t.Fatalf("%s status=%d account=%d token=%d data=%d", request.path, w.Code, store.reads, verifier.tokenCalls, *reads)
		}
	}
}

func TestHTTPRejectsInvalidIdentityBeforeMaintenanceDisclosure(t *testing.T) {
	h, store, verifier, reads := boundaryFixture()
	gate := &maintenanceStub{checkErr: apiError("MAINTENANCE_ACTIVE")}
	h.Maintenance = gate
	verifier.tokenErr = errors.New("invalid token")
	w := boundaryRequest(h, http.MethodGet, "/api/mypage", "")
	if w.Code != http.StatusUnauthorized || responseCode(t, w) != "AUTH_REQUIRED" || gate.checkCalls != 0 || store.reads != 0 || *reads != 0 {
		t.Fatalf("status=%d code=%s gate=%d account=%d data=%d", w.Code, responseCode(t, w), gate.checkCalls, store.reads, *reads)
	}
}

func TestPublicMaintenanceStatusHasBoundedUnauthenticatedRate(t *testing.T) {
	h := &HTTPHandler{PublicOrigin: "https://mypage.example.test", Maintenance: &maintenanceStub{status: MaintenanceStatus{State: MaintenanceNormal, UpdatedAt: time.Now(), MessageTemplate: MaintenanceMessageTemplate}}}
	for request := 1; request <= 31; request++ {
		r := httptest.NewRequest(http.MethodGet, "https://mypage.example.test/api/public/mypage-status", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if request <= 30 && w.Code != http.StatusOK {
			t.Fatalf("request %d status=%d", request, w.Code)
		}
		if request == 31 && (w.Code != http.StatusTooManyRequests || responseCode(t, w) != "RATE_LIMITED") {
			t.Fatalf("unbounded status endpoint: status=%d body=%s", w.Code, w.Body.String())
		}
	}
}

func TestMaintenanceCacheCoalescesConcurrentRefresh(t *testing.T) {
	var reads atomic.Int32
	release := make(chan struct{})
	now := time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC)
	gate := &MaintenanceGate{Environment: "development", Now: func() time.Time { return now }}
	gate.readControl = func(context.Context) (MaintenanceControl, error) {
		reads.Add(1)
		<-release
		return MaintenanceControl{Environment: "development", State: MaintenanceNormal, Revision: 1, UpdatedAt: now, MessageTemplate: MaintenanceMessageTemplate, ReasonCategory: "INITIALIZATION"}, nil
	}
	var wait sync.WaitGroup
	errors := make(chan error, 32)
	for range 32 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errors <- gate.Check(context.Background())
		}()
	}
	for reads.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	close(release)
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if reads.Load() != 1 {
		t.Fatalf("concurrent cache refresh reads=%d", reads.Load())
	}
}

func TestMaintenanceCacheWaiterDeadlineRemainsTemporaryUnavailable(t *testing.T) {
	var reads atomic.Int32
	release := make(chan struct{})
	started := make(chan struct{})
	now := time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC)
	gate := &MaintenanceGate{Environment: "development", Now: func() time.Time { return now }}
	gate.readControl = func(context.Context) (MaintenanceControl, error) {
		reads.Add(1)
		close(started)
		<-release
		return MaintenanceControl{Environment: "development", State: MaintenanceNormal, Revision: 1, UpdatedAt: now, MessageTemplate: MaintenanceMessageTemplate, ReasonCategory: "INITIALIZATION"}, nil
	}
	leader := make(chan error, 1)
	go func() { leader <- gate.Check(context.Background()) }()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := gate.Check(ctx)
	if errorCode(err) != "TEMPORARY_UNAVAILABLE" || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("code=%s err=%v", errorCode(err), err)
	}
	close(release)
	if err := <-leader; err != nil {
		t.Fatal(err)
	}
	if reads.Load() != 1 {
		t.Fatalf("waiter started an extra read: %d", reads.Load())
	}
}

func TestOAuthMaintenanceIdentitySurvivesCleanupFailure(t *testing.T) {
	now := time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC)
	store := &maintenanceFlowStore{tx: OAuthTransaction{Purpose: "login"}, failErr: errors.New("cleanup dependency unavailable")}
	gate := &maintenanceStub{authoritative: []error{apiError("MAINTENANCE_ACTIVE")}}
	service := &AuthService{Store: store, Provider: new(maintenanceProvider), Maintenance: gate, Now: func() time.Time { return now }}
	if code := errorCode(service.Callback(context.Background(), "transaction", "state", "code", false)); code != "MAINTENANCE_ACTIVE" {
		t.Fatalf("callback code=%s", code)
	}
}

func TestMaintenanceTransitionValidationIsClosedAndExact(t *testing.T) {
	now := time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC)
	valid := MaintenanceTransition{Authority: MaintenanceAuthorityGitHub, Environment: "development", State: MaintenanceActive, ExpectedRevision: 2, ExpectedPreviousState: MaintenanceNormal, ReasonCategory: "PLANNED_MAINTENANCE", ActionRef: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", RepositoryID: "340900071", RepositoryOwnerID: "54093651", ActorID: "54093651", TriggeringActorID: "54093651", RunID: "123", RunAttempt: 1, GitRef: "refs/heads/integration/mypage-canon-20261006", CommitSHA: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", WorkflowRef: "kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@refs/heads/integration/mypage-canon-20261006", CloudRunExecution: "mypage-maintenance-operator-abcde"}
	if err := ValidateMaintenanceTransition(valid, now); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*MaintenanceTransition){
		"unknown state":               func(v *MaintenanceTransition) { v.State = "NOTICE" },
		"free text reason":            func(v *MaintenanceTransition) { v.ReasonCategory = "operator says so" },
		"rerun":                       func(v *MaintenanceTransition) { v.RunAttempt = 2 },
		"repository substitution":     func(v *MaintenanceTransition) { v.RepositoryID = "1" },
		"owner substitution":          func(v *MaintenanceTransition) { v.RepositoryOwnerID = "1" },
		"actor substitution":          func(v *MaintenanceTransition) { v.TriggeringActorID = "1" },
		"matching actor substitution": func(v *MaintenanceTransition) { v.ActorID, v.TriggeringActorID = "1", "1" },
		"wrong ref":                   func(v *MaintenanceTransition) { v.GitRef = "refs/heads/feature" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if ValidateMaintenanceTransition(candidate, now) == nil {
				t.Fatal("invalid transition accepted")
			}
		})
	}
	invalidRecovery := valid
	invalidRecovery.State = MaintenanceNormal
	invalidRecovery.ExpectedPreviousState = MaintenanceActive
	invalidRecovery.ReasonCategory = "PLANNED_MAINTENANCE"
	if ValidateMaintenanceTransition(invalidRecovery, now) == nil {
		t.Fatal("ACTIVE to NORMAL accepted a non-RECOVERY reason")
	}
}

func TestMaintenanceReceiptValidationDoesNotExpireCommittedEstimate(t *testing.T) {
	now := time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC)
	estimate := now.Add(time.Minute)
	request := MaintenanceTransition{Authority: MaintenanceAuthorityGitHub, Environment: "development", State: MaintenanceActive, ExpectedRevision: 2, ExpectedPreviousState: MaintenanceNormal, EstimatedEndAt: &estimate, ReasonCategory: "PLANNED_MAINTENANCE", ActionRef: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", RepositoryID: "340900071", RepositoryOwnerID: "54093651", ActorID: "54093651", TriggeringActorID: "54093651", RunID: "123", RunAttempt: 1, GitRef: "refs/heads/integration/mypage-canon-20261006", CommitSHA: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", WorkflowRef: "kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@refs/heads/integration/mypage-canon-20261006", CloudRunExecution: "mypage-maintenance-operator-abcde"}
	if err := ValidateMaintenanceTransition(request, now); err != nil {
		t.Fatal(err)
	}
	if ValidateMaintenanceTransition(request, estimate) == nil {
		t.Fatal("new transition accepted an expired estimate")
	}
	if err := ValidateMaintenanceTransitionReceipt(request); err != nil {
		t.Fatalf("committed receipt expired: %v", err)
	}
}

func TestMaintenanceBreakGlassUsesTruthfulDistinctProvenance(t *testing.T) {
	now := time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC)
	value := MaintenanceTransition{Authority: MaintenanceAuthorityBreakGlass, Environment: "development", State: MaintenanceActive, ExpectedRevision: 2, ExpectedPreviousState: MaintenanceNormal, ReasonCategory: "SECURITY_MAINTENANCE", ActionRef: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", GitRef: "refs/heads/integration/mypage-canon-20261006", CommitSHA: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", CloudRunExecution: "mypage-maintenance-breakglass-abcde", BreakGlassApprovalRef: "INCIDENT_123", ClaimedOperator: "operator@example.com"}
	if err := ValidateMaintenanceTransition(value, now); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*MaintenanceTransition){
		"github field":      func(v *MaintenanceTransition) { v.ActorID = "54093651" },
		"missing approval":  func(v *MaintenanceTransition) { v.BreakGlassApprovalRef = "" },
		"invalid operator":  func(v *MaintenanceTransition) { v.ClaimedOperator = "free text" },
		"missing execution": func(v *MaintenanceTransition) { v.CloudRunExecution = "" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := value
			mutate(&candidate)
			if ValidateMaintenanceTransition(candidate, now) == nil {
				t.Fatal("invalid break-glass transition accepted")
			}
		})
	}
}
