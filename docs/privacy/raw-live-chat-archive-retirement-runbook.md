# Raw live-chat archive retirement runbook

## Goal

Retire persistent raw YouTube live-chat archival while preserving Study Space domain data and the normal in-memory command/moderation path.

Target end state:

```text
YouTube Live Chat API
  -> process in memory
  -> command / moderation
  -> Study Space domain data

raw live-chat-history
  -> no persistent archive
```

This runbook applies only to the raw `live-chat-history` archive. It does not delete or migrate `users`, seats, work segments, study time, RP, orders, user activities, membership data, or other Study Space domain data. Existing raw chat is not converted into new work history before deletion.

## Non-negotiable safety rules

1. Validate the full sequence in `development` before producer cutover or destructive cleanup in `production`.
2. Never partially delete `kind_live-chat-history/**` from a mixed Firestore export snapshot. GCS deletion uses the complete top-level snapshot prefix.
3. Never delete the mixed Firestore backup bucket or the `firestore_export` BigQuery dataset.
4. Preserve `firestore_export.user-activity-history` and `firestore_export.order-history`.
5. Every destructive preview/apply must identify the target environment, GCP project ID, and exact BigQuery table or GCS bucket.
6. Production BigQuery/GCS apply commands are manual operator actions. Do not automate them as part of deploy/CI.
7. Do not classify a GCP resource as removable from its name alone. Confirm that no retained data flow uses it.
8. After both environments are complete, remove one-shot DELETE capabilities from the repository. Useful read-only audits may remain.

## Migration components

The migration is intentionally split into small changes:

- #1002 prevents future GCS-to-BigQuery import of `live-chat-history`.
- #1028 stops the youtube-bot runtime from writing raw messages to Firestore `live-chat-history`.
- #997 provides the one-shot BigQuery raw-row purge capability.
- #1029 strengthens the BigQuery purge with environment/project/table-bound guards.
- #1001 provides guarded whole-prefix cleanup for mixed GCS Firestore export snapshots.
- #1030 strengthens the GCS purge with environment/project/bucket-bound guards.
- #1032 gates GCS cleanup on Firestore drain plus multiple newer clean snapshots.
- #1031 provides a permanent read-only Firestore drain audit.

Do not use a destructive command until the guard changes it depends on are present in the deployed/operator checkout.

## Record the environment targets

Before any deploy or deletion, record the actual values from read-only inventory. Do not copy project IDs or bucket names from memory.

| Environment | GCP project ID | BigQuery dataset | Raw table | GCS Firestore export bucket |
| --- | --- | --- | --- | --- |
| development | `<fill from inventory>` | `firestore_export` | `live-chat-history` | `<fill from config/inventory>` |
| production | `<fill from inventory>` | `firestore_export` | `live-chat-history` | `<fill from config/inventory>` |

Run the repository inventory helper for both environments before cleanup planning:

```bash
scripts/gcp/raw-chat-resource-inventory.sh development <development-project-id> \
  > /tmp/raw-chat-development-inventory.txt

scripts/gcp/raw-chat-resource-inventory.sh production <production-project-id> \
  > /tmp/raw-chat-production-inventory.txt
```

This is read-only. Review Cloud Asset Inventory, Scheduler, Functions, Cloud Run services/jobs, Pub/Sub, Eventarc, service accounts/IAM, Firestore indexes, GCS buckets, and BigQuery resources. Save the output with the migration record rather than committing environment-specific credentials or sensitive configuration.

## Phase 1: development producer cutover

### 1. Preflight

Confirm the development deployment contains:

- the `live-chat-history` BigQuery archive exclusion from #1002;
- the Firestore raw-write removal from #1028;
- the read-only drain audit from #1031.

Do not treat a merge to `dev` as proof that the development runtime has been deployed.

### 2. Deploy to development only

Deploy the producer/archive changes to the development environment. Record the exact deployment timestamp as **Day 0**.

Do not deploy the producer cutover to production yet.

### 3. Verify the service path still works

Confirm representative live-chat commands/moderation behavior still operates. The message-processing path must continue to use the fetched in-memory YouTube message; it must not depend on a successful raw history write.

