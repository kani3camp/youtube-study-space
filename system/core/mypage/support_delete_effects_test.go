package mypage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"app.modules/core/supportdelete"
)

type unavailableDeletionProbe struct{ calls int }

func (p *unavailableDeletionProbe) Apply(context.Context, supportdelete.Operation) (supportdelete.Evidence, error) {
	p.calls++
	return supportdelete.Evidence{}, nil
}

func TestFirestoreDeletionMissingClientFailsClosedBeforeRestoreProbe(t *testing.T) {
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	e := supportdelete.Execution{Selector: supportdelete.MockSelector(), Manifest: supportdelete.Manifest{Ref: digest("synthetic-manifest"), Mode: "emulator"}, OwnerRef: digest("synthetic-owner"), Revision: 1, Generation: 1, GuardSince: now, Cutoff: now, AcceptedAt: now.Add(-time.Hour), DeleteBy: now.Add(-time.Hour).Add(7 * 24 * time.Hour), UpdatedAt: now, Cursor: 5, EvidenceDigest: digest("synthetic-chain")}
	require.NoError(t, e.Validate())
	probe := &unavailableDeletionProbe{}
	adapter := &FirestoreDeletionEffects{Store: &FirestoreDeletionStore{}, RestoreGuard: probe, Clock: func() time.Time { return now }}
	_, err := adapter.Apply(context.Background(), supportdelete.NewOperation(e, now))
	require.ErrorIs(t, err, supportdelete.ErrUnavailable)
	require.Zero(t, probe.calls)
}
