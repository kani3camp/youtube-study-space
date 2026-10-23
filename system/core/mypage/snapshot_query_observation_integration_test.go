//go:build integration

package mypage

import (
	"context"
	"os"
	"testing"
	"time"

	"app.modules/core/repository"
	"app.modules/internal/integrationtest"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// Fault injection is confined to this Emulator client, not production hooks or
// global state. The real reader/transaction handles an RPC failure as usual.
type snapshotQueryFailureStream struct {
	grpc.ClientStream
	failCollections map[string]bool
	failQuery       bool
}

func (s *snapshotQueryFailureStream) SendMsg(message any) error {
	if query, ok := message.(*firestorepb.RunQueryRequest); ok {
		for _, from := range query.GetStructuredQuery().GetFrom() {
			if s.failCollections[from.GetCollectionId()] {
				s.failQuery = true
			}
		}
	}
	return s.ClientStream.SendMsg(message)
}

func (s *snapshotQueryFailureStream) RecvMsg(message any) error {
	if s.failQuery {
		// Non-retryable so failure assertions do not depend on SDK backoff.
		return status.Error(codes.FailedPrecondition, "synthetic query failure")
	}
	return s.ClientStream.RecvMsg(message)
}

// Use only after RequireFirestoreEmulator has checked the loopback endpoint.
// These are the Emulator's synthetic admin credentials, not real credentials.
type snapshotEmulatorCredentials struct{}

func (snapshotEmulatorCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer owner"}, nil
}

func (snapshotEmulatorCredentials) RequireTransportSecurity() bool { return false }

func TestSnapshotObservationDistinguishesQueryFailureFromEmptySuccess(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	for _, tc := range []struct {
		name              string
		failCollections   map[string]bool
		seatQueryErrors   int
		segmentQueryError bool
	}{
		{name: "empty-success"},
		{name: "history-failure", failCollections: map[string]bool{"work-segments": true}, segmentQueryError: true},
		{name: "standard-seat-failure", failCollections: map[string]bool{"seats": true}, seatQueryErrors: 1},
		{name: "member-seat-failure", failCollections: map[string]bool{"member-seats": true}, seatQueryErrors: 1},
		{name: "both-seat-failures", failCollections: map[string]bool{"seats": true, "member-seats": true}, seatQueryErrors: 2},
		{name: "all-query-failures", failCollections: map[string]bool{"seats": true, "member-seats": true, "work-segments": true}, seatQueryErrors: 2, segmentQueryError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interceptor := func(ctx context.Context, desc *grpc.StreamDesc, conn *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
				stream, err := streamer(ctx, desc, conn, method, opts...)
				if err != nil {
					return nil, err
				}
				return &snapshotQueryFailureStream{ClientStream: stream, failCollections: tc.failCollections}, nil
			}
			// Emulator mode supplies a prebuilt connection, so dial options alone
			// do not install interceptors. Pass our guarded connection explicitly.
			conn, err := grpc.NewClient("passthrough:///"+os.Getenv("FIRESTORE_EMULATOR_HOST"),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithPerRPCCredentials(snapshotEmulatorCredentials{}),
				grpc.WithChainStreamInterceptor(interceptor))
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })
			client, err := firestore.NewClient(context.Background(), "demo-youtube-study-space-ci", option.WithGRPCConn(conn))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			uid, err := opaque()
			require.NoError(t, err)
			userRef := client.Collection("users").Doc(uid)
			_, err = userRef.Set(context.Background(), repository.UserDoc{TotalStudySec: 600})
			require.NoError(t, err)
			t.Cleanup(func() {
				_, err := userRef.Delete(context.Background())
				require.NoError(t, err)
			})
			now := time.Now().UTC()
			var observations []SnapshotReadObservation
			reader := FirestoreSnapshotReader{
				Client: client, Coverage: []Interval{{Start: now.AddDate(0, 0, -30), End: now.Add(time.Hour)}},
				Observe: func(observation SnapshotReadObservation) { observations = append(observations, observation) },
			}
			snapshot, err := reader.Read(context.Background(), uid)
			require.NoError(t, err, "query failures remain partial snapshots, not whole-request failures")
			require.Len(t, observations, 1, "report once after the read-only transaction")
			stats := observations[0]
			require.Equal(t, 1, stats.UserLookups)
			require.Equal(t, 2, stats.SeatQueries)
			require.Equal(t, 1, stats.SegmentQueries)
			require.Zero(t, stats.SeatsReturned)
			require.Zero(t, stats.SegmentsReturned)
			require.Equal(t, tc.seatQueryErrors, stats.SeatQueryErrors)
			require.Equal(t, tc.segmentQueryError, stats.SegmentQueryError)
			require.Equal(t, tc.seatQueryErrors > 0, snapshot.SeatUnavailable)
			require.Equal(t, tc.segmentQueryError, snapshot.HistoryUnavailable)
			response, err := Aggregate(snapshot, fixtureAccount())
			require.NoError(t, err)
			if tc.seatQueryErrors > 0 || tc.segmentQueryError {
				require.Equal(t, Unavailable, response.Summary.Data.Today.Availability)
				require.NotNil(t, response.Summary.Data.Today.ReasonCode)
				require.Equal(t, SourceUnavailable, *response.Summary.Data.Today.ReasonCode)
				require.Nil(t, response.Summary.Data.Today.WorkSec)
			} else {
				expectSeconds(t, response.Summary.Data.Today, 0)
			}
		})
	}
}
