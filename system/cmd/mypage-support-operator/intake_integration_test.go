//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"app.modules/core/mypage"
	"app.modules/core/serviceaccess"
	"app.modules/internal/integrationtest"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
)

type deskOAuth struct{ channel string }

func (p *deskOAuth) AuthorizationURL(state string) string {
	return "https://example.invalid/oauth?state=" + state
}

func (p *deskOAuth) Resolve(context.Context, string) ([]mypage.Channel, error) {
	return []mypage.Channel{{ID: p.channel, DisplayName: "Synthetic channel"}}, nil
}

type forbiddenDeskMinter struct{ t *testing.T }

func (m forbiddenDeskMinter) Mint(context.Context, string) (string, error) {
	m.t.Fatal("support proof must never mint a login token")
	return "", errIntake
}

func deskDigest(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }

func TestHumanDeskCLIEmulatorCreatesReceiptThenRequiresExistingFreshOAuth(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, humanMockProject, option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	for _, purpose := range []mypage.SupportPurpose{mypage.SupportDelete, mypage.SupportRevoke, mypage.SupportDisclosure} {
		t.Run(string(purpose), func(t *testing.T) {
			input := mockHumanRequest(purpose)
			p := proposal{Action: "create", Environment: "development", Project: humanMockProject, Purpose: purpose, Mock: true}
			var receipt intakeReceipt
			var oauthIDs []string
			var delivered int
			factory := func(_ context.Context, got proposal) (*intakeRuntime, error) {
				return newFirestoreIntakeRuntime(got, client, func(_ context.Context, intent intakeIntent) (intakeGrant, error) {
					require.Equal(t, intakeIntent{Action: p.Action, Environment: p.Environment, Project: p.Project, Purpose: purpose, Request: input}, intent)
					return intakeGrant{Actor: "synthetic-operator", Intent: intent}, nil
				}, func(_ context.Context, intent intakeIntent, r intakeReceipt) error {
					require.Equal(t, input, intent.Request)
					delivered++
					receipt = r
					return nil
				}, time.Now)
			}
			t.Cleanup(func() {
				// Delete only this fixture's receipt, indexes and OAuth transactions.
				if receipt.RequestRef == "" {
					return
				}
				for _, ref := range []*firestore.DocumentRef{
					client.Collection("support-requests").Doc(receipt.RequestRef),
					client.Collection("support-request-ids").Doc(deskDigest("development:" + input.RequestID)),
					client.Collection("support-challenges").Doc(deskDigest(receipt.Challenge)),
				} {
					_, err := ref.Delete(ctx)
					require.NoError(t, err)
				}
				for _, id := range oauthIDs {
					_, err := client.Collection("oauth-transactions").Doc(id).Delete(ctx)
					require.NoError(t, err)
				}
			})
			args := append(humanArgs(humanFile(t, input), purpose), "--mock")
			var out bytes.Buffer
			require.Zero(t, commandWithIntake(ctx, args, testGet, &out, factory))
			require.Equal(t, "MOCK_AWAITING_PROOF", privateReport(t, out.String()).Code)
			require.Equal(t, 1, delivered)
			require.Len(t, receipt.RequestRef, 64)
			require.Len(t, receipt.Challenge, 64)
			ref := client.Collection("support-requests").Doc(receipt.RequestRef)
			doc, err := ref.Get(ctx)
			require.NoError(t, err)
			var record mypage.SupportRequest
			require.NoError(t, doc.DataTo(&record))
			require.Equal(t, "awaiting_proof", record.Status)
			require.Equal(t, input.RequestID, record.RequestID)
			require.Equal(t, input.Channel, record.TargetChannel)
			require.Equal(t, purpose, record.Purpose)
			require.Equal(t, input.AcceptedAt, record.AcceptedAt)
			require.Empty(t, record.Body)
			require.Empty(t, record.ProofRef)
			require.NotEqual(t, receipt.Challenge, record.ChallengeHash)
			if purpose == mypage.SupportDelete {
				require.NotNil(t, record.DeleteBy)
				require.Equal(t, input.AcceptedAt.Add(7*24*time.Hour), *record.DeleteBy)
			} else {
				require.Nil(t, record.DeleteBy)
			}
			// Duplicate/retry never creates another receipt or resets acceptance/SLA.
			out.Reset()
			require.Equal(t, 1, commandWithIntake(ctx, args, testGet, &out, factory))
			require.Equal(t, "INTAKE_RESULT_UNCERTAIN", privateReport(t, out.String()).Code)
			require.Equal(t, 1, delivered)
			docs, err := client.Collection("support-requests").Where("requestId", "==", input.RequestID).Documents(ctx).GetAll()
			require.NoError(t, err)
			require.Len(t, docs, 1)
			require.Equal(t, doc.Data(), docs[0].Data())
			store := &mypage.FirestoreSupportStore{Client: client, Environment: "development"}
			_, err = (&mypage.FirestoreSupportStore{Client: client, Environment: "production"}).Resolve(ctx, receipt.Challenge, time.Now())
			require.Error(t, err)
			selector := mypage.DryRunSelector{Environment: "development", ExpectedProject: humanMockProject, RequestRef: receipt.RequestRef, Purpose: purpose, Now: time.Now().UTC()}
			_, err = mypage.BuildDryRun(ctx, selector, &mypage.FirestoreDryRunReader{Client: client})
			require.Error(t, err, "receipt alone never supplies proof")
			provider := &deskOAuth{channel: "UCsynthetic0000000000002"}
			auth := &mypage.AuthService{Store: &mypage.FirestoreAuthStore{Client: client}, Support: store, Provider: provider, Minter: forbiddenDeskMinter{t: t}, Access: &serviceaccess.FirestoreStore{Client: client}, Policy: mypage.Policy{Privacy: "synthetic-p", Terms: "synthetic-t"}, Now: time.Now}
			start := func() string {
				t.Helper()
				response, id, err := auth.Start(ctx, "", mypage.StartRequest{PrivacyPolicyVersion: auth.Policy.Privacy, PrivacyAccepted: true, TermsVersion: auth.Policy.Terms, TermsAccepted: true, SupportChallenge: &receipt.Challenge})
				require.NoError(t, err)
				oauthIDs = append(oauthIDs, id)
				u, err := url.Parse(response.AuthorizationURL)
				require.NoError(t, err)
				require.NoError(t, auth.Callback(ctx, id, u.Query().Get("state"), "synthetic-code", false))
				return id
			}
			wrongID := start()
			_, err = auth.Channel(ctx, wrongID)
			require.Error(t, err, "another channel cannot confirm the claim")
			wrongDoc, err := client.Collection("oauth-transactions").Doc(wrongID).Get(ctx)
			require.NoError(t, err)
			var wrong mypage.OAuthTransaction
			require.NoError(t, wrongDoc.DataTo(&wrong))
			_, err = auth.ConfirmResult(ctx, wrongID, wrong.ConfirmationRef)
			require.Error(t, err)
			provider.channel = input.Channel
			id := start()
			channel, err := auth.Channel(ctx, id)
			require.NoError(t, err)
			require.Equal(t, "support", channel.Purpose)
			require.Equal(t, purpose, channel.SupportPurpose)
			proof, err := auth.ConfirmResult(ctx, id, channel.ConfirmationRef)
			require.NoError(t, err)
			require.Equal(t, receipt.RequestRef, proof.SupportRequestRef)
			require.Empty(t, proof.CustomToken)
			require.Len(t, proof.RequestRef, 64)
			_, err = auth.ConfirmResult(ctx, id, channel.ConfirmationRef)
			require.Error(t, err, "fresh proof is one-use")
			_, err = store.Resolve(ctx, receipt.Challenge, time.Now())
			require.Error(t, err, "challenge consumed by proof")
			verified, err := ref.Get(ctx)
			require.NoError(t, err)
			require.NoError(t, verified.DataTo(&record))
			require.Equal(t, "verified", record.Status)
			require.Equal(t, proof.RequestRef, record.ProofRef)
			require.Equal(t, input.AcceptedAt, record.AcceptedAt)
			if purpose == mypage.SupportDelete {
				require.Equal(t, input.AcceptedAt.Add(7*24*time.Hour), *record.DeleteBy)
			}
			selector.Now = time.Now().UTC()
			diagnostic, err := mypage.BuildDryRun(ctx, selector, &mypage.FirestoreDryRunReader{Client: client})
			require.NoError(t, err)
			require.False(t, diagnostic.ExecutionAuthorized)
			require.False(t, diagnostic.InventoryComplete)
			require.Equal(t, "verified", diagnostic.ProofTrace)
			require.Equal(t, "unknown", diagnostic.Counts["backups-and-exports"].State)
			if purpose == mypage.SupportDelete {
				require.True(t, diagnostic.Overdue, "an old request's deadline must not be silently postponed")
			}
			wrongProject := p
			wrongProject.Project = "demo-other-project"
			_, err = newFirestoreIntakeRuntime(wrongProject, client, func(context.Context, intakeIntent) (intakeGrant, error) {
				t.Fatal("wrong client project reached authority")
				return intakeGrant{}, nil
			}, func(context.Context, intakeIntent, intakeReceipt) error { return nil }, time.Now)
			require.Error(t, err)
			// No deletion/mark-complete/effect adapter is available from the intake port.
			require.False(t, strings.Contains(out.String(), receipt.Challenge))
		})
	}
}
