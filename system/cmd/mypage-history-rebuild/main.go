// Bounded DEV-only operator migration. Normal API handlers never invoke this.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"

	"app.modules/core/repository"
	"app.modules/core/serviceaccess"
	"app.modules/core/timeutil"
)

type migrationInput struct {
	UID           string    `json:"uid"`
	Month         string    `json:"month"`
	Checkpoint    string    `json:"checkpoint"`
	SourceVersion string    `json:"sourceVersion"`
	UserRevision  time.Time `json:"userRevision"`
}

type migrationReport struct {
	Operation     string    `json:"operation"`
	ReadOnly      bool      `json:"readOnly"`
	Checkpoint    string    `json:"checkpoint"`
	SourceVersion string    `json:"sourceVersion,omitempty"`
	UserRevision  time.Time `json:"userRevision,omitempty"`
	Documents     int       `json:"documents"`
	MaxDocuments  int       `json:"maxDocuments"`
	PreviousSec   int64     `json:"previousSec"`
	RebuiltSec    int64     `json:"rebuiltSec"`
	Changed       bool      `json:"changed"`
	Applied       bool      `json:"applied"`
}

var errMigration = errors.New("bounded migration unavailable; inspect target, fence, limits and reviewed source")

func run(ctx context.Context, args []string, input io.Reader, output io.Writer, get func(string) string) (resultErr error) {
	flags := flag.NewFlagSet("mypage-history-rebuild", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	project := flags.String("project", "", "explicit existing DEV project")
	environment := flags.String("environment", "", "development only")
	operation := flags.String("operation", "rebuild", "rebuild, cutover, rollback")
	limit := flags.Int("max-documents", 1000, "raw documents in the selected UID scan; hard max 10000")
	execute := flags.Bool("execute", false, "apply a matching reviewed dry run")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *environment != "development" || (*operation != "rebuild" && *operation != "cutover" && *operation != "rollback") || *limit < 1 || *limit > 10000 {
		return errMigration
	}
	emulator := get("FIRESTORE_EMULATOR_HOST")
	if (emulator != "" && (emulator != "127.0.0.1:8080" || *project != "demo-youtube-study-space-ci")) || (emulator == "" && *project != "test-youtube-study-space") || get("GOOGLE_APPLICATION_CREDENTIALS") != "" || get("FIREBASE_AUTH_EMULATOR_HOST") != "" {
		return errMigration
	}
	for _, key := range []string{"GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT", "FIREBASE_PROJECT_ID"} {
		if value := get(key); value != "" && value != *project {
			return errMigration
		}
	}
	bytes, err := io.ReadAll(io.LimitReader(input, 4097))
	if err != nil || len(bytes) > 4096 {
		return errMigration
	}
	var selection migrationInput
	decoder := json.NewDecoder(strings.NewReader(string(bytes)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&selection) != nil || decoder.Decode(new(any)) != io.EOF || !serviceaccess.ValidChannel(selection.UID) {
		return errMigration
	}
	if *operation == "rebuild" {
		month, err := time.ParseInLocation("2006-01", selection.Month, timeutil.JapanLocation())
		if err != nil || month.Format("2006-01") != selection.Month {
			return errMigration
		}
	}
	if *execute && (selection.Checkpoint == "" || (*operation == "rebuild" && len(selection.SourceVersion) != 64) || (*operation != "rebuild" && selection.UserRevision.IsZero())) {
		return errMigration
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	clientOption := option.WithScopes("https://www.googleapis.com/auth/datastore")
	if emulator != "" {
		clientOption = option.WithoutAuthentication()
	}
	repo, err := repository.NewFirestoreControllerForProject(ctx, *project, clientOption)
	if err != nil {
		return errMigration
	}
	defer func() {
		if repo.FirestoreClient().Close() != nil {
			resultErr = errMigration
		}
	}()
	client, ok := repo.FirestoreClient().(*firestore.Client)
	if !ok {
		return errMigration
	}
	if selection.Checkpoint == "" {
		control, err := (&serviceaccess.FirestoreStore{Client: client}).Read(ctx, selection.UID)
		if err != nil || control.Control.PrivacyDeletion.Active {
			return errMigration
		}
		selection.Checkpoint = control.Checkpoint()
	}
	report := migrationReport{Operation: *operation, ReadOnly: !*execute, Checkpoint: selection.Checkpoint, MaxDocuments: *limit}
	if *operation == "rebuild" {
		value, err := repo.RebuildWorkMonth(ctx, repository.WorkMonthBackfill{UID: selection.UID, Month: selection.Month, WriterEpoch: repository.MonthlyWorkEpoch, ControlCheckpoint: selection.Checkpoint, ExpectedVersion: selection.SourceVersion, MaxDocuments: *limit, Apply: *execute})
		if err != nil {
			return errMigration
		}
		report.SourceVersion = value.SourceVersion
		report.Documents = value.Documents
		report.PreviousSec = value.PreviousSec
		report.RebuiltSec = value.RebuiltSec
		report.Changed = value.Changed
		report.Applied = value.Applied
	} else {
		if selection.UserRevision.IsZero() {
			doc, err := client.Collection(repository.USERS).Doc(selection.UID).Get(ctx)
			if err != nil {
				return errMigration
			}
			selection.UserRevision = doc.UpdateTime
		}
		if repo.SetWorkMonthReader(ctx, selection.UID, selection.Checkpoint, selection.UserRevision, *operation == "cutover", *execute) != nil {
			return errMigration
		}
		report.UserRevision = selection.UserRevision
		report.Applied = *execute
	}
	if json.NewEncoder(output).Encode(report) != nil {
		return errMigration
	}
	return nil
}

func main() {
	if run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Getenv) != nil {
		fmt.Fprintln(os.Stderr, errMigration)
		os.Exit(1)
	}
}
