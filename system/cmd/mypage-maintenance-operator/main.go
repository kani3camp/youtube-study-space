// MyPage maintenance mode's Cloud Run Job entrypoint. It accepts only the
// fixed v1 transition schema and never prints document contents or credentials.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"app.modules/core/mypage"

	"cloud.google.com/go/firestore"
)

var errOperator = errors.New("maintenance operation rejected; verify environment, immutable GitHub identity, revision and reviewed inputs")

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

type clientFactory func(context.Context, string) (*firestore.Client, error)

func run(ctx context.Context, args []string, output io.Writer, get func(string) string, now func() time.Time, connect clientFactory) (resultErr error) {
	flags := flag.NewFlagSet("mypage-maintenance-operator", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	operation := flags.String("operation", "", "preview, apply or verify")
	authority := flags.String("authority", "", "GITHUB_ACTIONS or GCP_BREAK_GLASS")
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
		return errOperator
	}
	wantProject := map[string]string{"development": "test-youtube-study-space", "production": "youtube-study-space"}[*environment]
	if wantProject == "" || *project != wantProject || get("GOOGLE_CLOUD_PROJECT") != *project || get("MYPAGE_ENVIRONMENT") != *environment {
		return errOperator
	}
	var estimate *time.Time
	if *estimatedEnd != "" {
		parsed, err := time.Parse(time.RFC3339, *estimatedEnd)
		if err != nil || parsed.Format(time.RFC3339) != *estimatedEnd {
			return errOperator
		}
		parsed = parsed.UTC()
		estimate = &parsed
	}
	request := mypage.MaintenanceTransition{Authority: *authority, Environment: *environment, State: *state, ExpectedRevision: *expectedRevision, EstimatedEndAt: estimate, ReasonCategory: *reason, ActionRef: *actionRef, RepositoryID: *repositoryID, RepositoryOwnerID: *ownerID, ActorID: *actorID, TriggeringActorID: *triggeringActorID, RunID: *runID, RunAttempt: *runAttempt, GitRef: *gitRef, CommitSHA: *commitSHA, WorkflowRef: *workflowRef, CloudRunExecution: get("CLOUD_RUN_EXECUTION"), BreakGlassApprovalRef: *breakGlassApproval, ClaimedOperator: *claimedOperator}
	operationNow := now().UTC()
	if (*operation != "preview" && *operation != "apply" && *operation != "verify") || mypage.ValidateMaintenanceTransition(request, operationNow) != nil {
		return errOperator
	}
	client, err := connect(ctx, *project)
	if err != nil {
		return errOperator
	}
	defer func() {
		if client.Close() != nil {
			resultErr = errOperator
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
		return errOperator
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

func main() {
	err := run(context.Background(), os.Args[1:], os.Stdout, os.Getenv, time.Now, func(ctx context.Context, project string) (*firestore.Client, error) {
		return firestore.NewClient(ctx, project)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, errOperator)
		os.Exit(1)
	}
}
