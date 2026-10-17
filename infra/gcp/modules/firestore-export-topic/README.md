# Firestore export topic adoption

#1173 production Gen1 Node.js 22 version 6 natural E2E passed on 2026-10-06.
The development natural-E2E import gate is open. Production root ownership
remains disabled. This module owns only the existing user-managed topic;
subscriptions, topic IAM and generated Function resources are excluded.

## Fresh inventory and wave order

1. Topic: development `initiateFirestoreCollectionsExport`, no labels, KMS,
   retention, schema or storage-policy overrides. The existing Scheduler and
   Function trigger both reference this exact topic.
2. Scheduler: `scheduledFirestoreCollectionsExport`, asia-southeast2,
   `0 0 * * *` / Asia/Tokyo / ENABLED, existing base64 payload and retry values.
3. Gen1 Function: `firestoreCollectionsExport`, nodejs22/version8, entry point
   `scheduledFirestoreExport`, 256 MB/60 s/maxInstances1/retry false, existing
   execution SA and explicit development guard variables.

2026-10-06 pinned-provider isolated read-only import plans for topic and
Scheduler each showed import1/no-op1, change fields0/drift0. This is not a
full-root plan or a remote-state import. No apply was run. Function import also reached import1/no-op1/change fields0/drift0 after
leaving the existing reserved deployment label externally owned; effective labels
were unchanged and all source archive/repository fields remained unset. No source
rebuild/upload occurred. This import-only configuration cannot recreate a deleted
Function without a separately reviewed source/deploy definition.

The existing Google-managed Gen1 subscription stays outside ownership.
Generated source bucket, build/artifact resources and default SA also remain
external. Preserve deployed package/lock hashes and provenance in the Function
wave; do not upload/rebuild or set a new source archive to make import work.
The provider treats labels as non-authoritative: do not acquire the reserved
`deployment-tool` label as Terraform configuration merely because it appears
in live metadata. Require unchanged effective labels in the actual plan.

## Protected adoption prerequisites

The dev root defaults `manage_export_topic=false`; the authenticated workflow
does not enable it. Before opening it, use the existing trusted integration
branch, independent Environments and same-SHA plan/re-plan route:

- Resolve the separate #1162 quota representation-only drift through its own
  explicit approval/gate. Do not combine a state refresh with this import.
- Scope the CI read prerequisite to `pubsub.topics.get`. The existing custom
  role lacks this permission. Do not add publish, subscription, topic mutation,
  IAM-write, broad roles, API or production grants. Later waves need their own
  exact GET inventory (`cloudscheduler.jobs.get`, `cloudfunctions.functions.get`).
- Fresh full-root topic plan must have exactly import1 with no topic change and
  the existing eight resources no-op; create/update/delete/replace/drift/other0.
- Protected saved-plan apply may adopt state only. Then require import0/no-op9,
  unchanged cloud metadata, same S3 lineage, state version +1 and released lock.
- Review a follow-up that keeps topic ownership enabled before the next wave.
  Never turn ownership off after adoption: that would plan destruction.

`prevent_destroy` and the unchanged global import-only/drift guard are mandatory.
No manual Scheduler/PubSub/export/downstream triggers are part of validation.
No production data, signed source URLs, raw state or plan enter public output.
