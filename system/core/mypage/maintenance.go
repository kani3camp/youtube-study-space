package mypage

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	MaintenanceNormal          = "NORMAL"
	MaintenanceActive          = "ACTIVE"
	MaintenanceMessageTemplate = "MYPAGE_MAINTENANCE_V1"
	maintenanceCollection      = "mypage-runtime-control"
	maintenanceDocument        = "current"
	maintenanceEvents          = "mypage-maintenance-events"
)

var (
	numericIdentity    = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)
	commitSHA          = regexp.MustCompile(`^[0-9a-f]{40}$`)
	cloudRunExecution  = regexp.MustCompile(`^mypage-maintenance-operator-[a-z0-9-]{1,63}$`)
	breakGlassApproval = regexp.MustCompile(`^[A-Z][A-Z0-9_-]{2,63}$`)
	claimedOperator    = regexp.MustCompile(`^[a-z0-9][a-z0-9._+-]{0,63}@[a-z0-9.-]{1,190}$`)
)

const (
	MaintenanceAuthorityGitHub     = "GITHUB_ACTIONS"
	MaintenanceAuthorityBreakGlass = "GCP_BREAK_GLASS"
)

func validMaintenanceReason(reason string) bool {
	switch reason {
	case "INITIALIZATION", "PLANNED_MAINTENANCE", "DATA_MIGRATION", "SECURITY_MAINTENANCE", "RECOVERY":
		return true
	default:
		return false
	}
}

// MaintenanceControl is the authoritative per-environment singleton. The
// environment is stored in the document as a second defence against a client
// or operator pointing at the wrong Firebase project.
type MaintenanceControl struct {
	Environment     string     `firestore:"environment" json:"-"`
	State           string     `firestore:"state" json:"state"`
	Revision        int64      `firestore:"revision" json:"-"`
	UpdatedAt       time.Time  `firestore:"updatedAt" json:"updatedAt"`
	EstimatedEndAt  *time.Time `firestore:"estimatedEndAt,omitempty" json:"estimatedEndAt,omitempty"`
	MessageTemplate string     `firestore:"messageTemplate" json:"messageTemplate"`
	ReasonCategory  string     `firestore:"reasonCategory" json:"-"`
}

