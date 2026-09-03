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

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"app.modules/core/mybigquery"
	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
)

const (
	defaultLookbackDays   = 90
	firestoreCountAlias   = "row_count"
	userActivityMarker    = "/all_namespaces/kind_user-activities/"
	orderHistoryMarker    = "/all_namespaces/kind_order-history/"
	maxSnapshotOutputRows = 180
)

type report struct {
	GeneratedAt time.Time        `json:"generated_at"`
	From        time.Time        `json:"from"`
	Config      configAudit      `json:"config"`
	Firestore   storeAudit       `json:"firestore"`
	BigQuery    storeAudit       `json:"bigquery"`
	GCS         gcsCoverageAudit `json:"gcs"`
	Findings    []string         `json:"findings,omitempty"`
}

type configAudit struct {
	HistoryRetentionDays            int       `json:"history_retention_days"`
	LastTransferCollectionHistoryBQ time.Time `json:"last_transfer_collection_history_bigquery"`
	GCSExportBucketName             string    `json:"gcs_export_bucket_name"`
	GCPRegion                       string    `json:"gcp_region"`
}

type storeAudit struct {
	UserActivities historyCollectionAudit `json:"user_activities"`
	OrderHistory   historyCollectionAudit `json:"order_history"`
}

type historyCollectionAudit struct {
	CollectionOrTable string       `json:"collection_or_table"`
	TotalRows         int64        `json:"total_rows"`
	RowsSinceFrom     int64        `json:"rows_since_from"`
	OldestTimestamp   string       `json:"oldest_timestamp,omitempty"`
	NewestTimestamp   string       `json:"newest_timestamp,omitempty"`
	DailyRows         []dailyCount `json:"daily_rows,omitempty"`
}

type dailyCount struct {
	Date string `json:"date" bigquery:"date"`
	Rows int64  `json:"rows" bigquery:"row_count"`
}

type gcsCoverageAudit struct {
	BucketName                   string             `json:"bucket_name"`
	SnapshotCountSinceFrom       int                `json:"snapshot_count_since_from"`
	LatestSnapshotCreatedAt      string             `json:"latest_snapshot_created_at,omitempty"`
	LatestUserActivitiesSnapshot string             `json:"latest_user_activities_snapshot,omitempty"`
	LatestOrderHistorySnapshot   string             `json:"latest_order_history_snapshot,omitempty"`
	Snapshots                    []snapshotCoverage `json:"snapshots,omitempty"`
	OutputTruncated              bool               `json:"output_truncated"`
	InspectionError              string             `json:"inspection_error,omitempty"`
}

type snapshotCoverage struct {
	Prefix                string `json:"prefix"`
	CreatedAt             string `json:"created_at"`
	TotalObjects          int64  `json:"total_objects"`
	TotalBytes            int64  `json:"total_bytes"`
	UserActivitiesObjects int64  `json:"user_activities_objects"`
	UserActivitiesBytes   int64  `json:"user_activities_bytes"`
	OrderHistoryObjects   int64  `json:"order_history_objects"`
	OrderHistoryBytes     int64  `json:"order_history_bytes"`
}

type snapshotAccumulator struct {
	Prefix                string
	CreatedAt             time.Time
	TotalObjects          int64
	TotalBytes            int64
	UserActivitiesObjects int64
	UserActivitiesBytes   int64
	OrderHistoryObjects   int64
	OrderHistoryBytes     int64
}

type bqTableSpec struct {
	TableName       string
	TimestampColumn string
}

