#!/usr/bin/env python3
"""Review a development Email import that changes sensitivity metadata only.

This validator neither authorizes nor executes apply. The authenticated import
workflow remains disabled for this resource. A separate reviewed execution path
and explicit approval are required before adopting its state metadata.
"""
from __future__ import annotations

import json
from typing import Any

CHANNEL = "module.notification_channels.google_monitoring_notification_channel.primary_email[0]"
EXISTING = {
    "google_firestore_backup_schedule.daily",
    "module.firestore_export_dataset.google_bigquery_dataset.retained",
    "module.order_history.google_bigquery_table.retained",
    "module.backup_bucket.google_storage_bucket.retained",
}


def canonical(value: Any) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"))


def has_unknown(value: Any) -> bool:
    if isinstance(value, dict):
        return any(has_unknown(item) for item in value.values())
    if isinstance(value, list):
        return any(has_unknown(item) for item in value)
    return value is not False and value is not None


def validate(plan: dict[str, Any], *, email: str, channel_name: str) -> None:
    """Reject every resource/value/unknown change except the reviewed mark."""
    def require(condition: bool) -> None:
        if not condition:
            raise ValueError("Email state-metadata adoption STOP; private details suppressed.")

    require(isinstance(plan, dict) and not plan.get("resource_drift"))
    changes = plan.get("resource_changes", [])
    require(isinstance(changes, list) and len(changes) == len(EXISTING) + 1)
    require({item.get("address") for item in changes} == EXISTING | {CHANNEL})
    require(channel_name.startswith("projects/test-youtube-study-space/notificationChannels/"))
    for output in plan.get("output_changes", {}).values():
        require(output.get("actions") == ["no-op"])
    for item in changes:
        require(item.get("mode") == "managed")
        change = item.get("change", {})
        if item["address"] != CHANNEL:
            require(change.get("actions") == ["no-op"] and not change.get("importing"))
            continue
        require(item.get("type") == "google_monitoring_notification_channel")
        require(change.get("actions") == ["update"])
        require(change.get("importing", {}).get("id") == channel_name)
        before, after = change.get("before"), change.get("after")
        require(isinstance(before, dict) and isinstance(after, dict))
        require(canonical(before) == canonical(after) and not has_unknown(change.get("after_unknown", {})))
        require(after.get("project") == "test-youtube-study-space" and after.get("name") == channel_name)
        require(after.get("type") == "email" and after.get("enabled") is True and after.get("force_delete") is False)
        require(after.get("labels") == {"email_address": email})
        require(after.get("verification_status") != "UNVERIFIED")
        bs, ats = change.get("before_sensitive", {}), change.get("after_sensitive", {})
        require(isinstance(bs, dict) and isinstance(ats, dict))
        require({key for key in bs.keys() | ats.keys() if bs.get(key) != ats.get(key)} == {"labels"})
        require(ats.get("labels") == {"email_address": True})

