# Primary Email notification channel

One logical primary Email channel per project, owned separately from alert policies. Both environment roots share this module and opt in through their managed inputs; the default is disabled. Preserve the existing selection when operating on an adopted channel.

Both roots own an existing operator-created channel by import at `module.notification_channels.google_monitoring_notification_channel.primary_email[0]`. The production root never creates a channel: selecting `manage_primary_email` without a numeric `primary_email_channel_name` of the production project is rejected. `user_labels` declares the channel's exact labels (null keeps the attribute unset), so the import preserves every value.

## Current inputs and operation

The authenticated workflow reads `GCP_TERRAFORM_MANAGED_INPUTS_JSON` from the matching Plan/Apply GitHub Environments. Maintain the environment-specific values in that payload's `tfvars`:

- `manage_primary_email`: keep ownership selected for an already managed channel.
- `primary_email_address`: private destination address.
- `primary_email_channel_name`: private existing-channel reference used by the root's import contract; it must belong to the selected project.

Preserve the complete managed-input transport, including its schema, manifest, integrity key, target project and other resource selections. Do not replace it with a notification-only fragment. Plan/Apply inputs for the same target must agree. Mailbox and channel identifiers remain private rather than being placed in `normal.tfvars.json` or source.

Use the canonical [DEV normal Plan/Apply guide](../../README.md#通常のdev-terraform開発) or [PROD main-only guide](../../README.md#通常のprod-terraform開発main固定) for normal changes. The normal controller rejects imports except typed exceptions; [`primary_email_adoption.py`](../../scripts/primary_email_adoption.py) admits only this address, an import ID equal to the selected private channel name, and identical before/after values (an `update` that only marks the address label sensitive, or a `no-op`). The production procedure, including the follow-up quota notification repair and rollback, is in [production readiness](../../docs/production-readiness.md#本番の主要メール通知先の採用と通知の修復). See also the [authentication boundary](../../docs/terraform-authentication-boundary.md).

## Protection and verification

`force_delete=false` and `prevent_destroy` protect the channel and references. `sensitive` propagates through the channel label, module output and quota input; it is not encryption. Email labels can be persisted in the encrypted, access-restricted S3 state. No mailbox or deployed channel ID belongs in source, Issues, public logs, Summary or artifacts.

Reading existing channel metadata requires `monitoring.notificationChannels.get`, which the normal Plan and Apply identities already have. Changing channel/alert infrastructure uses the reviewed normal-operation permissions. Do not add Monitoring Editor/admin merely to work around an import guard.

An enabled channel or a successful no-op does not prove delivery/receipt. An unspecified Email verification status is distinct from `UNVERIFIED`. No test email is sent by validation.

## Historical adoption (not executable)

[Issue #1162](https://github.com/kani3camp/youtube-study-space/issues/1162) recorded the one-time development adoption of an operator-created channel. Its `email-adoption` exception allowed only identical before/after values with an email sensitivity-mark change, after backup bucket adoption, and used a five-resource post-plan no-op. Those resource counts and steps describe that historical operation, not the current workflow.

The old `GCP_MONITORING_PRIMARY_EMAIL` / `GCP_MONITORING_PRIMARY_EMAIL_CHANNEL` Secret handoff is no longer consumed by the authenticated workflow. `email-adoption` is retired and rejected by preflight before credentials, including when old feature flags are set. Do not dispatch it or recreate the old Secret handoff as an execution procedure.

The retained `terraform_email_adoption_gate.py` checks the historical exception offline; it neither executes Apply nor authorizes a current operation, and does not replace the current import-only or normal-operation guards.
