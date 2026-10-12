package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckConfigurationIsOfflineAndNeverClaimsInfrastructureOrLiveReadiness(t *testing.T) {
	// Every validator constructor must remain lazy: even a future accidental
	// request through the default transport fails this command's contract.
	transport := http.DefaultTransport
	http.DefaultTransport = rejectingTransport{t: t}
	t.Cleanup(func() { http.DefaultTransport = transport })
	for _, tc := range []struct {
		name, readiness, declaration, environment, region, port, metadataKey string
	}{
		{"before infrastructure", "", "pending", "development", "asia-southeast2", "", ""},
		{"unready declaration", "false", "pending", "development", "asia-southeast2", "", ""},
		{"declared infrastructure", "true", "declared", "development", "asia-southeast2", "1", "synthetic-api-key"},
		{"production configuration", "", "pending", "production", "asia-northeast2", "65535", "synthetic-api-key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := syntheticConfiguration()
			values["MYPAGE_INFRASTRUCTURE_READY"] = tc.readiness
			values["MYPAGE_ENVIRONMENT"] = tc.environment
			values["MYPAGE_REGION"] = tc.region
			values["PORT"] = tc.port
			values["MYPAGE_YOUTUBE_API_KEY"] = tc.metadataKey
			get := func(key string) string { return values[key] }
			var stdout, stderr bytes.Buffer
			code := command([]string{"--check-config"}, get, &stdout, &stderr, func() error {
				t.Fatal("check-only command reached server bootstrap")
				return nil
			})
			require.Equal(t, 0, code)
			require.Empty(t, stderr.String())
			require.JSONEq(t, expectedReport("valid", tc.declaration), stdout.String())
			require.Less(t, stdout.Len(), 300)
			for _, value := range []string{values["GOOGLE_CLOUD_PROJECT"], values["MYPAGE_SIGNER_EMAIL"], values["MYPAGE_OAUTH_CLIENT_ID"], values["MYPAGE_OAUTH_CLIENT_SECRET"], "synthetic-api-key"} {
				require.NotContains(t, stdout.String(), value)
			}
			_, err := configurationFrom(get)
			if tc.readiness == "true" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, "MyPage configuration unavailable")
			}
		})
	}
}

func TestCheckConfigurationRejectsRuntimeMistakesWithoutDisclosureOrBootstrap(t *testing.T) {
	// Construct a synthetic invalid authority without a literal Basic Auth URL.
	passwordSentinel := "SYNTHETIC_PASSWORD_SENTINEL"
	credentialOrigin := (&url.URL{Scheme: "https", Host: "example.invalid", User: url.UserPassword("synthetic-test-user", passwordSentinel), RawQuery: "synthetic-query"}).String()
	for _, tc := range []struct{ name, key, value string }{
		{"Firestore emulator", "FIRESTORE_EMULATOR_HOST", "private-host:8080"},
		{"Auth emulator", "FIREBASE_AUTH_EMULATOR_HOST", "private-host:9099"},
		{"credential path", "GOOGLE_APPLICATION_CREDENTIALS", "/private/secret-key.json"},
		{"environment", "MYPAGE_ENVIRONMENT", "private-invalid-environment"},
		{"region", "MYPAGE_REGION", "asia-northeast2"},
		{"project", "GOOGLE_CLOUD_PROJECT", "private invalid project"},
		{"project number", "MYPAGE_PROJECT_NUMBER", "private-invalid-number"},
		{"app binding", "MYPAGE_WEB_APP_ID", "1:987654321:web:private"},
		{"origin", "MYPAGE_PUBLIC_ORIGIN", credentialOrigin},
		{"signer", "MYPAGE_SIGNER_EMAIL", "private@demo-other.iam.gserviceaccount.com"},
		{"OAuth ID", "MYPAGE_OAUTH_CLIENT_ID", ""},
		{"OAuth secret", "MYPAGE_OAUTH_CLIENT_SECRET", ""},
		{"privacy", "MYPAGE_PRIVACY_VERSION", ""},
		{"terms", "MYPAGE_TERMS_VERSION", ""},
		{"port text", "PORT", "private-invalid-port"},
		{"port zero", "PORT", "0"},
		{"port negative", "PORT", "-1"},
		{"port overflow", "PORT", "65536"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := syntheticConfiguration()
			values["MYPAGE_INFRASTRUCTURE_READY"] = ""
			values[tc.key] = tc.value
			var stdout, stderr bytes.Buffer
			code := command([]string{"--check-config"}, func(key string) string { return values[key] }, &stdout, &stderr, func() error {
				t.Fatal("invalid check-only configuration reached server bootstrap")
				return nil
			})
			require.Equal(t, 1, code)
			require.Empty(t, stderr.String())
			require.JSONEq(t, expectedReport("invalid", "pending"), stdout.String())
			require.NotContains(t, stdout.String(), "private")
			require.NotContains(t, stdout.String(), "synthetic-secret")
			require.NotContains(t, stdout.String(), passwordSentinel)
		})
	}
}

