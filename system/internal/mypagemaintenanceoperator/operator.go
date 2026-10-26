// Package mypagemaintenanceoperator implements the fixed transition runner
// shared by distinct GitHub and break-glass binaries. Callers compile in the
// authority; it is deliberately not a command-line or environment option.
package mypagemaintenanceoperator

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"app.modules/core/mypage"

	"cloud.google.com/go/firestore"
)

var ErrOperator = errors.New("maintenance operation rejected; verify environment, fixed authority, revision and reviewed inputs")

type report struct {
	Operation        string     `json:"operation"`
	Authority        string     `json:"authority"`
	Environment      string     `json:"environment"`
	State            string     `json:"state"`
	PreviousRevision int64      `json:"previousRevision"`
	Revision         int64      `json:"revision"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	EstimatedEndAt   *time.Time `json:"estimatedEndAt,omitempty"`
	MessageTemplate  string     `json:"messageTemplate"`
	ReasonCategory   string     `json:"reasonCategory"`
	ReadbackVerified bool       `json:"readbackVerified"`
	MutationApplied  bool       `json:"mutationApplied"`
}

type ClientFactory func(context.Context, string) (*firestore.Client, error)

func Run(ctx context.Context, args []string, output io.Writer, get func(string) string, now func() time.Time, connect ClientFactory, authority string) (resultErr error) {
	if authority != mypage.MaintenanceAuthorityGitHub && authority != mypage.MaintenanceAuthorityBreakGlass {
		return ErrOperator
	}
	flags := flag.NewFlagSet("mypage-maintenance-operator", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	operation := flags.String("operation", "", "preview, apply or verify")
	project := flags.String("project", "", "fixed environment project")
	environment := flags.String("environment", "", "development or production")
	state := flags.String("state", "", "NORMAL or ACTIVE")
	expectedRevision := flags.Int64("expected-revision", -1, "authoritative compare-and-set revision")
	estimatedEnd := flags.String("estimated-end-at", "", "optional RFC3339 estimate for ACTIVE")
	reason := flags.String("reason-category", "", "fixed reason category")
	actionRef := flags.String("action-ref", "", "opaque immutable workflow action reference")
	repositoryID := flags.String("repository-id", "", "immutable repository numeric id")
	ownerID := flags.String("repository-owner-id", "", "immutable owner numeric id")
	actorID := flags.String("actor-id", "", "immutable actor numeric id")
	triggeringActorID := flags.String("triggering-actor-id", "", "immutable triggering actor numeric id")
	runID := flags.String("run-id", "", "immutable Actions run id")
	runAttempt := flags.Int64("run-attempt", 0, "must be the first attempt")
	gitRef := flags.String("git-ref", "", "reviewed branch ref")
	commitSHA := flags.String("commit-sha", "", "reviewed workflow commit")
	workflowRef := flags.String("workflow-ref", "", "immutable workflow ref")
	breakGlassApproval := flags.String("break-glass-approval-ref", "", "approved break-glass reference")
	claimedOperator := flags.String("claimed-operator", "", "operator identity claimed for Cloud Audit Log reconciliation")
	if flags.Parse(args) != nil || flags.NArg() != 0 || get("GOOGLE_APPLICATION_CREDENTIALS") != "" || get("FIRESTORE_EMULATOR_HOST") != "" || get("FIREBASE_AUTH_EMULATOR_HOST") != "" {
		return ErrOperator
	}
	wantProject := map[string]string{"development": "test-youtube-study-space", "production": "youtube-study-space"}[*environment]
	if wantProject == "" || *project != wantProject || get("GOOGLE_CLOUD_PROJECT") != *project || get("MYPAGE_ENVIRONMENT") != *environment {
		return ErrOperator
	}
	var estimate *time.Time
	if *estimatedEnd != "" {
		parsed, err := time.Parse(time.RFC3339Nano, *estimatedEnd)
		if err != nil || parsed.Format(time.RFC3339Nano) != *estimatedEnd {
			return ErrOperator
		}
		parsed = parsed.UTC()
		estimate = &parsed
	}
	request := mypage.MaintenanceTransition{Authority: authority, Environment: *environment, State: *state, ExpectedRevision: *expectedRevision, EstimatedEndAt: estimate, ReasonCategory: *reason, ActionRef: *actionRef, RepositoryID: *repositoryID, RepositoryOwnerID: *ownerID, ActorID: *actorID, TriggeringActorID: *triggeringActorID, RunID: *runID, RunAttempt: *runAttempt, GitRef: *gitRef, CommitSHA: *commitSHA, WorkflowRef: *workflowRef, CloudRunExecution: get("CLOUD_RUN_EXECUTION"), BreakGlassApprovalRef: *breakGlassApproval, ClaimedOperator: *claimedOperator}
	operationNow := now().UTC()
	if *operation != "preview" && *operation != "apply" && *operation != "verify" {
		return ErrOperator
	}
	if (*operation == "verify" && mypage.ValidateMaintenanceTransitionReceipt(request) != nil) ||
		(*operation != "verify" && mypage.ValidateMaintenanceTransition(request, operationNow) != nil) {
		return ErrOperator
	}
	client, err := connect(ctx, *project)
	if err != nil {
		return ErrOperator
	}
	defer func() {
		if client.Close() != nil {
			resultErr = ErrOperator
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var result mypage.MaintenanceTransitionResult
	switch *operation {
	case "preview":
		result, err = mypage.PreviewMaintenanceTransition(ctx, client, request, operationNow)
	case "apply":
		result, err = mypage.ApplyMaintenanceTransition(ctx, client, request, operationNow)
	case "verify":
		result, err = mypage.VerifyMaintenanceTransition(ctx, client, request, operationNow)
	}
	if err != nil {
		return ErrOperator
	}
	previousRevision := int64(0)
	if result.Previous != nil {
		previousRevision = result.Previous.Revision
	} else if *operation == "verify" {
		previousRevision = request.ExpectedRevision
	}
	value := result.Current
	if err := json.NewEncoder(output).Encode(report{Operation: *operation, Authority: request.Authority, Environment: value.Environment, State: value.State, PreviousRevision: previousRevision, Revision: value.Revision, UpdatedAt: value.UpdatedAt, EstimatedEndAt: value.EstimatedEndAt, MessageTemplate: value.MessageTemplate, ReasonCategory: value.ReasonCategory, ReadbackVerified: *operation != "preview", MutationApplied: *operation == "apply"}); err != nil {
		return fmt.Errorf("encode maintenance operation report: %w", err)
	}
	return nil
}
