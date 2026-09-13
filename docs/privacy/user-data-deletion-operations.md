# User data deletion operations

## Purpose

This document describes the operator workflow for responding to a verified user request to delete Study Space data associated with a YouTube channel ID.

It is an internal engineering/operations document. The public Privacy Policy must separately explain how users request deletion and what categories of data are affected.

## Commands

### Inspect only

```bash
cd system
CREDENTIAL_FILE_LOCATION=/path/to/service-account.json \
  go run ./cmd/privacy-admin inspect UCxxxxxxxx
```

This command is read-only. It reports attributable document/row counts from the primary Firestore and BigQuery stores.

### Delete primary stores

```bash
cd system
CREDENTIAL_FILE_LOCATION=/path/to/service-account.json \
  go run ./cmd/privacy-admin delete-primary UCxxxxxxxx --confirm UCxxxxxxxx
```

`--confirm` must exactly match the target YouTube channel ID. This is intentionally verbose because the operation is destructive.

The command performs:

1. pre-delete inspection;
2. removal from primary Firestore collections;
3. removal from primary BigQuery history tables;
4. post-delete inspection;
5. a non-zero exit if primary-store data remains.

The command is designed to be idempotent. If a partial failure occurs, inspect and rerun after the underlying problem is fixed.

## Firestore scope

Primary deletion covers data attributable by YouTube channel ID in:

- `users`
- `seats`
- `member-seats`
- `user-activities`
- `work-segments`
- `order-history`
- `seat-limits-black-list`
- `seat-limits-white-list`
- `member-seat-limits-black-list`
- `member-seat-limits-white-list`
- `live-chat-history`
- `mypage-youtube-channel-owners`, when present
- corresponding `mypage-users` document, when present

Active seat documents are removed directly instead of running the normal `!out` path. A privacy deletion should not manufacture a new exit activity or final work segment immediately before deleting the same user's history.

## BigQuery scope

Primary deletion covers attributable rows from:

- `firestore_export.live-chat-history` by `author_channel_id`
- `firestore_export.user-activity-history` by `user_id`
- `firestore_export.order-history` by `user_id`

## Important exclusions

`delete-primary` is deliberately not named `delete-all`. It does **not** guarantee erasure from every system.

It does not delete:

- historical Firestore export snapshots in Google Cloud Storage;
- Firebase Authentication user records;
- Discord moderation/log messages;
- data stored by YouTube itself;
- data outside systems controlled by Study Space.

GCS export retention is tracked separately because snapshots may be disaster-recovery backups containing unrelated collections and must not be rewritten or blindly deleted per user.

## Operator process

Before deletion:

1. verify the requester controls the target YouTube channel;
2. record the request date and target channel ID in the private operator record;
3. run `privacy-admin inspect` and save the output privately;
4. confirm there is no channel-ID mismatch;
5. run `delete-primary` with the repeated confirmation value.

After deletion:

1. retain the command's post-delete verification output privately as evidence of completion;
2. complete the GCS/Firebase Auth/Discord steps defined by their respective retention procedures;
3. confirm to the requester when the applicable deletion process is complete;
4. do not publish the channel ID, Firebase UID, or deletion inventory in a public GitHub issue.

## Reuse after deletion

A user who posts in the live chat or uses Study Space again after deletion may cause new data to be created. The public Privacy Policy should state this clearly.