type MaintenanceStatus struct {
	State           string     `json:"state"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	EstimatedEndAt  *time.Time `json:"estimatedEndAt,omitempty"`
	MessageTemplate string     `json:"messageTemplate"`
}

type MaintenanceReader interface {
	Check(context.Context) error
	CheckAuthoritative(context.Context) error
	PublicStatus(context.Context) (MaintenanceStatus, error)
}

type MaintenanceGate struct {
	Client      *firestore.Client
	Environment string
	Now         func() time.Time
	TTL         time.Duration
	readControl func(context.Context) (MaintenanceControl, error)

	mu       sync.Mutex
	cached   MaintenanceControl
	expires  time.Time
	refresh  chan struct{}
	lastErr  error
	errUntil time.Time
}

func maintenanceRef(client *firestore.Client) *firestore.DocumentRef {
	return client.Collection(maintenanceCollection).Doc(maintenanceDocument)
}

func validateMaintenanceControl(value MaintenanceControl, environment string) error {
	if !validSupportEnvironment(environment) || value.Environment != environment || value.Revision < 1 || value.UpdatedAt.IsZero() ||
		(value.State != MaintenanceNormal && value.State != MaintenanceActive) || value.MessageTemplate != MaintenanceMessageTemplate || !validMaintenanceReason(value.ReasonCategory) {
		return apiError("TEMPORARY_UNAVAILABLE")
	}
	if value.State == MaintenanceNormal && value.EstimatedEndAt != nil {
		return apiError("TEMPORARY_UNAVAILABLE")
	}
	if value.EstimatedEndAt != nil && value.EstimatedEndAt.IsZero() {
		return apiError("TEMPORARY_UNAVAILABLE")
	}
	return nil
}

func decodeMaintenanceDocument(doc *firestore.DocumentSnapshot, environment string) (MaintenanceControl, error) {
	if doc == nil || !doc.Exists() {
		return MaintenanceControl{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	data := doc.Data()
	allowed := map[string]bool{"environment": true, "state": true, "revision": true, "updatedAt": true, "estimatedEndAt": true, "messageTemplate": true, "reasonCategory": true}
	for key := range data {
		if !allowed[key] {
			return MaintenanceControl{}, apiError("TEMPORARY_UNAVAILABLE")
		}
	}
	for _, key := range []string{"environment", "state", "revision", "updatedAt", "messageTemplate", "reasonCategory"} {
		if _, ok := data[key]; !ok {
			return MaintenanceControl{}, apiError("TEMPORARY_UNAVAILABLE")
		}
	}
	var value MaintenanceControl
	if err := doc.DataTo(&value); err != nil {
		return MaintenanceControl{}, fmt.Errorf("decode maintenance control: %w: %w", apiError("TEMPORARY_UNAVAILABLE"), err)
	}
	if err := validateMaintenanceControl(value, environment); err != nil {
		return MaintenanceControl{}, err
	}
	return value, nil
}

func (g *MaintenanceGate) read(ctx context.Context) (MaintenanceControl, error) {
	if g != nil && g.readControl != nil {
		return g.readControl(ctx)
	}
	if g == nil || g.Client == nil || !validSupportEnvironment(g.Environment) {
		return MaintenanceControl{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	doc, err := maintenanceRef(g.Client).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return MaintenanceControl{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	if err != nil {
		return MaintenanceControl{}, fmt.Errorf("read maintenance control: %w: %w", apiError("TEMPORARY_UNAVAILABLE"), err)
	}
	return decodeMaintenanceDocument(doc, g.Environment)
}

func (g *MaintenanceGate) cachedRead(ctx context.Context) (MaintenanceControl, error) {
	for {
		now := time.Now()
		if g.Now != nil {
			now = g.Now()
		}
		g.mu.Lock()
		if !g.expires.IsZero() && now.Before(g.expires) {
			value := g.cached
			g.mu.Unlock()
			return value, nil
		}
		if g.lastErr != nil && now.Before(g.errUntil) {
			err := g.lastErr
			g.mu.Unlock()
			return MaintenanceControl{}, err
		}
		if g.refresh != nil {
			refresh := g.refresh
			g.mu.Unlock()
			select {
			case <-refresh:
				continue
			case <-ctx.Done():
				return MaintenanceControl{}, fmt.Errorf("wait for maintenance refresh: %w", ctx.Err())
			}
		}
		refresh := make(chan struct{})
		g.refresh = refresh
		g.mu.Unlock()

		value, err := g.read(ctx)
		ttl := g.TTL
		if ttl <= 0 || ttl > 10*time.Second {
			ttl = 5 * time.Second
		}
		g.mu.Lock()
		if err == nil {
			g.cached, g.expires = value, now.Add(ttl)
			g.lastErr, g.errUntil = nil, time.Time{}
		} else {
			// A short error cache coalesces a failing burst without ever treating
			// an unavailable or malformed control document as NORMAL.
			g.lastErr, g.errUntil = err, now.Add(time.Second)
		}
		g.refresh = nil
		close(refresh)
		g.mu.Unlock()
		return value, err
	}
}

func maintenanceVerdict(value MaintenanceControl) error {
	if value.State == MaintenanceActive {
		return apiError("MAINTENANCE_ACTIVE")
	}
	if value.State != MaintenanceNormal {
		return apiError("TEMPORARY_UNAVAILABLE")
	}
	return nil
}

func (g *MaintenanceGate) Check(ctx context.Context) error {
	value, err := g.cachedRead(ctx)
	if err != nil {
		return err
	}
	return maintenanceVerdict(value)
}

func (g *MaintenanceGate) CheckAuthoritative(ctx context.Context) error {
	value, err := g.read(ctx)
	if err != nil {
		return err
	}
	return maintenanceVerdict(value)
}

func (g *MaintenanceGate) PublicStatus(ctx context.Context) (MaintenanceStatus, error) {
	value, err := g.cachedRead(ctx)
	if err != nil {
		return MaintenanceStatus{}, err
	}
	return MaintenanceStatus{State: value.State, UpdatedAt: value.UpdatedAt, EstimatedEndAt: value.EstimatedEndAt, MessageTemplate: value.MessageTemplate}, nil
}

// RequireMaintenanceBootstrapNormal is the read-only deployment barrier used
// before a serving revision containing the maintenance guard may be planned.
// It accepts only an already initialized, exact NORMAL control document.
func RequireMaintenanceBootstrapNormal(ctx context.Context, client *firestore.Client, environment string) error {
	value, err := (&MaintenanceGate{Client: client, Environment: environment}).read(ctx)
	if err != nil {
		return err
	}
	if value.State != MaintenanceNormal {
		return apiError("TEMPORARY_UNAVAILABLE")
	}
	return nil
}

func transactionMaintenance(tx *firestore.Transaction, client *firestore.Client, environment string) error {
	if client == nil || environment == "" {
		return nil // Test-only stores predating the runtime gate remain injectable.
	}
	doc, err := tx.Get(maintenanceRef(client))
	if status.Code(err) == codes.NotFound {
		return apiError("TEMPORARY_UNAVAILABLE")
	}
	if err != nil {
		return fmt.Errorf("read maintenance control in transaction: %w: %w", apiError("TEMPORARY_UNAVAILABLE"), err)
	}
	value, err := decodeMaintenanceDocument(doc, environment)
	if err != nil {
		return err
	}
	return maintenanceVerdict(value)
}

type MaintenanceTransition struct {
	Authority             string
	Environment           string
	State                 string
	ExpectedRevision      int64
	EstimatedEndAt        *time.Time
	ReasonCategory        string
	ActionRef             string
	RepositoryID          string
	RepositoryOwnerID     string
	ActorID               string
	TriggeringActorID     string
	RunID                 string
	RunAttempt            int64
	GitRef                string
	CommitSHA             string
	WorkflowRef           string
	CloudRunExecution     string
	BreakGlassApprovalRef string
	ClaimedOperator       string
}

type MaintenanceTransitionResult struct {
	Previous *MaintenanceControl
	Current  MaintenanceControl
}

type maintenanceAuditEvent struct {
	Authority             string     `firestore:"authority"`
	Environment           string     `firestore:"environment"`
	PreviousState         string     `firestore:"previousState"`
	State                 string     `firestore:"state"`
	PreviousRevision      int64      `firestore:"previousRevision"`
	Revision              int64      `firestore:"revision"`
	UpdatedAt             time.Time  `firestore:"updatedAt"`
	EstimatedEndAt        *time.Time `firestore:"estimatedEndAt,omitempty"`
	MessageTemplate       string     `firestore:"messageTemplate"`
	ReasonCategory        string     `firestore:"reasonCategory"`
	RepositoryID          string     `firestore:"repositoryId"`
	RepositoryOwnerID     string     `firestore:"repositoryOwnerId"`
	ActorID               string     `firestore:"actorId"`
	TriggeringActorID     string     `firestore:"triggeringActorId"`
	RunID                 string     `firestore:"runId"`
	RunAttempt            int64      `firestore:"runAttempt"`
	GitRef                string     `firestore:"gitRef"`
	CommitSHA             string     `firestore:"commitSha"`
	WorkflowRef           string     `firestore:"workflowRef"`
	CloudRunExecution     string     `firestore:"cloudRunExecution"`
	BreakGlassApprovalRef string     `firestore:"breakGlassApprovalRef,omitempty"`
	ClaimedOperator       string     `firestore:"claimedOperator,omitempty"`
	Result                string     `firestore:"result"`
}

func expectedMaintenanceRef(environment string) string {
	if environment == "production" {
		return "refs/heads/main"
	}
	return "refs/heads/integration/mypage-canon-20261006"
}

func ValidateMaintenanceTransition(request MaintenanceTransition, now time.Time) error {
	if !validSupportEnvironment(request.Environment) || (request.State != MaintenanceNormal && request.State != MaintenanceActive) ||
		request.ExpectedRevision < 0 || !validMaintenanceReason(request.ReasonCategory) || !validOpaque(request.ActionRef) ||
		!cloudRunExecution.MatchString(request.CloudRunExecution) || !commitSHA.MatchString(request.CommitSHA) ||
		request.GitRef != expectedMaintenanceRef(request.Environment) {
		return apiError("INVALID_REQUEST")
	}
	switch request.Authority {
	case MaintenanceAuthorityGitHub:
		if request.RunAttempt != 1 || !numericIdentity.MatchString(request.RepositoryID) || !numericIdentity.MatchString(request.RepositoryOwnerID) ||
			!numericIdentity.MatchString(request.ActorID) || !numericIdentity.MatchString(request.TriggeringActorID) || !numericIdentity.MatchString(request.RunID) ||
			request.ActorID != request.TriggeringActorID || request.WorkflowRef != "kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@"+request.GitRef ||
			request.BreakGlassApprovalRef != "" || request.ClaimedOperator != "" {
			return apiError("INVALID_REQUEST")
		}
	case MaintenanceAuthorityBreakGlass:
		if request.RepositoryID != "" || request.RepositoryOwnerID != "" || request.ActorID != "" || request.TriggeringActorID != "" ||
			request.RunID != "" || request.RunAttempt != 0 || request.WorkflowRef != "" ||
			!breakGlassApproval.MatchString(request.BreakGlassApprovalRef) || !claimedOperator.MatchString(request.ClaimedOperator) {
			return apiError("INVALID_REQUEST")
		}
	default:
		return apiError("INVALID_REQUEST")
	}
	if request.State == MaintenanceNormal && request.EstimatedEndAt != nil {
		return apiError("INVALID_REQUEST")
	}
	if request.EstimatedEndAt != nil && (!request.EstimatedEndAt.After(now) || request.EstimatedEndAt.After(now.Add(14*24*time.Hour))) {
		return apiError("INVALID_REQUEST")
	}
	if request.ExpectedRevision == 0 && (request.State != MaintenanceNormal || request.ReasonCategory != "INITIALIZATION") {
		return apiError("INVALID_REQUEST")
	}
	if request.ExpectedRevision > 0 && request.ReasonCategory == "INITIALIZATION" {
		return apiError("INVALID_REQUEST")
	}
	return nil
}

func prepareMaintenanceTransition(previous *MaintenanceControl, request MaintenanceTransition, now time.Time) (MaintenanceTransitionResult, error) {
	if previous == nil {
		if request.ExpectedRevision != 0 || request.State != MaintenanceNormal || request.ReasonCategory != "INITIALIZATION" {
			return MaintenanceTransitionResult{}, apiError("INVALID_REQUEST")
		}
	} else {
		if previous.Revision != request.ExpectedRevision || request.ExpectedRevision == 0 {
			return MaintenanceTransitionResult{}, apiError("MAINTENANCE_REVISION_MISMATCH")
		}
		if (previous.State == MaintenanceNormal && request.State != MaintenanceActive) ||
			(previous.State == MaintenanceActive && request.State != MaintenanceNormal) {
			return MaintenanceTransitionResult{}, apiError("INVALID_REQUEST")
		}
	}
	next := MaintenanceControl{Environment: request.Environment, State: request.State, Revision: request.ExpectedRevision + 1, UpdatedAt: now.UTC(), EstimatedEndAt: request.EstimatedEndAt, MessageTemplate: MaintenanceMessageTemplate, ReasonCategory: request.ReasonCategory}
	if err := validateMaintenanceControl(next, request.Environment); err != nil {
		return MaintenanceTransitionResult{}, err
	}
	return MaintenanceTransitionResult{Previous: previous, Current: next}, nil
}

// PreviewMaintenanceTransition performs the authoritative read and the exact
// transition validation without creating either the control or audit event.
func PreviewMaintenanceTransition(ctx context.Context, client *firestore.Client, request MaintenanceTransition, now time.Time) (MaintenanceTransitionResult, error) {
	if client == nil || ValidateMaintenanceTransition(request, now.UTC()) != nil {
		return MaintenanceTransitionResult{}, apiError("INVALID_REQUEST")
	}
	doc, err := maintenanceRef(client).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return prepareMaintenanceTransition(nil, request, now)
	}
	if err != nil {
		return MaintenanceTransitionResult{}, fmt.Errorf("preview maintenance transition: %w", err)
	}
	previous, err := decodeMaintenanceDocument(doc, request.Environment)
	if err != nil {
		return MaintenanceTransitionResult{}, err
	}
	return prepareMaintenanceTransition(&previous, request, now)
}

// ApplyMaintenanceTransition performs the compare-and-set and immutable audit
// append in one transaction. The returned readback is fetched after commit and
// compared with the committed revision before success is reported.
func ApplyMaintenanceTransition(ctx context.Context, client *firestore.Client, request MaintenanceTransition, now time.Time) (MaintenanceTransitionResult, error) {
	now = now.UTC()
	if client == nil || ValidateMaintenanceTransition(request, now) != nil {
		return MaintenanceTransitionResult{}, apiError("INVALID_REQUEST")
	}
	controlRef := maintenanceRef(client)
	eventRef := client.Collection(maintenanceEvents).Doc(request.ActionRef)
	var result MaintenanceTransitionResult
	err := client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(controlRef)
		var previous *MaintenanceControl
		if status.Code(err) == codes.NotFound {
			if request.ExpectedRevision != 0 {
				return apiError("MAINTENANCE_REVISION_MISMATCH")
			}
		} else if err != nil {
			return fmt.Errorf("read maintenance control for transition: %w", err)
		} else {
			value, err := decodeMaintenanceDocument(doc, request.Environment)
			if err != nil {
				return err
			}
			previous = &value
		}
		prepared, err := prepareMaintenanceTransition(previous, request, now)
		if err != nil {
			return err
		}
		next := prepared.Current
		previousState := "MISSING"
		if previous != nil {
			previousState = previous.State
		}
		event := maintenanceAuditEvent{Authority: request.Authority, Environment: request.Environment, PreviousState: previousState, State: next.State, PreviousRevision: request.ExpectedRevision, Revision: next.Revision, UpdatedAt: now, EstimatedEndAt: next.EstimatedEndAt, MessageTemplate: next.MessageTemplate, ReasonCategory: next.ReasonCategory, RepositoryID: request.RepositoryID, RepositoryOwnerID: request.RepositoryOwnerID, ActorID: request.ActorID, TriggeringActorID: request.TriggeringActorID, RunID: request.RunID, RunAttempt: request.RunAttempt, GitRef: request.GitRef, CommitSHA: request.CommitSHA, WorkflowRef: request.WorkflowRef, CloudRunExecution: request.CloudRunExecution, BreakGlassApprovalRef: request.BreakGlassApprovalRef, ClaimedOperator: request.ClaimedOperator, Result: "COMMITTED"}
		if err := tx.Create(eventRef, event); err != nil {
			return fmt.Errorf("append maintenance audit event: %w", err)
		}
		if previous == nil {
			if err := tx.Create(controlRef, next); err != nil {
				return fmt.Errorf("initialize maintenance control: %w", err)
			}
		} else if err := tx.Set(controlRef, next); err != nil {
			return fmt.Errorf("update maintenance control: %w", err)
		}
		result = MaintenanceTransitionResult{Previous: previous, Current: next}
		return nil
	})
	if err != nil {
		return MaintenanceTransitionResult{}, fmt.Errorf("apply maintenance transition: %w", err)
	}
	readback, err := (&MaintenanceGate{Client: client, Environment: request.Environment}).read(ctx)
	if err != nil {
		return MaintenanceTransitionResult{}, fmt.Errorf("read back maintenance transition: %w", err)
	}
	if readback.Revision != result.Current.Revision || readback.State != result.Current.State || !readback.UpdatedAt.Equal(result.Current.UpdatedAt) {
		return MaintenanceTransitionResult{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	result.Current = readback
	return result, nil
}

func sameOptionalTime(left, right *time.Time) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && left.Equal(*right))
}

func maintenanceAuditProvenanceMatches(event maintenanceAuditEvent, request MaintenanceTransition) bool {
	if request.Authority == MaintenanceAuthorityGitHub {
		return event.RepositoryID == request.RepositoryID && event.RepositoryOwnerID == request.RepositoryOwnerID &&
			event.ActorID == request.ActorID && event.TriggeringActorID == request.TriggeringActorID && event.RunID == request.RunID &&
			event.RunAttempt == request.RunAttempt && event.GitRef == request.GitRef && event.WorkflowRef == request.WorkflowRef &&
			event.BreakGlassApprovalRef == "" && event.ClaimedOperator == ""
	}
	return event.RepositoryID == "" && event.RepositoryOwnerID == "" && event.ActorID == "" && event.TriggeringActorID == "" &&
		event.RunID == "" && event.RunAttempt == 0 && event.GitRef == request.GitRef && event.WorkflowRef == "" &&
		event.BreakGlassApprovalRef == request.BreakGlassApprovalRef && event.ClaimedOperator == request.ClaimedOperator
}

// VerifyMaintenanceTransition is a non-mutating receipt check. It requires the
// committed control and immutable audit event to match the requested CAS.
func VerifyMaintenanceTransition(ctx context.Context, client *firestore.Client, request MaintenanceTransition, now time.Time) (MaintenanceTransitionResult, error) {
	if client == nil || ValidateMaintenanceTransition(request, now.UTC()) != nil {
		return MaintenanceTransitionResult{}, apiError("INVALID_REQUEST")
	}
	current, err := (&MaintenanceGate{Client: client, Environment: request.Environment}).read(ctx)
	if err != nil || current.Revision != request.ExpectedRevision+1 || current.State != request.State ||
		current.ReasonCategory != request.ReasonCategory || !sameOptionalTime(current.EstimatedEndAt, request.EstimatedEndAt) {
		return MaintenanceTransitionResult{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	doc, err := client.Collection(maintenanceEvents).Doc(request.ActionRef).Get(ctx)
	if err != nil {
		return MaintenanceTransitionResult{}, fmt.Errorf("read maintenance audit receipt: %w", err)
	}
	var event maintenanceAuditEvent
	if doc.DataTo(&event) != nil || event.Authority != request.Authority || event.Environment != request.Environment || event.State != request.State ||
		event.PreviousRevision != request.ExpectedRevision || event.Revision != current.Revision || event.ReasonCategory != request.ReasonCategory ||
		event.CloudRunExecution == "" || event.Result != "COMMITTED" || event.CommitSHA != request.CommitSHA ||
		!maintenanceAuditProvenanceMatches(event, request) || !sameOptionalTime(event.EstimatedEndAt, request.EstimatedEndAt) {
		return MaintenanceTransitionResult{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	return MaintenanceTransitionResult{Current: current}, nil
}
