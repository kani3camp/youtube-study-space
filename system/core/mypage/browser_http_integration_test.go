//go:build integration

package mypage

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"

	"app.modules/core/repository"
	"app.modules/core/serviceaccess"
	"app.modules/internal/integrationtest"
)

type browserHTTPFirebase struct {
	fakeFirebaseClient
	verifications atomic.Int32
}

func (f *browserHTTPFirebase) VerifyIDToken(ctx context.Context, raw string) (*auth.Token, error) {
	f.verifications.Add(1)
	if raw != "synthetic-browser-id-token" {
		return nil, errors.New("synthetic token rejected")
	}
	return f.fakeFirebaseClient.VerifyIDToken(ctx, raw)
}

type browserHTTPStatus struct {
	http.ResponseWriter
	status int
}

func (w *browserHTTPStatus) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

type browserHTTPEvent struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	RetryAfter string `json:"retryAfter,omitempty"`
	Verified   int32  `json:"tokenVerifications"`
	NoStore    bool   `json:"noStore"`
	NoReferrer bool   `json:"noReferrer"`
	RequestID  bool   `json:"requestIDPresent"`
}

// Opt-in only: ordinary integration CI compiles this seam but does not launch
// a browser/Vite. No product entrypoint can construct the fake dependencies.
func TestBrowserHTTPDemo(t *testing.T) {
	if os.Getenv("MYPAGE_BROWSER_HTTP_QA") != "1" {
		t.Skip("opt-in localhost browser HTTP acceptance")
	}
	integrationtest.RequireFirestoreEmulator(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	vite, err := url.Parse(os.Getenv("MYPAGE_QA_VITE"))
	require.NoError(t, err)
	require.Equal(t, "http", vite.Scheme)
	require.Equal(t, "127.0.0.1:18081", vite.Host)
	require.Empty(t, vite.Path)
	output := os.Getenv("MYPAGE_QA_OUTPUT")
	require.True(t, filepath.IsAbs(output))
	require.NoError(t, os.MkdirAll(output, 0o700))
	project := "demo-youtube-study-space-ci"
	uid := "UC" + strings.Repeat("0", 21) + "1"
	client, err := firestore.NewClient(ctx, project, option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	refs := []*firestore.DocumentRef{
		client.Collection("users").Doc(uid),
		client.Collection("seats").Doc(uid),
		client.Collection("work-segments").Doc(uid),
		client.Collection("web-accounts").Doc(uid),
		client.Collection(serviceaccess.Collection).Doc(uid),
	}
	t.Cleanup(func() {
		for _, ref := range refs {
			_, deleteErr := ref.Delete(context.Background())
			require.NoError(t, deleteErr)
		}
	})
	now := time.Now().UTC().Truncate(time.Second)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	appToken := syntheticAppCheck(t, key, "synthetic-key", syntheticAppClaims(now), "JWT")
	jwks := publicJWKS(t, key, "synthetic-key")
	sdk := &browserHTTPFirebase{fakeFirebaseClient: fakeFirebaseClient{token: &auth.Token{
		UID: uid, Subject: uid, Issuer: "https://securetoken.google.com/" + project,
		Audience: project, Expires: now.Add(time.Hour).Unix(), IssuedAt: now.Unix(), AuthTime: now.Unix(),
		Firebase: auth.FirebaseInfo{SignInProvider: "custom"},
	}}}
	provider := &fakeOAuth{channels: []Channel{{ID: uid, DisplayName: "Synthetic HTTP channel"}}}
	proxy := httputil.NewSingleHostReverseProxy(vite)
	var mu sync.Mutex
	var handler *HTTPHandler
	var events []browserHTTPEvent
	var protected []string
	var serverLog bytes.Buffer
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/_qa/") {
			if r.URL.Path == "/" || r.URL.Path == "/login" || r.URL.Path == "/login/channel-confirm" || strings.HasPrefix(r.URL.Path, "/mypage") {
				r.URL.Path = "/http-qa.html"
			}
			proxy.ServeHTTP(w, r)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/_qa/bootstrap":
			w.Header().Set("Cache-Control", "no-store")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"uid": uid, "appCheck": appToken}))
			return
		case "/_qa/scenario":
			name := r.URL.Query().Get("name")
			switch name {
			case "unknown", "complete", "gap":
			default:
				http.Error(w, "invalid synthetic scenario", 400)
				return
			}
			for _, ref := range refs {
				_, deleteErr := ref.Delete(ctx)
				require.NoError(t, deleteErr)
			}
			_, err = refs[0].Set(ctx, repository.UserDoc{TotalStudySec: 600})
			require.NoError(t, err)
			_, err = refs[2].Set(ctx, repository.WorkSegmentDoc{
				UserID: uid, SegmentType: repository.WorkState,
				StartedAt: now.Add(-10 * time.Minute), EndedAt: now, DurationSec: 600,
			})
			require.NoError(t, err)
			var coverage []Interval
			if name != "unknown" {
				coverage = []Interval{{Start: now.AddDate(0, 0, -30), End: now.Add(time.Hour)}}
			}
			if name == "gap" {
				gap := StatisticsWindows(now).Recent.AddDate(0, 0, 2).Add(time.Hour)
				coverage = []Interval{{Start: coverage[0].Start, End: gap}, {Start: gap.Add(time.Nanosecond), End: coverage[0].End}}
			}
			handler, err = NewMyPageServer(ServerConfig{
				Environment: "development", ProjectID: project, ProjectNumber: "123456789",
				WebAppID: "1:123456789:web:synthetic", PublicOrigin: "https://" + r.Host,
				Policy: Policy{Privacy: "synthetic-p", Terms: "synthetic-t"}, Coverage: coverage,
			}, ServerDependencies{
				Firestore: client, Firebase: sdk, OAuth: provider,
				AppCheckHTTP: &http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) {
					return providerResponse(200, jwks), nil
				})},
			})
			require.NoError(t, err)
			require.Nil(t, handler.Intake)
			w.WriteHeader(204)
			return
		case "/_qa/restrict":
			reason := serviceaccess.Moderation
			change := serviceaccess.Change{Reason: reason, Active: true, ReasonCode: "MODERATION", Reference: digest("synthetic-browser-restriction")}
			if r.URL.Query().Get("kind") == "deletion" {
				change = serviceaccess.Change{Reason: serviceaccess.PrivacyDeletion, Active: true, ReasonCode: "PRIVACY_DELETION", Reference: digest("synthetic-browser-delete")}
			}
			_, err = (&serviceaccess.FirestoreStore{Client: client}).Change(ctx, uid, change, time.Now())
			require.NoError(t, err)
			w.WriteHeader(204)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			require.NotNil(t, handler)
			if r.URL.Path == "/api/auth/youtube/callback" {
				protected = append(protected, r.URL.Query().Get("state"), r.URL.Query().Get("code"))
			}
			if c := transactionID(r); c != "" {
				protected = append(protected, c)
				t.Cleanup(func() {
					_, deleteErr := client.Collection("oauth-transactions").Doc(c).Delete(context.Background())
					require.NoError(t, deleteErr)
				})
			}
			before := sdk.verifications.Load()
			status := &browserHTTPStatus{ResponseWriter: w, status: 200}
			handler.ServeHTTP(status, r)
			events = append(events, browserHTTPEvent{
				Method: r.Method, Path: r.URL.Path, Status: status.status,
				RetryAfter: w.Header().Get("Retry-After"), Verified: sdk.verifications.Load() - before,
				NoStore:    w.Header().Get("Cache-Control") == "private, no-store",
				NoReferrer: w.Header().Get("Referrer-Policy") == "no-referrer",
				RequestID:  w.Header().Get("X-Request-Id") != "",
			})
			return
		}
		http.NotFound(w, r)
	})
	server := httptest.NewUnstartedServer(api)
	server.Config.ErrorLog = log.New(&serverLog, "", 0)
	proxy.ErrorLog = server.Config.ErrorLog
	server.StartTLS()
	t.Cleanup(server.Close)
	foreign := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, writeErr := fmt.Fprintf(w, "<form method='POST' action='%s/api/auth/session/complete'><button>foreign origin</button></form>", server.URL)
		require.NoError(t, writeErr)
	}))
	t.Cleanup(foreign.Close)
	script, err := filepath.Abs("../../../mypage/scripts/http-qa.py")
	require.NoError(t, err)
	fingerprint := sha256.Sum256(server.Certificate().RawSubjectPublicKeyInfo)
	command := exec.CommandContext(ctx, "python3", script, "--base", server.URL, "--foreign", foreign.URL, "--output", output,
		"--trusted-spki", base64.StdEncoding.EncodeToString(fingerprint[:]))
	result, runErr := command.CombinedOutput()
	server.Close() // Drain HTTP writers before inspecting their private log buffer.
	if runErr != nil {
		mu.Lock()
		failedEvidence, encodeErr := json.MarshalIndent(events, "", "  ")
		require.NoError(t, encodeErr)
		require.NoError(t, os.WriteFile(filepath.Join(output, "backend-failure.json"), failedEvidence, 0o600))
		mu.Unlock()
		// A browser traceback can include a callback URL. Never print it,
		// including on failed acceptance; Python writes fixed diagnostics.
		t.Fatalf("synthetic browser HTTP acceptance failed (%T); see fixed failure report", runErr)
	}
	mu.Lock()
	defer mu.Unlock()
	browserJSON, err := os.ReadFile(filepath.Join(output, "browser-report.json"))
	require.NoError(t, err)
	var browserReport struct {
		Responses []browserHTTPEvent `json:"responses"`
	}
	require.NoError(t, json.Unmarshal(browserJSON, &browserReport))
	observed := func(values []browserHTTPEvent) map[string]int {
		counts := make(map[string]int)
		for _, value := range values {
			counts[fmt.Sprintf("%s %s %d", value.Method, value.Path, value.Status)]++
		}
		return counts
	}
	require.Equal(t, observed(events), observed(browserReport.Responses), "browser/API observations must be the same real handler calls")
	for _, event := range events {
		require.True(t, event.NoStore && event.NoReferrer && event.RequestID)
	}
	evidence, err := json.MarshalIndent(events, "", "  ")
	require.NoError(t, err)
	protected = append(protected, appToken, "synthetic-browser-id-token", "synthetic-custom-token", "synthetic-browser-code")
	for _, value := range protected {
		if value != "" {
			require.False(t, strings.Contains(string(evidence)+string(browserJSON)+serverLog.String()+string(result), value), "protected marker appeared in recorded output")
		}
	}
	require.NoError(t, os.WriteFile(filepath.Join(output, "backend-report.json"), evidence, 0o600))
	t.Logf("PASS: browser HTTP acceptance; %d real handler calls; callback/token values absent from recorded output", len(events))
}