### 4. Verify Firestore raw rows stop growing

Run:

```bash
cd system
go run ./cmd/raw-chat-drain-audit development <development-project-id>
```

Immediately after Day 0, `total_rows` may still be non-zero because pre-cutover rows remain. Re-run the audit after normal chat activity. The total must not increase from newly persisted raw messages and should decrease as the existing retention cleanup runs.

If raw rows increase after the producer cutover, stop the migration and find the remaining writer before proceeding.

## Phase 2: development wait window

The current Firestore history retention is five days. Existing rows therefore do not disappear immediately after the producer stops.

Expected sequence:

```text
Day 0
  raw Firestore producer stops

approximately Day 5-7
  Firestore live-chat-history reaches 0

then subsequent daily Firestore exports
  begin producing raw-free GCS snapshots

then preserve multiple clean generations
  at least 7 clean snapshots newer than the newest raw snapshot
```

With daily exports this can commonly require roughly **12-14 days from Day 0**, but calendar age is not the authority. The read-only Firestore count and GCS preview safety gates are authoritative.

### Firestore drain gate

Do not proceed to GCS apply until:

```json
{
  "total_rows": 0,
  "drained": true
}
```

is reported for development.

### GCS clean-snapshot gate

Use the final guarded GCS command only in preview mode:

```bash
cd system
go run ./cmd/privacy-gcs-admin preview \
  development <development-project-id> <development-bucket-name>
```

Do not apply until `ready_to_apply` is true. The command must verify at least:

- Firestore raw row count is zero;
- the expected project and configured bucket match the operator target;
- object versioning is disabled;
- raw-containing snapshot prefixes exist;
- at least seven clean snapshot prefixes are newer than the newest raw snapshot;
- the latest overall snapshot is recent.

## Phase 3: development purge rehearsal and manual apply

### BigQuery preview

Use only the target-bound command from #1029:

```bash
cd system
go run ./cmd/privacy-retention-admin preview-bigquery \
  development <development-project-id> <cutoff-rfc3339>
```

Check the preview target contains exactly:

- `environment = development`;
- the expected development project ID;
- dataset `firestore_export`;
- table `live-chat-history`.

### BigQuery apply

The operator manually copies the candidate count and exact token from the immediately preceding preview:

```bash
go run ./cmd/privacy-retention-admin apply-bigquery \
  development <development-project-id> <cutoff-rfc3339> \
  --expected-rows <preview-count> \
  --confirm '<exact-preview-token>'
```

Do not reuse a token from another environment or an earlier preview.

### GCS preview and apply

Run a fresh GCS preview immediately before apply. If any gate changed, stop and re-evaluate.

```bash
go run ./cmd/privacy-gcs-admin preview \
  development <development-project-id> <development-bucket-name>
```

The operator then manually uses only the values from that preview:

```bash
go run ./cmd/privacy-gcs-admin apply \
  development <development-project-id> <development-bucket-name> \
  --expected-prefixes <preview-value> \
  --expected-objects <preview-value> \
  --confirm '<exact-preview-token>'
```

GCS soft delete means removal from the live namespace is not immediate hard deletion. Record the configured soft-delete retention and the expected hard-delete window.

### Development verification

Before production cutover, verify:

- `raw-chat-drain-audit` still reports zero Firestore rows;
- BigQuery raw purge post-check reports no candidate rows for the selected cutoff;
- GCS post-check reports zero live raw-containing snapshot prefixes;
- retained BigQuery tables and the mixed GCS bucket still exist;
- representative Study Space functionality still works.

## Phase 4: production producer cutover

Only after the development sequence has been validated:

1. confirm #1002 and #1028 are included in the production deployment artifact;
2. deploy the producer/archive changes to production;
3. record a separate production **Day 0** timestamp;
4. do not run BigQuery/GCS apply immediately after deployment;
5. monitor the raw Firestore count with the production target explicitly supplied.

```bash
cd system
go run ./cmd/raw-chat-drain-audit production <production-project-id>
```

## Phase 5: production wait window

Repeat the same gates used in development:

