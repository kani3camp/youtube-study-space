package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

const (
	gcsObjectNameSampleLimit = 20
	gcsPrefixSampleLimit     = 20
)

type gcsPrefixSummary struct {
	Prefix      string `json:"prefix"`
	ObjectCount int64  `json:"object_count"`
}

type gcsObjectInventory struct {
	LiveObjectCount             int64              `json:"live_object_count"`
	LiveBytes                   int64              `json:"live_bytes"`
	LiveObjectsOlderThanCutoff  int64              `json:"live_objects_older_than_cutoff"`
	OldestLiveCreatedAt         string             `json:"oldest_live_created_at,omitempty"`
	NewestLiveCreatedAt         string             `json:"newest_live_created_at,omitempty"`
	SoftDeletedObjectCount      int64              `json:"soft_deleted_object_count"`
	SoftDeletedBytes            int64              `json:"soft_deleted_bytes"`
	OldestSoftDeleteTime        string             `json:"oldest_soft_delete_time,omitempty"`
	LatestSoftDeleteHardDelete  string             `json:"latest_soft_delete_hard_delete_time,omitempty"`
	TopLevelPrefixCount         int                `json:"top_level_prefix_count"`
	TopLevelPrefixSamples       []gcsPrefixSummary `json:"top_level_prefix_samples,omitempty"`
	KnownCollectionObjectCounts map[string]int64   `json:"known_collection_object_counts,omitempty"`
	SampleLiveObjectNames       []string           `json:"sample_live_object_names,omitempty"`
	LiveListError               string             `json:"live_list_error,omitempty"`
	SoftDeletedListError        string             `json:"soft_deleted_list_error,omitempty"`
}

type gcsInventoryAccumulator struct {
	inventory   gcsObjectInventory
	prefixCount map[string]int64
}

func newGCSInventoryAccumulator(knownCollections []string) *gcsInventoryAccumulator {
	knownCollectionCounts := make(map[string]int64, len(knownCollections))
	for _, collection := range knownCollections {
		knownCollectionCounts[collection] = 0
	}
	return &gcsInventoryAccumulator{
		inventory: gcsObjectInventory{
			KnownCollectionObjectCounts: knownCollectionCounts,
		},
		prefixCount: make(map[string]int64),
	}
}

func inspectGCSObjectInventory(
	ctx context.Context,
	bucket *storage.BucketHandle,
	cutoff time.Time,
	knownCollections []string,
) gcsObjectInventory {
	accumulator := newGCSInventoryAccumulator(knownCollections)

	if err := scanGCSObjects(ctx, bucket.Objects(ctx, nil), func(attrs *storage.ObjectAttrs) {
		accumulator.recordLiveObject(attrs, cutoff)
	}); err != nil {
		accumulator.inventory.LiveListError = err.Error()
	}

	softDeletedQuery := &storage.Query{SoftDeleted: true}
	if err := scanGCSObjects(ctx, bucket.Objects(ctx, softDeletedQuery), func(attrs *storage.ObjectAttrs) {
		accumulator.recordSoftDeletedObject(attrs)
	}); err != nil {
		accumulator.inventory.SoftDeletedListError = err.Error()
	}

	accumulator.finalizePrefixes()
	return accumulator.inventory
}

