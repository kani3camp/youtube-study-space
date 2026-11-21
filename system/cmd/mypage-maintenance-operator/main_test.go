package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"app.modules/core/mypage"
)

func TestRunValidatesAndReportsOutcome(t *testing.T) {
	for _, tt := range []struct {
		arg  string
		want bool
	}{{"true", true}, {"false", false}} {
		t.Run(tt.arg, func(t *testing.T) {
			var output bytes.Buffer
			err := run(context.Background(), []string{"--enabled=" + tt.arg}, &output, func(_ context.Context, enabled bool) (mypage.MaintenanceControl, bool, error) {
				if enabled != tt.want {
					t.Fatal("wrong enabled value")
				}
				return mypage.MaintenanceControl{Enabled: enabled, UpdatedAt: time.Unix(1, 0).UTC()}, false, nil
			})
			if err != nil || !strings.Contains(output.String(), `"changed":false`) {
				t.Fatalf("%v %s", err, output.String())
			}
		})
	}
	called := false
	err := run(context.Background(), []string{"--enabled=1"}, &bytes.Buffer{}, func(context.Context, bool) (mypage.MaintenanceControl, bool, error) {
		called = true
		return mypage.MaintenanceControl{}, false, nil
	})
	if err == nil || called {
		t.Fatal("invalid input reached Firestore")
	}
	failure := errors.New("firestore failure")
	err = run(context.Background(), []string{"--enabled=true"}, &bytes.Buffer{}, func(context.Context, bool) (mypage.MaintenanceControl, bool, error) {
		return mypage.MaintenanceControl{}, false, failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("lost error: %v", err)
	}
}
