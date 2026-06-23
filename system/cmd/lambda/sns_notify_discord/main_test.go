package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBuildDiscordNotificationTruncatesMessageUTF8Safely(t *testing.T) {
	subject := "subject"
	message := strings.Repeat("勉強🚀", 700)

	notify := buildDiscordNotification(subject, message)

	if len([]rune(notify)) > maxDiscordMessageLength {
		t.Fatalf("expected notification to fit length limit, got %d", len([]rune(notify)))
	}
	if !utf8.ValidString(notify) {
		t.Fatalf("expected valid UTF-8 notification, got %q", notify)
	}
	if !strings.HasSuffix(notify, truncatedSuffix) {
		t.Fatalf("expected truncated suffix, got %q", notify)
	}
}

func TestBuildDiscordNotificationAccountsForSubjectLength(t *testing.T) {
	subject := strings.Repeat("件名", 600)
	message := strings.Repeat("本文", 700)

	notify := buildDiscordNotification(subject, message)

	if len([]rune(notify)) > maxDiscordMessageLength {
		t.Fatalf("expected notification to fit length limit, got %d", len([]rune(notify)))
	}
	if !utf8.ValidString(notify) {
		t.Fatalf("expected valid UTF-8 notification, got %q", notify)
	}
	if !strings.Contains(notify, "件名") {
		t.Fatalf("expected notification to retain subject context, got %q", notify)
	}
}
