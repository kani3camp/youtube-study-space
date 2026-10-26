package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
)

func operatorArgs() []string {
	return []string{
		"--operation=apply",
		"--authority=GITHUB_ACTIONS",
		"--project=test-youtube-study-space",
		"--environment=development",
		"--state=ACTIVE",
		"--expected-revision=1",
		"--reason-category=PLANNED_MAINTENANCE",
		"--action-ref=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"--repository-id=340900071",
		"--repository-owner-id=54093651",
		"--actor-id=54093651",
		"--triggering-actor-id=54093651",
		"--run-id=123",
		"--run-attempt=1",
		"--git-ref=refs/heads/integration/mypage-canon-20261006",
		"--commit-sha=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"--workflow-ref=kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@refs/heads/integration/mypage-canon-20261006",
	}
}

func TestOperatorRejectsBeforeCredentialDiscovery(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC) }
	baseEnv := map[string]string{"GOOGLE_CLOUD_PROJECT": "test-youtube-study-space", "MYPAGE_ENVIRONMENT": "development", "CLOUD_RUN_EXECUTION": "mypage-maintenance-operator-abcde"}
	for name, mutate := range map[string]func([]string, map[string]string) ([]string, map[string]string){
		"unknown operation": func(args []string, env map[string]string) ([]string, map[string]string) {
			args[0] = "--operation=delete"
			return args, env
		},
		"unknown state": func(args []string, env map[string]string) ([]string, map[string]string) {
			args[4] = "--state=NOTICE"
			return args, env
		},
		"wrong project": func(args []string, env map[string]string) ([]string, map[string]string) {
			env["GOOGLE_CLOUD_PROJECT"] = "youtube-study-space"
			return args, env
		},
		"credential file": func(args []string, env map[string]string) ([]string, map[string]string) {
			env["GOOGLE_APPLICATION_CREDENTIALS"] = "/tmp/forbidden"
			return args, env
		},
		"missing execution": func(args []string, env map[string]string) ([]string, map[string]string) {
			delete(env, "CLOUD_RUN_EXECUTION")
			return args, env
		},
		"rerun": func(args []string, env map[string]string) ([]string, map[string]string) {
			args[13] = "--run-attempt=2"
			return args, env
		},
	} {
		t.Run(name, func(t *testing.T) {
			args := append([]string(nil), operatorArgs()...)
			env := map[string]string{}
			for key, value := range baseEnv {
				env[key] = value
			}
			args, env = mutate(args, env)
			connected := false
			var output bytes.Buffer
			err := run(context.Background(), args, &output, func(key string) string { return env[key] }, now, func(context.Context, string) (*firestore.Client, error) {
				connected = true
				return nil, nil
			})
			if err == nil || connected || output.Len() != 0 {
				t.Fatalf("err=%v connected=%v output=%q", err, connected, output.String())
			}
		})
	}
}
