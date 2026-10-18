#!/usr/bin/env python3
"""Credentialless, offline policy for reviewed Terraform create/update proposals.

Not an execution or credential authorization. Never expose plan attributes, secrets
or resource addresses in stdout, stderr, exceptions or returned summaries.
"""
from __future__ import annotations

import argparse
import hashlib
import hmac
import json
from pathlib import Path
import re
import uuid

TERRAFORM_VERSION = "1.16.4"
PROVIDER = "registry.terraform.io/hashicorp/google"
SUPPORTED_TYPES = frozenset({"google_cloud_run_v2_service"})
SCHEMA = "gcp-normal-change-review-v1"
BOUND_SCHEMA = "gcp-normal-change-review-v2"
BASELINE_SCHEMA = "gcp-normal-change-baseline-v1"
MAX_BYTES = 16 * 1024 * 1024
ADDRESS = re.compile(
    r"(?:module\.[A-Za-z_][A-Za-z0-9_-]*\.)*"
    r"google_cloud_run_v2_service\.[A-Za-z_][A-Za-z0-9_]*"
)
ERROR_CODES = frozenset({
    "INPUT", "REVIEW", "PLAN", "ACTION", "UNAPPROVED", "UNKNOWN", "OUTPUT", "DRIFT",
    "SOURCE", "STATE", "BINDING",
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



def digest(value) -> str:
    """Private canonical JSON hash; never publish a value, address or digest."""
    try:
        raw = json.dumps(value, sort_keys=True, ensure_ascii=False,
                         separators=(",", ":"), allow_nan=False).encode("utf-8")
    except (TypeError, ValueError, OverflowError, RecursionError):
        raise Rejected("INPUT") from None
    need(len(raw) <= MAX_BYTES, "INPUT")
    return hashlib.sha256(raw).hexdigest()


def _hex(value, count):
    return type(value) is str and re.fullmatch("[0-9a-f]{" + str(count) + "}", value) is not None


def _root_resources(prior_values):
    """Flatten managed resources in Terraform's prior_state.values only.

    The collector must separately prove the S3 VersionId, lock and state lineage.
    This helper checks a supplied snapshot, never performs that cloud read.
    """
    need(type(prior_values) is dict and type(prior_values.get("root_module")) is dict, "STATE")
    result = {}

    def visit(module):
        need(type(module) is dict, "STATE")
        resources, children = module.get("resources", []), module.get("child_modules", [])
        need(type(resources) is list and type(children) is list
             and len(result) + len(resources) <= 1000, "STATE")
        for item in resources:
            need(type(item) is dict and type(item.get("address")) is str
                 and item["address"] not in result and item.get("mode") == "managed"
                 and type(item.get("type")) is str and item["type"].startswith("google_")
                 and item.get("provider_name") == PROVIDER and "values" in item, "STATE")
            result[item["address"]] = item
        for child in children:
            visit(child)

    visit(prior_values["root_module"])
    return result


def validate_bound(plan: dict, review: dict, baseline: dict, *, source_sha: str) -> dict:
    """Strict offline join of immutable source selector, reviewed plan and supplied State.

    No GitHub approval, trusted State read, AWS/GCP permission or Apply authorization
    is established by these user-supplied inputs.
    """
    need(_hex(source_sha, 40), "SOURCE")
    need(type(review) is dict and
         set(review) == {"schema_version", "environment", "source_sha", "baseline",
                         "plan_sha256", "changes"} and
         review["schema_version"] == BOUND_SCHEMA and review["environment"] == "dev"
         and review["source_sha"] == source_sha, "REVIEW")
    need(type(baseline) is dict and
         set(baseline) == {"schema_version", "environment", "source_sha",
                           "version_id", "lineage", "serial", "prior_values"} and
         baseline["schema_version"] == BASELINE_SCHEMA
         and baseline["environment"] == "dev" and baseline["source_sha"] == source_sha
         and type(baseline["version_id"]) is str and
         0 < len(baseline["version_id"]) <= 256
         and not any(ch in baseline["version_id"] for ch in "\r\n\x00")
         and type(baseline["lineage"]) is str
         and type(baseline["serial"]) is int and baseline["serial"] >= 0, "STATE")
    try:
        need(str(uuid.UUID(baseline["lineage"])) == baseline["lineage"], "STATE")
    except (ValueError, AttributeError):
        raise Rejected("STATE") from None
    pinned = review["baseline"]
    need(type(pinned) is dict and
         set(pinned) == {"version_id", "lineage", "serial", "prior_values_sha256"}
         and _hex(pinned["prior_values_sha256"], 64)
         and pinned["version_id"] == baseline["version_id"]
         and pinned["lineage"] == baseline["lineage"]
         and type(pinned["serial"]) is int and pinned["serial"] == baseline["serial"]
         and hmac.compare_digest(pinned["prior_values_sha256"],
                                 digest(baseline["prior_values"])), "BINDING")
    need(_hex(review["plan_sha256"], 64) and
         hmac.compare_digest(review["plan_sha256"], digest(plan)), "BINDING")
    prior = plan.get("prior_state")
    need(type(prior) is dict and prior.get("format_version") == "1.0"
         and type(prior.get("values")) is dict
         and digest(prior["values"]) == pinned["prior_values_sha256"], "STATE")
    old = _root_resources(baseline["prior_values"])
    changes = review["changes"]
    need(type(changes) is list and len(changes) <= 3, "REVIEW")
    simple = []
    change_hashes = {}
    for change in changes:
        need(type(change) is dict and
             set(change) == {"address", "type", "action", "change_sha256"}
             and _hex(change["change_sha256"], 64), "REVIEW")
        simple.append({key: change[key] for key in ("address", "type", "action")})
        change_hashes[change["address"]] = change["change_sha256"]
    # Keep the v1 action/name/cap contract, including rejecting all forbidden
    # deletes, moves, unknown update outcomes, imports, drift and output writes.
    result = validate(plan, {"schema_version": SCHEMA, "environment": "dev",
                             "changes": simple})
    current = set()
    for resource in plan["resource_changes"]:
        address, action = resource["address"], resource["change"]["actions"]
        item = old.get(address)
        if action == ["create"]:
            need(item is None, "STATE")
        else:
            need(item is not None and resource.get("mode") == item["mode"]
                 and resource.get("type") == item["type"]
                 and resource.get("provider_name") == item["provider_name"]
                 and "before" in resource["change"]
                 and _same_json(item["values"], resource["change"]["before"]), "STATE")
        if item is not None:
            current.add(address)
        if action in (["create"], ["update"]):
            need(hmac.compare_digest(digest(resource["change"]),
                                     change_hashes[address]), "BINDING")
    need(current == set(old), "STATE")
    return dict(result, policy="offline-bound-only", source_bound=True,
                baseline_bound=True, plan_bound=True)

def main() -> int:
    parser = argparse.ArgumentParser(
        description="Credentialless reviewed-change shape check, not an Apply authorization")
    parser.add_argument("--plan", required=True, type=Path)
    parser.add_argument("--review", required=True, type=Path)
    parser.add_argument("--baseline", type=Path,
                        help="Private independently collected State projection for v2")
    parser.add_argument("--source-sha", help="Explicit reviewed source SHA for v2")
    args = parser.parse_args()
    try:
        plan = load_private(args.plan)
        review = load_private(args.review)
        if review.get("schema_version") == BOUND_SCHEMA:
            need(args.baseline is not None and args.source_sha is not None, "SOURCE")
            summary = validate_bound(plan, review, load_private(args.baseline),
                                     source_sha=args.source_sha)
        else:
            need(args.baseline is None and args.source_sha is None, "SOURCE")
            summary = validate(plan, review)
    except (Rejected, OSError, TypeError, AttributeError, KeyError) as error:
        code = error.code if isinstance(error, Rejected) else "INPUT"
        print("GCP_NORMAL_CHANGE_OFFLINE: STOP; code=" + code)
        return 1
    print("GCP_NORMAL_CHANGE_OFFLINE: PASS; policy=" + summary["policy"] + "; "
          + "; ".join(f"{key}={summary[key]}" for key in
                      ("create", "update", "no_op", "delete", "replace", "drift")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
