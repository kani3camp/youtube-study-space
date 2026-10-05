# Development Firestore export Scheduler

This definition is disabled. It follows the completed topic wave (#1184/#1186,
protected run37342000553: import0/no-op9/drift0). It does not activate CI ownership,
grant IAM, import state, change the job or manually run it.

## Fresh inventory and fidelity

After topic completion, read the existing job again. The observed development
contract on 2026-10-06 was:

| Field | Observed value |
| --- | --- |
| Project / region | `test-youtube-study-space` / `asia-southeast2` |
| Name | `scheduledFirestoreCollectionsExport` |
| Schedule / time zone | `0 0 * * *` / `Asia/Tokyo` |
| State | `ENABLED` (`paused = false`) |
| Pub/Sub target | `projects/test-youtube-study-space/topics/initiateFirestoreCollectionsExport` |
| Payload | `c3RhcnQgZXhwb3J0` (base64 of `start export`) |
| Retry | count3, max duration0s, min backoff5s, max backoff3600s, doublings5 |
| Description / attempt deadline | absent; configure null, provider read normalizes to empty string |
| Labels | absent in metadata; pinned Scheduler provider schema has no labels field |
| Other targets | no HTTP or App Engine target |

The module references the adopted topic output; a development-only root
validation rejects Scheduler ownership without topic ownership. Default false
keeps the protected nine-resource graph unchanged. `prevent_destroy` preserves
the existing job. Generated delivery subscriptions, service agents, Function
source/artifacts and IAM remain externally owned.

## Read permission boundary

The locked provider's [Scheduler Read implementation](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/google/services/cloudscheduler/resource_cloud_scheduler_job.go#L740)
uses an exact job GET, without a list RPC. A fresh actual operator provider import
plan passed import1/no-op1/drift0/unknown0; the full-root candidate passed
Scheduler import1 plus existing9 no-op (ten no-op action records including the
imported job), with create/update/delete/replace/drift/other/unknown0.
No apply or raw plan publication occurred.

Candidate permission: **`cloudscheduler.jobs.get` only**, appended to the existing
development custom read role while preserving its existing principals/bindings.
The topic run's actual plan/apply identity `testIamPermissions` smoke confirmed
this GET remains ungranted; the live role also lacks it. A local attempt to use
the existing CI identity could not mint its token because the operator lacks
impersonation permission. No impersonation grant was added. The operator probe
proves resource fidelity; a subsequent protected CI plan must prove the exact
identity can read the job after separately approved GET provisioning.

**STOP before IAM provisioning or Scheduler activation.** Current approval does
not include Scheduler IAM. Do not add list/create/update/delete/run/pause/resume,
Pub/Sub publish, IAM-write, broad roles, API changes or production grants.
Rollback of an approved permission addition removes only this permission using
a freshly read role etag, preserving all other grants and bindings.

## Subsequent import wave

After new explicit approval, re-inventory the job, provision only the approved
GET, and review a separate CI activation plus exact ten-resource validator.
Preserve the unchanged global import-only guard. Require a complete full-root
plan: one exact Scheduler import, existing9 no-op, all other actions/drift/unknown0.
Never use `-target` or overwrite existing state with a historical S3 version.

Use independent plan/apply Environments, the same SHA re-plan, matching sanitized
projection and saved import-only apply. Complete with a normal post-plan
import0/no-op10/drift0, state resource +1/serial +1/version +1 with unchanged
lineage and existing ownership, exact cloud metadata equality, native lock
release and public secret/raw-state/raw-plan/artifact audit. Stop on any
unexpected action. Only then begin the Gen1 Function wave. Never mix source
build/upload/redeployment into Function import.
