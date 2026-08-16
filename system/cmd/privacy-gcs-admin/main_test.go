package main

import (
	"strings"
	"testing"
	"time"
)

func TestBuildPreviewReadyWhenRawSnapshotsAreOldAndCleanSnapshotsFollow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 14, 14, 15, 0, 0, time.UTC)
	oldRaw := time.Date(2026, time.June, 23, 15, 0, 0, 0, time.UTC)

	prefixes := []prefixInventory{
		{
			Prefix:          "2026-06-23T15:00:00_1/",
			ObjectCount:     12,
			Bytes:           120,
			LatestCreatedAt: oldRaw,
			ContainsRawChat: true,
		},
	}
	for day := 1; day <= minimumCleanSnapshotsAfterRaw; day++ {
		prefixes = append(prefixes, prefixInventory{
			Prefix:          "clean-" + time.Date(2026, time.August, day+6, 15, 0, 0, 0, time.UTC).Format("2006-01-02") + "/",
			ObjectCount:     9,
			Bytes:           90,
			LatestCreatedAt: time.Date(2026, time.August, day+6, 15, 0, 0, 0, time.UTC),
		})
	}

	got := buildPreview(now, "backup-bucket", bucketInventory{Prefixes: prefixes})

	if !got.ReadyToApply {
		t.Fatalf("ReadyToApply = false, safety checks = %#v", got.SafetyChecks)
	}
	if got.RawChatSnapshotPrefixes != 1 || got.ExpiredRawChatSnapshotPrefixes != 1 {
		t.Fatalf("raw prefix counts = %d/%d, want 1/1", got.RawChatSnapshotPrefixes, got.ExpiredRawChatSnapshotPrefixes)
	}
	if got.CandidateObjectCount != 12 || got.CandidateBytes != 120 {
		t.Fatalf("candidate objects/bytes = %d/%d, want 12/120", got.CandidateObjectCount, got.CandidateBytes)
	}
	if got.CleanSnapshotsAfterNewestRaw != minimumCleanSnapshotsAfterRaw {
		t.Fatalf("CleanSnapshotsAfterNewestRaw = %d, want %d", got.CleanSnapshotsAfterNewestRaw, minimumCleanSnapshotsAfterRaw)
	}
	if got.RequiredConfirmation == "" {
		t.Fatal("RequiredConfirmation is empty")
	}
}

func TestBuildPreviewRefusesWhenRecentSnapshotStillContainsRawChat(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 14, 14, 15, 0, 0, time.UTC)
	inventory := bucketInventory{Prefixes: []prefixInventory{
		{
			Prefix:          "2026-08-13T15:00:00_1/",
			ObjectCount:     9,
			LatestCreatedAt: now.Add(-23 * time.Hour),
			ContainsRawChat: true,
		},
	}}

	got := buildPreview(now, "backup-bucket", inventory)
	if got.ReadyToApply {
		t.Fatalf("ReadyToApply = true, want false")
	}
	if got.ExpiredRawChatSnapshotPrefixes != 0 {
		t.Fatalf("ExpiredRawChatSnapshotPrefixes = %d, want 0", got.ExpiredRawChatSnapshotPrefixes)
	}
	if got.RequiredConfirmation != "" {
		t.Fatalf("RequiredConfirmation = %q, want empty", got.RequiredConfirmation)
	}
}

func TestBuildPreviewRefusesWhenVersioningEnabled(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 14, 14, 15, 0, 0, time.UTC)
	prefixes := []prefixInventory{{
		Prefix:          "raw/",
		ObjectCount:     3,
		LatestCreatedAt: now.AddDate(0, 0, -60),
		ContainsRawChat: true,
	}}
	for day := 1; day <= minimumCleanSnapshotsAfterRaw; day++ {
		prefixes = append(prefixes, prefixInventory{
			Prefix:          "clean-" + time.Date(2026, time.August, day+6, 15, 0, 0, 0, time.UTC).Format("2006-01-02") + "/",
			LatestCreatedAt: time.Date(2026, time.August, day+6, 15, 0, 0, 0, time.UTC),
		})
	}

	got := buildPreview(now, "backup-bucket", bucketInventory{
		Prefixes:          prefixes,
		VersioningEnabled: true,
	})
	if got.ReadyToApply {
		t.Fatal("ReadyToApply = true with versioning enabled")
	}
}

func TestDeletionOrderKeepsRawObjectsUntilLast(t *testing.T) {
	t.Parallel()

	objects := []objectRef{
		{Name: "snapshot/all_namespaces/kind_live-chat-history/output-0"},
		{Name: "snapshot/all_namespaces/kind_users/output-0"},
		{Name: "snapshot/snapshot.overall_export_metadata"},
		{Name: "snapshot/all_namespaces/kind_live-chat-history/all_namespaces_kind_live-chat-history.export_metadata"},
	}

	ordered := deletionOrder(objects)
	seenRaw := false
	for _, object := range ordered {
		isRaw := strings.Contains(object.Name, rawLiveChatPathMarker)
		if isRaw {
			seenRaw = true
			continue
		}
		if seenRaw {
			t.Fatalf("non-raw object %q appears after raw object", object.Name)
		}
	}
}

func TestTopLevelPrefix(t *testing.T) {
	t.Parallel()

	if got := topLevelPrefix("2026-08-13T15:00:06_83201/all_namespaces/kind_users/output-0"); got != "2026-08-13T15:00:06_83201/" {
		t.Fatalf("topLevelPrefix() = %q", got)
	}
}
