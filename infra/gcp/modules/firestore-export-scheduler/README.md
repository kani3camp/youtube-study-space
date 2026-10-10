# Development Firestore export Scheduler

The root default remains disabled; the approved protected development CI wave enables ownership. It follows the completed topic wave (#1184/#1186,
protected run37342000553: import0/no-op9/drift0). PR #1187 was definition-only. A separately approved activation adds exact graph
and CI identity validation; importing state must not change or manually run the job.

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
validation rejects Scheduler ownership without topic ownership. The root default keeps Scheduler out of credentialless validation; protected CI
keeps the approved ten-resource graph persistent. `prevent_destroy` preserves
the existing job. Generated delivery subscriptions, service agents, Function
source/artifacts and IAM remain externally owned.

## Read permission boundary

The locked provider's [Scheduler Read implementation](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/google/services/cloudscheduler/resource_cloud_scheduler_job.go#L740)
uses an exact job GET, without a list RPC. A fresh actual operator provider import
plan passed import1/no-op1/drift0/unknown0; the full-root candidate passed
Scheduler import1 plus existing9 no-op (ten no-op action records including the
imported job), with create/update/delete/replace/drift/other/unknown0.
No apply or raw plan publication occurred.

Issue #1162 [comment6005196430](https://github.com/kani3camp/youtube-study-space/issues/1162#issuecomment-6005196430)
approves **`cloudscheduler.jobs.get` only**, appended to the existing development
custom read role with existing principals/bindings preserved. No list/create/
update/delete/run/pause/resume, Pub/Sub publish, IAM-write, broad role, API or
production grant is added. The [resume RPC](https://docs.cloud.google.com/scheduler/docs/reference/rest/v1/projects.locations.jobs/resume)
uses `cloudscheduler.jobs.enable`; the real CI identity smoke rejects that grant,
not a nonexistent jobs.resume permission. Function GET remains ungranted.

The earlier local operator could not mint the CI identity token. Existing SSO
was refreshed without changing IAM; the protected WIF flow now proves actual
plan and apply identities independently. No impersonation permission is added.
Rollback of the read grant removes only Scheduler GET using a fresh role etag,
preserving all other permissions and bindings. Do not remove a read grant while
its persistent ownership still requires it; coordinate any rollback separately.

## Protected import wave

Re-inventory the job, provision only the approved GET, and review the separate
CI activation plus exact ten-resource validator.
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
