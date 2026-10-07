// Package supportdelete coordinates request-bound deletion. It contains no SDK
// bootstrap: trusted stores and effect adapters must be supplied explicitly.
package supportdelete

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"time"

	"app.modules/core/serviceaccess"
)

var (
	ErrInvalid     = errors.New("invalid deletion selection")
	ErrUnavailable = errors.New("deletion adapter unavailable")
	ErrEvidence    = errors.New("deletion evidence incomplete")
	ErrConflict    = errors.New("deletion checkpoint conflict")
	ErrBusy        = errors.New("deletion execution owned")
	ErrCompleted   = errors.New("deletion execution completed")
	opaque         = regexp.MustCompile(`^[a-f0-9]{64}$`)
	project        = regexp.MustCompile(`^[a-z][a-z0-9-]{4,61}[a-z0-9]$`)
)

type Target struct {
	Environment string `json:"environment" firestore:"environment"`
	ProjectID   string `json:"projectID" firestore:"projectID"`
	ChannelID   string `json:"channelID" firestore:"channelID"`
}
type Selector struct {
	Target       Target `json:"target" firestore:"target"`
	RequestRef   string `json:"requestRef" firestore:"requestRef"`
	ExecutionRef string `json:"executionRef" firestore:"executionRef"`
	ProofRef     string `json:"proofRef" firestore:"proofRef"`
}

func (s Selector) Validate() error {
	if (s.Target.Environment != "development" && s.Target.Environment != "production") || !project.MatchString(s.Target.ProjectID) || !serviceaccess.ValidChannel(s.Target.ChannelID) || !ValidRef(s.RequestRef) || !ValidRef(s.ExecutionRef) || !ValidRef(s.ProofRef) {
		return ErrInvalid
	}
	return nil
}
func ValidRef(s string) bool { return opaque.MatchString(s) }

// Manifest is supplied by a trusted adapter registry, never a user manifest.
// Its digest identifies the exact writer fleet, restore policy and prior state.
type Manifest struct {
	Ref  string `firestore:"ref"`
	Mode string `firestore:"mode"`
}

func (m Manifest) Validate() error {
	if !ValidRef(m.Ref) || (m.Mode != "mock" && m.Mode != "emulator" && m.Mode != "live") {
		return ErrInvalid
	}
	return nil
}

type Execution struct {
	Selector       Selector  `firestore:"selector"`
	Manifest       Manifest  `firestore:"manifest"`
	OwnerRef       string    `firestore:"ownerRef"`
	Revision       int64     `firestore:"revision"`
	Generation     int64     `firestore:"generation"`
	GuardSince     time.Time `firestore:"guardSince"`
	Cutoff         time.Time `firestore:"cutoff"`
	AcceptedAt     time.Time `firestore:"acceptedAt"`
	DeleteBy       time.Time `firestore:"deleteBy"`
	UpdatedAt      time.Time `firestore:"updatedAt"`
	Cursor         int       `firestore:"cursor"`
	EvidenceDigest string    `firestore:"evidenceDigest"`
}

func (e Execution) Validate() error {
	if e.Selector.Validate() != nil || e.Manifest.Validate() != nil || !ValidRef(e.OwnerRef) || e.Revision < 1 || e.Generation < 1 || e.GuardSince.IsZero() || !e.Cutoff.Equal(e.GuardSince) || e.AcceptedAt.IsZero() || e.AcceptedAt.After(e.Cutoff) || !e.DeleteBy.Equal(e.AcceptedAt.Add(7*24*time.Hour)) || e.UpdatedAt.Before(e.Cutoff) || e.Cursor < 0 || e.Cursor > len(Steps()) || !ValidRef(e.EvidenceDigest) {
		return ErrInvalid
	}
	return nil
}

type Start struct {
	Selector Selector
	Manifest Manifest
	OwnerRef string
	Now      time.Time
}
type Recovery struct {
	Selector                                             Selector
	PreviousOwnerRef, NewOwnerRef, ManifestRef, TraceRef string
	Revision, Generation                                 int64
	Stopped                                              bool
	ObservedAt                                           time.Time
}

