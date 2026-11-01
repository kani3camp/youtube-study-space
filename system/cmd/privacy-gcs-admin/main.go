package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"app.modules/core/repository"
	"app.modules/core/utils"
)

const (
	rawLiveChatPathMarker         = "/all_namespaces/kind_live-chat-history/"
	firestoreCountAlias           = "row_count"
	minimumCleanSnapshotsAfterRaw = 7
	maximumLatestSnapshotAge      = 72 * time.Hour
)

type objectRef struct {
	Name       string
	Generation int64
	Size       int64
	Created    time.Time
}

type prefixInventory struct {
	Prefix          string
	Objects         []objectRef
	ObjectCount     int64
	Bytes           int64
	LatestCreatedAt time.Time
	ContainsRawChat bool
}

type bucketInventory struct {
	Prefixes          []prefixInventory
	VersioningEnabled bool
	SoftDelete        time.Duration
}

type safetyCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type previewOutput struct {
	GeneratedAt                      time.Time      `json:"generated_at"`
	Target                           gcsPurgeTarget `json:"target"`
	FirestoreRawChatRows             int64          `json:"firestore_raw_chat_rows"`
	VersioningEnabled                bool           `json:"versioning_enabled"`
	SoftDeleteRetention              string         `json:"soft_delete_retention,omitempty"`
	TotalSnapshotPrefixes            int            `json:"total_snapshot_prefixes"`
	RawChatSnapshotPrefixes          int            `json:"raw_chat_snapshot_prefixes"`
	CandidateRawChatSnapshotPrefixes int            `json:"candidate_raw_chat_snapshot_prefixes"`
	CandidateObjectCount             int64          `json:"candidate_object_count"`
	CandidateBytes                   int64          `json:"candidate_bytes"`
	OldestRawChatPrefix              string         `json:"oldest_raw_chat_prefix,omitempty"`
	NewestRawChatPrefix              string         `json:"newest_raw_chat_prefix,omitempty"`
	NewestRawChatCreatedAt           string         `json:"newest_raw_chat_created_at,omitempty"`
	CleanSnapshotsAfterNewestRaw     int            `json:"clean_snapshots_after_newest_raw"`
	NewestCleanPrefix                string         `json:"newest_clean_prefix,omitempty"`
	LatestSnapshotPrefix             string         `json:"latest_snapshot_prefix,omitempty"`
	LatestSnapshotCreatedAt          string         `json:"latest_snapshot_created_at,omitempty"`
	SafetyChecks                     []safetyCheck  `json:"safety_checks"`
	ReadyToApply                     bool           `json:"ready_to_apply"`
	RequiredConfirmation             string         `json:"required_confirmation,omitempty"`
	Notes                            []string       `json:"notes,omitempty"`
}

func main() {
	if err := run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "privacy-gcs-admin:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) < 5 {
		return usageError()
	}

	command := strings.TrimSpace(args[1])
	environment := strings.TrimSpace(args[2])
	expectedProjectID := strings.TrimSpace(args[3])
	expectedBucketName := strings.TrimSpace(args[4])
	if command == "preview" && len(args) != 5 {
		return usageError()
	}
	if command != "preview" && command != "apply" {
		return usageError()
	}

	utils.LoadEnv(".env")
	credentialFilePath := strings.TrimSpace(os.Getenv("CREDENTIAL_FILE_LOCATION"))
	if credentialFilePath == "" {
		return errors.New("CREDENTIAL_FILE_LOCATION is required")
	}
	//nolint:staticcheck // Operator-controlled service-account JSON used only by this admin command.
	clientOption := option.WithCredentialsFile(credentialFilePath)

	repo, err := repository.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("initialize Firestore: %w", err)
	}
	defer func() {
		if err := repo.FirestoreClient().Close(); err != nil {
			fmt.Fprintln(os.Stderr, "privacy-gcs-admin: close Firestore:", err)
		}
	}()

	constants, err := repo.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return fmt.Errorf("read system constants: %w", err)
	}
	actualProjectID, err := utils.GetGcpProjectID(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("resolve GCP project ID: %w", err)
	}
	configuredBucketName := strings.TrimSpace(constants.GcsFirestoreExportBucketName)
	target, err := buildGCSPurgeTarget(
		environment,
		expectedProjectID,
		actualProjectID,
		expectedBucketName,
		configuredBucketName,
	)
	if err != nil {
		return err
	}

	firestoreRawChatRows, err := countRawLiveChatRows(ctx, repo.FirestoreClient())
	if err != nil {
		return err
	}

	storageClient, err := storage.NewClient(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("initialize GCS client: %w", err)
	}
	defer func() {
		if err := storageClient.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "privacy-gcs-admin: close GCS client:", err)
		}
	}()
	bucket := storageClient.Bucket(target.BucketName)

	now := time.Now().UTC()
	inventory, err := inspectBucket(ctx, bucket)
	if err != nil {
		return err
	}
	preview := buildPreview(now, target, inventory, firestoreRawChatRows)

	switch command {
	case "preview":
		return writeJSON(preview)
	case "apply":
		return apply(ctx, bucket, preview, inventory, args[5:])
	default:
		return usageError()
	}
}