type firestoreCollectionSpec struct {
	CollectionName string
	TimestampField string
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "history-transfer-gap-audit:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("history-transfer-gap-audit", flag.ContinueOnError)
	fromArg := flags.String("from", "", "audit start date in JST (YYYY-MM-DD); default is 90 days ago")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	jst := timeutil.JapanLocation()
	now := time.Now().UTC()
	from, err := parseFrom(*fromArg, now, jst)
	if err != nil {
		return err
	}

	utils.LoadEnv(".env")
	credentialFilePath := strings.TrimSpace(os.Getenv("CREDENTIAL_FILE_LOCATION"))
	if credentialFilePath == "" {
		return errors.New("CREDENTIAL_FILE_LOCATION is required")
	}
	//nolint:staticcheck // Credential file path is operator-controlled for this read-only audit.
	clientOption := option.WithCredentialsFile(credentialFilePath)

	repo, err := repository.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("initialize Firestore: %w", err)
	}
	defer func() {
		if err := repo.FirestoreClient().Close(); err != nil {
			fmt.Fprintln(os.Stderr, "history-transfer-gap-audit: close Firestore:", err)
		}
	}()

	constants, err := repo.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return fmt.Errorf("read system constants: %w", err)
	}

	firestoreAudit, err := inspectFirestoreHistory(ctx, repo.FirestoreClient(), from, jst)
	if err != nil {
		return fmt.Errorf("inspect Firestore history: %w", err)
	}

	projectID, err := utils.GetGcpProjectID(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("resolve GCP project ID: %w", err)
	}
	bqClient, err := mybigquery.NewBigqueryClient(ctx, projectID, clientOption, constants.GcpRegion)
	if err != nil {
		return fmt.Errorf("initialize BigQuery: %w", err)
	}
	defer bqClient.CloseClient()

	bqAudit, err := inspectBigQueryHistory(ctx, bqClient, from)
	if err != nil {
		return fmt.Errorf("inspect BigQuery history: %w", err)
	}

	gcsAudit := inspectGCSCoverage(ctx, clientOption, constants.GcsFirestoreExportBucketName, from)

	out := report{
		GeneratedAt: now,
		From:        from,
		Config: configAudit{
			HistoryRetentionDays:            constants.CollectionHistoryRetentionDays,
			LastTransferCollectionHistoryBQ: constants.LastTransferCollectionHistoryBigquery,
			GCSExportBucketName:             constants.GcsFirestoreExportBucketName,
			GCPRegion:                       constants.GcpRegion,
		},
		Firestore: firestoreAudit,
		BigQuery:  bqAudit,
		GCS:       gcsAudit,
	}
	out.Findings = buildFindings(out)

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	return nil
}

func parseFrom(raw string, now time.Time, location *time.Location) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		date := now.In(location).AddDate(0, 0, -defaultLookbackDays)
		return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, location)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse --from as YYYY-MM-DD: %w", err)
	}
	return parsed, nil
}

func inspectFirestoreHistory(
	ctx context.Context,
	client repository.DBClient,
	from time.Time,
	location *time.Location,
) (storeAudit, error) {
	userActivities, err := inspectFirestoreCollection(ctx, client, firestoreCollectionSpec{
		CollectionName: repository.UserActivities,
		TimestampField: repository.TakenAtDocProperty,
	}, from, location)
	if err != nil {
		return storeAudit{}, err
	}
	orderHistory, err := inspectFirestoreCollection(ctx, client, firestoreCollectionSpec{
		CollectionName: repository.OrderHistory,
		TimestampField: repository.OrderedAtDocProperty,
	}, from, location)
	if err != nil {
		return storeAudit{}, err
	}
	return storeAudit{UserActivities: userActivities, OrderHistory: orderHistory}, nil
}

func inspectFirestoreCollection(
	ctx context.Context,
	client repository.DBClient,
	spec firestoreCollectionSpec,
	from time.Time,
	location *time.Location,
) (historyCollectionAudit, error) {
	result := historyCollectionAudit{CollectionOrTable: spec.CollectionName}
	collection := client.Collection(spec.CollectionName)

	total, err := firestoreCount(ctx, collection.Query)
	if err != nil {
		return result, fmt.Errorf("count %s: %w", spec.CollectionName, err)
	}
	result.TotalRows = total

	sinceQuery := collection.Where(spec.TimestampField, ">=", from)
	since, err := firestoreCount(ctx, sinceQuery)
	if err != nil {
		return result, fmt.Errorf("count %s since from: %w", spec.CollectionName, err)
	}
	result.RowsSinceFrom = since

	oldest, err := firestoreBoundaryTimestamp(ctx, collection.Query, spec.TimestampField, firestore.Asc)
	if err != nil {
		return result, fmt.Errorf("read oldest %s timestamp: %w", spec.CollectionName, err)
	}
	newest, err := firestoreBoundaryTimestamp(ctx, collection.Query, spec.TimestampField, firestore.Desc)
	if err != nil {
		return result, fmt.Errorf("read newest %s timestamp: %w", spec.CollectionName, err)
	}
	result.OldestTimestamp = formatTimestamp(oldest)
	result.NewestTimestamp = formatTimestamp(newest)

	result.DailyRows, err = firestoreDailyCounts(ctx, collection.Query, spec.TimestampField, from, location)
	if err != nil {
		return result, fmt.Errorf("read %s daily rows: %w", spec.CollectionName, err)
	}
	return result, nil
}

