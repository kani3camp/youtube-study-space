package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func fixture() map[string]string {
	return map[string]string{"GOOGLE_CLOUD_PROJECT": "demo-synthetic-mypage", "MYPAGE_ENVIRONMENT": "development", "MYPAGE_SUPPORT_PURPOSE": "delete", "FIRESTORE_EMULATOR_HOST": "127.0.0.1:8080"}
}

func TestEmulatorOnlyConfigurationRefusesLiveProjectsCredentialsAndExecute(t *testing.T) {
	for _, change := range []map[string]string{{"GOOGLE_CLOUD_PROJECT": "synthetic-live"}, {"MYPAGE_ENVIRONMENT": "production"}, {"GCLOUD_PROJECT": "demo-other"}, {"FIRESTORE_EMULATOR_HOST": "example.invalid:8080"}, {"FIRESTORE_EMULATOR_HOST": "localhost:8080"}, {"FIRESTORE_EMULATOR_HOST": "127.0.0.1:0"}, {"GOOGLE_APPLICATION_CREDENTIALS": "synthetic-key.json"}, {"FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:9099"}, {"MYPAGE_SUPPORT_PURPOSE": "execute"}} {
		config := fixture()
		for k, v := range change {
			config[k] = v
		}
		get := func(k string) string { return config[k] }
		_, err := configurationFrom(get, nil)
		require.Error(t, err)
		var output bytes.Buffer
		err = run(context.Background(), get, nil, strings.NewReader("private"), &output)
		require.Error(t, err)
		require.Empty(t, output.String())
	}
	config := fixture()
	get := func(k string) string { return config[k] }
	_, err := configurationFrom(get, []string{"--execute"})
	require.Error(t, err)
	_, err = configurationFrom(get, nil)
	require.NoError(t, err)
}

func TestReceiptReferenceUsesBoundedStdinAndDoesNotEchoPrivateInput(t *testing.T) {
	ref := strings.Repeat("a", 64)
	value, err := readRequestRef(strings.NewReader(ref + "\n"))
	require.NoError(t, err)
	require.Equal(t, ref, value)
	for _, input := range []string{"", ref + "\nprivate", strings.Repeat("g", 64), " " + ref, strings.Repeat("a", 1000)} {
		_, err := readRequestRef(strings.NewReader(input))
		require.Error(t, err)
		require.NotContains(t, err.Error(), input+"private")
	}
}
