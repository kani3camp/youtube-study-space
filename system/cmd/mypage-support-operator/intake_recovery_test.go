package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"app.modules/core/mypage"

	"github.com/stretchr/testify/require"
)

type recoveryFuncs struct {
	inspect func(context.Context, mypage.SupportRecoverySelector, time.Time) (mypage.SupportReceiptInspection, error)
	reissue func(context.Context, mypage.SupportRecoverySelector, time.Time, time.Time) (string, string, error)
}

func (f recoveryFuncs) InspectRequestID(ctx context.Context, s mypage.SupportRecoverySelector, now time.Time) (mypage.SupportReceiptInspection, error) {
	return f.inspect(ctx, s, now)
}

func (f recoveryFuncs) ReissueRequestID(ctx context.Context, s mypage.SupportRecoverySelector, version, now time.Time) (string, string, error) {
	return f.reissue(ctx, s, version, now)
}

func recoveryArgs(t *testing.T, action string, purpose mypage.SupportPurpose) []string {
	t.Helper()
	args := humanArgs(humanFile(t, mockRequestForAction(purpose, action)), purpose)
	args[1] = action
	return args
}

func TestRecoveryCLIRequiresExplicitPrivateVersionAndNeverExecutes(t *testing.T) {
	for _, purpose := range []mypage.SupportPurpose{mypage.SupportDelete, mypage.SupportRevoke, mypage.SupportDisclosure} {
		for _, action := range []string{"inspect", "recover"} {
			t.Run(string(purpose)+"/"+action, func(t *testing.T) {
				var out bytes.Buffer
				args := recoveryArgs(t, action, purpose)
				require.Zero(t, command(append(args, "--mock"), testGet, &out))
				r := privateReport(t, out.String())
				require.Equal(t, action == "inspect", r.ReadOnly)
				if action == "inspect" {
					require.Equal(t, "MOCK_RECEIPT_MATCHED", r.Code)
				} else {
					require.Equal(t, "MOCK_AWAITING_PROOF", r.Code)
				}
				out.Reset()
				require.Equal(t, 1, commandWithIntake(context.Background(), append(args, "--execute"), testGet, &out, func(context.Context, proposal) (*intakeRuntime, error) {
					t.Fatal("execute reached recovery factory")
					return nil, nil
				}))
				require.Equal(t, "TRUSTED_OPERATOR_ADAPTER_UNAVAILABLE", privateReport(t, out.String()).Code)
			})
		}
	}
	for _, action := range []string{"create", "inspect", "recover"} {
		t.Run("version input/"+action, func(t *testing.T) {
			input := mockHumanRequest(mypage.SupportDelete)
			if action != "recover" {
				input.ExpectedRevision = mockReceiptVersion()
			}
			args := humanArgs(humanFile(t, input), mypage.SupportDelete)
			args[1] = action
			var out bytes.Buffer
			require.Equal(t, 2, commandWithIntake(context.Background(), append(args, "--mock"), testGet, &out, func(context.Context, proposal) (*intakeRuntime, error) {
				t.Fatal("invalid version shape reached factory")
				return nil, nil
			}))
		})
	}
}

func TestRecoveryCLIUsesActionAndVersionBoundAuthorityAndNoCreateFallback(t *testing.T) {
	for _, stage := range []string{"success inspect", "missing index", "success recover", "reissue ACK lost", "handoff failed", "reused inspect grant", "changed reviewed version"} {
		t.Run(stage, func(t *testing.T) {
			action := "recover"
			if stage == "success inspect" || stage == "missing index" {
				action = "inspect"
			}
			p := proposal{Action: action, Environment: "development", Project: humanMockProject, Purpose: mypage.SupportDelete, Mock: true}
			input := mockRequestForAction(p.Purpose, action)
			runtime, err := mockIntakeFactory(context.Background(), p)
			require.NoError(t, err)
			creates, inspects, reissues, deliveries := 0, 0, 0, 0
			runtime.creator = createFunc(func(context.Context, string, string, mypage.SupportPurpose, time.Time, time.Time) (string, string, error) {
				creates++
				return "", "", errIntake
			})
			runtime.recovery = recoveryFuncs{
				inspect: func(_ context.Context, s mypage.SupportRecoverySelector, _ time.Time) (mypage.SupportReceiptInspection, error) {
					inspects++
					require.Equal(t, input.RequestID, s.RequestID)
					if stage == "missing index" {
						return mypage.SupportReceiptInspection{}, errors.New("synthetic-private")
					}
					return mypage.SupportReceiptInspection{RequestRef: strings.Repeat("a", 64), Revision: mockReceiptVersion()}, nil
				},
				reissue: func(_ context.Context, s mypage.SupportRecoverySelector, version, _ time.Time) (string, string, error) {
					reissues++
					require.Equal(t, input.AcceptedAt, s.AcceptedAt)
					require.Equal(t, input.ExpectedRevision, version)
					if stage == "reissue ACK lost" {
						return "", "", errors.New("synthetic-private")
					}
					return strings.Repeat("a", 64), strings.Repeat("b", 64), nil
				},
			}
			if stage == "reused inspect grant" || stage == "changed reviewed version" {
				runtime.authorize = func(_ context.Context, i intakeIntent) (intakeGrant, error) {
					if stage == "reused inspect grant" {
						i.Action = "inspect"
					} else {
						i.Request.ExpectedRevision = i.Request.ExpectedRevision.Add(time.Microsecond)
					}
					return intakeGrant{Actor: "synthetic-operator", Intent: i}, nil
				}
			}
			runtime.deliver = func(_ context.Context, i intakeIntent, r intakeReceipt) error {
				deliveries++
				require.Equal(t, action, i.Action)
				if action == "inspect" {
					require.Empty(t, r.Challenge)
					require.Equal(t, mockReceiptVersion(), r.Revision)
				}
				if stage == "handoff failed" {
					return errors.New("synthetic-private")
				}
				return nil
			}
			code := runHumanIntake(context.Background(), p, input, func(context.Context, proposal) (*intakeRuntime, error) { return runtime, nil })
			require.Zero(t, creates, "recovery must never retry Create even on missing index or uncertain outcome")
			switch stage {
			case "success inspect":
				require.Equal(t, "MOCK_RECEIPT_MATCHED", code)
				require.Equal(t, 1, inspects)
				require.Zero(t, reissues)
				require.Equal(t, 1, deliveries)
			case "missing index":
				require.Equal(t, "RECEIPT_UNRESOLVED", code)
				require.Zero(t, reissues)
				require.Zero(t, deliveries)
			case "success recover":
				require.Equal(t, "MOCK_AWAITING_PROOF", code)
				require.Equal(t, 1, reissues)
				require.Equal(t, 1, deliveries)
			case "reissue ACK lost":
				require.Equal(t, "RECOVERY_RESULT_UNCERTAIN", code)
				require.Equal(t, 1, reissues)
				require.Zero(t, deliveries)
			case "handoff failed":
				require.Equal(t, "INTAKE_HANDOFF_PENDING", code)
				require.Equal(t, 1, reissues)
				require.Equal(t, 1, deliveries)
			default:
				require.Equal(t, "TRUSTED_INTAKE_DENIED", code)
				require.Zero(t, inspects)
				require.Zero(t, reissues)
				require.Zero(t, deliveries)
			}
		})
	}
}
