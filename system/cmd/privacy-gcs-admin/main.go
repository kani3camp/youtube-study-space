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

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"app.modules/core/repository"
	"app.modules/core/utils"
)

const (
	rawLiveChatPathMarker          = "/all_namespaces/kind_live-chat-history/"
	rawYouTubeDataRetentionDays   = 30
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
	GeneratedAt                   time.Time     `json:"generated_at"`
	Cutoff                        time.Time     `json:"cutoff"`
	BucketName                    string        `json:"bucket_name"`
	VersioningEnabled             bool          `json:"versioning_enabled"`
	SoftDeleteRetention           string        `json:"soft_delete_retention,omitempty"`
	TotalSnapshotPrefixes         int           `json:"total_snapshot_prefixes"`
	RawChatSnapshotPrefixes       int           `json:"raw_chat_snapshot_prefixes"`
	ExpiredRawChatSnapshotPrefixes int          `json:"expired_raw_chat_snapshot_prefixes"`
	CandidateObjectCount          int64         `json:"candidate_object_count"`
	CandidateBytes                int64         `json:"candidate_bytes"`
	OldestRawChatPrefix           string        `json:"oldest_raw_chat_prefix,omitempty"`
	NewestRawChatPrefix           string        `json:"newest_raw_chat_prefix,omitempty"`
	NewestRawChatCreatedAt        string        `json:"newest_raw_chat_created_at,omitempty"`
	CleanSnapshotsAfterNewestRaw  int           `json:"clean_snapshots_after_newest_raw"`
	NewestCleanPrefix             string        `json:"newest_clean_prefix,omitempty"`
	LatestSnapshotPrefix          string        `json:"latest_snapshot_prefix,omitempty"`
	LatestSnapshotCreatedAt       string        `json:"latest_snapshot_created_at,omitempty"`
	SafetyChecks                  []safetyCheck `json:"safety_checks"`
	ReadyToApply                  bool          `json:"ready_to_apply"`
	RequiredConfirmation          string        `json:"required_confirmation,omitempty"`
	Notes                         []string      `json:"notes,omitempty"`
}

