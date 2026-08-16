# GCS raw YouTube chat snapshot cleanup

## Production finding

The 2026-08-14 read-only retention audit found that the configured Firestore export bucket contains long-lived mixed snapshots rather than a disposable transfer-only staging area.

Observed production values:

- bucket: `firestore-backup-test-youtube-study-space`
- live objects: 18,910
- live bytes: about 1.53 GB
- live objects older than the 30-day raw YouTube data cutoff: 18,640
- oldest live object: 2022-03-19
- newest live object: 2026-08-13
- top-level Firestore export prefixes: 1,614
- object versioning: disabled
- soft delete: 7 days
- bucket lifecycle: none

The bucket contains export artifacts for at least:

- `live-chat-history`
- `users`
- `user-activities`
- `order-history`

Therefore a blanket short lifecycle on the entire bucket would also shorten recovery history for Study Space-owned data.

## Cleanup strategy

Do not delete only `kind_live-chat-history/**` from a mixed Firestore export snapshot. That would leave a partially missing snapshot whose overall export metadata may no longer describe a complete recoverable export.

Instead, treat the top-level Firestore export prefix as the deletion unit:

1. identify every snapshot prefix that contains `kind_live-chat-history`;
2. require every such snapshot to be older than 30 days;
3. require at least seven newer clean snapshot prefixes after the newest raw-chat snapshot;
4. require the newest overall snapshot to be recent;
5. require object versioning to be disabled;
6. delete the complete old snapshot prefixes that contain raw chat;
7. verify that no live GCS snapshot prefix containing raw chat remains.

This deliberately sacrifices old copies of other collections inside the affected mixed snapshots. Newer clean snapshots remain available for disaster recovery.

## Guarded operator command

Preview is read-only:

```bash
cd system
go run ./cmd/privacy-gcs-admin preview
```

The preview reports candidate prefix count, candidate object count/bytes, newest raw-chat snapshot, clean snapshots after it, safety checks, and an exact confirmation token.

Only when every safety check passes can the destructive command run:

```bash
go run ./cmd/privacy-gcs-admin apply \
  --expected-prefixes <preview value> \
  --expected-objects <preview value> \
  --confirm '<exact preview token>'
```

The command re-scans the bucket immediately before deletion. A changed prefix/object count or confirmation token stops the operation.

Within each candidate prefix, non-raw objects are deleted before raw-chat objects. This ordering keeps at least one raw marker present until the rest of that snapshot has been deleted, so a retry after a partial failure can still rediscover the snapshot as a cleanup candidate.

## Soft delete

The production bucket currently has a seven-day soft-delete policy. Deleting a live object therefore does not immediately hard-delete it. The object becomes soft-deleted and recoverable until the soft-delete retention expires.

The cleanup command reports this explicitly and only verifies removal from the live object namespace. Permanent hard deletion follows the bucket soft-delete policy.

## Follow-up

The durable fix is to ensure future Firestore exports do not include raw live-chat data at all. BigQuery raw-chat archival is also being retired separately. The general backup retention policy for Study Space-owned data should be decided independently from YouTube raw-data retention.