// Acquire atomically consumes proof into a single execution and turns on the
// privacy guard. Refresh/Commit/Finalize fence owner, revision, request and
// control generation. There is no lease timeout or implicit takeover.
type Store interface {
	Acquire(context.Context, Start) (Execution, error)
	Refresh(context.Context, Execution, time.Time) (Execution, error)
	Commit(context.Context, Execution, Evidence, time.Time) (Execution, error)
	Finalize(context.Context, Execution, time.Time) error
	Recover(context.Context, Recovery, time.Time) error
}
type Step struct{ Action, Scope string }

func RuntimeScopes() []string { return []string{"runtime-my-page", "runtime-legacy"} }
func DeletionScopes() []string {
	return []string{"seat-state", "firestore-primary", "firestore-web", "firestore-oauth", "firestore-support-relations", "legacy-mappings", "firebase-auth", "bigquery", "backups-exports", "derived-vendor", "platform-logs"}
}

func Steps() []Step {
	steps := []Step{{"revoke", "firebase-auth"}}
	for _, s := range RuntimeScopes() {
		steps = append(steps, Step{"pause", s})
	}
	for _, s := range RuntimeScopes() {
		steps = append(steps, Step{"drain", s})
	}
	for _, s := range DeletionScopes() {
		steps = append(steps, Step{"delete", s})
	}
	for _, s := range DeletionScopes() {
		steps = append(steps, Step{"inspect", s})
	}
	for _, s := range RuntimeScopes() {
		steps = append(steps, Step{"resume", s})
	}
	return steps
}
func InspectionStart() int { return 1 + 2*len(RuntimeScopes()) + len(DeletionScopes()) }
func ResumeStart() int     { return InspectionStart() + len(DeletionScopes()) }
func (e Execution) Stage() string {
	if e.Cursor >= len(Steps()) {
		return "finalize"
	}
	return Steps()[e.Cursor].Action
}

type Operation struct {
	Execution     Execution
	Step          Step
	ID            string
	ObservedAfter time.Time
}

func NewOperation(e Execution, now time.Time) Operation {
	step := Steps()[e.Cursor]
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(e.Selector.ExecutionRef+":"+step.Action+":"+step.Scope)))
	return Operation{Execution: e, Step: step, ID: id, ObservedAfter: now}
}

type Evidence struct {
	Selector                                                         Selector
	ManifestRef, OperationID, Action, Scope, Mode, TraceRef          string
	Generation                                                       int64
	Cutoff, ObservedAt                                               time.Time
	Known, Complete, AllInstances, OldQueueRejected, RestoreExcluded bool
	Remaining, Pending                                               int64
}

func (v Evidence) Validate(op Operation, now time.Time) error {
	e := op.Execution
	if !v.Known || !v.Complete || v.Selector != e.Selector || v.ManifestRef != e.Manifest.Ref || v.Mode != e.Manifest.Mode || v.OperationID != op.ID || !ValidRef(v.TraceRef) || v.Action != op.Step.Action || v.Scope != op.Step.Scope || v.Generation != e.Generation || !v.Cutoff.Equal(e.Cutoff) || v.ObservedAt.Before(op.ObservedAfter) || v.ObservedAt.After(now.Add(time.Minute)) || v.Remaining != 0 || v.Pending != 0 {
		return ErrEvidence
	}
	if (v.Action == "pause" || v.Action == "drain" || v.Action == "resume") && (!v.AllInstances || !v.OldQueueRejected) {
		return ErrEvidence
	}
	if (v.Action == "delete" || v.Action == "inspect") && !v.RestoreExcluded {
		return ErrEvidence
	}
	return nil
}

// Apply must reconcile its stable operation ID after timeout/lost acknowledgement.
// Returning unknown is safe; inferring fleet drain from handler counts is not.
type Effects interface {
	Apply(context.Context, Operation) (Evidence, error)
}
type Outcome struct {
	Code, Stage, Mode        string
	Offline, ActualExecution bool
}

func EvidenceDigest(previous string, v Evidence) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(previous+":"+v.OperationID+":"+v.TraceRef)))
}
