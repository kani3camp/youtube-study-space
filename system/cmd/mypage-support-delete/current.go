package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"app.modules/core/mybigquery"
	"app.modules/core/mypage"
	"app.modules/core/supportdelete"
)

type (
	liveRun    func(context.Context, request, options) (supportdelete.Outcome, error)
	checkpoint struct {
		Selector   supportdelete.Selector `json:"selector"`
		Generation int64                  `json:"generation"`
		Cutoff     time.Time              `json:"cutoff"`
		Cursor     int                    `json:"cursor"`
		UpdatedAt  time.Time              `json:"updatedAt"`
	}
)

func writeCheckpoint(path string, e supportdelete.Execution) error {
	if path == "" {
		return errInput
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		return errInput
	}
	defer f.Close() //nolint:errcheck // Fixed diagnostic only; never disclose a private path.
	info, err := f.Stat()
	if err != nil || !privateRegular(info) {
		return errInput
	}
	if info.Size() > 0 {
		var old checkpoint
		d := json.NewDecoder(io.LimitReader(f, maxInputBytes))
		d.DisallowUnknownFields()
		if d.Decode(&old) != nil || old.Selector != e.Selector {
			return errInput
		}
	}
	if f.Truncate(0) != nil {
		return errInput
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return errInput
	}
	if json.NewEncoder(f).Encode(checkpoint{Selector: e.Selector, Generation: e.Generation, Cutoff: e.Cutoff, Cursor: e.Cursor, UpdatedAt: e.UpdatedAt}) != nil || f.Sync() != nil {
		return errInput
	}
	return nil
}

func readGates(path string) (mypage.OwnerDeletionGates, error) {
	var gates mypage.OwnerDeletionGates
	if path == "" {
		return gates, mypage.ErrDeletionGate
	}
	input, closeInput, err := inputFrom(options{manifest: path}, nil)
	if err != nil {
		return gates, err
	}
	defer closeInput()
	decoder := json.NewDecoder(io.LimitReader(input, maxInputBytes+1))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&gates) != nil || decoder.Decode(new(any)) != io.EOF {
		return gates, errInput
	}
	return gates, nil
}

func executeCurrent(ctx context.Context, r request, o options) (supportdelete.Outcome, error) {
	out := supportdelete.Outcome{Mode: "live", Code: "DELETION_INCOMPLETE", Stage: "blocked"}
	expected := map[string]string{"development": "test-youtube-study-space", "production": "youtube-study-space"}
	if ctx == nil || r.Validate() != nil || expected[r.Target.Environment] != r.Target.ProjectID || o.checkpoint == "" || (r.Target.Environment == "production" && !o.allowProduction) {
		return out, errInput
	}
	credentials, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil || (credentials.ProjectID != "" && credentials.ProjectID != r.Target.ProjectID) {
		return out, errInput
	}
	database, err := firestore.NewClient(ctx, r.Target.ProjectID, option.WithCredentials(credentials))
	if err != nil {
		return out, errInput
	}
	defer database.Close() //nolint:errcheck // Never expose SDK diagnostics.
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: r.Target.ProjectID}, option.WithCredentials(credentials))
	if err != nil {
		return out, errInput
	}
	authClient, err := app.Auth(ctx)
	if err != nil {
		return out, errInput
	}
	bq, err := bigquery.NewClient(ctx, r.Target.ProjectID, option.WithCredentials(credentials))
	if err != nil {
		return out, errInput
	}
	defer bq.Close() //nolint:errcheck // Never expose SDK diagnostics.
	store := &mypage.FirestoreDeletionStore{Client: database}
	manifest := supportdelete.Manifest{Ref: fmt.Sprintf("%x", sha256.Sum256([]byte("current-stores-v1:channel-uid:firestore:firestore_export:owner-gates"))), Mode: "live"}
	owner := fmt.Sprintf("%x", sha256.Sum256([]byte("owner-cli:"+r.ExecutionRef)))
	out.ActualExecution = true
	e, err := store.Acquire(ctx, supportdelete.Start{Selector: r, Manifest: manifest, OwnerRef: owner, Now: time.Now()})
	if errors.Is(err, supportdelete.ErrCompleted) {
		out.Code, out.Stage = "COMPLETED", "completed"
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("current deletion run: %w", err)
	}
	if writeCheckpoint(o.checkpoint, e) != nil {
		return out, errInput
	}
	gates, err := readGates(o.gates)
	if err != nil {
		out.Code = "MANUAL_GATE_REQUIRED"
		return out, mypage.ErrDeletionGate
	}
	effects := &mypage.CurrentDeletionEffects{Store: store, Auth: authClient, BigQuery: &mybigquery.BigqueryController{Client: bq}, Gates: gates, Clock: time.Now}
	runner := supportdelete.Engine{Store: store, Effects: effects, Manifest: manifest, Clock: time.Now}
	result, err := runner.Run(ctx, r, owner)
	// Never extend interruption handling indefinitely; Firestore is the authoritative checkpoint.
	reportCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if next, loadErr := store.Acquire(reportCtx, supportdelete.Start{Selector: r, Manifest: manifest, OwnerRef: owner, Now: time.Now()}); loadErr == nil {
		if writeCheckpoint(o.checkpoint, next) != nil {
			return out, errInput
		}
	}
	if errors.Is(err, mypage.ErrDeletionGate) {
		out.Code = "MANUAL_GATE_REQUIRED"
		return out, fmt.Errorf("current deletion run: %w", err)
	}
	if err != nil {
		return result, fmt.Errorf("current deletion checkpoint: %w", err)
	}
	return result, nil
}