func scanGCSObjects(
	ctx context.Context,
	objectIterator *storage.ObjectIterator,
	record func(*storage.ObjectAttrs),
) error {
	for {
		attrs, err := objectIterator.Next()
		if err == iterator.Done {
			return nil
		}
		if err != nil {
			return fmt.Errorf("list GCS objects: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("list GCS objects: %w", err)
		}
		record(attrs)
	}
}

func (accumulator *gcsInventoryAccumulator) recordLiveObject(attrs *storage.ObjectAttrs, cutoff time.Time) {
	accumulator.inventory.LiveObjectCount++
	accumulator.inventory.LiveBytes += attrs.Size
	if !attrs.Created.IsZero() {
		if attrs.Created.Before(cutoff) {
			accumulator.inventory.LiveObjectsOlderThanCutoff++
		}
		accumulator.inventory.OldestLiveCreatedAt = earlierTimestamp(
			accumulator.inventory.OldestLiveCreatedAt,
			attrs.Created,
		)
		accumulator.inventory.NewestLiveCreatedAt = laterTimestamp(
			accumulator.inventory.NewestLiveCreatedAt,
			attrs.Created,
		)
	}

	prefix := topLevelPrefix(attrs.Name)
	accumulator.prefixCount[prefix]++

	for collection := range accumulator.inventory.KnownCollectionObjectCounts {
		if strings.Contains(attrs.Name, collection) {
			accumulator.inventory.KnownCollectionObjectCounts[collection]++
		}
	}

	if len(accumulator.inventory.SampleLiveObjectNames) < gcsObjectNameSampleLimit {
		accumulator.inventory.SampleLiveObjectNames = append(
			accumulator.inventory.SampleLiveObjectNames,
			attrs.Name,
		)
	}
}

func (accumulator *gcsInventoryAccumulator) recordSoftDeletedObject(attrs *storage.ObjectAttrs) {
	accumulator.inventory.SoftDeletedObjectCount++
	accumulator.inventory.SoftDeletedBytes += attrs.Size
	if !attrs.SoftDeleteTime.IsZero() {
		accumulator.inventory.OldestSoftDeleteTime = earlierTimestamp(
			accumulator.inventory.OldestSoftDeleteTime,
			attrs.SoftDeleteTime,
		)
	}
	if !attrs.HardDeleteTime.IsZero() {
		accumulator.inventory.LatestSoftDeleteHardDelete = laterTimestamp(
			accumulator.inventory.LatestSoftDeleteHardDelete,
			attrs.HardDeleteTime,
		)
	}
}

func (accumulator *gcsInventoryAccumulator) finalizePrefixes() {
	prefixes := make([]string, 0, len(accumulator.prefixCount))
	for prefix := range accumulator.prefixCount {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	accumulator.inventory.TopLevelPrefixCount = len(prefixes)
	for _, prefix := range sampledPrefixNames(prefixes, gcsPrefixSampleLimit) {
		accumulator.inventory.TopLevelPrefixSamples = append(
			accumulator.inventory.TopLevelPrefixSamples,
			gcsPrefixSummary{
				Prefix:      prefix,
				ObjectCount: accumulator.prefixCount[prefix],
			},
		)
	}
}

func sampledPrefixNames(prefixes []string, limit int) []string {
	if limit <= 0 || len(prefixes) == 0 {
		return nil
	}
	if len(prefixes) <= limit {
		return append([]string(nil), prefixes...)
	}

	frontCount := limit / 2
	backCount := limit - frontCount
	samples := make([]string, 0, limit)
	samples = append(samples, prefixes[:frontCount]...)
	samples = append(samples, prefixes[len(prefixes)-backCount:]...)
	return samples
}

func topLevelPrefix(objectName string) string {
	index := strings.IndexByte(objectName, '/')
	if index < 0 {
		return "(root)"
	}
	return objectName[:index+1]
}

func earlierTimestamp(current string, candidate time.Time) string {
	if candidate.IsZero() {
		return current
	}
	if current == "" {
		return candidate.UTC().Format(time.RFC3339)
	}
	parsed, err := time.Parse(time.RFC3339, current)
	if err != nil || candidate.Before(parsed) {
		return candidate.UTC().Format(time.RFC3339)
	}
	return current
}

func laterTimestamp(current string, candidate time.Time) string {
	if candidate.IsZero() {
		return current
	}
	if current == "" {
		return candidate.UTC().Format(time.RFC3339)
	}
	parsed, err := time.Parse(time.RFC3339, current)
	if err != nil || candidate.After(parsed) {
		return candidate.UTC().Format(time.RFC3339)
	}
	return current
}
