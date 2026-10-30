//go:build integration

package mypage

import (
	"context"
	"net/http"
	"testing"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/repository"
	"app.modules/core/serviceaccess"
	"app.modules/core/supportdelete"
)

type currentTestAuth struct {
	uid      string
	created  int64
	missing  bool
	notFound error
	deletes  int
}

func (a *currentTestAuth) GetUser(context.Context, string) (*auth.UserRecord, error) {
	if a.missing {
		return nil, a.notFound
	}
	return &auth.UserRecord{UserInfo: &auth.UserInfo{UID: a.uid}, UserMetadata: &auth.UserMetadata{CreationTimestamp: a.created}}, nil
}
func (*currentTestAuth) RevokeRefreshTokens(context.Context, string) error { return nil }
func (a *currentTestAuth) DeleteUser(context.Context, string) error {
	a.missing = true
	a.deletes++
	return nil
}

type currentTestBQ struct {
	calls int
	err   error
}

func (a *currentTestBQ) ApplyChannelDeletion(context.Context, string, string, bool) error {
	a.calls++
	return a.err
}

func currentAdapterFixture(t *testing.T) (*CurrentDeletionEffects, supportdelete.Start, *currentTestAuth) {
	t.Helper()
	store, start, _ := deletionFixture(t)
	e, err := store.Acquire(context.Background(), start)
	require.NoError(t, err)
	// Obtain a correctly classified SDK error through a local HTTP transport, never the real API.
	app, err := firebase.NewApp(context.Background(), &firebase.Config{ProjectID: "demo-youtube-study-space-ci"}, option.WithHTTPClient(&http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) {
		return providerResponse(400, `{"error":{"message":"USER_NOT_FOUND"}}`), nil
	})}))
	require.NoError(t, err)
	sdk, err := app.Auth(context.Background())
	require.NoError(t, err)
	_, notFound := sdk.GetUser(context.Background(), start.Selector.Target.ChannelID)
	require.True(t, auth.IsUserNotFound(notFound))
	client := &currentTestAuth{uid: start.Selector.Target.ChannelID, created: 123456, notFound: notFound}
	gates := OwnerDeletionGates{Selector: e.Selector, Generation: e.Generation, Cutoff: e.Cutoff, ExpiresAt: start.Now.Add(30 * time.Minute), WritersStoppedAt: start.Now, CachePurgedAt: start.Now, RestoreExcludedAt: start.Now, Manual: map[string]DeletionGateRecord{}}
	for _, scope := range []string{"legacy-mappings", "backups-exports", "derived-vendor", "platform-logs"} {
		gates.Manual[scope] = DeletionGateRecord{Status: "absent", EvidenceRef: digest(scope), ObservedAt: start.Now}
	}
	return &CurrentDeletionEffects{Store: store, Auth: client, BigQuery: &currentTestBQ{}, Gates: gates, Clock: func() time.Time { return start.Now }}, start, client
}

