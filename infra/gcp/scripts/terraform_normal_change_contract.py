#!/usr/bin/env python3
"""Credentialless, offline policy for reviewed Terraform create/update proposals.

Not an execution or credential authorization. Never expose plan attributes, secrets
or resource addresses in stdout, stderr, exceptions or returned summaries.
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import re

TERRAFORM_VERSION = "1.16.4"
PROVIDER = "registry.terraform.io/hashicorp/google"
SUPPORTED_TYPES = frozenset({"google_cloud_run_v2_service"})
SCHEMA = "gcp-normal-change-review-v1"
MAX_BYTES = 16 * 1024 * 1024
ADDRESS = re.compile(
    r"(?:module\.[A-Za-z_][A-Za-z0-9_-]*\.)*"
    r"google_cloud_run_v2_service\.[A-Za-z_][A-Za-z0-9_]*"
)
ERROR_CODES = frozenset({
    "INPUT", "REVIEW", "PLAN", "ACTION", "UNAPPROVED", "UNKNOWN", "OUTPUT", "DRIFT",
})


class Rejected(ValueError):
    def __init__(self, code: str):
        # Never embed any untrusted values or resource identities in an error.
        self.code = code if code in ERROR_CODES else "INPUT"
        super().__init__("normal-change contract rejected: " + self.code)


def need(condition: bool, code: str) -> None:
    if not condition:
        raise Rejected(code)


def _same_json(before, after):
    """Compare canonical JSON values without Python's bool/int equality alias."""
    try:
        return json.dumps(before, sort_keys=True, ensure_ascii=False,
                          separators=(",", ":"), allow_nan=False) == json.dumps(
                              after, sort_keys=True, ensure_ascii=False,
                              separators=(",", ":"), allow_nan=False)
    except (TypeError, ValueError, RecursionError, OverflowError):
        return False


def unique_pairs(items):
    result = {}
    for key, value in items:
        need(key not in result, "INPUT")
        result[key] = value
    return result


def load_private(path: Path) -> dict:
    with path.open("rb") as stream:
        raw = stream.read(MAX_BYTES + 1)
    need(0 < len(raw) <= MAX_BYTES, "INPUT")
    try:
        result = json.loads(raw, object_pairs_hook=unique_pairs,
                            parse_constant=lambda _: need(False, "INPUT"))
    except (ValueError, UnicodeDecodeError, RecursionError) as exc:
        raise Rejected("INPUT") from None
    need(type(result) is dict, "INPUT")
    return result


def reviewed_changes(review: dict) -> dict[str, str]:
    need(type(review) is dict and set(review) == {"schema_version", "environment", "changes"}
         and review["schema_version"] == SCHEMA and review["environment"] == "dev"
         and type(review["changes"]) is list and len(review["changes"]) <= 3, "REVIEW")
    allowed = {}
    for item in review["changes"]:
        need(type(item) is dict and set(item) == {"address", "type", "action"}
             and type(item["address"]) is str
             and type(item["type"]) is str and item["type"] in SUPPORTED_TYPES
             and item["action"] in ("create", "update")
             and ADDRESS.fullmatch(item["address"]) is not None, "REVIEW")
        need(item["address"] not in allowed, "REVIEW")
        allowed[item["address"]] = item["action"]
    return allowed


def validate(plan: dict, review: dict) -> dict:
    """Assess a native terraform show -json saved plan without side effects."""
    allowed = reviewed_changes(review)
    need(type(plan) is dict and plan.get("format_version") == "1.2"
         and plan.get("terraform_version") == TERRAFORM_VERSION
         and plan.get("complete") is True and plan.get("errored") is False, "PLAN")
    if "applyable" in plan:
        need(type(plan["applyable"]) is bool, "PLAN")
    for key in ("deferred_changes", "action_invocations", "deferred_action_invocations",
                "resource_drift"):
        need(plan.get(key, []) == [], "DRIFT" if key == "resource_drift" else "PLAN")
    checks = plan.get("checks", [])
    need(type(checks) is list and all(type(c) is dict and c.get("status") == "pass"
                                      for c in checks), "PLAN")
    outputs = plan.get("output_changes", {})
    need(type(outputs) is dict, "OUTPUT")
    for output in outputs.values():
        need(type(output) is dict and output.get("actions") == ["no-op"], "OUTPUT")
        need("before" in output and "after" in output
             and _same_json(output["before"], output["after"]), "OUTPUT")
        need(output.get("after_unknown", {}) == {}, "OUTPUT")
        need(_same_json(output.get("before_sensitive", {}),
                        output.get("after_sensitive", {})), "OUTPUT")
    resources = plan.get("resource_changes")
    need(type(resources) is list and len(resources) <= 1000, "PLAN")
    seen, actual, no_op = set(), {}, 0
    for resource in resources:
        need(type(resource) is dict and type(resource.get("address")) is str
             and resource["address"] not in seen, "PLAN")
        address = resource["address"]
        seen.add(address)
        change = resource.get("change")
        need(type(change) is dict and type(change.get("actions")) is list, "PLAN")
        need("previous_address" not in resource and "importing" not in change
             and not change.get("replace_paths"), "ACTION")
        actions = change["actions"]
        if actions == ["no-op"]:
            need(address not in allowed, "UNAPPROVED")
            need("before" in change and "after" in change
                 and _same_json(change["before"], change["after"]), "ACTION")
            need(change.get("after_unknown", {}) == {}, "UNKNOWN")
            need(_same_json(change.get("before_sensitive", {}),
                            change.get("after_sensitive", {})), "ACTION")
            no_op += 1
            continue
        need(actions in (["create"], ["update"]), "ACTION")
        need(address in allowed and allowed[address] == actions[0], "UNAPPROVED")
        need(resource.get("mode") == "managed"
             and resource.get("type") in SUPPORTED_TYPES
             and resource.get("provider_name") == PROVIDER
             and ADDRESS.fullmatch(address) is not None, "UNAPPROVED")
        need((change.get("before") is None if actions == ["create"]
              else type(change.get("before")) is dict)
             and type(change.get("after")) is dict, "ACTION")
        if actions == ["update"]:
            need(change.get("after_unknown", {}) in ({}, False, None), "UNKNOWN")
            need(change["before"] != change["after"], "ACTION")
        actual[address] = actions[0]
    need(actual == allowed, "UNAPPROVED")
    # Native no-change plans may be non-applyable. Mutating plans must be
    # positively applyable, rather than merely complete and non-errored.
    need(not actual or plan.get("applyable") is True, "PLAN")
    return {"schema_version": "gcp-normal-change-offline-result-v1",
            "policy": "offline-only", "environment": "dev",
            "create": sum(action == "create" for action in actual.values()),
            "update": sum(action == "update" for action in actual.values()),
            "no_op": no_op, "delete": 0, "replace": 0, "drift": 0}


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Credentialless reviewed-change shape check, not an Apply authorization")
    parser.add_argument("--plan", required=True, type=Path)
    parser.add_argument("--review", required=True, type=Path)
    args = parser.parse_args()
    try:
        summary = validate(load_private(args.plan), load_private(args.review))
    except (Rejected, OSError, TypeError, AttributeError, KeyError) as error:
        code = error.code if isinstance(error, Rejected) else "INPUT"
        print("GCP_NORMAL_CHANGE_OFFLINE: STOP; code=" + code)
        return 1
    print("GCP_NORMAL_CHANGE_OFFLINE: PASS; "
          + "; ".join(f"{key}={summary[key]}" for key in
                      ("create", "update", "no_op", "delete", "replace", "drift")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
