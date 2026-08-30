package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const (
	defaultDeleteConcurrency = 16
	deleteProgressInterval    = 5 * time.Second
)

type objectDeleteFunc func(context.Context, objectRef) error

type deletionPhaseSummary struct {
	Phase   string
	Total   int64
	Deleted int64
	Failed  int64
	Skipped int64
}

type deletionResult struct {
	err error
}

func splitDeletionPhases(inventory bucketInventory) (nonRawObjects, rawMarkerObjects []objectRef) {
	for _, prefix := range inventory.Prefixes {
		if !prefix.ContainsRawChat {
			continue
		}
		for _, object := range prefix.Objects {
			if strings.Contains(object.Name, rawLiveChatPathMarker) {
				rawMarkerObjects = append(rawMarkerObjects, object)
				continue
			}
			nonRawObjects = append(nonRawObjects, object)
		}
	}
	return nonRawObjects, rawMarkerObjects
}

func deleteObjectsBounded(
	ctx context.Context,
	phase string,
	objects []objectRef,
	concurrency int,
	progressInterval time.Duration,
	progressWriter io.Writer,
	deleteObject objectDeleteFunc,
) (deletionPhaseSummary, error) {
	summary := deletionPhaseSummary{
		Phase: phase,
		Total: int64(len(objects)),
	}
	if len(objects) == 0 {
		return summary, nil
	}
	if concurrency <= 0 {
		return summary, fmt.Errorf("delete phase %q concurrency must be positive: %d", phase, concurrency)
	}
	if progressInterval <= 0 {
		progressInterval = deleteProgressInterval
	}
	if progressWriter == nil {
		progressWriter = io.Discard
	}
	if deleteObject == nil {
		return summary, fmt.Errorf("delete phase %q delete function is required", phase)
	}

	workerCount := concurrency
	if workerCount > len(objects) {
		workerCount = len(objects)
	}

	jobs := make(chan objectRef)
	results := make(chan deletionResult, workerCount)
	var workers sync.WaitGroup
	workers.Add(workerCount)

	for range workerCount {
		go func() {
			defer workers.Done()
			for object := range jobs {
				if err := ctx.Err(); err != nil {
					return
				}
				results <- deletionResult{err: deleteObject(ctx, object)}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, object := range objects {
			select {
			case jobs <- object:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		workers.Wait()
		close(results)
	}()

	fmt.Fprintf(
		progressWriter,
		"privacy-gcs-admin: delete phase=%s started total=%d concurrency=%d\n",
		phase,
		summary.Total,
		workerCount,
	)

	ticker := time.NewTicker(progressInterval)
	defer ticker.Stop()

	var firstErr error
	for {
		select {
		case result, ok := <-results:
			if !ok {
				summary.Skipped = summary.Total - summary.Deleted - summary.Failed
				fmt.Fprintf(
					progressWriter,
					"privacy-gcs-admin: delete phase=%s finished deleted=%d failed=%d skipped=%d total=%d\n",
					phase,
					summary.Deleted,
					summary.Failed,
					summary.Skipped,
					summary.Total,
				)
				if err := ctx.Err(); err != nil {
					return summary, fmt.Errorf(
						"delete phase %q canceled after deleted=%d failed=%d skipped=%d total=%d; rerun preview before retrying: %w",
						phase,
						summary.Deleted,
						summary.Failed,
						summary.Skipped,
						summary.Total,
						err,
					)
				}
				if summary.Failed > 0 {
					return summary, fmt.Errorf(
						"delete phase %q completed with failures: deleted=%d failed=%d skipped=%d total=%d; rerun preview before retrying: %w",
						phase,
						summary.Deleted,
						summary.Failed,
						summary.Skipped,
						summary.Total,
						firstErr,
					)
				}
				return summary, nil
			}
			if result.err != nil {
				summary.Failed++
				if firstErr == nil {
					firstErr = result.err
				}
				continue
			}
			summary.Deleted++
		case <-ticker.C:
			completed := summary.Deleted + summary.Failed
			fmt.Fprintf(
				progressWriter,
				"privacy-gcs-admin: delete phase=%s progress completed=%d/%d deleted=%d failed=%d\n",
				phase,
				completed,
				summary.Total,
				summary.Deleted,
				summary.Failed,
			)
		}
	}
}
