package mypage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGoogleOperatorChallengeRecordRejectsAmbiguousOrPrivateState(t *testing.T) {
	raw := map[string]any{"schemaVersion": int64(1), "nonceHash": digest("nonce"), "binding": digest("binding"), "issuedAt": "2026-10-15T00:00:00.123456789Z", "expiresAt": "2026-10-15T00:05:00.123456789Z", "consumed": false}
	record, valid := decodeOperatorChallenge(raw)
	require.True(t, valid)
	require.Equal(t, 123456789, record.IssuedAt.Nanosecond())
	require.Equal(t, googleOperatorChallengeLifetime, record.ExpiresAt.Sub(record.IssuedAt))
	for name, change := range map[string]func(map[string]any){
		"unknown private field": func(v map[string]any) { v["token"] = "synthetic-token" },
		"missing binding":       func(v map[string]any) { delete(v, "binding") },
		"wrong version type":    func(v map[string]any) { v["schemaVersion"] = "1" },
		"wrong consumed type":   func(v map[string]any) { v["consumed"] = "false" },
		"already consumed":      func(v map[string]any) { v["consumed"] = true },
		"raw nonce":             func(v map[string]any) { v["nonceHash"] = "synthetic-nonce" },
		"uppercase hash":        func(v map[string]any) { v["binding"] = strings.ToUpper(digest("binding")) },
		"invalid time":          func(v map[string]any) { v["issuedAt"] = "invalid" },
		"noncanonical time":     func(v map[string]any) { v["issuedAt"] = "2026-10-15T00:00:00.123456789+00:00" },
		"extended lifetime":     func(v map[string]any) { v["expiresAt"] = "2026-10-15T00:05:01.123456789Z" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := make(map[string]any, len(raw))
			for k, v := range raw {
				candidate[k] = v
			}
			change(candidate)
			_, ok := decodeOperatorChallenge(candidate)
			require.False(t, ok)
		})
	}
}

func TestGoogleOperatorChallengeUnconfiguredStoreDeniesWithoutClockOrDiscovery(t *testing.T) {
	s, err := NewFirestoreGoogleOperatorChallenges(nil, "demo-youtube-study-space-ci")
	require.Nil(t, s)
	require.ErrorIs(t, err, ErrOperatorDenied)
	for _, store := range []*FirestoreGoogleOperatorChallenges{nil, {}} {
		require.ErrorIs(t, store.Put(context.Background(), GoogleOperatorChallengeRecord{}), ErrOperatorDenied)
		_, err := store.Consume(context.Background(), digest("nonce"), digest("binding"), func() time.Time {
			t.Fatal("invalid configuration must fail before reading the clock")
			return time.Time{}
		})
		require.ErrorIs(t, err, ErrOperatorDenied)
	}
}
