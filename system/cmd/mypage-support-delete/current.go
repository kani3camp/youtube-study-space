package main

import (
	"context"
	"crypto/rand"
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
		OwnerRef   string                 `json:"ownerRef"`
		Generation int64                  `json:"generation"`
		Cutoff     time.Time              `json:"cutoff"`
		Cursor     int                    `json:"cursor"`
		UpdatedAt  time.Time              `json:"updatedAt"`
	}
)

// Lock the private checkpoint for the whole invocation. A new file gets a distinct owner.
func openCheckpoint(path string, selector supportdelete.Selector) (*os.File, string, error) {
	if path == "" || selector.Validate() != nil {
		return nil, "", errInput
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, "", errInput
	}
	fail := func() (*os.File, string, error) {
		f.Close() //nolint:errcheck // Fixed private-input diagnostic only.
		return nil, "", errInput
	}
	info, err := f.Stat()
	if err != nil || !privateRegular(info) || syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		return fail()
	}
	var stored checkpoint
	if info.Size() > 0 {
		d := json.NewDecoder(io.LimitReader(f, maxInputBytes+1))
		d.DisallowUnknownFields()
		if d.Decode(&stored) != nil || d.Decode(new(any)) != io.EOF || stored.Selector != selector || !supportdelete.ValidRef(stored.OwnerRef) {
			return fail()
		}
	} else {
		var owner [32]byte
		if _, err := rand.Read(owner[:]); err != nil {
			return fail()
		}
		stored.Selector, stored.OwnerRef = selector, fmt.Sprintf("%x", owner)
		if saveCheckpoint(f, stored) != nil {
			return fail()
		}
	}
	return f, stored.OwnerRef, nil
}

func saveCheckpoint(f *os.File, stored checkpoint) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return errInput
	}
	if f.Truncate(0) != nil || json.NewEncoder(f).Encode(stored) != nil || f.Sync() != nil {
		return errInput
	}
	return nil
}

func writeCheckpoint(f *os.File, e supportdelete.Execution) error {
	return saveCheckpoint(f, checkpoint{Selector: e.Selector, OwnerRef: e.OwnerRef, Generation: e.Generation, Cutoff: e.Cutoff, Cursor: e.Cursor, UpdatedAt: e.UpdatedAt})
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
	checkpointFile, owner, err := openCheckpoint(o.checkpoint, r)
	if err != nil {
		return out, errInput
	}
	defer checkpointFile.Close() //nolint:errcheck // Closing releases the exclusive invocation lock.
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
	out.ActualExecution = true
	e, err := store.Acquire(ctx, supportdelete.Start{Selector: r, Manifest: manifest, OwnerRef: owner, Now: time.Now()})
	if errors.Is(err, supportdelete.ErrCompleted) {
		out.Code, out.Stage = "COMPLETED", "completed"
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("current deletion run: %w", err)
	}
	if writeCheckpoint(checkpointFile, e) != nil {
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
		if writeCheckpoint(checkpointFile, next) != nil {
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