func firestoreCount(ctx context.Context, query firestore.Query) (int64, error) {
	aggregation, err := query.NewAggregationQuery().WithCount(firestoreCountAlias).Get(ctx)
	if err != nil {
		return 0, fmt.Errorf("run count aggregation: %w", err)
	}
	rawCount, ok := aggregation[firestoreCountAlias]
	if !ok {
		return 0, fmt.Errorf("count aggregation missing alias %q", firestoreCountAlias)
	}
	value, ok := rawCount.(*firestorepb.Value)
	if !ok {
		return 0, fmt.Errorf("count aggregation has unexpected type %T", rawCount)
	}
	return value.GetIntegerValue(), nil
}

func firestoreBoundaryTimestamp(
	ctx context.Context,
	query firestore.Query,
	field string,
	direction firestore.Direction,
) (time.Time, error) {
	iter := query.OrderBy(field, direction).Limit(1).Documents(ctx)
	defer iter.Stop()
	doc, err := iter.Next()
	if err == iterator.Done {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("read boundary document: %w", err)
	}
	value, err := doc.DataAt(field)
	if err != nil {
		return time.Time{}, fmt.Errorf("read field %q: %w", field, err)
	}
	timestamp, ok := value.(time.Time)
	if !ok {
		return time.Time{}, fmt.Errorf("field %q has unexpected type %T", field, value)
	}
	return timestamp, nil
}

func firestoreDailyCounts(
	ctx context.Context,
	query firestore.Query,
	field string,
	from time.Time,
	location *time.Location,
) ([]dailyCount, error) {
	iter := query.Where(field, ">=", from).OrderBy(field, firestore.Asc).Documents(ctx)
	defer iter.Stop()
	counts := map[string]int64{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate documents: %w", err)
		}
		value, err := doc.DataAt(field)
		if err != nil {
			return nil, fmt.Errorf("read field %q: %w", field, err)
		}
		timestamp, ok := value.(time.Time)
		if !ok {
			return nil, fmt.Errorf("field %q has unexpected type %T", field, value)
		}
		counts[timestamp.In(location).Format("2006-01-02")]++
	}
	return sortedDailyCounts(counts), nil
}

func inspectBigQueryHistory(
	ctx context.Context,
	client *mybigquery.BigqueryController,
	from time.Time,
) (storeAudit, error) {
	userActivities, err := inspectBigQueryTable(ctx, client, bqTableSpec{
		TableName:       mybigquery.UserActivityHistoryMainTableName,
		TimestampColumn: "taken_at",
	}, from)
	if err != nil {
		return storeAudit{}, err
	}
	orderHistory, err := inspectBigQueryTable(ctx, client, bqTableSpec{
		TableName:       mybigquery.OrderHistoryMainTableName,
		TimestampColumn: "ordered_at",
	}, from)
	if err != nil {
		return storeAudit{}, err
	}
	return storeAudit{UserActivities: userActivities, OrderHistory: orderHistory}, nil
}

