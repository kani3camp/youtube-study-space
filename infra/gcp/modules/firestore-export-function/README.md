# Development Gen1 export Function adoption

既存 Gen1 Function の default-off import definition。Scheduler wave 完了後に別 wave として扱う。production root、CI activation、IAM、source deploy はこの定義に含めない。

## Fresh inventory / provider probe — 2026-10-06

| Field | Observed development value |
| --- | --- |
| Name / region | `firestoreCollectionsExport` / `asia-southeast2` |
| Generation / runtime / status / version | Gen1 / `nodejs22` / ACTIVE / 8 |
| Entry point | `scheduledFirestoreExport` |
| Trigger | `google.pubsub.topic.publish`, existing adopted topic output |
| Runtime limits | 256 MB / 60 s / min 0 / max 1 |
| Retry / ingress | false / ALLOW_ALL |
| Execution identity | Fresh inventory value supplied privately; preserve exactly |
| Environment | exactly `YSS_EXPORT_ENVIRONMENT=development`, `YSS_EXPORT_PROJECT_ID=test-youtube-study-space` |
| Description / HTTP / VPC / KMS | absent |
| Reserved label | `deployment-tool=cli-gcloud`, externally owned |
| Source | existing upload deployment; archive/repository Terraform fields absent |

The pinned Google provider's actual isolated import plan passes import1 / no-op1 / drift0 / unknown0. Leaving `labels` unset preserves the effective reserved label. Do not add a label, provider default label or Terraform attribution label. No blanket `ignore_changes` hides a configured difference.

Deployment identity/version/build/update metadata match the successful #1148 deployment. Its six-file source ZIP SHA-256 is `29084c7480cd38364d52be496ee1285376172af29ff4ff50021296a2930b6508`; every root runtime file and dependency lock is byte-identical to source commit `db2604d51ea5f8e0325384bc4ec1bb2af70f40a9`, on the separate `feature/gcp-firestore-export-node22` source stack. This IaC PR does not merge or copy that source stack. `package.json` and lock match Node22 and Firestore 8.7.1. The source provenance is verified against the recorded deployment and existing offline archive, without generating a download URL, uploading, rebuilding or redeploying.

## Permission and activation boundary

[Pinned provider Read](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/google/services/cloudfunctions/resource_cloudfunctions_function.go#L678) calls the exact v1 Function GET. The [official API contract](https://docs.cloud.google.com/functions/docs/reference/rest/v1/projects.locations.functions/get) requires `cloudfunctions.functions.get`. The current CI smoke confirms Function GET is ungranted; this is a separate user approval boundary.

Only that GET is a candidate addition to the existing development custom read role. Preserve existing principals/bindings. Do not add list/create/update/delete/call/invoke/sourceCodeGet/sourceCodeSet, IAM-write, API permissions, broad roles or production grants. Source/archive/build GET permissions used by an operator for provenance do not become CI requirements. This PR grants no permissions and keeps `manage_export_function=false` in the workflow/default root.

For an operator's read-only full-root candidate, explicitly enable the existing topic/Scheduler and Function flags and supply the fresh execution identity privately. Require exactly Function import1 + existing10 no-op, drift/unknown/unexpected action0. Never use `-target` or apply a definition-only probe.

After separately approved GET and reviewed activation, retain independent Environment approvals, same-SHA re-plan/projection equality, the global import-only guard plus an exact eleven-resource validator, protected saved-plan apply and post no-op11/drift0. These activation/apply changes are future work.

## Ownership and rollback

Only the existing Function is an ownership candidate. Google-managed trigger subscription, build artifacts, generated buckets, Artifact Registry, service agents, execution IAM and source remain external. `prevent_destroy` protects the Function. A Function update, source change, unknown configured value or metadata mismatch stops the wave.

Definition rollback is reverting this disabled definition. After any future authorized import, recover state using the versioned S3 recovery procedure under the native lock, preserving later writes and previous ten resources. Never destroy or redeploy a Function to undo state adoption. Keep the daily natural export chain running; manual triggers remain prohibited.

Credentialless mock contracts reject default activation, wrong project, missing execution identity or adoption before topic/Scheduler ownership. They verify the preserved trigger/runtime/limits/environment and source/label boundary; the unchanged global guard rejects a missing-resource create.
