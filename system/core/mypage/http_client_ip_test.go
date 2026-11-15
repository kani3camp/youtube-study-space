package mypage

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// clientIPRequestは、転送ヘッダーとRemoteAddrだけを変えた保護対象リクエストを送る。
// App Checkを失敗させるため、IP制限を通過すれば403、制限に達すれば429になる。
func clientIPRequest(h *HTTPHandler, method, path string, forwarded []string) *httptest.ResponseRecorder {
	body := ""
	if method == http.MethodPost {
		body = `{}`
	}
	r := newBoundaryRequest(h, method, path, body)
	r.RemoteAddr = "192.0.2.10:4321"
	for _, value := range forwarded {
		r.Header.Add("X-Forwarded-For", value)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestHTTPIPLimitKeysOnTrustedForwardedHop(t *testing.T) {
	for _, tc := range []struct {
		name            string
		hops            int
		exhaust, probe  []string
		sharesBucketHit bool
	}{
		{"default-ignores-forwarded", 0, []string{"198.51.100.7, 35.191.0.1"}, []string{"198.51.100.8, 35.191.0.1"}, true},
		{"spoofed-leftmost-does-not-change-key", 2, []string{"203.0.113.9, 198.51.100.7, 35.191.0.1"}, []string{"10.0.0.1, 203.0.113.10, 198.51.100.7, 35.191.0.2"}, true},
		{"spoof-on-separate-header-line", 2, []string{"203.0.113.9", "198.51.100.7, 35.191.0.1"}, []string{"198.51.100.7", "35.191.0.1"}, true},
		{"different-clients-behind-same-proxy", 2, []string{"198.51.100.7, 35.191.0.1"}, []string{"198.51.100.8, 35.191.0.1"}, false},
		{"single-trusted-hop-uses-rightmost", 1, []string{"198.51.100.7, 35.191.0.1"}, []string{"198.51.100.8, 35.191.0.1"}, true},
		{"single-trusted-hop-separates-rightmost", 1, []string{"198.51.100.7, 35.191.0.1"}, []string{"198.51.100.7, 35.191.0.2"}, false},
		{"missing-header-falls-back", 2, nil, nil, true},
		{"forwarded-client-separate-from-fallback", 2, nil, []string{"198.51.100.7, 35.191.0.1"}, false},
		{"too-few-entries-falls-back", 2, nil, []string{"35.191.0.1"}, true},
		{"malformed-entry-falls-back", 2, nil, []string{"unknown, 35.191.0.1"}, true},
		{"entry-with-port-falls-back", 2, nil, []string{"198.51.100.7:443, 35.191.0.1"}, true},
		{"entry-with-zone-falls-back", 2, nil, []string{"fe80::1%eth0, 35.191.0.1"}, true},
		{"empty-entry-falls-back", 2, nil, []string{", 35.191.0.1"}, true},
		{"ipv6-same-64-shares-bucket", 2, []string{"2001:db8:1:2::a, 35.191.0.1"}, []string{"2001:db8:1:2:ffff::b, 35.191.0.1"}, true},
		{"ipv6-different-64-separates", 2, []string{"2001:db8:1:2::a, 35.191.0.1"}, []string{"2001:db8:1:3::a, 35.191.0.1"}, false},
		{"ipv4-mapped-ipv6-matches-ipv4", 2, []string{"198.51.100.7, 35.191.0.1"}, []string{"::ffff:198.51.100.7, 35.191.0.1"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, v, _ := boundaryFixture()
			h.TrustedProxyHops = tc.hops
			v.appErr = errors.New("synthetic rejection")
			// IP単位の大まかな制限はburst20。21回目以降は同じキーなら429になる。
			for attempt := 0; attempt < 20; attempt++ {
				if w := clientIPRequest(h, "GET", "/api/mypage", tc.exhaust); w.Code != 403 {
					t.Fatalf("attempt=%d status=%d", attempt, w.Code)
				}
			}
			if w := clientIPRequest(h, "GET", "/api/mypage", tc.exhaust); w.Code != 429 {
				t.Fatalf("exhausted key status=%d", w.Code)
			}
			w := clientIPRequest(h, "GET", "/api/mypage", tc.probe)
			if got := w.Code == 429; got != tc.sharesBucketHit {
				t.Fatalf("probe status=%d sharesBucket=%v", w.Code, tc.sharesBucketHit)
			}
		})
	}
}

func TestHTTPLoginStartLimitIsPerForwardedClient(t *testing.T) {
	h, _, v, _ := boundaryFixture()
	h.TrustedProxyHops = 2
	v.appErr = errors.New("synthetic rejection")
	first := []string{"203.0.113.1, 198.51.100.7, 35.191.0.1"}
	for attempt := 0; attempt < 5; attempt++ {
		if w := clientIPRequest(h, "POST", "/api/auth/youtube/start", first); w.Code != 403 {
			t.Fatalf("attempt=%d status=%d", attempt, w.Code)
		}
	}
	// 同じ利用者は左端を変えても上限を回避できない。
	if w := clientIPRequest(h, "POST", "/api/auth/youtube/start", []string{"203.0.113.2, 198.51.100.7, 35.191.0.1"}); w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("spoofed start status=%d", w.Code)
	}
	// 同じプロキシを経由する別の利用者は、他人の開始回数で429にならない。
	if w := clientIPRequest(h, "POST", "/api/auth/youtube/start", []string{"198.51.100.8, 35.191.0.1"}); w.Code != 403 {
		t.Fatalf("other client status=%d", w.Code)
	}
}
