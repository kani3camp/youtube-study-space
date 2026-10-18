#!/usr/bin/env python3
"""Exact development quota contract; never authorizes IAM grants or apply."""
from __future__ import annotations

from pathlib import Path
import hashlib
from terraform_email_adoption_gate import CHANNEL, EXISTING, canonical, has_unknown

PROJECT = "test-youtube-study-space"
MANAGED = EXISTING | {CHANNEL}
TYPES = {
    "minute_80": ("minute", "Queries per minute - high", "0.8", []),
    "day_80": ("day", "Queries per day - high", "0.8", []),
    "day_60": ("day", "Queries per day - warning", "0.6", ["OPENED", "CLOSED"]),
}
ADDRESSES = {f'module.youtube_quota_alerts.google_monitoring_alert_policy.quota["{key}"]' for key in TYPES}
COMPUTED = {"id", "name", "creation_record", "deletion_policy"}
# The #1175 queries are the approved design, not a moving implementation target.
QUERY_HASHES = {
    "day": "98b1e65f51708bfa52127f7567cf2e49753baba0527b600ba03eca9a9ee5d769",
    "minute": "422748775db2cdf9f09efa3e38ed06fa45ed544c08138516e2e5c067d175a307",
}


def expected(key: str, channel_name: str, phase="before") -> dict:
    period, label, ratio, prompts = TYPES[key]
    source = Path(__file__).resolve().parents[2] / "infra/gcp/modules/youtube-quota-alerts/queries"
    template = (source / (period + ".mql.tftpl")).read_bytes()
    if hashlib.sha256(template).hexdigest() != QUERY_HASHES[period]:
        raise ValueError("Approved quota query changed; re-review required.")
    query = template.decode().strip().replace("${project_id}", PROJECT).replace("${threshold}", ratio)
    result = {
        "project": PROJECT, "display_name": "[development] YouTube quota " + label,
        "combiner": "OR", "enabled": True, "severity": None, "notification_channels": [channel_name],
        "user_labels": {}, "timeouts": None,
        "alert_strategy": [{"auto_close": "604800s", "notification_channel_strategy": [],
                            "notification_prompts": prompts, "notification_rate_limit": []}],
        "conditions": [{"display_name": "Quota usage reached defined threshold", "condition_absent": [],
                        "condition_matched_log": [], "condition_prometheus_query_language": [], "condition_sql": [],
                        "condition_threshold": [], "condition_monitoring_query_language": [{
                            "duration": "60s", "evaluation_missing_data": None, "query": query,
                            "trigger": [{"count": 1, "percent": None}]}]}],
        "documentation": [{"content": "[development] Review YouTube quota usage: https://console.cloud.google.com/iam-admin/quotas?service=${resource.label.service}&metric=${metric.label.quota_metric}&limit=${metric.label.limit_name}&project=" + PROJECT + "&fromNotifications=1",
                           "mime_type": "text/markdown", "links": [], "subject": None}],
    }

    if phase == "post":
        # Provider 8.5.0 read-back materializes unset scalar defaults. Match the
        # exact observed defaults, never normalize arbitrary configured values.
        result["severity"] = ""
        result["documentation"][0]["subject"] = ""
        mql = result["conditions"][0]["condition_monitoring_query_language"][0]
        mql["evaluation_missing_data"] = ""
        mql["trigger"][0]["percent"] = 0
    return result


def unknown_paths(value, prefix=()):
    if isinstance(value, dict):
        return set().union(*(unknown_paths(v, prefix + (k,)) for k, v in value.items())) if value else set()
    if isinstance(value, list):
        return set().union(*(unknown_paths(v, prefix + (i,)) for i, v in enumerate(value))) if value else set()
    return {prefix} if value is not False and value is not None else set()


def stable(value):
    value = {k: v for k, v in value.items() if k not in COMPUTED}
    value["conditions"] = [{k: v for k, v in condition.items() if k != "name"} for condition in value["conditions"]]
    return value


def validate(plan, *, channel_name, email, phase="before"):
    def require(condition):
        if not condition: raise ValueError("Development quota contract STOP; private diagnostic suppressed.")
    require(phase in {"before", "post"} and channel_name.startswith(f"projects/{PROJECT}/notificationChannels/"))
    require(not plan.get("resource_drift"))
    changes = plan.get("resource_changes", [])
    require(len(changes) == 8 and {r.get("address") for r in changes} == MANAGED | ADDRESSES)
    for output in plan.get("output_changes", {}).values():
        require(output.get("actions") == ["no-op"] and not has_unknown(output.get("after_unknown", {})))
    for resource in changes:
        change = resource.get("change", {})
        require(resource.get("mode") == "managed" and not change.get("importing"))
        if resource["address"] in MANAGED or phase == "post":
            require(change.get("actions") == ["no-op"] and not has_unknown(change.get("after_unknown", {})))
            require(canonical(change.get("before")) == canonical(change.get("after")))
        else:
            require(change.get("actions") == ["create"] and change.get("before") is None)
            allowed = {(key,) for key in COMPUTED} | {("conditions", 0, "name")}
            require(unknown_paths(change.get("after_unknown", {})) <= allowed)
        after = change.get("after") or {}
        if resource["address"] == CHANNEL:
            require(after.get("project") == PROJECT and after.get("name") == channel_name)
            require(after.get("type") == "email" and after.get("enabled") is True)
            require(after.get("labels") == {"email_address": email} and after.get("verification_status") != "UNVERIFIED")
        elif resource["address"] in ADDRESSES:
            require(resource.get("type") == "google_monitoring_alert_policy")
            require(after.get("deletion_policy") in {None, "DELETE"})
            key = next(key for key in TYPES if resource["address"].endswith(f'["{key}"]'))
            require(canonical(stable(after)) == canonical(expected(key, channel_name, phase=phase)))
            if phase == "post": require(after.get("name", "").startswith(f"projects/{PROJECT}/alertPolicies/"))
