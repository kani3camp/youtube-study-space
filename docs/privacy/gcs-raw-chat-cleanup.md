# GCS raw YouTube chat snapshot cleanup

## 2026-08-14 development bucket finding

The 2026-08-14 read-only retention audit found that the configured Firestore export bucket contained long-lived mixed snapshots rather than a disposable transfer-only staging area.

The audited bucket was the **development** bucket:

- GCP project: `test-youtube-study-space`
- bucket: `firestore-backup-test-youtube-study-space`
- live objects: 18,910
- live bytes: about 1.53 GB
- live objects older than the previous raw-data cutoff: 18,640
- oldest live object: 2022-03-19
- newest live object: 2026-08-13
- top-level Firestore export prefixes: 1,614
- object versioning: disabled
- soft delete: 7 days
- bucket lifecycle: none

The confirmed production target is separate:

- GCP project: `youtube-study-space`
- bucket: `firestore-backup-youtube-study-space`

Do not apply the development bucket observations to production by assumption. Every production preview must inspect and report the current attributes of `firestore-backup-youtube-study-space` before any apply is considered.

The audited development bucket contains export artifacts for at least:

- `live-chat-history`
- `users`
- `user-activities`
- `order-history`

Therefore a blanket short lifecycle on the entire bucket would also shorten recovery history for Study Space-owned data.

## Cleanup strategy

Do not delete only `kind_live-chat-history/**` from a mixed Firestore export snapshot. That would leave a partially missing snapshot whose overall export metadata may no longer describe a complete recoverable export.

Instead, treat the top-level Firestore export prefix as the deletion unit. GCS cleanup becomes eligible only after the raw producer has stopped and the short Firestore retention window has fully drained:

1. require Firestore `live-chat-history` to contain zero documents;
2. identify every snapshot prefix that still contains `kind_live-chat-history`;
3. require at least seven newer clean snapshot prefixes after the newest raw-chat snapshot;
4. require the newest overall snapshot to be recent;
5. require object versioning to be disabled;
6. delete the complete snapshot prefixes that contain raw chat;
7. verify that no live GCS snapshot prefix containing raw chat remains.

This deliberately sacrifices old copies of other collections inside the affected mixed snapshots. Newer clean snapshots remain available for disaster recovery.

With the current five-day Firestore retention and daily exports, the producer cutover to GCS eligibility may take roughly 12 to 14 days: about five to seven days for Firestore to drain, followed by enough daily clean snapshot generations. Actual timing depends on cleanup and export cadence, so the command gates are authoritative rather than the calendar estimate.

## Guarded operator command

Every invocation must identify the target environment, expected GCP project ID, and expected GCS bucket. The command resolves the project ID from the active credentials and the bucket name from Firestore configuration, then refuses to continue if either differs from the operator-supplied target.

Development preview is read-only:

```bash
cd system
go run ./cmd/privacy-gcs-admin preview \
  development \
  test-youtube-study-space \
  firestore-backup-test-youtube-study-space
```

The preview reports the target environment/project/bucket, current Firestore raw-chat row count, candidate prefix count, candidate object count/bytes, newest raw-chat snapshot, clean snapshots after it, safety checks, and an exact target-bound confirmation token.

`ready_to_apply` remains false while any Firestore raw-chat document exists or fewer than seven clean snapshots exist after the newest raw snapshot.

Only when every safety check passes can the destructive development command run:

```bash
go run ./cmd/privacy-gcs-admin apply \
  development \
  test-youtube-study-space \
  firestore-backup-test-youtube-study-space \
  --expected-prefixes <preview value> \
  --expected-objects <preview value> \
  --confirm '<exact preview token>'
```

Production preview is also read-only and uses the separately confirmed production target:

```bash
go run ./cmd/privacy-gcs-admin preview \
  production \
  youtube-study-space \
  firestore-backup-youtube-study-space
```

Production apply additionally requires the explicit production switch:

```bash
go run ./cmd/privacy-gcs-admin apply \
  production \
  youtube-study-space \
  firestore-backup-youtube-study-space \
  --expected-prefixes <preview value> \
  --expected-objects <preview value> \
  --confirm '<exact preview token>' \
  --allow-production
```