func inspectBigQueryTable(
	ctx context.Context,
	client *mybigquery.BigqueryController,
	spec bqTableSpec,
	from time.Time,
) (historyCollectionAudit, error) {
	result := historyCollectionAudit{CollectionOrTable: spec.TableName}
	table := fmt.Sprintf("`%s.%s.%s`", client.Client.Project(), mybigquery.DatasetName, spec.TableName)

	summarySQL := fmt.Sprintf(`
SELECT
  COUNT(*) AS total_rows,
  COUNTIF(%s >= @from) AS rows_since_from,
  IFNULL(FORMAT_TIMESTAMP('%%Y-%%m-%%dT%%H:%%M:%%SZ', MIN(%s), 'UTC'), '') AS oldest_timestamp,
  IFNULL(FORMAT_TIMESTAMP('%%Y-%%m-%%dT%%H:%%M:%%SZ', MAX(%s), 'UTC'), '') AS newest_timestamp
FROM %s`, spec.TimestampColumn, spec.TimestampColumn, spec.TimestampColumn, table)
	query := client.Client.Query(summarySQL)
	query.Location = client.WorkingRegion
	query.Parameters = []bigquery.QueryParameter{{Name: "from", Value: from}}
	iter, err := query.Read(ctx)
	if err != nil {
		return result, fmt.Errorf("run BigQuery summary for %s: %w", spec.TableName, err)
	}
	var summary struct {
		TotalRows       int64  `bigquery:"total_rows"`
		RowsSinceFrom   int64  `bigquery:"rows_since_from"`
		OldestTimestamp string `bigquery:"oldest_timestamp"`
		NewestTimestamp string `bigquery:"newest_timestamp"`
	}
	if err := iter.Next(&summary); err != nil {
		return result, fmt.Errorf("read BigQuery summary for %s: %w", spec.TableName, err)
	}
	result.TotalRows = summary.TotalRows
	result.RowsSinceFrom = summary.RowsSinceFrom
	result.OldestTimestamp = summary.OldestTimestamp
	result.NewestTimestamp = summary.NewestTimestamp

	dailySQL := fmt.Sprintf(`
SELECT
  FORMAT_DATE('%%Y-%%m-%%d', DATE(%s, 'Asia/Tokyo')) AS date,
  COUNT(*) AS row_count
FROM %s
WHERE %s >= @from
GROUP BY date
ORDER BY date`, spec.TimestampColumn, table, spec.TimestampColumn)
	dailyQuery := client.Client.Query(dailySQL)
	dailyQuery.Location = client.WorkingRegion
	dailyQuery.Parameters = []bigquery.QueryParameter{{Name: "from", Value: from}}
	dailyIter, err := dailyQuery.Read(ctx)
	if err != nil {
		return result, fmt.Errorf("run BigQuery daily query for %s: %w", spec.TableName, err)
	}
	for {
		var row dailyCount
		err := dailyIter.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return result, fmt.Errorf("read BigQuery daily row for %s: %w", spec.TableName, err)
		}
		result.DailyRows = append(result.DailyRows, row)
	}
	return result, nil
}

