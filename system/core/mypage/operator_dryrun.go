package mypage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrDryRunUnavailable = errors.New("operator dryrun unavailable")

type DryRunSelector struct {
	Environment     string
	ExpectedProject string
	RequestRef      string
	Purpose         SupportPurpose
	Now             time.Time
}

// DryRunReader has no mutation, auth-revocation or Google API method. Its
// snapshot binds the receipt, available proof and inventory to one read time.
// A TTL-removed OAuth trace stays unknown and cannot authorize execution.
type DryRunReader interface {
	Inspect(context.Context, DryRunSelector) (DryRunSnapshot, error)
}
type DryRunSnapshot struct {
	Project      string
	AsOf         time.Time
	Receipt      SupportRequest
	OAuth        OAuthTransaction
	Counts       map[string]InventoryCount
	OAuthMissing bool
}
type InventoryCount struct {
	State string `json:"state"`
	Count *int   `json:"count,omitempty"`
	Scope string `json:"scope"`
}
type DryRunReport struct {
	ReadOnly            bool                      `json:"readOnly"`
	ExecutionAuthorized bool                      `json:"executionAuthorized"`
	InventoryComplete   bool                      `json:"inventoryComplete"`
	Purpose             SupportPurpose            `json:"purpose"`
	ProofTrace          string                    `json:"proofTrace"`
	Status              string                    `json:"receiptStatus"`
	ObservedAt          time.Time                 `json:"observedAt"`
	DeleteBy            *time.Time                `json:"deleteBy,omitempty"`
	Overdue             bool                      `json:"overdue"`
	Counts              map[string]InventoryCount `json:"counts"`
}

func validateDryRunSelector(s DryRunSelector) error {
	if !validSupportEnvironment(s.Environment) || !validSupportPurpose(s.Purpose) || !validOpaque(s.RequestRef) || s.ExpectedProject == "" || strings.ContainsAny(s.ExpectedProject, "/\\ \n\r\t") || s.Now.IsZero() {
		return ErrDryRunUnavailable
	}
	return nil
}

func validateDryRunReceipt(value SupportRequest, s DryRunSelector) error {
	if value.Environment != s.Environment || value.Purpose != s.Purpose || value.Status != "verified" || !validRequestID(value.RequestID) || !youtubeChannelID.MatchString(value.TargetChannel) || value.AcceptedAt.IsZero() || value.AcceptedAt.After(s.Now) || value.VerifiedAt == nil || value.VerifiedAt.Before(value.AcceptedAt) || value.VerifiedAt.After(s.Now) || !validOpaque(value.ProofRef) || !validOpaque(value.OAuthTransactionID) || value.ChallengeHash != "" || !value.ChallengeExpiresAt.IsZero() || value.CompletedAt != nil {
		return ErrDryRunUnavailable
	}
	if value.Purpose == SupportDelete {
		if value.DeleteBy == nil || !value.DeleteBy.Equal(value.AcceptedAt.Add(7*24*time.Hour)) {
			return ErrDryRunUnavailable
		}
	} else if value.DeleteBy != nil {
		return ErrDryRunUnavailable
	}
	return nil
}

func validateDryRunProof(value SupportRequest, oauth OAuthTransaction, s DryRunSelector) error {
	b := oauth.Support
	if oauth.Purpose != "support" || oauth.Status != "consumed" || b == nil || b.RequestRef != s.RequestRef || b.RequestID != value.RequestID || b.Environment != s.Environment || b.Purpose != s.Purpose || !validOpaque(b.ChallengeHash) || oauth.Channel.ID != "" || oauth.StateHash != "" || oauth.ConfirmationRef != "" || oauth.CreatedAt.IsZero() || oauth.CreatedAt.After(oauth.VerifiedAt) || oauth.VerifiedAt.IsZero() || oauth.VerifiedAt.After(oauth.ConsumedAt) || !oauth.ConsumedAt.Equal(*value.VerifiedAt) || !oauth.ExpiresAt.After(oauth.ConsumedAt) || oauth.ExpiresAt.Sub(oauth.CreatedAt) > 10*time.Minute || oauth.CreatedAt.Before(value.AcceptedAt) {
		return ErrDryRunUnavailable
	}
	return nil
}

// BuildDryRun is a sanitized diagnostic. A verified receipt and observed counts
// never authorize execution or prove cross-store cleanup.
func BuildDryRun(ctx context.Context, s DryRunSelector, reader DryRunReader) (DryRunReport, error) {
	if err := validateDryRunSelector(s); err != nil || reader == nil {
		return DryRunReport{}, ErrDryRunUnavailable
	}
	snapshot, err := reader.Inspect(ctx, s)
	if err != nil {
		return DryRunReport{}, fmt.Errorf("read operator inventory: %w", err)
	}
	if snapshot.Project != s.ExpectedProject || snapshot.AsOf.IsZero() || snapshot.AsOf.After(s.Now.Add(time.Minute)) || snapshot.AsOf.Before(s.Now.Add(-time.Minute)) {
		return DryRunReport{}, ErrDryRunUnavailable
	}
	if err := validateDryRunReceipt(snapshot.Receipt, s); err != nil {
		return DryRunReport{}, err
	}
	proofTrace := "unknown"
	if !snapshot.OAuthMissing {
		if err := validateDryRunProof(snapshot.Receipt, snapshot.OAuth, s); err != nil {
			return DryRunReport{}, err
		}
		proofTrace = "verified"
	}
	counts := make(map[string]InventoryCount, len(dryRunLookups)+len(uninspectedDryRunStores))
	for name, lookup := range dryRunLookups {
		count := snapshot.Counts[name]
		if count.Scope != lookup.scope || count.Count == nil || *count.Count < 0 || (count.State != "known" && count.State != "at_least") || (count.State == "at_least" && *count.Count != 1001) || (count.State == "known" && *count.Count > 1000) {
			count = InventoryCount{State: "unknown", Scope: lookup.scope}
		}
		counts[name] = count
	}
	for _, name := range uninspectedDryRunStores {
		counts[name] = InventoryCount{State: "unknown", Scope: "external"}
	}
	deadline := snapshot.Receipt.DeleteBy
	return DryRunReport{ReadOnly: true, ExecutionAuthorized: false, InventoryComplete: false, Purpose: s.Purpose, ProofTrace: proofTrace, Status: "verified", ObservedAt: snapshot.AsOf, DeleteBy: deadline, Overdue: deadline != nil && !s.Now.Before(*deadline), Counts: counts}, nil
}