The command re-scans Firestore and the bucket immediately before deletion. A changed target, non-zero Firestore raw-chat count, insufficient clean snapshots, changed prefix/object count, or confirmation token mismatch stops the operation. Production deletion remains a manual operator action.

Within each candidate prefix, non-raw objects are deleted before raw-chat objects. This ordering keeps at least one raw marker present until the rest of that snapshot has been deleted, so a retry after a partial failure can still rediscover the snapshot as a cleanup candidate.

## Soft delete

The audited development bucket had a seven-day soft-delete policy on 2026-08-14. Deleting a live object from that bucket therefore does not immediately hard-delete it; the object remains recoverable until the configured soft-delete retention expires.

Do not assume the production bucket has the same current policy. The production preview must report the actual `firestore-backup-youtube-study-space` soft-delete retention before any production apply. The cleanup command verifies removal from the live object namespace; permanent hard deletion follows the policy reported for the target bucket.

## Follow-up

### 2026-10-14 IaC migration preparation (#983)

The current migration authorization permits read-only inventory and deletion
preparation. It **does not authorize deleting any historical mixed GCS snapshot**.
A `ready_to_apply=true` preview or a confirmation token is a tool condition, not
human authorization. Keep apply disabled for this workstream.

The [2026-10-06 Issue #983 update](https://github.com/kani3camp/youtube-study-space/issues/983#issuecomment-6006495416)
records a production natural export containing exactly `users`, `user-activities`
and `order-history`, with the downstream retained-history transfer successful.
This is dated evidence, not proof of the currently deployed writer or current
cleanup eligibility. Earlier issue comments called the development bucket a
production observation; use the explicit environment/project/bucket binding above
and authenticated fresh metadata rather than those historical labels.

Before presenting deletion candidates for a separate decision, collect privately:

- the deployed producer/export source and natural-run collection groups, confirming
  no `live-chat-history` re-entry and no unexpected redeploy;
- a fresh Firestore drain count, target-bound GCS preview, all candidate object
  generations, bytes, complete top-level prefixes and retained collections;
- bucket location/storage class/versioning/soft delete, retention policy and lock;
- each candidate object's retention expiration, temporary/event-based hold and
  object-retention settings; a prefix is eligible only if **every object** is eligible;
- at least seven newer clean generations and latest export age at most 72 hours,
  confirming natural export success and a usable retained-data recovery set;
- the loss of older `users`/`user-activities`/`order-history` backups, retained clean
  backup coverage, private rollback/recovery path and hard-delete window;
- request, soft-delete storage and COLDLINE early-deletion fees within the allowed
  budget. Do not assume retention expiry alone eliminates storage-class charges.

The current `privacy-gcs-admin` reads versioning and soft-delete policy and counts
prefixes from object names. It does **not** inspect bucket retention, individual
object retention/holds, or prove snapshot completeness/restorability. Its candidate
set includes every live raw-containing prefix; it cannot select an eligible-only
retention wave. Therefore its `ready_to_apply` result is insufficient for a
production retention decision. Do not run the current all-prefix apply against a
partly eligible inventory, remove retention/holds, or turn partial deletion into
a retry loop. Retention-aware wave support and review are required before any
later separately authorized deletion.

Production's recorded COLDLINE / 93-day retention is intentional and must remain.
Fresh metadata may show a different effective eligibility time per object. Compute
eligibility from object metadata, not the prefix name or a dev-derived date.
Preserve complete clean snapshots and the shared bucket/export infrastructure.

The approval packet should contain only sanitized aggregate counts/bytes,
eligibility ranges, retained backup coverage, expected fees and a private manifest
reference. Keep raw object names, configuration, confirmation tokens and business
data out of public logs/artifacts. If no raw prefixes remain, record that fresh
result and avoid repeating a completed deletion.

After the raw archive migration is complete in both environments, remove the one-shot deletion capability from the repository. Keep useful read-only audits. The general backup retention policy for Study Space-owned data remains independent from raw live-chat retirement.
