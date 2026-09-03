# Raw live-chat archive retirement: environment operations

This document records the non-secret environment values and the actual operator workflow used for the raw live-chat archive retirement. It supplements `raw-live-chat-archive-retirement-runbook.md` and intentionally does not contain service-account JSON paths, credentials, tokens, or other secrets.

## Confirmed environment targets

| Environment | GCP project ID | BigQuery dataset | Raw table | Firestore backup GCS bucket | AWS CLI profile |
| --- | --- | --- | --- | --- | --- |
| development | `test-youtube-study-space` | `firestore_export` | `live-chat-history` | `firestore-backup-test-youtube-study-space` | `soraride-dev` |
| production | `youtube-study-space` | `firestore_export` | `live-chat-history` | `firestore-backup-youtube-study-space` | `soraride-prod` |

These values are operator-confirmed. The destructive admin commands still resolve the project from the selected service-account credential and refuse to continue when it differs from the explicit expected project. The GCS command also checks the bucket configured in Firestore system constants against the explicit expected bucket.

## Local operator environment

The long-running `youtube-bot` runs on the same PC as the OBS / youtube-monitor streaming setup.

The normal backend startup is:

```bash
cd system
go run ./cmd/youtube-bot
```

The Go process loads `system/.env`. `CREDENTIAL_FILE_LOCATION` points to a service-account JSON stored outside the repository. Development / production selection is currently performed by changing which environment-specific entry is active in `.env`.

Do not commit the service-account path or JSON. Do not infer the active target only from which line appears uncommented in `.env`.

### Mandatory project check before answering `yes`

`youtube-bot` resolves the actual project ID from the selected credential and prints it before startup. It then asks for an explicit `yes` confirmation.

For development, the only expected value is:

```text
Project ID: test-youtube-study-space
```

For production, the only expected value is:

```text
Project ID: youtube-study-space
```

If any other project ID is printed, answer anything other than `yes` and correct `.env` before retrying.

This startup check is part of the environment-switch safety boundary. The `.env` comment-out state alone is not sufficient evidence of the selected environment.

## Development Day 0 procedure

The Firestore producer change and the BigQuery archive change run in different places and must both be represented in the development environment before the migration wait window is treated as started.

### A. Streaming PC: update and start the development youtube-bot

Use a checkout that contains the raw Firestore writer removal from #1028.

Before startup:

1. configure `.env` to select the development service-account credential;
2. run the bot from `system/`;
3. verify the printed project ID is exactly `test-youtube-study-space`;
4. only then answer `yes`.

```bash
cd system
go run ./cmd/youtube-bot
```

Use an actual development live chat to verify representative command processing still works. Development live-chat testing is available, so this is a required smoke test rather than a theoretical check.

Recommended smoke set:

- one ordinary chat message;
- one read-only command such as `!info`;
- one state-changing command such as `!in` followed by `!out`, using a test user / seat where safe;
- any normal moderation path that can be exercised without intentionally creating harmful content.

The purpose is to verify that fetched chat messages are still processed in memory even though raw `live-chat-history` persistence has stopped.

### B. AWS development environment: deploy the daily-batch archive exclusion

#1002 changes the `transfer-bq` code used by the ECS Fargate daily batch. Updating the streaming-PC bot does not update this batch runtime.

Use the confirmed development AWS profile from `aws-cdk/`:

```bash
cd aws-cdk
pnpm cdk:diff --profile soraride-dev
pnpm cdk:deploy --profile soraride-dev
```

Do not substitute the production profile.

The migration's development Day 0 should be recorded only after both A and B are complete and the development bot smoke test has succeeded. Record the timestamp in JST and UTC if convenient.

## Development read-only checks

All Go admin / audit commands below load the same `system/.env` and resolve the credential project. They do not require changing the active `gcloud` project.

Before each development command, select the development credential in `.env`. The command must refuse to continue if the credential project is not `test-youtube-study-space`.

### Firestore drain

```bash
cd system
go run ./cmd/raw-chat-drain-audit \
  development test-youtube-study-space
```

Expected target section:

```json
{
  "environment": "development",
  "project_id": "test-youtube-study-space",
  "collection": "live-chat-history"
}
```

Immediately after Day 0, `total_rows` can be greater than zero. Continue the migration only after it reaches zero and `drained` is `true`.

### GCS readiness preview

```bash
cd system
go run ./cmd/privacy-gcs-admin preview \
  development \
  test-youtube-study-space \
  firestore-backup-test-youtube-study-space
```

The preview itself is read-only. It must remain `ready_to_apply: false` until all migration gates are satisfied, including zero Firestore raw rows and at least seven clean snapshot generations after the newest raw-containing snapshot.

### BigQuery purge preview

Choose the cutoff at execution time and keep the same exact value for the immediately following apply.

