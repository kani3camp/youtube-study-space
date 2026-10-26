package main

import (
	"context"
	"testing"

	"cloud.google.com/go/firestore"
)

func TestBootstrapCheckRejectsBeforeCredentialDiscovery(t *testing.T) {
	base := map[string]string{
		"GOOGLE_CLOUD_PROJECT": "test-youtube-study-space",
		"MYPAGE_ENVIRONMENT":   "development",
	}
	for name, mutate := range map[string]func([]string, map[string]string){
		"arguments": func(args []string, _ map[string]string) { args[0] = "unexpected" },
		"wrong project": func(_ []string, env map[string]string) {
			env["GOOGLE_CLOUD_PROJECT"] = "youtube-study-space"
		},
		"credential file": func(_ []string, env map[string]string) {
			env["GOOGLE_APPLICATION_CREDENTIALS"] = "/tmp/forbidden"
		},
	} {
		t.Run(name, func(t *testing.T) {
			args := []string{""}
			env := map[string]string{}
			for key, value := range base {
				env[key] = value
			}
			mutate(args, env)
			if name != "arguments" {
				args = nil
			}
			connected := false
			err := run(context.Background(), args, func(key string) string { return env[key] }, func(context.Context, string) (*firestore.Client, error) {
				connected = true
				return nil, nil
			})
			if err == nil || connected {
				t.Fatalf("err=%v connected=%v", err, connected)
			}
		})
	}
}