func usageError() error {
	return errors.New(
		"usage: privacy-gcs-admin preview <development|production> <expected-project-id> <expected-bucket-name> OR " +
			"privacy-gcs-admin apply <development|production> <expected-project-id> <expected-bucket-name> " +
			"--expected-prefixes <count> --expected-objects <count> --confirm <token> [--allow-production]",
	)
}

func countRawLiveChatRows(ctx context.Context, client repository.DBClient) (int64, error) {
	query := client.Collection(repository.LiveChatHistory).Query
	result, err := query.NewAggregationQuery().WithCount(firestoreCountAlias).Get(ctx)
	if err != nil {
		return 0, fmt.Errorf("count Firestore raw live chat rows: %w", err)
	}
	rawCount, ok := result[firestoreCountAlias]
	if !ok {
		return 0, fmt.Errorf("count aggregation missing alias %q", firestoreCountAlias)
	}
	countValue, ok := rawCount.(*firestorepb.Value)
	if !ok {
		return 0, fmt.Errorf("count aggregation has unexpected type %T", rawCount)
	}
	return countValue.GetIntegerValue(), nil
}

func inspectBucket(ctx context.Context, bucket *storage.BucketHandle) (bucketInventory, error) {
	attrs, err := bucket.Attrs(ctx)
	if err != nil {
		return bucketInventory{}, fmt.Errorf("read bucket attributes: %w", err)
	}

	byPrefix := make(map[string]*prefixInventory)
	it := bucket.Objects(ctx, nil)
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return bucketInventory{}, fmt.Errorf("list GCS objects: %w", err)
		}
		prefix := topLevelPrefix(attrs.Name)
		entry, ok := byPrefix[prefix]
		if !ok {
			entry = &prefixInventory{Prefix: prefix}
			byPrefix[prefix] = entry
		}
		entry.Objects = append(entry.Objects, objectRef{
			Name:       attrs.Name,
			Generation: attrs.Generation,
			Size:       attrs.Size,
			Created:    attrs.Created,
		})
		entry.ObjectCount++
		entry.Bytes += attrs.Size
		if attrs.Created.After(entry.LatestCreatedAt) {
			entry.LatestCreatedAt = attrs.Created
		}
		if strings.Contains(attrs.Name, rawLiveChatPathMarker) {
			entry.ContainsRawChat = true
		}
	}

	prefixes := make([]prefixInventory, 0, len(byPrefix))
	for _, entry := range byPrefix {
		prefixes = append(prefixes, *entry)
	}
	sort.Slice(prefixes, func(i, j int) bool {
		if prefixes[i].LatestCreatedAt.Equal(prefixes[j].LatestCreatedAt) {
			return prefixes[i].Prefix < prefixes[j].Prefix
		}
		return prefixes[i].LatestCreatedAt.Before(prefixes[j].LatestCreatedAt)
	})

	var softDelete time.Duration
	if attrs.SoftDeletePolicy != nil {
		softDelete = attrs.SoftDeletePolicy.RetentionDuration
	}

	return bucketInventory{
		Prefixes:          prefixes,
		VersioningEnabled: attrs.VersioningEnabled,
		SoftDelete:        softDelete,
	}, nil
}