func TestCurrentDeletionDeletesOnlyTargetAndWaitsForActualWriterResume(t *testing.T) {
	a, start, identity := currentAdapterFixture(t)
	ctx := context.Background()
	uid := start.Selector.Target.ChannelID
	_, err := a.Store.Client.Collection(repository.USERS).Doc(uid).Set(ctx, map[string]any{"total-study-sec": 123})
	require.NoError(t, err)
	other := "UCother000000000000000001"
	_, err = a.Store.Client.Collection(repository.USERS).Doc(other).Set(ctx, map[string]any{"total-study-sec": 456})
	require.NoError(t, err)
	runner := supportdelete.Engine{Store: a.Store, Effects: a, Manifest: start.Manifest, Clock: a.Clock}
	_, err = runner.Run(ctx, start.Selector, start.OwnerRef)
	require.ErrorIs(t, err, ErrDeletionGate)
	e, err := a.Store.Acquire(ctx, start)
	require.NoError(t, err)
	require.Equal(t, supportdelete.ResumeStart(), e.Cursor)
	_, err = a.Store.Client.Collection(repository.USERS).Doc(uid).Get(ctx)
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = a.Store.Client.Collection(repository.USERS).Doc(other).Get(ctx)
	require.NoError(t, err)
	control, err := (&serviceaccess.FirestoreStore{Client: a.Store.Client}).Read(ctx, uid)
	require.NoError(t, err)
	require.True(t, control.Control.PrivacyDeletion.Active)
	require.Equal(t, 1, identity.deletes)
	now := start.Now.Add(time.Second)
	a.Clock = func() time.Time { now = now.Add(time.Microsecond); return now }
	runner.Clock = a.Clock
	a.Gates.WritersResumedAt = now
	_, err = runner.Run(ctx, start.Selector, start.OwnerRef)
	require.NoError(t, err)
	_, err = runner.Run(ctx, start.Selector, start.OwnerRef)
	require.NoError(t, err)
	require.Equal(t, 1, identity.deletes)
	_, err = a.Store.Client.Collection(currentAuthTargets).Doc(start.Selector.ExecutionRef).Get(ctx)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestCurrentDeletionCannotAdoptRecreatedAuthUIDOrUnknownManualScope(t *testing.T) {
	a, start, identity := currentAdapterFixture(t)
	ctx := context.Background()
	e, err := a.Store.Acquire(ctx, start)
	require.NoError(t, err)
	_, err = a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
	require.NoError(t, err)
	cursor := 0
	for i, step := range supportdelete.Steps() {
		if step.Action == "delete" && step.Scope == "firebase-auth" {
			cursor = i
		}
	}
	e = advanceDeletion(t, a.Store, e, cursor, start.Now)
	identity.created++
	_, err = a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
	require.ErrorIs(t, err, supportdelete.ErrConflict)
	require.Zero(t, identity.deletes)
	a.Gates.Manual["legacy-mappings"] = DeletionGateRecord{Status: "unknown", EvidenceRef: digest("unknown"), ObservedAt: start.Now}
	// A missing/expired/other-generation gate rejects every action before any SDK side effect.
	a.Gates.Generation++
	_, err = a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
	require.ErrorIs(t, err, ErrDeletionGate)
}

func TestCurrentDeletionRejectsEarlyWriterResumeAndTimestampRewrite(t *testing.T) {
	for _, before := range []supportdelete.Step{
		{Action: "revoke", Scope: "firebase-auth"},
		{Action: "delete", Scope: "firestore-primary"},
		{Action: "inspect", Scope: "firestore-web"},
	} {
		t.Run(before.Action, func(t *testing.T) {
			a, start, identity := currentAdapterFixture(t)
			ctx := context.Background()
			now := start.Now
			a.Clock = func() time.Time { now = now.Add(time.Microsecond); return now }
			e, err := a.Store.Acquire(ctx, start)
			require.NoError(t, err)
			for supportdelete.Steps()[e.Cursor] != before {
				v, applyErr := a.Apply(ctx, supportdelete.NewOperation(e, a.Clock()))
				require.NoError(t, applyErr)
				e, err = a.Store.Commit(ctx, e, v, a.Clock())
				require.NoError(t, err)
			}
			// In the inspect case, firestore-primary has already been inspected.
			// No stage before resume may accept evidence that writers are running.
			deletes, bqCalls := identity.deletes, a.BigQuery.(*currentTestBQ).calls
			a.Gates.WritersResumedAt = a.Clock()
			runner := supportdelete.Engine{Store: a.Store, Effects: a, Manifest: start.Manifest, Clock: a.Clock}
			_, err = runner.Run(ctx, start.Selector, start.OwnerRef)
			require.ErrorIs(t, err, ErrDeletionGate)
			stopped, err := a.Store.Acquire(ctx, start)
			require.NoError(t, err)
			assert.Equal(t, e, stopped, "early resume must not advance the checkpoint")
			assert.Equal(t, deletes, identity.deletes, "early resume must precede Auth effects")
			assert.Equal(t, bqCalls, a.BigQuery.(*currentTestBQ).calls, "early resume must precede BigQuery effects")

			// Reproduce the reviewed bypass: a writer recreates inspected data,
			// then the owner updates only the resume timestamp and retries.
			ref := a.Store.Client.Collection(repository.USERS).Doc(start.Selector.Target.ChannelID)
			_, err = ref.Set(ctx, map[string]any{"total-study-sec": 999})
			require.NoError(t, err)
			a.Gates.WritersResumedAt = a.Clock()
			_, err = runner.Run(ctx, start.Selector, start.OwnerRef)
			require.ErrorIs(t, err, ErrDeletionGate, "rewriting the timestamp must not permit finalization")
			stopped, err = a.Store.Acquire(ctx, start)
			require.NoError(t, err)
			require.Equal(t, e, stopped)
			doc, err := ref.Get(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(999), doc.Data()["total-study-sec"])
			control, err := (&serviceaccess.FirestoreStore{Client: a.Store.Client}).Read(ctx, start.Selector.Target.ChannelID)
			require.NoError(t, err)
			require.True(t, control.Control.PrivacyDeletion.Active)
		})
	}
}
