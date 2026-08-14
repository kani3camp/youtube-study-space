package workspaceapp

import (
	"testing"
	"time"
)

func TestRawYouTubeDataCutoff(t *testing.T) {
	t.Parallel()

	jst := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, time.August, 14, 21, 30, 45, 0, jst)

	got := rawYouTubeDataCutoff(now)
	want := time.Date(2026, time.July, 15, 21, 30, 45, 0, jst)

	if !got.Equal(want) {
		t.Fatalf("rawYouTubeDataCutoff() = %s, want %s", got, want)
	}
}

func TestRawYouTubeDataRetentionDaysIsThirty(t *testing.T) {
	t.Parallel()

	if rawYouTubeDataRetentionDays != 30 {
		t.Fatalf("rawYouTubeDataRetentionDays = %d, want 30", rawYouTubeDataRetentionDays)
	}
}