func buildPreview(
	now time.Time,
	target gcsPurgeTarget,
	inventory bucketInventory,
	firestoreRawChatRows int64,
) previewOutput {
	out := previewOutput{
		GeneratedAt:           now,
		Target:                target,
		FirestoreRawChatRows:  firestoreRawChatRows,
		VersioningEnabled:     inventory.VersioningEnabled,
		TotalSnapshotPrefixes: len(inventory.Prefixes),
	}
	if inventory.SoftDelete > 0 {
		out.SoftDeleteRetention = inventory.SoftDelete.String()
	}

	var newestRaw time.Time
	var latestSnapshot time.Time
	for _, prefix := range inventory.Prefixes {
		if prefix.LatestCreatedAt.After(latestSnapshot) {
			latestSnapshot = prefix.LatestCreatedAt
			out.LatestSnapshotPrefix = prefix.Prefix
			out.LatestSnapshotCreatedAt = formatTime(prefix.LatestCreatedAt)
		}
		if !prefix.ContainsRawChat {
			continue
		}
		out.RawChatSnapshotPrefixes++
		out.CandidateRawChatSnapshotPrefixes++
		out.CandidateObjectCount += prefix.ObjectCount
		out.CandidateBytes += prefix.Bytes
		if out.OldestRawChatPrefix == "" {
			out.OldestRawChatPrefix = prefix.Prefix
		}
		if prefix.LatestCreatedAt.After(newestRaw) {
			newestRaw = prefix.LatestCreatedAt
			out.NewestRawChatPrefix = prefix.Prefix
			out.NewestRawChatCreatedAt = formatTime(prefix.LatestCreatedAt)
		}
	}

	for _, prefix := range inventory.Prefixes {
		if prefix.ContainsRawChat || newestRaw.IsZero() || !prefix.LatestCreatedAt.After(newestRaw) {
			continue
		}
		out.CleanSnapshotsAfterNewestRaw++
		out.NewestCleanPrefix = prefix.Prefix
	}

	latestRecent := !latestSnapshot.IsZero() && now.Sub(latestSnapshot) <= maximumLatestSnapshotAge
	checks := []safetyCheck{
		{
			Name:   "firestore_raw_chat_drained",
			Passed: firestoreRawChatRows == 0,
			Detail: fmt.Sprintf("firestore_raw_chat_rows=%d", firestoreRawChatRows),
		},
		{
			Name:   "object_versioning_disabled",
			Passed: !inventory.VersioningEnabled,
			Detail: fmt.Sprintf("versioning_enabled=%t", inventory.VersioningEnabled),
		},
		{
			Name:   "raw_chat_snapshots_present",
			Passed: out.RawChatSnapshotPrefixes > 0,
			Detail: fmt.Sprintf("raw=%d", out.RawChatSnapshotPrefixes),
		},
		{
			Name:   "clean_snapshots_exist_after_raw",
			Passed: out.CleanSnapshotsAfterNewestRaw >= minimumCleanSnapshotsAfterRaw,
			Detail: fmt.Sprintf("clean_after_raw=%d required=%d", out.CleanSnapshotsAfterNewestRaw, minimumCleanSnapshotsAfterRaw),
		},
		{
			Name:   "latest_snapshot_is_recent",
			Passed: latestRecent,
			Detail: fmt.Sprintf("latest=%s max_age=%s", out.LatestSnapshotCreatedAt, maximumLatestSnapshotAge),
		},
	}
	out.SafetyChecks = checks
	out.ReadyToApply = true
	for _, check := range checks {
		if !check.Passed {
			out.ReadyToApply = false
		}
	}
	if out.ReadyToApply {
		out.RequiredConfirmation = confirmationToken(out)
	}
	out.Notes = []string{
		"preview is read-only",
		"apply deletes complete Firestore export prefixes that contain raw live-chat data; it does not partially edit snapshots",
		"Firestore live-chat-history must be fully drained before GCS deletion becomes ready",
		"at least seven clean snapshots newer than the newest raw-chat snapshot are preserved",
	}
	if inventory.SoftDelete > 0 {
		out.Notes = append(out.Notes, "deleted objects remain recoverable in GCS soft delete until the configured retention expires")
	}
	return out
}

