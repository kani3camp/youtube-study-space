package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"app.modules/core/mypage"
)

const (
	humanMockProject = "demo-youtube-study-space-ci"
	humanMockChannel = "UCsynthetic0000000000001"
)

func mockHumanRequest(purpose mypage.SupportPurpose) humanRequest {
	return humanRequest{RequestID: fmt.Sprintf("%x", sha256.Sum256([]byte("human-privacy-mock:"+string(purpose)))), Channel: humanMockChannel, AcceptedAt: time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)}
}

func fixedMockIntake(p proposal, input humanRequest) bool {
	if p.Action == "recover" {
		if input.ExpectedRevision.IsZero() {
			return false
		}
		// The original human receipt is fixed; the reviewed version comes from
		// private inspection (including real Emulator UpdateTime in tests).
		input.ExpectedRevision = mockReceiptVersion()
	}
	return humanAction(p.Action) && p.Mock && !p.Execute && p.Environment == "development" && p.Project == humanMockProject && intakePurpose(p.Purpose) && input == mockRequestForAction(p.Purpose, p.Action)
}

// This fixed fake verifies CLI plumbing only. Store semantics and fresh OAuth
// are separately tested with the real libraries against Emulator. No SDK,
// network, persistent state, identity verification or deletion is simulated.
type mockSupportCreator struct {
	expected humanRequest
	purpose  mypage.SupportPurpose
}

func (s mockSupportCreator) Create(ctx context.Context, id, channel string, purpose mypage.SupportPurpose, accepted, now time.Time) (string, string, error) {
	if ctx.Err() != nil || id != s.expected.RequestID || channel != s.expected.Channel || purpose != s.purpose || !accepted.Equal(s.expected.AcceptedAt) || now.Before(accepted) {
		return "", "", errIntake
	}
	return strings.Repeat("a", 64), strings.Repeat("b", 64), nil
}

func mockIntakeFactory(_ context.Context, p proposal) (*intakeRuntime, error) {
	input := mockRequestForAction(p.Purpose, p.Action)
	if !fixedMockIntake(p, input) {
		return nil, errIntake
	}
	expected := intakeIntent{Action: p.Action, Environment: p.Environment, Project: p.Project, Purpose: p.Purpose, Request: input}
	return &intakeRuntime{
		environment: p.Environment, project: p.Project,
		creator:  mockSupportCreator{expected: input, purpose: p.Purpose},
		recovery: mockSupportRecovery{},
		authorize: func(_ context.Context, intent intakeIntent) (intakeGrant, error) {
			if intent != expected {
				return intakeGrant{}, errIntake
			}
			return intakeGrant{Actor: "synthetic-operator", Intent: expected}, nil
		},
		deliver: func(_ context.Context, intent intakeIntent, receipt intakeReceipt) error {
			if intent != expected || receipt.RequestRef != strings.Repeat("a", 64) || (p.Action != "inspect" && receipt.Challenge != strings.Repeat("b", 64)) || (p.Action == "inspect" && (!receipt.Revision.Equal(mockReceiptVersion()) || receipt.Challenge != "")) {
				return errIntake
			}
			return nil
		},
		clock: func() time.Time { return input.AcceptedAt.Add(time.Hour) },
	}, nil
}

func mockReceiptVersion() time.Time { return time.Date(2026, 10, 6, 0, 30, 0, 0, time.UTC) }
func mockRequestForAction(p mypage.SupportPurpose, action string) humanRequest {
	input := mockHumanRequest(p)
	if action == "recover" {
		input.ExpectedRevision = mockReceiptVersion()
	}
	return input
}

type mockSupportRecovery struct{}

func (mockSupportRecovery) InspectRequestID(ctx context.Context, s mypage.SupportRecoverySelector, _ time.Time) (mypage.SupportReceiptInspection, error) {
	if ctx.Err() != nil || s.Environment != "development" || s.ProjectID != humanMockProject || s.RequestID != mockHumanRequest(s.Purpose).RequestID || s.Channel != humanMockChannel || !s.AcceptedAt.Equal(mockHumanRequest(s.Purpose).AcceptedAt) {
		return mypage.SupportReceiptInspection{}, errIntake
	}
	return mypage.SupportReceiptInspection{RequestRef: strings.Repeat("a", 64), Revision: mockReceiptVersion()}, nil
}

func (m mockSupportRecovery) ReissueRequestID(ctx context.Context, s mypage.SupportRecoverySelector, expected, now time.Time) (string, string, error) {
	v, err := m.InspectRequestID(ctx, s, now)
	if err != nil || !expected.Equal(v.Revision) {
		return "", "", errIntake
	}
	return v.RequestRef, strings.Repeat("b", 64), nil
}
