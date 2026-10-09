# Trusted Privacy support operator (source only)

## Task contract

**Goal:** Review a verified in-app Privacy receipt, stage one operator reply with an atomic body-free audit event, and close disclosure or Firebase-session revoke requests only after independently verified action evidence and reply-delivery acknowledgement. Deletion closes only through the existing fenced deletion `Finalize` workflow.

**Invariants:** Request ref and support challenge are not operator authority. Environment, project, purpose, request ref, proof ref, stable operation ID and expected revision must agree. A reply never means the requested action is complete. A deletion claim blocks a new reply. Terminal records contain no body, reply, channel, proof, OAuth link or submission index. Public HTTP routes cannot write operator state. Both Privacy intake feature flags stay off.

**Non-goals:** Live execution, operator identity provisioning, IAM, external response delivery, provider action execution, historical audit backfill, audit retention policy, a no-session status credential, and Google-grant/session/data linkage changes.

**Acceptance:** A missing or mismatched trusted authority fails closed. Identical reply retries survive lost acknowledgements; changed content or a second operation conflicts. Completion needs authority-verified action and delivery evidence and scrubs sensitive fields and indexes atomically. Deletion finalization emits a minimal terminal audit event inside its existing transaction. Emulator and CLI tests cover the denial and cleanup boundaries.

**Verification:** `go test -shuffle=on ./...`, `go test -race -shuffle=on ./core/mypage ./cmd/mypage-support-operator`, lint, Firestore Emulator integration and client-rules denial tests, documentation references, and public diff and secret checks.

## Boundary and operation

`FirestoreSupportOperator` is a server-only library. Its `OperatorAuthority` interface must obtain a real operator identity and minimal permission for the bound environment, project, purpose, request and action from a trusted runtime. The production adapter is intentionally absent. An empty adapter, an operator name typed at a CLI, an intake challenge, or a Firestore server credential alone cannot authorize a call. Synthetic tests inject a narrow fake. The adapter also verifies independent action and delivery evidence for completion; evidence references supplied by a caller are not self-authenticating.

`Reply` requires the actual verified receipt and proof reference, no deletion proof claim, and the expected operator revision. It stores the private body and a keyed fingerprint on the request, and creates one bounded audit event in the same transaction. The audit event contains only schema, environment, purpose, action, trusted actor, keyed receipt binding, keyed operation fingerprint and timestamp. It contains no body, channel, raw proof, or raw request reference. A repeated operation with the same payload and actor is idempotent while the request remains open; changed payloads and another operation conflict. The status remains `verified`.

`Complete` applies to `disclosure` and `revoke` only. Its trusted adapter must verify both action evidence and acknowledgement that the operator reply was delivered through the agreed channel. The transaction checks the exact reply operation and revision, then replaces the receipt with a minimal terminal marker, removes the request-ID index and OAuth transaction, and creates a body-free completion audit event. The receipt replacement removes private body, reply, channel, proof, challenge, submission fingerprint and reply metadata. This does not execute or attest to a Google OAuth grant cancellation. A session-revoke action must have its own independently verified evidence.

For `delete`, the existing deletion engine retains sole control of `Finalize`. That transaction already removes the receipt's sensitive fields and related index and OAuth record; it now also creates one body-free terminal audit event. No old receipt is automatically deleted and no historical audit event is invented. Audit retention and disposal require an approved policy before any real execution.

## Offline CLI

`system/cmd/mypage-support-operator` accepts explicit `--environment`, `--project`, `--purpose`, `--request-ref`, `--proof-ref`, `--operation-id`, and `--expected-revision`. Its default read-only preview checks those values against `MYPAGE_ENVIRONMENT` and `GOOGLE_CLOUD_PROJECT` and rejects conflicting project aliases or live credential/emulator environment. For `--action reply`, `--reply-file` must be an exact `0600` regular file; final symlinks, oversized data and invalid text are rejected. Its contents never appear in stdout or error output. For `--action complete`, the private input includes `--reply-operation-id`, `--action-evidence` and `--delivery-ack` instead of a body. These are proposal references, not proof by themselves. The CLI does not read Firestore or resolve a live identity. `--execute` always returns `TRUSTED_OPERATOR_ADAPTER_UNAVAILABLE` without SDK bootstrap or mutation.

## Remaining release facts

Before a live operator path can be implemented, the owner must supply the real operator identity source and minimal permission scope, approve audit retention and access controls, establish a reply-capable delivery and acknowledgement procedure, and publish the human exception contact for users without a usable Firebase session. The response channel must handle account deletion and lost sessions. D02's 2026-10-09 design decision is to avoid refresh-token storage and periodic polling solely for external Google grant-cancellation detection; policy compliance and the handling of retained API data after cancellation remain unconfirmed for verification/compliance review. D03 contact values remain undecided. The 0B gates and both feature flags remain unchanged; passing synthetic tests or merging source does not mean a released Privacy feature.
