//go:build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"app.modules/core/mypage"
	"app.modules/internal/integrationtest"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
)

func TestRecoveryCLIEmulatorReconcilesUnknownACKAndPrivateHandoffFailure(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, humanMockProject, option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	for _, failure := range []string{"reissue ACK lost", "handoff failed"} {
		t.Run(failure, func(t *testing.T) {
			input := mockHumanRequest(mypage.SupportDelete)
			store := &mypage.FirestoreSupportStore{Client: client, Environment: "development"}
			// Simulate a committed Create whose ACK was lost: CLI only knows input.
			ref, original, err := store.Create(ctx, input.RequestID, input.Channel, mypage.SupportDelete, input.AcceptedAt, time.Now())
			require.NoError(t, err)
			t.Cleanup(func() {
				indexes, err := client.Collection("support-challenges").Where("requestRef", "==", ref).Documents(ctx).GetAll()
				require.NoError(t, err)
				for _, doc := range indexes {
					_, err := doc.Ref.Delete(ctx)
					require.NoError(t, err)
				}
				_, err = client.Collection("support-request-ids").Doc(deskDigest("development:" + input.RequestID)).Delete(ctx)
				require.NoError(t, err)
				_, err = client.Collection("support-requests").Doc(ref).Delete(ctx)
				require.NoError(t, err)
			})
			var handoff intakeReceipt
			var committedChallenge string
			calls, deliveries := 0, 0
			fail := false
			factory := func(_ context.Context, p proposal) (*intakeRuntime, error) {
				r, err := newFirestoreIntakeRuntime(p, client, func(_ context.Context, i intakeIntent) (intakeGrant, error) {
					require.Equal(t, p.Action, i.Action)
					require.Equal(t, input.RequestID, i.Request.RequestID)
					return intakeGrant{Actor: "synthetic-operator", Intent: i}, nil
				}, func(_ context.Context, _ intakeIntent, receipt intakeReceipt) error {
					deliveries++
					handoff = receipt
					if fail && failure == "handoff failed" {
						return errors.New("synthetic-private")
					}
					return nil
				}, time.Now)
				if err != nil {
					return nil, err
				}
				port := r.recovery
				r.creator = createFunc(func(context.Context, string, string, mypage.SupportPurpose, time.Time, time.Time) (string, string, error) {
					t.Fatal("recovery must never create a duplicate receipt")
					return "", "", errIntake
				})
				r.recovery = recoveryFuncs{inspect: port.InspectRequestID, reissue: func(ctx context.Context, s mypage.SupportRecoverySelector, version, now time.Time) (string, string, error) {
					calls++
					requestRef, challenge, err := port.ReissueRequestID(ctx, s, version, now)
					if err == nil {
						committedChallenge = challenge
						if fail && failure == "reissue ACK lost" {
							return "", "", errors.New("synthetic-private")
						}
					}
					if err != nil {
						return "", "", fmt.Errorf("synthetic recovery port: %w", err)
					}
					return requestRef, challenge, nil
				}}
				return r, nil
			}
			invoke := func(action string, request humanRequest) (string, int) {
				t.Helper()
				args := humanArgs(humanFile(t, request), mypage.SupportDelete)
				args[1] = action
				var out bytes.Buffer
				exit := commandWithIntake(ctx, append(args, "--mock"), testGet, &out, factory)
				r := privateReport(t, out.String())
				require.NotContains(t, out.String(), original)
				if committedChallenge != "" {
					require.NotContains(t, out.String(), committedChallenge)
				}
				return r.Code, exit
			}
			code, exit := invoke("inspect", input)
			require.Zero(t, exit)
			require.Equal(t, "MOCK_RECEIPT_MATCHED", code)
			require.Empty(t, handoff.Challenge)
			require.Equal(t, ref, handoff.RequestRef)
			require.Equal(t, input.AcceptedAt.Add(7*24*time.Hour), *handoff.DeleteBy)
			input.ExpectedRevision = handoff.Revision
			fail = true
			code, exit = invoke("recover", input)
			require.Equal(t, 1, exit)
			if failure == "reissue ACK lost" {
				require.Equal(t, "RECOVERY_RESULT_UNCERTAIN", code)
				require.Equal(t, 1, deliveries)
			} else {
				require.Equal(t, "INTAKE_HANDOFF_PENDING", code)
				require.Equal(t, 2, deliveries)
			}
			require.Equal(t, 1, calls)
			_, err = store.Resolve(ctx, original, time.Now())
			require.Error(t, err)
			_, err = store.Resolve(ctx, committedChallenge, time.Now())
			require.NoError(t, err)
			doc, err := client.Collection("support-requests").Doc(ref).Get(ctx)
			require.NoError(t, err)
			priorDeliveries := deliveries
			// A copied retry of the old command cannot rotate or resend anything.
			code, exit = invoke("recover", input)
			require.Equal(t, 1, exit)
			require.Equal(t, "RECOVERY_RESULT_UNCERTAIN", code)
			require.Equal(t, priorDeliveries, deliveries)
			unchanged, err := doc.Ref.Get(ctx)
			require.NoError(t, err)
			require.Equal(t, doc.UpdateTime, unchanged.UpdateTime)
			// Explicit fresh inspection gives only metadata, never the prior secret.
			fail = false
			inspectInput := input
			inspectInput.ExpectedRevision = time.Time{}
			code, exit = invoke("inspect", inspectInput)
			require.Zero(t, exit)
			require.Equal(t, "MOCK_RECEIPT_MATCHED", code)
			require.Empty(t, handoff.Challenge)
			require.False(t, handoff.Revision.Equal(input.ExpectedRevision))
			require.Equal(t, 2, calls, "inspection must not trigger another reissue")
			var record mypage.SupportRequest
			require.NoError(t, unchanged.DataTo(&record))
			require.Equal(t, input.AcceptedAt, record.AcceptedAt)
			require.Equal(t, input.AcceptedAt.Add(7*24*time.Hour), *record.DeleteBy)
			docs, err := client.Collection("support-requests").Where("requestId", "==", input.RequestID).Documents(ctx).GetAll()
			require.NoError(t, err)
			require.Len(t, docs, 1)
		})
	}
}
