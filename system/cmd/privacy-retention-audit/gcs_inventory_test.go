package main

import (
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGCSInventoryAccumulator_RecordLiveObject(t *testing.T) {
	cutoff := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	accumulator := newGCSInventoryAccumulator([]string{"live-chat-history", "users"})

	accumulator.recordLiveObject(&storage.ObjectAttrs{
		Name:    "2021-11-13/all_namespaces_kind_live-chat-history/output-0",
		Size:    100,
		Created: time.Date(2021, 11, 13, 8, 0, 0, 0, time.UTC),
	}, cutoff)
	accumulator.recordLiveObject(&storage.ObjectAttrs{
		Name:    "2026-08-14/all_namespaces_kind_users/output-0",
		Size:    50,
		Created: time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC),
	}, cutoff)
	accumulator.finalizePrefixes()

	inventory := accumulator.inventory
	assert.EqualValues(t, 2, inventory.LiveObjectCount)
	assert.EqualValues(t, 150, inventory.LiveBytes)
	assert.EqualValues(t, 1, inventory.LiveObjectsOlderThanCutoff)
	assert.Equal(t, "2021-11-13T08:00:00Z", inventory.OldestLiveCreatedAt)
	assert.Equal(t, "2026-08-14T08:00:00Z", inventory.NewestLiveCreatedAt)
	assert.EqualValues(t, 1, inventory.KnownCollectionObjectCounts["live-chat-history"])
	assert.EqualValues(t, 1, inventory.KnownCollectionObjectCounts["users"])
	require.Len(t, inventory.TopLevelPrefixSamples, 2)
	assert.Equal(t, "2021-11-13/", inventory.TopLevelPrefixSamples[0].Prefix)
	assert.Equal(t, "2026-08-14/", inventory.TopLevelPrefixSamples[1].Prefix)
}

func TestGCSInventoryAccumulator_RecordSoftDeletedObject(t *testing.T) {
	accumulator := newGCSInventoryAccumulator(nil)
	accumulator.recordSoftDeletedObject(&storage.ObjectAttrs{
		Size:           42,
		SoftDeleteTime: time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC),
		HardDeleteTime: time.Date(2026, 8, 21, 1, 0, 0, 0, time.UTC),
	})
	accumulator.recordSoftDeletedObject(&storage.ObjectAttrs{
		Size:           8,
		SoftDeleteTime: time.Date(2026, 8, 13, 1, 0, 0, 0, time.UTC),
		HardDeleteTime: time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC),
	})

	inventory := accumulator.inventory
	assert.EqualValues(t, 2, inventory.SoftDeletedObjectCount)
	assert.EqualValues(t, 50, inventory.SoftDeletedBytes)
	assert.Equal(t, "2026-08-13T01:00:00Z", inventory.OldestSoftDeleteTime)
	assert.Equal(t, "2026-08-21T01:00:00Z", inventory.LatestSoftDeleteHardDelete)
}

func TestSampledPrefixNames(t *testing.T) {
	prefixes := []string{
		"01/", "02/", "03/", "04/", "05/", "06/",
		"07/", "08/", "09/", "10/", "11/", "12/",
	}

	assert.Equal(
		t,
		[]string{"01/", "02/", "03/", "10/", "11/", "12/"},
		sampledPrefixNames(prefixes, 6),
	)
	assert.Equal(t, prefixes, sampledPrefixNames(prefixes, 20))
	assert.Nil(t, sampledPrefixNames(prefixes, 0))
}

func TestTopLevelPrefix(t *testing.T) {
	assert.Equal(t, "2026-08-14/", topLevelPrefix("2026-08-14/export/file"))
	assert.Equal(t, "(root)", topLevelPrefix("root-object"))
}
