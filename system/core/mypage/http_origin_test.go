package mypage

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPRejectsForeignOriginHostAndAmbiguousCookieBeforeVerification(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, host string
		origins, cookies         []string
	}{
		{"foreign-origin", "POST", "/api/auth/youtube/start", "mypage.example.test", []string{"https://other.example.test"}, nil},
		{"missing-origin", "POST", "/api/auth/youtube/start", "mypage.example.test", nil, nil},
		{"opaque-origin", "POST", "/api/auth/youtube/confirm", "mypage.example.test", []string{"null"}, nil},
		{"duplicate-origin", "POST", "/api/auth/youtube/start", "mypage.example.test", []string{"https://mypage.example.test", "https://other.example.test"}, nil},
		{"off-host-callback", "GET", "/api/auth/youtube/callback?state=s&code=c", "other.example.test", nil, nil},
		{"duplicate-cookie", "POST", "/api/auth/youtube/start", "mypage.example.test", []string{"https://mypage.example.test"}, []string{"__session=" + strings.Repeat("a", 64), "__session=" + strings.Repeat("b", 64)}},
		{"malformed-duplicate-cookie", "POST", "/api/auth/youtube/start", "mypage.example.test", []string{"https://mypage.example.test"}, []string{"__session=" + strings.Repeat("a", 64) + "; __session"}},
		{"whitespace-duplicate-cookie", "POST", "/api/auth/youtube/start", "mypage.example.test", []string{"https://mypage.example.test"}, []string{"__session=" + strings.Repeat("a", 64) + "; __session =" + strings.Repeat("b", 64)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, s, v, reads := boundaryFixture()
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"privacyPolicyVersion":"p1","privacyAccepted":true,"termsVersion":"t1","termsAccepted":true}`))
			if tc.method == "GET" {
				r = httptest.NewRequest(tc.method, tc.path, nil)
			}
			r.Host = tc.host
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-Firebase-AppCheck", "synthetic")
			for _, origin := range tc.origins {
				r.Header.Add("Origin", origin)
			}
			for _, cookie := range tc.cookies {
				r.Header.Add("Cookie", cookie)
			}
			r.Header.Set("X-Forwarded-Host", "mypage.example.test")
			r.Header.Set("X-Forwarded-Proto", "https")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 400 || s.reads != 0 || v.appCalls != 0 || v.tokenCalls != 0 || *reads != 0 {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if w.Header().Get("Set-Cookie") != "" {
				t.Fatal("rejected request mutated transaction cookie")
			}
		})
	}
	h, _, _, _ := boundaryFixture()
	h.PublicOrigin = ""
	if w := boundaryRequest(h, "GET", "/api/mypage", ""); w.Code != 503 {
		t.Fatal("missing fixed origin did not fail closed")
	}
}

func TestHTTPIPLimitBoundsRejectedVerificationAndIgnoresForwardedIP(t *testing.T) {
	for _, appFailure := range []bool{true, false} {
		h, s, v, reads := boundaryFixture()
		if appFailure {
			v.appErr = errors.New("synthetic rejection")
		} else {
			v.tokenErr = errors.New("synthetic rejection")
		}
		for attempt := 0; attempt < 25; attempt++ {
			r := httptest.NewRequest("GET", "/api/mypage", nil)
			r.Host = "mypage.example.test"
			r.Header.Set("X-Firebase-AppCheck", "synthetic")
			r.Header.Set("Authorization", "Bearer synthetic")
			r.Header.Set("X-Forwarded-For", strings.Repeat("x", attempt+1))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if attempt >= 20 && (w.Code != 429 || w.Header().Get("Retry-After") == "") {
				t.Fatalf("attempt=%d status=%d", attempt, w.Code)
			}
		}
		if v.appCalls != 20 || s.reads != 0 || *reads != 0 {
			t.Fatalf("app=%d account=%d snapshot=%d", v.appCalls, s.reads, *reads)
		}
		wantTokenCalls := 20
		if appFailure {
			wantTokenCalls = 0
		}
		if v.tokenCalls != wantTokenCalls {
			t.Fatal("ID-token verifier escaped IP limit")
		}
	}
}

func TestHostingRewriteRequiresPinnedBackendAndSingleFixedForwardedOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, backend, host, method, origin string
		hosts, protocols                    []string
		want                               int
	}{
		{"forwarded-get", "mypage-api-observed-et.a.run.app", "mypage-api-observed-et.a.run.app", "GET", "", []string{"mypage.example.test"}, []string{"https"}, 403},
		{"forwarded-post", "mypage-api-observed-et.a.run.app", "mypage-api-observed-et.a.run.app", "POST", "https://mypage.example.test", []string{"mypage.example.test"}, []string{"https"}, 403},
		{"off-by-default", "", "mypage-api-observed-et.a.run.app", "GET", "", []string{"mypage.example.test"}, []string{"https"}, 400},
		{"other-backend", "mypage-api-observed-et.a.run.app", "other.a.run.app", "GET", "", []string{"mypage.example.test"}, []string{"https"}, 400},
		{"direct-backend", "mypage-api-observed-et.a.run.app", "mypage-api-observed-et.a.run.app", "GET", "", nil, []string{"https"}, 400},
		{"foreign-host", "mypage-api-observed-et.a.run.app", "mypage-api-observed-et.a.run.app", "GET", "", []string{"other.example.test"}, []string{"https"}, 400},
		{"duplicate-host", "mypage-api-observed-et.a.run.app", "mypage-api-observed-et.a.run.app", "GET", "", []string{"mypage.example.test", "mypage.example.test"}, []string{"https"}, 400},
		{"host-list", "mypage-api-observed-et.a.run.app", "mypage-api-observed-et.a.run.app", "GET", "", []string{"mypage.example.test,other.example.test"}, []string{"https"}, 400},
		{"http-forwarded", "mypage-api-observed-et.a.run.app", "mypage-api-observed-et.a.run.app", "GET", "", []string{"mypage.example.test"}, []string{"http"}, 400},
		{"duplicate-proto", "mypage-api-observed-et.a.run.app", "mypage-api-observed-et.a.run.app", "GET", "", []string{"mypage.example.test"}, []string{"https", "https"}, 400},
		{"foreign-origin", "mypage-api-observed-et.a.run.app", "mypage-api-observed-et.a.run.app", "POST", "https://other.example.test", []string{"mypage.example.test"}, []string{"https"}, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, store, verifier, reads := boundaryFixture()
			h.HostingBackendHost = tc.backend
			path := "/api/mypage"
			body := ""
			if tc.method == "POST" {
				path, body = "/api/auth/youtube/start", `{}`
			}
			r := httptest.NewRequest(tc.method, path, strings.NewReader(body))
			r.Host = tc.host
			r.Header.Set("Content-Type", "application/json")
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			for _, host := range tc.hosts {
				r.Header.Add("X-Forwarded-Host", host)
			}
			for _, proto := range tc.protocols {
				r.Header.Add("X-Forwarded-Proto", proto)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want || store.reads != 0 || verifier.appCalls != 0 || verifier.tokenCalls != 0 || *reads != 0 {
				t.Fatalf("status=%d expected=%d", w.Code, tc.want)
			}
			if tc.want == 403 && !strings.Contains(w.Body.String(), "APP_CHECK_REQUIRED") {
				t.Fatal("routing metadata bypassed App Check")
			}
		})
	}
}