```bash
cd system
go run ./cmd/privacy-retention-admin preview-bigquery \
  development \
  test-youtube-study-space \
  <cutoff-rfc3339>
```

The preview target must identify:

```text
environment: development
project: test-youtube-study-space
dataset: firestore_export
table: live-chat-history
```

### Development destructive apply

Actual BigQuery and GCS deletion remains a manual operator action. Use only values returned by the immediately preceding preview.

BigQuery:

```bash
cd system
go run ./cmd/privacy-retention-admin apply-bigquery \
  development \
  test-youtube-study-space \
  <same-cutoff-rfc3339> \
  --expected-rows <preview-count> \
  --confirm '<exact-preview-token>'
```

GCS:

```bash
cd system
go run ./cmd/privacy-gcs-admin apply \
  development \
  test-youtube-study-space \
  firestore-backup-test-youtube-study-space \
  --expected-prefixes <preview-value> \
  --expected-objects <preview-value> \
  --confirm '<exact-preview-token>'
```

Never partially delete `kind_live-chat-history/**` inside a mixed export. The tool operates on whole raw-containing snapshot prefixes only.

## Production release and Day 0

Production is not updated directly from `dev`.

The normal release boundary is:

1. create / merge the normal release from `dev` to `main`;
2. update the streaming PC checkout from the released `main` state (`git pull` as normally operated);
3. deploy AWS/CDK changes as required for the released daily-batch code;
4. only then start the production youtube-bot with the production credential.

The raw-chat migration follows the same release process. Development must have completed its producer cutover, drain, clean-snapshot wait, manual cleanup, and verification before beginning the production producer cutover.

### Streaming PC production startup

Before startup, switch `.env` to the production service-account credential.

```bash
cd system
go run ./cmd/youtube-bot
```

The printed project ID must be exactly:

```text
Project ID: youtube-study-space
```

Only then answer `yes`.

### AWS production deployment

Use the confirmed production AWS profile after the `dev` to `main` release has been completed:

```bash
cd aws-cdk
pnpm cdk:diff --profile soraride-prod
pnpm cdk:deploy --profile soraride-prod
```

Do not substitute the development profile.

Production Day 0 is recorded only after the released production bot and production daily-batch archive exclusion are both active.

## Production read-only checks

Before these commands, `.env` must select the production service-account credential. Each command also receives the expected production project explicitly.

### Firestore drain

```bash
cd system
go run ./cmd/raw-chat-drain-audit \
  production youtube-study-space
```

### GCS readiness preview

```bash
cd system
go run ./cmd/privacy-gcs-admin preview \
  production \
  youtube-study-space \
  firestore-backup-youtube-study-space
```

### BigQuery purge preview

```bash
cd system
go run ./cmd/privacy-retention-admin preview-bigquery \
  production \
  youtube-study-space \
  <cutoff-rfc3339>
```

## Production destructive apply

Production BigQuery / GCS applies remain explicit manual operator actions and additionally require `--allow-production`.

BigQuery:

```bash
cd system
go run ./cmd/privacy-retention-admin apply-bigquery \
  production \
  youtube-study-space \
  <same-cutoff-rfc3339> \
  --expected-rows <preview-count> \
  --confirm '<exact-preview-token>' \
  --allow-production
```

GCS:

```bash
cd system
go run ./cmd/privacy-gcs-admin apply \
  production \
  youtube-study-space \
  firestore-backup-youtube-study-space \
  --expected-prefixes <preview-value> \
  --expected-objects <preview-value> \
  --confirm '<exact-preview-token>' \
  --allow-production
```

Do not execute either production apply until the production Firestore drain and GCS clean-snapshot gates have independently passed.

## `gcloud` inventory is separate from the normal operator path

The normal Study Space operator workflow does not rely on direct local GCP CLI use. The repository inventory helper under `scripts/gcp/` uses GCP command-line inventory APIs and is intended for the later dedicated-resource cleanup phase, not as a prerequisite for starting Day 0.

The Firestore drain, BigQuery preview/apply, and GCS preview/apply commands above use the service-account credential selected through `system/.env` and perform their own explicit target checks.

When cloud-resource cleanup is reached, run the read-only inventory from an environment with authenticated GCP CLI access, or perform equivalent read-only inventory through the Google Cloud console. Do not delete a resource solely because its name contains `live-chat` or `raw-chat`.

## Values intentionally not fixed in this document

The following values are runtime/operator values and should not be hard-coded:

- service-account JSON file paths;
- confirmation tokens returned by previews;
- preview candidate row / object / prefix counts;
- BigQuery cutoff RFC3339 timestamp;
- Day 0 timestamps.

The AWS CLI profile names are now confirmed and fixed in this document as `soraride-dev` for development and `soraride-prod` for production.

All destructive values must come from a fresh preview or from the explicitly selected operator environment.
