package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBootstrapRequiresExplicitReadinessAndRejectsUnsafeConfigurationWithoutADC(t *testing.T) {
	values := map[string]string{"MYPAGE_INFRASTRUCTURE_READY": "true", "MYPAGE_ENVIRONMENT": "development", "MYPAGE_REGION": "asia-southeast2", "GOOGLE_CLOUD_PROJECT": "demo-mypage", "MYPAGE_PROJECT_NUMBER": "123456789", "MYPAGE_WEB_APP_ID": "1:123456789:web:synthetic", "MYPAGE_PUBLIC_ORIGIN": "https://example.invalid", "MYPAGE_SIGNER_EMAIL": "mypage-signer@demo-mypage.iam.gserviceaccount.com", "MYPAGE_OAUTH_CLIENT_ID": "synthetic-client", "MYPAGE_OAUTH_CLIENT_SECRET": "synthetic-secret", "MYPAGE_PRIVACY_VERSION": "synthetic-p", "MYPAGE_TERMS_VERSION": "synthetic-t"}
	get := func(key string) string { return values[key] }
	config, err := configurationFrom(get)
	require.NoError(t, err)
	require.Equal(t, "8080", config.port)
	for _, tc := range []struct{ key, value string }{{"MYPAGE_INFRASTRUCTURE_READY", ""}, {"FIRESTORE_EMULATOR_HOST", "127.0.0.1:8080"}, {"FIREBASE_AUTH_EMULATOR_HOST", "127.0.0.1:9099"}, {"GOOGLE_APPLICATION_CREDENTIALS", "synthetic-key-file"}, {"MYPAGE_ENVIRONMENT", "unknown"}, {"MYPAGE_REGION", "asia-northeast2"}, {"MYPAGE_PUBLIC_ORIGIN", "http://example.invalid"}, {"MYPAGE_SIGNER_EMAIL", "synthetic@demo-other.iam.gserviceaccount.com"}, {"MYPAGE_OAUTH_CLIENT_SECRET", ""}, {"MYPAGE_WEB_APP_ID", "1:987654321:web:synthetic"}, {"PORT", "synthetic"}, {"PORT", "65536"}, {"GOOGLE_CLOUD_PROJECT", "invalid project"}} {
		t.Run(tc.key, func(t *testing.T) {
			old := values[tc.key]
			values[tc.key] = tc.value
			defer func() { values[tc.key] = old }()
			_, err := configurationFrom(get)
			require.Equal(t, "MyPage configuration unavailable", err.Error())
		})
	}
}
