#!/usr/bin/env python3
"""Disabled-by-default, state-only reconciliation of three empty label maps.

Requires a saved full-root -refresh-only plan. It cannot authorize workload
changes, general drift reconciliation, imports or IAM changes.
"""
from __future__ import annotations

import copy
from terraform_email_adoption_gate import canonical, has_unknown
from terraform_quota_create_gate import ADDRESSES, MANAGED, validate as validate_quota


def resources(module):
    result = list(module.get("resources", []))
    for child in module.get("child_modules", []):
        result.extend(resources(child))
    return result


def validate(plan, *, channel_name, email):
    def require(condition):
        if not condition:
            raise ValueError("Development quota state refresh STOP; private diagnostic suppressed.")

    require(plan.get("complete") is True and plan.get("errored") is False and plan.get("applyable") is True)
    # Refresh-only JSON omits resource_changes and planned resource values;
    # prior_state is the complete provider-refreshed graph.
    require(not plan.get("resource_changes") and not plan.get("deferred_changes"))
    require(not resources(plan.get("planned_values", {}).get("root_module", {})))
    require(all(c.get("status") == "pass" for c in plan.get("checks", [])))
    graph = resources(plan.get("prior_state", {}).get("values", {}).get("root_module", {}))
    require(len(graph) == 8 and {r.get("address") for r in graph} == MANAGED | ADDRESSES)
    require(set(plan.get("output_changes", {})) == {"environment", "project_id"})
    for output in plan["output_changes"].values():
        require(output.get("actions") == ["no-op"] and not has_unknown(output.get("after_unknown", {})))
        require(canonical(output.get("before")) == canonical(output.get("after")))
    drift = plan.get("resource_drift", [])
    require(len(drift) == 3 and {r.get("address") for r in drift} == ADDRESSES)
    for resource in drift:
        change = resource.get("change", {})
        require(resource.get("mode") == "managed" and resource.get("type") == "google_monitoring_alert_policy")
        require(change.get("actions") == ["update"] and not change.get("importing"))
        require(not has_unknown(change.get("after_unknown", {})))
        before, after = copy.deepcopy(change.get("before")), change.get("after")
        require(isinstance(before, dict) and isinstance(after, dict))
        require("user_labels" in before and before["user_labels"] is None and after.get("user_labels") == {})
        before["user_labels"] = {}
        require(canonical(before) == canonical(after))
        old_mask, new_mask = copy.deepcopy(change.get("before_sensitive")), change.get("after_sensitive")
        require(isinstance(old_mask, dict) and isinstance(new_mask, dict))
        require(old_mask.get("user_labels") in (None, False) and new_mask.get("user_labels") == {})
        old_mask["user_labels"] = {}
        require(canonical(old_mask) == canonical(new_mask))
        value = next(r.get("values") for r in graph if r["address"] == resource["address"])
        require(canonical(value) == canonical(after))
    # Independently enforce the existing fixed policy contract on the refreshed
    # graph. This synthetic graph is validation-only; never used for execution
    # or to conceal the actual three drift records in the public summary.
    projection = {"resource_changes": [
        {"address": r["address"], "mode": r["mode"], "type": r["type"], "change": {
            "actions": ["no-op"], "before": r["values"], "after": r["values"], "after_unknown": {}}}
        for r in graph
    ], "output_changes": plan["output_changes"]}
    validate_quota(projection, channel_name=channel_name, email=email, phase="post")
