# Development history execution window

This source change stages the owner-approved development history-only execution window.
Source approval and a merged activation commit do not approve an unseen plan or start an apply.
A fresh protected plan, the owner's exact plan/run approval, and both protected Environment
reviews remain mandatory. PR CI is credentialless; only the trusted integration branch's
initial-attempt manual dispatch can use the authenticated workflow.

## Scope and gates

The existing `test-youtube-study-space.firestore_export.user-activity-history` table in
`asia-southeast2` is imported into
`module.user_activity_history[0].google_bigquery_table.retained` once. Existing eleven
resources remain no-op. Table rows/schema/configuration, IAM/API, runtime and production
resources must have no updates. The full-root sanitizer rejects any other action.

Only these four execution gates change to true:

- `DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED`
- `DEV_HISTORY_POST_NOOP_ENABLED`
- `DEV_HISTORY_RECEIPT_EMITTER_ENABLED`
- `DEV_OWNERSHIP_RECEIPT_ARTIFACT_ENABLED`

`DEV_HISTORY_POST_NOOP12_READY`, `DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED`, security probe,
production and quota exceptional gates remain false. Retained resource flags stay true.
The issuer catalog remains empty until a separately reviewed historical receipt closure.
Authentication permissions, trust, protected reviewers and branch policy are unchanged.

## Prepare, freeze, plan, execute, close

1. Prepare the private canonical metadata snapshot and random 32-byte comparison nonce.
   Set the same strict history envelope in the workflow-designated
   `GCP_RUNTIME_OWNERSHIP_PACKET_JSON` slot of both existing protected development
   Environments. These comparison packets are not credentials; existing authentication
   secrets are not changed. Missing designated slots may be created by the approved two
   packet settings. Keep plaintext only in the designated private escrow and Secrets.
2. Complete independent activation review and exact-head credentialless CI, merge into
   `feature/gcp-terraform-iac`, and freeze the combined source SHA before making the new plan.
   Stage every required source/private input first; do not create a source/replan loop.
3. Dispatch one fresh protected full-root plan with no runtime wave or post-noop selector.
   Expected: existing11 no-op, exact history import1, all updates/drift/unknown/other0.
   The owner reviews the source, run, sanitized summary and fresh safety input. Binary plans
   stay private and are deleted; earlier expired plans cannot be reused.
4. After the owner approves that exact plan/run, prepare strict one-shot approval v2 and
   separately approve plan/apply Environment jobs. Same-SHA local re-plan/projection/seal
   precede one saved-plan apply/import. Exact state11→12, unchanged original11 and stable
   table metadata, lineage/serial, lock absence, full-root post no-op12 and successful
   outcomes are required before the authentic history root is emitted and published.
5. At the same frozen SHA, run the separate protected post-noop12 once. Close all four
   execution gates while retaining history ownership. A reviewed closure may pin the
   historical issuer SHA for `history12` only after validating actual GitHub provenance,
   protected approvals, ordered successful steps, archive/receipt digests and private HMAC.
   Catalog admission or receipt recovery never requires another import.

The owner-provided state-writer quiet window covers the bounded execution. Do not infer
out-of-band writer absence solely from an empty Actions inventory. Ordinary BigQuery row
writers may continue; output-only statistics may vary but stable metadata must match.

## Publication, bounds and STOP

The first successful history apply requires its root emitter and sanitized publication.
Exactly one fixed canonical public JSON file, at most 16 KiB, is uploaded as a bounded
at most 64 KiB ZIP with 90-day retention. Context identifiers, counts/results and opaque
HMAC commitments are public. Nonce, backend/identity values, state, metadata/schema/data,
private packets, binary plans, logs and credentials are excluded. Signed-in users with
read access to this public repository can download it; downloaded copies cannot be recalled.
See [GitHub artifact access](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/download-workflow-artifacts).
The artifact alone is not authority; all trusted issuer/approval/private binding constraints
in the [receipt contract](runtime-wif-api-ownership.md) remain mandatory.

Maximum 3 authenticated dispatches, 5 Terraform plans, 4 inits, 1 apply and 1 import.
Plan/apply job timeouts remain 15/20 minutes. The additional preparation metadata read is
one exact tables.get. Existing explicit helper accounting is preserved; provider/auth/backend
internal request totals and actual cost remain unknown. Routine cost-only blocking is waived.
No previous security probe rerun, new grant/login/PAT/service or scope expansion is included.

Mismatch, expiry, missing permission, timeout, unknown outcome, upload/cleanup failure or
artifact deletion/expiry means STOP. No automatic rerun, force-unlock, state rm, blind old-state
restore, DDL or table recreation. If state is still11, correction requires a fresh plan/approval.
If ownership is already12, retain it and separately authorize needed no-op/receipt recovery.
Runtime WIF/API adoption, data migration and production readiness are not proved by this window.