func TestCommandRejectsUnknownOrExtraArgumentsBeforeReadingConfiguration(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--check-config=true"}, {"--unknown-private-secret"}, {"private-secret"}, {"--check-config", "private-secret"}, {"--check-config", "--check-config"}, {"--", "--check-config"}} {
		var stdout, stderr bytes.Buffer
		code := command(args, func(string) string {
			t.Fatal("invalid argument path read configuration")
			return ""
		}, &stdout, &stderr, func() error {
			t.Fatal("invalid argument path reached server bootstrap")
			return nil
		})
		require.Equal(t, 2, code)
		require.Empty(t, stdout.String())
		require.Equal(t, "Usage: mypage-server [--check-config]\n", stderr.String())
	}
}

func TestCheckConfigurationReportFailureDoesNotPrintRawWriterError(t *testing.T) {
	values := syntheticConfiguration()
	var stderr bytes.Buffer
	code := command([]string{"--check-config"}, func(key string) string { return values[key] }, failingWriter{}, &stderr, func() error {
		t.Fatal("report failure reached server bootstrap")
		return nil
	})
	require.Equal(t, 1, code)
	require.Equal(t, "MyPage configuration report unavailable\n", stderr.String())
}

func TestDefaultCommandCallsStartupAndSanitizesFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		var stdout, stderr bytes.Buffer
		calls := 0
		code := command(nil, func(string) string {
			t.Fatal("default dispatch must leave validation to startup")
			return ""
		}, &stdout, &stderr, func() error {
			calls++
			if fail {
				return errors.New("private-bootstrap-secret")
			}
			return nil
		})
		require.Equal(t, 1, calls)
		require.Empty(t, stdout.String())
		if fail {
			require.Equal(t, 1, code)
			require.Equal(t, "MyPage server unavailable\n", stderr.String())
		} else {
			require.Equal(t, 0, code)
			require.Empty(t, stderr.String())
		}
	}
}

func TestDefaultCommandPreservesOnlyExactSafeStartupStages(t *testing.T) {
	for _, stage := range []string{
		"MyPage configuration unavailable",
		"MyPage keyless bootstrap unavailable",
		"MyPage database bootstrap unavailable",
		"MyPage provider unavailable",
		"MyPage metadata provider unavailable",
		"MyPage handler unavailable",
		"MyPage HTTP server unavailable",
	} {
		for _, exact := range []bool{true, false} {
			message := stage
			if !exact {
				message += ": private-dependency-secret"
			}
			var stdout, stderr bytes.Buffer
			code := command(nil, nil, &stdout, &stderr, func() error { return errors.New(message) })
			require.Equal(t, 1, code)
			require.Empty(t, stdout.String())
			if exact {
				require.Equal(t, stage+"\n", stderr.String())
			} else {
				require.Equal(t, "MyPage server unavailable\n", stderr.String())
			}
		}
	}
}

func expectedReport(configuration, declaration string) string {
	return `{"schemaVersion":1,"mode":"check-config","offline":true,"configuration":"` + configuration + `","startupReadinessDeclaration":"` + declaration + `","infrastructureReadiness":"pending","liveE2E":"pending"}`
}

type rejectingTransport struct{ t *testing.T }

func (r rejectingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.t.Fatal("offline validation attempted an HTTP request")
	return nil, errors.New("offline validation attempted an HTTP request")
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("private-writer-secret")
}
