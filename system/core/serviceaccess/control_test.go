package serviceaccess

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func sampleChange(reason Reason) Change {
	code := "MODERATION"
	if reason == PrivacyDeletion {
		code = "PRIVACY_DELETION"
	}
	return Change{Reason: reason, Active: true, ReasonCode: code, Reference: strings.Repeat("a", 64)}
}

func TestIndependentReasonsIdempotenceAndMonotonicGeneration(t *testing.T) {
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	old := Snapshot{}
	require.NoError(t, old.Allowed())
	for _, reason := range []Reason{Moderation, PrivacyDeletion} {
		next, changed, err := Transition(old, sampleChange(reason), now)
		require.NoError(t, err)
		require.True(t, changed)
		require.Equal(t, old.Control.Generation+1, next.Generation)
		old = Snapshot{Control: next, Exists: true, Revision: now}
		same, changed, err := Transition(old, sampleChange(reason), now.Add(time.Second))
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, next, same)
	}
	require.ErrorIs(t, old.Allowed(), ErrDeletion)
	next, _, err := Transition(old, Change{Reason: Moderation}, now.Add(time.Second))
	require.NoError(t, err)
	require.True(t, next.PrivacyDeletion.Active)
	require.False(t, next.Moderation.Active)
	old = Snapshot{Control: next, Exists: true, Revision: now.Add(time.Second)}
	next, changed, err := Transition(old, Change{Reason: PrivacyDeletion}, now.Add(2*time.Second))
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, int64(4), next.Generation)
	require.NoError(t, (Snapshot{Control: next, Exists: true, Revision: now.Add(2 * time.Second)}).Allowed())
	require.NotEqual(t, "absent", (Snapshot{Control: next, Exists: true, Revision: now.Add(2 * time.Second)}).Checkpoint())
	_, changed, err = Transition(Snapshot{}, Change{Reason: Moderation}, now)
	require.NoError(t, err)
	require.False(t, changed)
}

func TestInvalidControlOrChangeFailsClosed(t *testing.T) {
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	c, _, err := Transition(Snapshot{}, sampleChange(Moderation), now)
	require.NoError(t, err)
	good := Snapshot{Control: c, Exists: true, Revision: now}
	for _, mutate := range []func(*Snapshot){
		func(s *Snapshot) { s.Exists = false },
		func(s *Snapshot) { s.Control.Generation = 0 },
		func(s *Snapshot) { s.Revision = time.Time{} },
		func(s *Snapshot) { s.Control.Moderation.ReasonCode = "free form" },
		func(s *Snapshot) { s.Control.Moderation.ActionRef = "invalid" },
		func(s *Snapshot) { s.Control.Moderation.RequestRef = strings.Repeat("a", 64) },
		func(s *Snapshot) { s.Control.PrivacyDeletion.Since = now },
		func(s *Snapshot) { s.Control.Moderation.Since = now.Add(time.Second) },
	} {
		value := good
		mutate(&value)
		require.ErrorIs(t, value.Allowed(), ErrUnavailable)
	}
	for _, change := range []Change{
		{Reason: "other"},
		{Reason: Moderation, Active: true},
		{Reason: Moderation, Reference: strings.Repeat("a", 64)},
		{Reason: PrivacyDeletion, Active: true, ReasonCode: "MODERATION", Reference: strings.Repeat("a", 64)},
	} {
		_, _, err := Transition(good, change, now)
		require.ErrorIs(t, err, ErrInvalidChange)
	}
	good.Control.Generation = math.MaxInt64
	_, _, err = Transition(good, Change{Reason: Moderation}, now)
	require.ErrorIs(t, err, ErrUnavailable)
	good.Control.Generation = 1
	_, _, err = Transition(good, Change{Reason: Moderation}, now.Add(-time.Second))
	require.ErrorIs(t, err, ErrUnavailable)
	for _, id := range []string{"", "UCsynthetic00000000000001", "UC/invalid"} {
		require.False(t, ValidChannel(id))
	}
	require.True(t, ValidChannel("UCsynthetic0000000000001"))
}
