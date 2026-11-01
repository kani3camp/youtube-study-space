package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSplitDeletionPhasesKeepsRawMarkersSeparate(t *testing.T) {
	t.Parallel()

	inventory := bucketInventory{Prefixes: []prefixInventory{
		{
			Prefix:          "raw/",
			ContainsRawChat: true,
			Objects: []objectRef{
				{Name: "raw/all_namespaces/kind_users/output-0"},
				{Name: "raw/all_namespaces/kind_live-chat-history/output-0"},
				{Name: "raw/snapshot.overall_export_metadata"},
			},
		},
		{
			Prefix: "clean/",
			Objects: []objectRef{
				{Name: "clean/all_namespaces/kind_users/output-0"},
			},
		},
	}}

	nonRaw, rawMarkers := splitDeletionPhases(inventory)
	if len(nonRaw) != 2 {
		t.Fatalf("len(nonRaw) = %d, want 2", len(nonRaw))
	}
	if len(rawMarkers) != 1 {
		t.Fatalf("len(rawMarkers) = %d, want 1", len(rawMarkers))
	}
	if !strings.Contains(rawMarkers[0].Name, rawLiveChatPathMarker) {
		t.Fatalf("raw marker = %q, want live-chat marker", rawMarkers[0].Name)
	}
}

func TestDeleteObjectsBoundedCapsConcurrency(t *testing.T) {
	t.Parallel()

	const concurrency = 4
	objects := make([]objectRef, 24)
	for i := range objects {
		objects[i] = objectRef{Name: "object"}
	}

	var inFlight atomic.Int64
	var maxInFlight atomic.Int64
	deleteObject := func(context.Context, objectRef) error {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			maximum := maxInFlight.Load()
			if current <= maximum || maxInFlight.CompareAndSwap(maximum, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	summary, err := deleteObjectsBounded(
		context.Background(),
		"test",
		objects,
		concurrency,
		time.Hour,
		&bytes.Buffer{},
		deleteObject,
	)
	if err != nil {
		t.Fatalf("deleteObjectsBounded() error = %v", err)
	}
	if summary.Deleted != int64(len(objects)) || summary.Failed != 0 || summary.Skipped != 0 {
		t.Fatalf("summary = %#v", summary)
	}
	if got := maxInFlight.Load(); got > concurrency {
		t.Fatalf("max in-flight = %d, want <= %d", got, concurrency)
	}
	if got := maxInFlight.Load(); got < 2 {
		t.Fatalf("max in-flight = %d, want concurrent deletion", got)
	}
}

func TestDeleteObjectsBoundedReportsPartialFailures(t *testing.T) {
	t.Parallel()

	objects := []objectRef{{Name: "ok-1"}, {Name: "fail"}, {Name: "ok-2"}}
	deleteObject := func(_ context.Context, object objectRef) error {
		if object.Name == "fail" {
			return errors.New("boom")
		}
		return nil
	}

	summary, err := deleteObjectsBounded(
		context.Background(),
		"non-raw",
		objects,
		2,
		time.Hour,
		&bytes.Buffer{},
		deleteObject,
	)
	if err == nil {
		t.Fatal("deleteObjectsBounded() error = nil, want failure")
	}
	if summary.Deleted != 2 || summary.Failed != 1 || summary.Skipped != 0 {
		t.Fatalf("summary = %#v, want deleted=2 failed=1 skipped=0", summary)
	}
	if !strings.Contains(err.Error(), "rerun preview before retrying") {
		t.Fatalf("error = %q, want rerun-preview guidance", err)
	}
}

func TestDeleteObjectsBoundedRespectsContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	objects := []objectRef{{Name: "one"}, {Name: "two"}}
	var calls atomic.Int64
	deleteObject := func(context.Context, objectRef) error {
		calls.Add(1)
		return nil
	}

	summary, err := deleteObjectsBounded(
		ctx,
		"test",
		objects,
		2,
		time.Hour,
		&bytes.Buffer{},
		deleteObject,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("delete calls = %d, want 0", calls.Load())
	}
	if summary.Skipped != int64(len(objects)) {
		t.Fatalf("summary = %#v, want all objects skipped", summary)
	}
}

func TestDeleteObjectsBoundedWritesProgress(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	objects := []objectRef{{Name: "one"}, {Name: "two"}}
	summary, err := deleteObjectsBounded(
		context.Background(),
		"raw-marker",
		objects,
		1,
		time.Hour,
		&output,
		func(context.Context, objectRef) error { return nil },
	)
	if err != nil {
		t.Fatalf("deleteObjectsBounded() error = %v", err)
	}
	if summary.Deleted != 2 {
		t.Fatalf("summary = %#v", summary)
	}
	got := output.String()
	if !strings.Contains(got, "phase=raw-marker started") || !strings.Contains(got, "phase=raw-marker finished") {
		t.Fatalf("progress output = %q", got)
	}
}