func inspectGCSCoverage(
	ctx context.Context,
	clientOption option.ClientOption,
	bucketName string,
	from time.Time,
) gcsCoverageAudit {
	audit := gcsCoverageAudit{BucketName: bucketName}
	if strings.TrimSpace(bucketName) == "" {
		audit.InspectionError = "GCS export bucket name is empty"
		return audit
	}
	client, err := storage.NewClient(ctx, clientOption)
	if err != nil {
		audit.InspectionError = fmt.Sprintf("initialize GCS client: %v", err)
		return audit
	}
	defer func() {
		if err := client.Close(); err != nil && audit.InspectionError == "" {
			audit.InspectionError = fmt.Sprintf("close GCS client: %v", err)
		}
	}()

	snapshots := map[string]*snapshotAccumulator{}
	objectIter := client.Bucket(bucketName).Objects(ctx, nil)
	for {
		attrs, err := objectIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			audit.InspectionError = fmt.Sprintf("list GCS objects: %v", err)
			return audit
		}
		prefix := topLevelPrefix(attrs.Name)
		if prefix == "" {
			continue
		}
		snapshot := snapshots[prefix]
		if snapshot == nil {
			snapshot = &snapshotAccumulator{Prefix: prefix}
			snapshots[prefix] = snapshot
		}
		snapshot.TotalObjects++
		snapshot.TotalBytes += attrs.Size
		if snapshot.CreatedAt.IsZero() || (!attrs.Created.IsZero() && attrs.Created.Before(snapshot.CreatedAt)) {
			snapshot.CreatedAt = attrs.Created
		}
		switch targetCollection(attrs.Name) {
		case repository.UserActivities:
			snapshot.UserActivitiesObjects++
			snapshot.UserActivitiesBytes += attrs.Size
		case repository.OrderHistory:
			snapshot.OrderHistoryObjects++
			snapshot.OrderHistoryBytes += attrs.Size
		}
	}

	rows := make([]snapshotCoverage, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.CreatedAt.IsZero() || snapshot.CreatedAt.Before(from) {
			continue
		}
		rows = append(rows, snapshotCoverage{
			Prefix:                snapshot.Prefix,
			CreatedAt:             formatTimestamp(snapshot.CreatedAt),
			TotalObjects:          snapshot.TotalObjects,
			TotalBytes:            snapshot.TotalBytes,
			UserActivitiesObjects: snapshot.UserActivitiesObjects,
			UserActivitiesBytes:   snapshot.UserActivitiesBytes,
			OrderHistoryObjects:   snapshot.OrderHistoryObjects,
			OrderHistoryBytes:     snapshot.OrderHistoryBytes,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt < rows[j].CreatedAt })
	audit.SnapshotCountSinceFrom = len(rows)
	if len(rows) > 0 {
		audit.LatestSnapshotCreatedAt = rows[len(rows)-1].CreatedAt
	}
	for _, row := range rows {
		if row.UserActivitiesObjects > 0 {
			audit.LatestUserActivitiesSnapshot = row.CreatedAt
		}
		if row.OrderHistoryObjects > 0 {
			audit.LatestOrderHistorySnapshot = row.CreatedAt
		}
	}
	if len(rows) > maxSnapshotOutputRows {
		audit.Snapshots = append([]snapshotCoverage(nil), rows[len(rows)-maxSnapshotOutputRows:]...)
		audit.OutputTruncated = true
	} else {
		audit.Snapshots = rows
	}
	return audit
}

func topLevelPrefix(objectName string) string {
	index := strings.IndexByte(objectName, '/')
	if index <= 0 {
		return ""
	}
	return objectName[:index+1]
}

func targetCollection(objectName string) string {
	switch {
	case strings.Contains(objectName, userActivityMarker):
		return repository.UserActivities
	case strings.Contains(objectName, orderHistoryMarker):
		return repository.OrderHistory
	default:
		return ""
	}
}

func buildFindings(report report) []string {
	findings := []string{}
	if !report.Config.LastTransferCollectionHistoryBQ.IsZero() {
		findings = append(findings, fmt.Sprintf(
			"Firestore last-transfer marker is %s",
			formatTimestamp(report.Config.LastTransferCollectionHistoryBQ),
		))
	}
	if report.BigQuery.UserActivities.NewestTimestamp != "" {
		findings = append(findings, fmt.Sprintf(
			"BigQuery user activity newest row is %s",
			report.BigQuery.UserActivities.NewestTimestamp,
		))
	}
	if report.BigQuery.OrderHistory.NewestTimestamp != "" {
		findings = append(findings, fmt.Sprintf(
			"BigQuery order history newest row is %s",
			report.BigQuery.OrderHistory.NewestTimestamp,
		))
	}
	if report.GCS.LatestUserActivitiesSnapshot != "" {
		findings = append(findings, fmt.Sprintf(
			"GCS has a user-activities export snapshot as late as %s",
			report.GCS.LatestUserActivitiesSnapshot,
		))
	}
	if report.GCS.LatestOrderHistorySnapshot != "" {
		findings = append(findings, fmt.Sprintf(
			"GCS has an order-history export snapshot as late as %s",
			report.GCS.LatestOrderHistorySnapshot,
		))
	}
	if report.GCS.InspectionError != "" {
		findings = append(findings, "GCS coverage could not be fully inspected; do not conclude recoverability yet")
	}
	return findings
}

func sortedDailyCounts(counts map[string]int64) []dailyCount {
	dates := make([]string, 0, len(counts))
	for date := range counts {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	result := make([]dailyCount, 0, len(dates))
	for _, date := range dates {
		result = append(result, dailyCount{Date: date, Rows: counts[date]})
	}
	return result
}

func formatTimestamp(timestamp time.Time) string {
	if timestamp.IsZero() {
		return ""
	}
	return timestamp.UTC().Format(time.RFC3339)
}
