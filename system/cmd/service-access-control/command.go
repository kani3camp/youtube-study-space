package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"app.modules/core/serviceaccess"
)

type ports struct {
	Store  serviceaccess.Store
	Revoke func(context.Context, string) error
	Close  func() error
}

type bootstrap func(context.Context, target) (ports, error)

// Reports contain only fixed status/code/state vocabulary and a counter. The
// target, references, timestamps, filesystem paths and SDK errors stay private.
type report struct {
	SchemaVersion   int    `json:"schemaVersion"`
	Mode            string `json:"mode"`
	Operation       string `json:"operation"`
	Offline         bool   `json:"offline"`
	Status          string `json:"status"`
	Code            string `json:"code"`
	Control         string `json:"control"`
	Moderation      string `json:"moderation"`
	PrivacyDeletion string `json:"privacyDeletion"`
	Generation      int64  `json:"generation,omitempty"`
	ReasonCode      string `json:"reasonCode,omitempty"`
	Revoke          string `json:"revoke"`
}

func initialReport() report {
	return report{SchemaVersion: 1, Mode: "plan", Operation: "unknown", Offline: true, Status: "refused", Code: "INVALID_ARGUMENTS", Control: "unknown", Moderation: "unknown", PrivacyDeletion: "unknown", Revoke: "not-run"}
}

func recordSnapshot(out *report, value serviceaccess.Snapshot) {
	out.Control = "allowed"
	out.Moderation, out.PrivacyDeletion = "inactive", "inactive"
	if !value.Exists {
		out.Control = "absent"
		return
	}
	out.Generation = value.Control.Generation
	if value.Control.Moderation.Active {
		out.Control, out.Moderation = "restricted", "active"
		out.ReasonCode = value.Control.Moderation.ReasonCode
	}
	if value.Control.PrivacyDeletion.Active {
		out.Control, out.PrivacyDeletion = "deletion", "active"
	}
}

func execute(ctx context.Context, r request, p ports, now time.Time, out *report) int {
	out.Offline = false
	out.Status, out.Code = "unavailable", "CONTROL_UNAVAILABLE"
	if p.Store == nil {
		return 1
	}
	var value serviceaccess.Snapshot
	var err error
	if r.Operation == "inspect" {
		value, err = p.Store.Read(ctx, r.Target.ChannelID)
	} else {
		change := serviceaccess.Change{Reason: serviceaccess.Moderation}
		if r.Operation == "block" {
			change.Active, change.ReasonCode, change.Reference = true, r.ReasonCode, r.ActionRef
		}
		value, err = p.Store.Change(ctx, r.Target.ChannelID, change, now)
	}
	if err != nil || value.Validate() != nil {
		return 1
	}
	if r.Operation != "inspect" {
		moderation := value.Control.Moderation
		if (r.Operation == "unblock" && moderation.Active) || (r.Operation == "block" && (!moderation.Active || moderation.ActionRef != r.ActionRef || moderation.ReasonCode != r.ReasonCode)) {
			out.Code = "CONTROL_CONFLICT"
			return 1
		}
	}
	recordSnapshot(out, value)
	out.Status, out.Code = "succeeded", "INSPECTED"
	if r.Operation == "unblock" {
		out.Code = "MODERATION_CLEARED"
	}
	if r.Operation == "block" {
		// A committed and validated guard is the primary enforcement boundary.
		// Retry revocation even on an idempotent block to recover partial failure.
		out.Code = "MODERATION_ACTIVE"
		if p.Revoke == nil || p.Revoke(ctx, r.Target.ChannelID) != nil {
			out.Status, out.Code, out.Revoke = "partial-failure", "REVOKE_UNAVAILABLE", "failed"
			return 3
		}
		out.Revoke = "succeeded"
	}
	return 0
}

func command(ctx context.Context, args []string, get func(string) string, stdin io.Reader, stdout, stderr io.Writer, start bootstrap, now func() time.Time) int {
	out := initialReport()
	finish := func(exitCode int) int {
		if json.NewEncoder(stdout).Encode(out) != nil {
			fmt.Fprintln(stderr, "Service access control report unavailable") //nolint:errcheck // Best-effort fixed diagnostic; raw writer errors and private inputs never become output.
			return 1
		}
		return exitCode
	}
	o, err := parseOptions(args)
	if err != nil {
		return finish(2)
	}
	out.Mode = o.mode
	config, err := configuredTarget(get)
	if err != nil {
		out.Code = "CONFIGURATION_INVALID"
		return finish(2)
	}
	input, closeInput, err := inputFrom(o, stdin)
	if err != nil {
		out.Code = "INVALID_INPUT"
		return finish(2)
	}
	r, err := readRequest(input)
	closeInput()
	if err != nil {
		out.Code = "INVALID_INPUT"
		return finish(2)
	}
	out.Operation = r.Operation
	if r.Target.Environment != config.Environment || r.Target.ProjectID != config.ProjectID {
		out.Code = "TARGET_MISMATCH"
		return finish(2)
	}
	if o.mode == "check-config" {
		out.Status, out.Code = "configuration-valid", "CONFIGURATION_VALID"
		return finish(0)
	}
	if o.mode == "plan" {
		out.Status, out.Code = "planned", "PLAN_ONLY"
		return finish(0)
	}
	if (config.Environment == "production") != o.allowProduction {
		out.Code = "PRODUCTION_ACK_MISMATCH"
		return finish(2)
	}
	out.Offline = false
	out.Status, out.Code = "unavailable", "BOOTSTRAP_UNAVAILABLE"
	if start == nil || ctx.Err() != nil {
		return finish(1)
	}
	p, err := start(ctx, r.Target)
	if err != nil {
		return finish(1)
	}
	if p.Close != nil {
		defer p.Close() //nolint:errcheck // SDK cleanup failure cannot reverse a committed restriction or expose dependency detail.
	}
	return finish(execute(ctx, r, p, now().UTC(), &out))
}
