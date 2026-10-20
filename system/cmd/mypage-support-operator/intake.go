package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"app.modules/core/mypage"

	"cloud.google.com/go/firestore"
)

var (
	errIntake     = errors.New("human Privacy intake unavailable")
	intakeChannel = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)
	intakeActor   = regexp.MustCompile(`^[A-Za-z0-9_-]{3,128}$`)
)

// Human correspondence stays at the human desk; no body/contact is copied.
// The target is a claim only, awaiting the existing request-bound fresh OAuth.
type humanRequest struct {
	RequestID        string
	Channel          string
	AcceptedAt       time.Time
	ExpectedRevision time.Time
}

type intakeIntent struct {
	Action               string
	Environment, Project string
	Purpose              mypage.SupportPurpose
	Request              humanRequest
}

type intakeGrant struct {
	Actor  string
	Intent intakeIntent
}

type supportCreator interface {
	Create(context.Context, string, string, mypage.SupportPurpose, time.Time, time.Time) (string, string, error)
}

type intakeReceipt struct {
	RequestRef, Challenge string
	Revision              time.Time
	DeleteBy              *time.Time
}

type intakeRuntime struct {
	environment, project string
	creator              supportCreator
	recovery             supportRecovery
	// A request ID, target channel, support challenge, Firebase identity or ADC
	// alone is not operator authority. No live implementation is supplied.
	authorize func(context.Context, intakeIntent) (intakeGrant, error)
	// Private handoff is separate from stdout/audit, and is not proof or an ACK
	// that the request's action has completed. No real delivery is supplied.
	deliver func(context.Context, intakeIntent, intakeReceipt) error
	clock   func() time.Time
}

type intakeFactory func(context.Context, proposal) (*intakeRuntime, error)

func intakePurpose(p mypage.SupportPurpose) bool {
	return p == mypage.SupportDelete || p == mypage.SupportRevoke || p == mypage.SupportDisclosure
}

func readHumanRequest(path, action string) (humanRequest, error) {
	data, err := privateFile(path, 1024)
	if err != nil {
		return humanRequest{}, errIntake
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return humanRequest{}, errIntake
	}
	values := make(map[string]string, 3)
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || (key != "requestId" && key != "targetChannel" && key != "acceptedAt" && (key != "expectedReceiptVersion" || action != "recover")) {
			return humanRequest{}, errIntake
		}
		if _, duplicate := values[key]; duplicate {
			return humanRequest{}, errIntake
		}
		var value string
		if decoder.Decode(&value) != nil || value == "" {
			return humanRequest{}, errIntake
		}
		values[key] = value
	}
	expectedFields := 3
	if action == "recover" {
		expectedFields = 4
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') || len(values) != expectedFields {
		return humanRequest{}, errIntake
	}
	if _, err := decoder.Token(); err != io.EOF {
		return humanRequest{}, errIntake
	}
	at, err := time.Parse(time.RFC3339Nano, values["acceptedAt"])
	if err != nil || at.IsZero() || at.After(time.Now()) || values["acceptedAt"] != at.UTC().Format(time.RFC3339Nano) || !at.Equal(at.Truncate(time.Microsecond)) || !opaque.MatchString(values["requestId"]) || !intakeChannel.MatchString(values["targetChannel"]) {
		return humanRequest{}, errIntake
	}
	input := humanRequest{RequestID: values["requestId"], Channel: values["targetChannel"], AcceptedAt: at}
	if action == "recover" {
		version, err := time.Parse(time.RFC3339Nano, values["expectedReceiptVersion"])
		if err != nil || version.IsZero() || values["expectedReceiptVersion"] != version.UTC().Format(time.RFC3339Nano) {
			return humanRequest{}, errIntake
		}
		input.ExpectedRevision = version
	}
	return input, nil
}

// newFirestoreIntakeRuntime assembles receipt creation and fenced recovery.
// Its caller owns the authorized client/identity/private-delivery boundary.
// It performs no credential discovery, authentication or network operation.
func newFirestoreIntakeRuntime(p proposal, client *firestore.Client, authorize func(context.Context, intakeIntent) (intakeGrant, error), deliver func(context.Context, intakeIntent, intakeReceipt) error, clock func() time.Time) (*intakeRuntime, error) {
	if client == nil || !project.MatchString(p.Project) || (p.Environment != "development" && p.Environment != "production") || authorize == nil || deliver == nil || clock == nil ||
		!strings.HasPrefix(client.Collection("support-requests").Path, "projects/"+p.Project+"/databases/(default)/documents/") {
		return nil, errIntake
	}
	store := &mypage.FirestoreSupportStore{Client: client, Environment: p.Environment}
	return &intakeRuntime{environment: p.Environment, project: p.Project, creator: store, recovery: store, authorize: authorize, deliver: deliver, clock: clock}, nil
}

func runHumanIntake(ctx context.Context, p proposal, input humanRequest, factory intakeFactory) string {
	if ctx == nil || ctx.Err() != nil || factory == nil {
		return "TRUSTED_INTAKE_UNAVAILABLE"
	}
	runtime, err := factory(ctx, p)
	if err != nil || ctx.Err() != nil || runtime == nil || (p.Action == "create" && runtime.creator == nil) || (p.Action != "create" && runtime.recovery == nil) || runtime.authorize == nil || runtime.deliver == nil || runtime.clock == nil || runtime.environment != p.Environment || runtime.project != p.Project {
		return "TRUSTED_INTAKE_UNAVAILABLE"
	}
	now := runtime.clock().UTC()
	if now.IsZero() || input.AcceptedAt.IsZero() || input.AcceptedAt.After(now) {
		return "INVALID_INPUT"
	}
	intent := intakeIntent{Action: p.Action, Environment: p.Environment, Project: p.Project, Purpose: p.Purpose, Request: input}
	grant, err := runtime.authorize(ctx, intent)
	if err != nil || ctx.Err() != nil || grant.Intent != intent || !intakeActor.MatchString(grant.Actor) {
		return "TRUSTED_INTAKE_DENIED"
	}
	// Read trusted time again after authorization; preserve the original receipt
	// time. Create is called once: a timeout/ACK loss must not reset its SLA.
	now = runtime.clock().UTC()
	if now.IsZero() || input.AcceptedAt.After(now) {
		return "INVALID_INPUT"
	}
	if p.Action != "create" {
		return runHumanRecovery(ctx, p, input, runtime, intent, now)
	}
	ref, challenge, err := runtime.creator.Create(ctx, input.RequestID, input.Channel, p.Purpose, input.AcceptedAt, now)
	if err != nil || ctx.Err() != nil || !opaque.MatchString(ref) || !opaque.MatchString(challenge) {
		return "INTAKE_RESULT_UNCERTAIN"
	}
	if runtime.deliver(ctx, intent, intakeReceipt{RequestRef: ref, Challenge: challenge}) != nil || ctx.Err() != nil {
		return "INTAKE_HANDOFF_PENDING"
	}
	return "MOCK_AWAITING_PROOF"
}
