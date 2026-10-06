package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type boundaryStore struct {
	AuthStore
	account WebAccount
	err     error
	reads   int
}

func (s *boundaryStore) ReadAccount(context.Context, string) (WebAccount, error) {
	s.reads++
	return s.account, s.err
}

type boundaryVerifier struct {
	appErr, tokenErr     error
	identity             VerifiedIdentity
	appCalls, tokenCalls int
}

func (v *boundaryVerifier) VerifyAppCheck(context.Context, string) error {
	v.appCalls++
	return v.appErr
}

func (v *boundaryVerifier) VerifyIDToken(context.Context, string) (VerifiedIdentity, error) {
	v.tokenCalls++
	return v.identity, v.tokenErr
}

func boundaryFixture() (*HTTPHandler, *boundaryStore, *boundaryVerifier, *int) {
	now := workFixture().AsOf
	s := &boundaryStore{account: healthyAccount(now)}
	v := &boundaryVerifier{identity: VerifiedIdentity{UID: "synthetic", Provider: "custom"}}
	reads := new(int)
	h := &HTTPHandler{Auth: &AuthService{Store: s, Policy: Policy{Privacy: "p1", Terms: "t1"}, Now: func() time.Time { return now }}, Verifier: v, BFF: &BFF{Environment: "demo", Now: func() time.Time { return now }, Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) { *reads++; return workFixture(), nil })}}
	h.PublicOrigin = "https://mypage.example.test"
	return h, s, v, reads
}

func boundaryRequest(h *HTTPHandler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = "mypage.example.test"
	r.Header.Set("Origin", h.PublicOrigin)
	r.Header.Set("X-Firebase-AppCheck", "synthetic-app-proof")
	r.Header.Set("Authorization", "Bearer synthetic-id-proof")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func responseCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Error.Code
}

func TestHTTPAuthorizationBeforeDataAndCache(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		setup      func(*boundaryStore, *boundaryVerifier)
	}{
		{"appcheck", "APP_CHECK_REQUIRED", func(_ *boundaryStore, v *boundaryVerifier) { v.appErr = errors.New("private appcheck detail") }},
		{"token", "AUTH_REQUIRED", func(_ *boundaryStore, v *boundaryVerifier) { v.tokenErr = errors.New("private auth detail") }},
		{"provider", "AUTH_REQUIRED", func(_ *boundaryStore, v *boundaryVerifier) { v.identity.Provider = "google.com" }},
		{"uid", "AUTH_REQUIRED", func(_ *boundaryStore, v *boundaryVerifier) { v.identity.UID = "" }},
		{"missing-account", "WEB_ACCOUNT_REQUIRED", func(s *boundaryStore, _ *boundaryVerifier) { s.err = apiError("WEB_ACCOUNT_REQUIRED") }},
		{"blocked", "AUTH_REQUIRED", func(s *boundaryStore, _ *boundaryVerifier) { s.account.AccessBlocked = true }},
		{"consent", "PRIVACY_RECONSENT_REQUIRED", func(s *boundaryStore, _ *boundaryVerifier) { s.account.TermsVersion = "old" }},
		{"dependency", "TEMPORARY_UNAVAILABLE", func(s *boundaryStore, _ *boundaryVerifier) { s.err = errors.New("private firestore detail") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, s, v, reads := boundaryFixture()
			tc.setup(s, v)
			w := boundaryRequest(h, "GET", "/api/mypage", "")
			if responseCode(t, w) != tc.want || *reads != 0 {
				t.Fatalf("status=%d code=%s reads=%d", w.Code, responseCode(t, w), *reads)
			}
			if strings.Contains(w.Body.String(), "private") {
				t.Fatal("dependency detail leaked")
			}
			if v.appErr != nil && (v.tokenCalls != 0 || s.reads != 0) {
				t.Fatal("AppCheck failure reached auth/account")
			}
		})
	}
	h, s, _, reads := boundaryFixture()
	for range 2 {
		if w := boundaryRequest(h, "GET", "/api/mypage", ""); w.Code != 200 {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
	if *reads != 1 || s.reads != 2 {
		t.Fatal("cached response bypassed fresh account gate")
	}
	s.account.AccessBlocked = true
	if w := boundaryRequest(h, "GET", "/api/mypage", ""); w.Code != 401 {
		t.Fatal("blocked account read cache")
	}
	s.account.AccessBlocked = false
	boundaryRequest(h, "GET", "/api/mypage", "")
	if *reads != 2 {
		t.Fatal("blocked identity left cached data behind")
	}
}

func TestHTTPRejectsAmbiguousInput(t *testing.T) {
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/api/auth/youtube/start", "null", 400},
		{"POST", "/api/auth/youtube/start", `{"PrivacyAccepted":true}`, 400},
		{"POST", "/api/auth/youtube/start", `{"privacyAccepted":false,"privacyAccepted":true}`, 400},
		{"POST", "/api/auth/youtube/start", `{"returnUrl":"https://example.invalid"}`, 400},
		{"POST", "/api/auth/youtube/start", "{} {}", 400},
		{"POST", "/api/auth/youtube/start", strings.Repeat("x", 4097), 413},
		{"POST", "/api/auth/session/complete", "{}", 400},
		{"GET", "/api/mypage", "{}", 400},
		{"GET", "/api/mypage?uid=other", "", 400},
		{"POST", "/api/mypage", "", 405},
		{"GET", "/api/unknown", "", 404},
		{"GET", "/api/auth/youtube/callback?state=x&code=a&code=b", "", 400},
		{"GET", "/api/auth/youtube/callback?state=x&code=a&returnUrl=https://example.invalid", "", 400},
	} {
		h, s, v, reads := boundaryFixture()
		w := boundaryRequest(h, tc.method, tc.path, tc.body)
		if w.Code != tc.status || s.reads != 0 || v.appCalls != 0 || *reads != 0 {
			t.Fatalf("%s %s status=%d", tc.method, tc.path, w.Code)
		}
		if w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("X-Request-Id") == "" {
			t.Fatal("missing private cache/request headers")
		}
	}
}

func TestHTTPRateLimitBeforeSnapshot(t *testing.T) {
	h, s, _, reads := boundaryFixture()
	for range 10 {
		if boundaryRequest(h, "GET", "/api/mypage", "").Code != 200 {
			t.Fatal("burst rejected early")
		}
	}
	w := boundaryRequest(h, "GET", "/api/mypage", "")
	if w.Code != 429 || w.Header().Get("Retry-After") == "" || s.reads != 10 || *reads != 1 {
		t.Fatal("rate limit/gate/cache mismatch")
	}
}

func TestOpaqueCookieAndErrorAllowlist(t *testing.T) {
	w := httptest.NewRecorder()
	setTransactionCookie(w, strings.Repeat("a", 64), 600)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("cookie missing")
	}
	c := cookies[0]
	if c.Name != "__session" || c.Domain != "" || c.Path != "/" || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.MaxAge != 600 {
		t.Fatal("cookie scope changed")
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(c)
	r.AddCookie(c)
	if transactionID(r) != "" {
		t.Fatal("duplicate transaction cookie accepted")
	}
	if errorCode(apiError("private unapproved detail")) != "INTERNAL_ERROR" {
		t.Fatal("public error code not allowlisted")
	}
	ip, err := (&HTTPHandler{}).clientIP(httptest.NewRequest("GET", "/", nil))
	if err != nil || ip == "" {
		t.Fatal("default remote IP extraction")
	}
}