func main() {
	if err := run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "privacy-gcs-admin:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) < 2 {
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
	bucketName := strings.TrimSpace(constants.GcsFirestoreExportBucketName)
	if bucketName == "" {
		return errors.New("gcs-firestore-export-bucket-name is empty")
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
	bucket := storageClient.Bucket(bucketName)

	now := time.Now().UTC()
	inventory, err := inspectBucket(ctx, bucket)
	if err != nil {
		return err
	}
	preview := buildPreview(now, bucketName, inventory)

	switch args[1] {
	case "preview":
		if len(args) != 2 {
			return usageError()
		}
		return writeJSON(preview)
	case "apply":
		return apply(ctx, bucket, preview, inventory, args[2:])
	default:
		return usageError()
	}
}

func usageError() error {
	return errors.New("usage: privacy-gcs-admin preview | privacy-gcs-admin apply --expected-prefixes <count> --expected-objects <count> --confirm <token>")
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

func buildPreview(now time.Time, bucketName string, inventory bucketInventory) previewOutput {
	cutoff := now.AddDate(0, 0, -rawYouTubeDataRetentionDays)
	out := previewOutput{
		GeneratedAt:         now,
		Cutoff:              cutoff,
		BucketName:          bucketName,
		VersioningEnabled:   inventory.VersioningEnabled,
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
		if out.OldestRawChatPrefix == "" {
			out.OldestRawChatPrefix = prefix.Prefix
		}
		if prefix.LatestCreatedAt.After(newestRaw) {
			newestRaw = prefix.LatestCreatedAt
			out.NewestRawChatPrefix = prefix.Prefix
			out.NewestRawChatCreatedAt = formatTime(prefix.LatestCreatedAt)
		}
		if prefix.LatestCreatedAt.Before(cutoff) {
			out.ExpiredRawChatSnapshotPrefixes++
			out.CandidateObjectCount += prefix.ObjectCount
			out.CandidateBytes += prefix.Bytes
		}
	}

	for _, prefix := range inventory.Prefixes {
		if prefix.ContainsRawChat || newestRaw.IsZero() || !prefix.LatestCreatedAt.After(newestRaw) {
			continue
		}
		out.CleanSnapshotsAfterNewestRaw++
		out.NewestCleanPrefix = prefix.Prefix
	}

	allRawExpired := out.RawChatSnapshotPrefixes > 0 && out.RawChatSnapshotPrefixes == out.ExpiredRawChatSnapshotPrefixes
	latestRecent := !latestSnapshot.IsZero() && now.Sub(latestSnapshot) <= maximumLatestSnapshotAge
	checks := []safetyCheck{
		{
			Name:   "object_versioning_disabled",
			Passed: !inventory.VersioningEnabled,
			Detail: fmt.Sprintf("versioning_enabled=%t", inventory.VersioningEnabled),
		},
		{
			Name:   "all_raw_chat_snapshots_expired",
			Passed: allRawExpired,
			Detail: fmt.Sprintf("raw=%d expired=%d", out.RawChatSnapshotPrefixes, out.ExpiredRawChatSnapshotPrefixes),
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
		"clean snapshots newer than the newest raw-chat snapshot are preserved",
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
	expectedPrefixes := flags.Int("expected-prefixes", -1, "expected expired raw-chat snapshot prefix count")
	expectedObjects := flags.Int64("expected-objects", -1, "expected object count across candidate prefixes")
	confirm := flags.String("confirm", "", "exact confirmation token returned by preview")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse apply flags: %w", err)
	}
	if flags.NArg() != 0 || *expectedPrefixes < 0 || *expectedObjects < 0 || *confirm == "" {
		return usageError()
	}
	if !preview.ReadyToApply {
		return errors.New("refusing GCS deletion because preview safety checks are not all satisfied")
	}
	if *expectedPrefixes != preview.ExpiredRawChatSnapshotPrefixes {
		return fmt.Errorf("candidate prefix count changed: expected=%d actual=%d", *expectedPrefixes, preview.ExpiredRawChatSnapshotPrefixes)
	}
	if *expectedObjects != preview.CandidateObjectCount {
		return fmt.Errorf("candidate object count changed: expected=%d actual=%d", *expectedObjects, preview.CandidateObjectCount)
	}
	if *confirm != confirmationToken(preview) {
		return errors.New("confirmation token does not match current preview")
	}

	deletedObjects := int64(0)
	for _, prefix := range inventory.Prefixes {
		if !prefix.ContainsRawChat || !prefix.LatestCreatedAt.Before(preview.Cutoff) {
			continue
		}
		for _, object := range deletionOrder(prefix.Objects) {
			objectHandle := bucket.Object(object.Name)
			if object.Generation != 0 {
				objectHandle = objectHandle.Generation(object.Generation)
			}
			if err := objectHandle.Delete(ctx); err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
				return fmt.Errorf("delete GCS object %q: %w", object.Name, err)
			}
			deletedObjects++
		}
	}

	postInventory, err := inspectBucket(ctx, bucket)
	if err != nil {
		return fmt.Errorf("post-delete bucket inspection: %w", err)
	}
	postPreview := buildPreview(time.Now().UTC(), preview.BucketName, postInventory)
	if postPreview.RawChatSnapshotPrefixes != 0 {
		return fmt.Errorf("post-delete verification failed: %d live raw-chat snapshot prefixes remain", postPreview.RawChatSnapshotPrefixes)
	}

	return writeJSON(struct {
		Status              string        `json:"status"`
		BucketName          string        `json:"bucket_name"`
		DeletedLiveObjects  int64         `json:"deleted_live_objects"`
		SoftDeleteRetention string        `json:"soft_delete_retention,omitempty"`
		PostCheck           previewOutput `json:"post_check"`
	}{
		Status:              "deleted live raw-chat snapshot prefixes",
		BucketName:          preview.BucketName,
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
		"DELETE %d GCS FIRESTORE EXPORT SNAPSHOTS CONTAINING RAW YOUTUBE CHAT (%d OBJECTS) FROM %s THROUGH %s",
		preview.ExpiredRawChatSnapshotPrefixes,
		preview.CandidateObjectCount,
		preview.BucketName,
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