1. wait for Firestore `live-chat-history` to reach zero, normally after the existing short retention window;
2. wait for subsequent raw-free daily Firestore exports;
3. retain at least seven clean snapshots newer than the final raw-containing snapshot;
4. require the GCS preview to report `ready_to_apply = true`.

Do not shorten this sequence merely because development succeeded.

## Phase 6: production manual purge

Production destructive operations are manual only.

### BigQuery

```bash
cd system
go run ./cmd/privacy-retention-admin preview-bigquery \
  production <production-project-id> <cutoff-rfc3339>
```

After checking the exact target and candidate count, the operator may run:

```bash
go run ./cmd/privacy-retention-admin apply-bigquery \
  production <production-project-id> <cutoff-rfc3339> \
  --expected-rows <preview-count> \
  --confirm '<exact-preview-token>' \
  --allow-production
```

### GCS

```bash
go run ./cmd/privacy-gcs-admin preview \
  production <production-project-id> <production-bucket-name>
```

Only when the fresh preview is ready may the operator run:

```bash
go run ./cmd/privacy-gcs-admin apply \
  production <production-project-id> <production-bucket-name> \
  --expected-prefixes <preview-value> \
  --expected-objects <preview-value> \
  --confirm '<exact-preview-token>' \
  --allow-production
```

Record the before/after JSON from both tools.

## Phase 7: GCP resource cleanup

After raw data cleanup has completed in **both** environments, rerun the read-only resource inventory for development and production.

Classify each candidate as `dedicated`, `shared`, or `uncertain`.

Potential dedicated cleanup candidates:

- BigQuery `firestore_export.live-chat-history` table;
- Scheduler jobs used only for raw chat archival;
- raw-chat-only Cloud Functions or Cloud Run jobs/services;
- raw-chat-only Pub/Sub topics/subscriptions or Eventarc triggers;
- service accounts used only by the retired raw archive;
- IAM grants needed only by those dedicated resources;
- Firestore indexes used only by retired raw history operations.

Resources that remain:

- BigQuery `firestore_export` dataset;
- `user-activity-history`;
- `order-history`;
- the mixed Firestore backup GCS bucket;
- any export/scheduler/function/service account/IAM resource also used by retained Study Space data.

Do not delete an `uncertain` or shared resource. First inspect its target, trigger, code, IAM bindings, and consumers.

Delete the obsolete BigQuery raw table itself only during this resource-cleanup phase, after confirming there is no retained consumer and both environment migrations are complete.

## Phase 8: repository deletion-tool cleanup

Once both environment migrations and cloud resource cleanup are complete, create a final cleanup PR. It should remove temporary destructive capabilities rather than leave dormant DELETE paths in the repository.

Remove or reduce at least:

- `privacy-retention-admin` BigQuery DELETE/apply capability;
- `privacy-gcs-admin` GCS DELETE/apply capability;
- one-shot purge helpers used only by this migration;
- one-shot purge tests that no longer protect retained behavior;
- migration-only code and documentation that is no longer operationally needed.

Keep useful read-only checks, including Firestore raw-row drain/reappearance auditing and broader retention/resource inventory where they remain useful.

## Final verification checklist

The migration is complete only when all of the following are true for development and production:

- [ ] youtube-bot no longer persists fetched raw messages to Firestore `live-chat-history`.
- [ ] Firestore `live-chat-history` total row count is zero.
- [ ] GCS-to-BigQuery transfer does not import `live-chat-history`.
- [ ] no live GCS Firestore export snapshot contains `kind_live-chat-history`.
- [ ] GCS soft-delete timing has been recorded and allowed to expire as required for hard deletion verification.
- [ ] BigQuery `firestore_export.live-chat-history` has been removed in the final resource-cleanup phase.
- [ ] `firestore_export.user-activity-history` remains.
- [ ] `firestore_export.order-history` remains.
- [ ] the `firestore_export` dataset remains.
- [ ] the mixed GCS backup bucket remains.
- [ ] raw-chat-only cloud resources/IAM/indexes identified by inventory have been removed, with shared resources preserved.
- [ ] one-shot DELETE capabilities have been removed from the repository.
- [ ] retained read-only audits still pass and do not expose a new raw persistence path.
