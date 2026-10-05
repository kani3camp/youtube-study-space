# YouTube quota alerts

Read-only baseline: 2026-10-05 production has three enabled MQL policies: minute 80%, day 80%, day 60%. Each uses `consumer_quota`, `serviceruntime.googleapis.com/quota/rate/net_usage` divided by `quota/limit`, YouTube default quota metric, OR, duration 60s, trigger count 1, and auto-close 7 days. The minute query uses 1-minute delta alignment; daily usage uses the America/Los_Angeles quota day and a 1-day window; both evaluate every 30s. The 60% daily policy explicitly prompts OPENED/CLOSED; other policies have no explicit prompts. Severity and user labels are absent.

Both projects currently have zero notification channels. All production policies reference the same missing channel. Do not copy that dangling reference or silently create an unnotified alert.

## Migration behavior

Both roots call this shared module with `manage_youtube_quota_alerts = false` by default. Therefore importing other resources does not create or acquire alert policies. Existing production policies are untouched. Threshold / duration / enabled / optional severity / channels are environment inputs. Stable keys retain the original policy provenance even if thresholds later change. New names and documentation include the explicit environment.

`manage_policies = true` requires verified existing channels in the same project. The precondition checks channel shape/project; **operator read-back must additionally prove existence and notification suitability** before opt-in. No channel is owned or created by this module. Notification channel provisioning is outside this work's approval.

The mock plan exercises all three logical types and both environments, and proves missing/foreign channels are rejected. Opt-in with fixture channels yields **create=3**, rejected by the unchanged import-only sanitizer. This is a credentialless synthetic plan, not an authenticated GCP plan or proof of channel delivery. Current migration roots yield zero alert resource actions. Actual development creation is deferred to a separately approved normal-change apply after a safe channel is available. Do not relax global migration guards to enable these alerts.

## Future production import

Production import/backend remain prohibited in this wave. After their separate approval, use the same module addresses with exact existing display-name overrides and verified channel configuration:

| Module key | Existing policy suffix |
| --- | --- |
| minute_80 | 15336510856705008605 |
| day_80 | 6495260290910048395 |
| day_60 | 7765713814956280710 |

Address: `module.youtube_quota_alerts.google_monitoring_alert_policy.quota["<key>"]`. Import ID: `projects/<production-project>/alertPolicies/<suffix>`. No production import blocks are added now. Existing names can be preserved with `youtube_quota_legacy_display_names`; environment naming/documentation and repaired channel changes require a separately reviewed normal change, not a no-op migration. Resolve notification-path drift before attempting ownership.

MQL is kept to preserve the observed quota semantics. Google still permits API-based creation of MQL policies, although console creation/support ended: [official MQL policy](https://docs.cloud.google.com/stackdriver/docs/deprecations/mql). PromQL migration is a separate behavior change.

## Verification

From repository root, `python3 .github/scripts/test_youtube_quota_alerts.py` stages the module with the root's pinned provider lock in a temporary directory, runs mock tests, validates real mock resource actions with the existing sanitizer, and removes temporary output. The normal Terraform CI job runs it credentiallessly. No saved plan/raw JSON artifact is uploaded.