func apply(
	ctx context.Context,
	bucket *storage.BucketHandle,
	preview previewOutput,
	inventory bucketInventory,
	args []string,
) error {
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	expectedPrefixes := flags.Int("expected-prefixes", -1, "expected raw-chat snapshot prefix count")
	expectedObjects := flags.Int64("expected-objects", -1, "expected object count across candidate prefixes")
	confirm := flags.String("confirm", "", "exact confirmation token returned by preview")
	allowProduction := flags.Bool("allow-production", false, "explicitly allow a production apply")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse apply flags: %w", err)
	}
	if flags.NArg() != 0 || *expectedPrefixes < 0 || *expectedObjects < 0 || *confirm == "" {
		return usageError()
	}
	if err := validateGCSApplyTarget(preview.Target, *allowProduction); err != nil {
		return err
	}
	if !preview.ReadyToApply {
		return errors.New("refusing GCS deletion because preview safety checks are not all satisfied")
	}
	if *expectedPrefixes != preview.CandidateRawChatSnapshotPrefixes {
		return fmt.Errorf("candidate prefix count changed: expected=%d actual=%d", *expectedPrefixes, preview.CandidateRawChatSnapshotPrefixes)
	}
	if *expectedObjects != preview.CandidateObjectCount {
		return fmt.Errorf("candidate object count changed: expected=%d actual=%d", *expectedObjects, preview.CandidateObjectCount)
	}
	if *confirm != confirmationToken(preview) {
		return errors.New("confirmation token does not match current preview")
	}

	nonRawObjects, rawMarkerObjects := splitDeletionPhases(inventory)
	phaseObjectCount := int64(len(nonRawObjects) + len(rawMarkerObjects))
	if phaseObjectCount != preview.CandidateObjectCount {
		return fmt.Errorf(
			"internal candidate object mismatch: preview=%d deletion_phases=%d; refusing deletion",
			preview.CandidateObjectCount,
			phaseObjectCount,
		)
	}

	deleteObject := func(ctx context.Context, object objectRef) error {
		objectHandle := bucket.Object(object.Name)
		if object.Generation != 0 {
			objectHandle = objectHandle.Generation(object.Generation)
		}
		if err := objectHandle.Delete(ctx); err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
			return fmt.Errorf("delete GCS object %q: %w", object.Name, err)
		}
		return nil
	}

	nonRawSummary, err := deleteObjectsBounded(
		ctx,
		"non-raw",
		nonRawObjects,
		defaultDeleteConcurrency,
		deleteProgressInterval,
		os.Stderr,
		deleteObject,
	)
	if err != nil {
		return err
	}

	rawSummary, err := deleteObjectsBounded(
		ctx,
		"raw-marker",
		rawMarkerObjects,
		defaultDeleteConcurrency,
		deleteProgressInterval,
		os.Stderr,
		deleteObject,
	)
	if err != nil {
		return err
	}
	deletedObjects := nonRawSummary.Deleted + rawSummary.Deleted

	postInventory, err := inspectBucket(ctx, bucket)
	if err != nil {
		return fmt.Errorf("post-delete bucket inspection: %w", err)
	}
	postPreview := buildPreview(time.Now().UTC(), preview.Target, postInventory, preview.FirestoreRawChatRows)
	if postPreview.RawChatSnapshotPrefixes != 0 {
		return fmt.Errorf("post-delete verification failed: %d live raw-chat snapshot prefixes remain", postPreview.RawChatSnapshotPrefixes)
	}

	return writeJSON(struct {
		Status              string         `json:"status"`
		Target              gcsPurgeTarget `json:"target"`
		DeletedLiveObjects  int64          `json:"deleted_live_objects"`
		SoftDeleteRetention string         `json:"soft_delete_retention,omitempty"`
		PostCheck           previewOutput  `json:"post_check"`
	}{
		Status:              "deleted live raw-chat snapshot prefixes",
		Target:              preview.Target,
		DeletedLiveObjects:  deletedObjects,
		SoftDeleteRetention: preview.SoftDeleteRetention,
		PostCheck:           postPreview,
	})
}

func deletionOrder(objects []objectRef) []objectRef {
	ordered := append([]objectRef(nil), objects...)
	sort.SliceStable(ordered, func(i, j int) bool {
		iRaw := strings.Contains(ordered[i].Name, rawLiveChatPathMarker)
		jRaw := strings.Contains(ordered[j].Name, rawLiveChatPathMarker)
		if iRaw != jRaw {
			return !iRaw
		}
		return ordered[i].Name < ordered[j].Name
	})
	return ordered
}

func confirmationToken(preview previewOutput) string {
	return fmt.Sprintf(
		"DELETE %d GCS FIRESTORE EXPORT SNAPSHOTS CONTAINING RAW YOUTUBE CHAT (%d OBJECTS) FROM %s PROJECT %s BUCKET %s THROUGH %s",
		preview.CandidateRawChatSnapshotPrefixes,
		preview.CandidateObjectCount,
		strings.ToUpper(preview.Target.Environment),
		preview.Target.ProjectID,
		preview.Target.BucketName,
		strings.TrimSuffix(preview.NewestRawChatPrefix, "/"),
	)
}

func topLevelPrefix(objectName string) string {
	index := strings.IndexByte(objectName, '/')
	if index < 0 {
		return "(root)"
	}
	return objectName[:index+1]
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	return nil
}
