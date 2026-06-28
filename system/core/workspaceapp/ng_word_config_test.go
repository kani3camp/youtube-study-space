package workspaceapp

import "testing"

func TestNGWordConfig_Count(t *testing.T) {
	cfg := NewNGWordConfig(
		[]string{"a", "b"},
		[]string{"c"},
		[]string{"d", "e", "f"},
		[]string{"g"},
	)

	if got, want := cfg.Count(), 7; got != want {
		t.Fatalf("Count() = %d, want %d", got, want)
	}
}

func TestNewNGWordConfig_ClonesSlices(t *testing.T) {
	blockChat := []string{"before"}
	blockChannel := []string{"before"}
	notificationChat := []string{"before"}
	notificationChannel := []string{"before"}

	cfg := NewNGWordConfig(blockChat, blockChannel, notificationChat, notificationChannel)

	blockChat[0] = "after"
	blockChannel[0] = "after"
	notificationChat[0] = "after"
	notificationChannel[0] = "after"

	if got, want := cfg.blockRegexesForChatMessage[0], "before"; got != want {
		t.Fatalf("blockRegexesForChatMessage[0] = %q, want %q", got, want)
	}
	if got, want := cfg.blockRegexesForChannelName[0], "before"; got != want {
		t.Fatalf("blockRegexesForChannelName[0] = %q, want %q", got, want)
	}
	if got, want := cfg.notificationRegexesForChatMessage[0], "before"; got != want {
		t.Fatalf("notificationRegexesForChatMessage[0] = %q, want %q", got, want)
	}
	if got, want := cfg.notificationRegexesForChannelName[0], "before"; got != want {
		t.Fatalf("notificationRegexesForChannelName[0] = %q, want %q", got, want)
	}
}
