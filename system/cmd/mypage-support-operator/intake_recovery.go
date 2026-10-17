package main

import (
	"context"
	"time"

	"app.modules/core/mypage"
)

type supportRecovery interface {
	InspectRequestID(context.Context, mypage.SupportRecoverySelector, time.Time) (mypage.SupportReceiptInspection, error)
	ReissueRequestID(context.Context, mypage.SupportRecoverySelector, time.Time, time.Time) (string, string, error)
}

func humanAction(action string) bool {
	return action == "create" || action == "inspect" || action == "recover"
}

// Recovery has no Create fallback and no automatic handoff retry. The explicit
// inspected version is part of the independent authority's exact intent.
func runHumanRecovery(ctx context.Context, p proposal, input humanRequest, runtime *intakeRuntime, intent intakeIntent, now time.Time) string {
	selector := mypage.SupportRecoverySelector{Environment: p.Environment, ProjectID: p.Project, RequestID: input.RequestID, Channel: input.Channel, Purpose: p.Purpose, AcceptedAt: input.AcceptedAt}
	var receipt intakeReceipt
	if p.Action == "inspect" {
		view, err := runtime.recovery.InspectRequestID(ctx, selector, now)
		if err != nil || ctx.Err() != nil || !opaque.MatchString(view.RequestRef) || view.Revision.IsZero() {
			return "RECEIPT_UNRESOLVED"
		}
		receipt = intakeReceipt{RequestRef: view.RequestRef, Revision: view.Revision, DeleteBy: view.DeleteBy}
	} else if p.Action == "recover" && !input.ExpectedRevision.IsZero() {
		ref, challenge, err := runtime.recovery.ReissueRequestID(ctx, selector, input.ExpectedRevision, now)
		if err != nil || ctx.Err() != nil || !opaque.MatchString(ref) || !opaque.MatchString(challenge) {
			return "RECOVERY_RESULT_UNCERTAIN"
		}
		receipt = intakeReceipt{RequestRef: ref, Challenge: challenge}
	} else {
		return "INVALID_INPUT"
	}
	if runtime.deliver(ctx, intent, receipt) != nil || ctx.Err() != nil {
		return "INTAKE_HANDOFF_PENDING"
	}
	if p.Action == "inspect" {
		return "MOCK_RECEIPT_MATCHED"
	}
	return "MOCK_AWAITING_PROOF"
}
