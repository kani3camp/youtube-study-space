#!/usr/bin/env python3
"""One protected history plan: bounded cost and private before/after evidence.

Only existing AWS/GCP read interfaces are used. No unlock, mutation, credential
minting or execution is provided. Cloud-side billing bounds are owner-reviewed
evidence, not a claim that this identity can inspect account billing settings.
"""
from __future__ import annotations

import argparse
from datetime import datetime, timedelta, timezone
from decimal import Decimal, ROUND_CEILING
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import uuid

from terraform_identity_diagnostics import RECEIPT_REASONS, ReceiptDependencyError, ReceiptInvariantError, at_stage
from terraform_identity_smoke import aws, google
from terraform_email_adoption_gate import CHANNEL, EXISTING
from terraform_quota_create_gate import ADDRESSES
from terraform_export_topic_gate import TOPIC
from terraform_export_scheduler_gate import SCHEDULER
from terraform_export_function_gate import FUNCTION

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "infra/gcp/scripts"))
from prepare_user_activity_history_adoption import prepare, unique_object
from prepare_user_activity_history_workflow import TABLE_PATH, PrivateArgumentParser
from validate_user_activity_history_plan import TABLE
from terraform_history_table_metadata import stable_table_metadata, volatile_table_metadata

BASELINE = EXISTING | {CHANNEL, TOPIC, SCHEDULER, FUNCTION} | ADDRESSES
ZERO_INSTANCE_CHECKS = frozenset({
    ("var", "module.user_activity_history.var.field_order"),
    ("var", "module.user_activity_history.var.field_descriptions"),
})
STATE_KEY = "youtube-study-space/dev/terraform.tfstate"
MODEL = "dev-history-plan-monthly-2026-10-09-v1"
MODEL_EXPIRES = datetime(2026, 10, 15, tzinfo=timezone.utc)
MAX_BYTES = 4 * 1024 * 1024
# Reviewed conservative ceilings, not free-tier estimates. See infra/gcp/README.
# 64 small-state downloads cover backend refreshes and native downloader body
# attempts; 256 requests cover HEAD/LIST/GET and one native lock lifecycle.
DOWNLOADS, REQUESTS, KMS_REQUESTS, HEADER_BYTES, LOCK_BYTES = 64, 256, 128, 16384, 32768
GIB = Decimal(1024 ** 3)
LIMIT = Decimal("0.25")
COST_KEYS = {"model", "git_sha", "issued_utc", "expires_utc", "max_state_bytes",
             "budget_month", "cloud_side_cost_usd", "cloud_side_evidence_reviewed",
             "rates_verified", "state_writers_quiescent"}


def require(value, reason):
    if reason not in RECEIPT_REASONS:
        raise ValueError("Invalid receipt diagnostic label")
    if not value:
        raise ReceiptInvariantError(reason)


def decode(raw):
    def unique(pairs):
        try:
            return unique_object(pairs)
        except ValueError:
            raise ReceiptInvariantError("json-duplicate") from None

    def invalid(_):
        raise ReceiptInvariantError("json-nonfinite")

    try:
        return json.loads(raw, object_pairs_hook=unique, parse_constant=invalid)
    except ReceiptInvariantError:
        raise
    except (ValueError, TypeError, UnicodeDecodeError, RecursionError):
        raise ReceiptInvariantError("json-parse") from None


def context(env):
    require(env.get("TF_VAR_project_id") == "test-youtube-study-space", "context-project")
    require(env.get("TF_VAR_manage_user_activity_history") == "true", "context-history")
    require(env.get("MODE") == "plan" and env.get("TF_WORKSPACE") == "default", "context-mode")
    require(re.fullmatch(r"[0-9a-f]{40}", env.get("GITHUB_SHA", "")), "context-sha")
    require(type(env.get("RUNNER_TEMP")) is str, "context-temp")
    directory = Path(env["RUNNER_TEMP"])
    require(directory.is_absolute() and directory.is_dir() and not directory.is_symlink(), "context-temp")
    return directory


def private_bytes(path):
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
        with os.fdopen(fd, "rb") as handle:
            info = os.fstat(handle.fileno())
            require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid() and not info.st_mode & 0o077, "file-owner-mode")
            raw = handle.read(MAX_BYTES + 1)
    except OSError:
        raise ReceiptDependencyError("file-open-read") from None
    require(len(raw) <= MAX_BYTES, "file-size")
    return raw


def private_json(path):
    return decode(private_bytes(path))


def write_private(path, value):
    try:
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(value, handle, separators=(",", ":"), allow_nan=False)
            handle.write("\n")
    except OSError:
        raise ReceiptDependencyError("file-open-write") from None


def timestamp(value):
    require(type(value) is str and re.fullmatch(r"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ", value), "timestamp-shape")
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        raise ReceiptInvariantError("timestamp-shape") from None


def cost_policy(raw, sha, *, now=None):
    require(type(raw) is str and len(raw) <= 4096, "cost-json-size")
    profile = decode(raw)
    require(type(profile) is dict and set(profile) == COST_KEYS, "cost-profile-shape")
    require(profile["model"] == MODEL and profile["git_sha"] == sha, "cost-model-sha")
    now = now or datetime.now(timezone.utc)
    issued, expires = timestamp(profile["issued_utc"]), timestamp(profile["expires_utc"])
    require(issued <= now < expires <= issued + timedelta(hours=24) and expires <= MODEL_EXPIRES, "cost-time-window")
    month = profile["budget_month"]
    require(type(month) is str and month == issued.strftime("%Y-%m") == now.strftime("%Y-%m"), "cost-month")
    # Exclude the month boundary: the plan job has a 15-minute timeout. The
    # additional margin covers cancellation and bounded post-safety reads.
    following = issued.replace(year=issued.year + (issued.month == 12), month=issued.month % 12 + 1,
                               day=1, hour=0, minute=0, second=0, microsecond=0)
    require(expires <= following - timedelta(minutes=30), "cost-month-margin")
    for key in ("rates_verified", "cloud_side_evidence_reviewed", "state_writers_quiescent"):
        require(profile[key] is True, "cost-attestation")
    size = profile["max_state_bytes"]
    require(type(size) is int and 0 < size <= MAX_BYTES, "cost-state-size")
    overhead = profile["cloud_side_cost_usd"]
    require(type(overhead) is str and re.fullmatch(r"0\.\d{1,9}", overhead), "cost-overhead-shape")
    overhead = Decimal(overhead)
    require(0 < overhead < LIMIT, "cost-overhead-range")
    # Reserve a full billing month of added storage, even for a late-month
    # execution. No finite deletion deadline or lifetime cost is asserted.
    # These are conservative estimates; the exact state size is checked later
    # by HEAD. Unknown side-cost facts must not be replaced by true flags.
    quantum = Decimal("0.000000001")
    once = (Decimal(REQUESTS) * Decimal("0.00001")
            + Decimal(KMS_REQUESTS) * Decimal("0.00001")
            + Decimal(DOWNLOADS * size + REQUESTS * HEADER_BYTES) / GIB * Decimal("0.25")
            + overhead).quantize(quantum, rounding=ROUND_CEILING)
    storage = (Decimal(2 * LOCK_BYTES) / GIB * Decimal("0.10")).quantize(quantum, rounding=ROUND_CEILING)
    total = once + storage
    require(total <= LIMIT, "cost-cap")
    ledger = {"schema_version": 1, "basis": "conservative-upper-bound-estimate",
              "entry_id": hashlib.sha256(json.dumps(profile, sort_keys=True, separators=(",", ":")).encode()).hexdigest(),
              "git_sha": sha, "budget_month": month, "status": "reserved-upper-bound",
              "one_time_and_current_month_side_cost_upper_bound_usd": str(once),
              "current_month_added_lock_storage_upper_bound_usd": str(storage),
              "current_month_added_cost_upper_bound_usd": str(total),
              "added_retained_bytes_upper_bound": 2 * LOCK_BYTES,
              "future_full_month_lock_storage_estimate_at_model_rates_usd": str(storage),
              "retention_end_utc": None, "retention_assumption": "no-assumed-deletion",
              "future_side_costs": "not-assumed-zero; reconcile-from-read-evidence",
              "future_rates": "revalidate-each-month"}
    return {"profile": profile, "upper_bound_usd": str(total), "monthly_ledger_entry": ledger}


def policy(env):
    directory = context(env)
    value = cost_policy(env.get("PLAN_COST_EVIDENCE", ""), env["GITHUB_SHA"])
    write_private(directory / "history-plan-cost.json", value)


def load_policy(env, *, fresh=True):
    directory = context(env)
    stored = private_json(str(directory / "history-plan-cost.json"))
    require(type(stored) is dict and set(stored) == {"profile", "upper_bound_usd", "monthly_ledger_entry"}, "policy-record-shape")
    # A late/failed job must still collect bounded safety reads. Expired evidence
    # can never produce PASS; validate freshness again after the post checks.
    check_time = None if fresh else timestamp(stored["profile"]["issued_utc"])
    require(cost_policy(json.dumps(stored["profile"]), env["GITHUB_SHA"], now=check_time) == stored, "policy-record-match")
    require(env.get("STATE_KEY") == STATE_KEY and env.get("STATE_AWS_REGION") == "ap-northeast-1", "policy-state-target")
    require(re.fullmatch(r"\d{12}", env.get("STATE_ACCOUNT_ID", "")), "policy-account-shape")
    require(re.fullmatch(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]", env.get("STATE_BUCKET", "")), "policy-bucket-shape")
    require(env.get("AWS_MAX_ATTEMPTS") == "1" and env.get("AWS_RETRY_MODE") == "standard", "policy-retry")
    return directory, stored


def s3(env, operation, *args, request=None):
    request = request or aws
    try:
        result = request("s3api", operation, "--bucket", env["STATE_BUCKET"],
                         "--expected-bucket-owner", env["STATE_ACCOUNT_ID"], *args)
    except Exception:
        raise ReceiptDependencyError("s3-request") from None
    if result.returncode != 0:
        raise ReceiptDependencyError("s3-cli-failure")
    require(len(result.stdout) <= 32768, "s3-response-bound")
    value = decode(result.stdout)
    require(type(value) is dict, "s3-json-envelope")
    return value


def absent(env, prefix, *, request=None):
    # A positive successful LIST response proves absence. HEAD 403/404,
    # network failures, malformed/truncated results are never treated as proof.
    value = s3(env, "list-objects-v2", "--prefix", prefix, "--max-keys", "1", request=request)
    # AWS CLI 2.37.9/botocore auto-requests EncodingType=url for ListObjectsV2.
    # Its response decoder keeps this field while decoding Prefix/Key values.
    require(set(value) <= {"Name", "Prefix", "MaxKeys", "KeyCount", "IsTruncated", "Contents", "CommonPrefixes", "NextContinuationToken", "EncodingType"}, "list-fields")
    require("EncodingType" not in value or value["EncodingType"] == "url", "list-encoding")
    require(value.get("Name") == env["STATE_BUCKET"] and value.get("Prefix") == prefix, "list-owner-prefix")
    require(type(value.get("MaxKeys")) is int and value["MaxKeys"] == 1, "list-max")
    require(type(value.get("KeyCount")) is int and value["KeyCount"] == 0, "list-empty-count")
    require(value.get("IsTruncated") is False and value.get("Contents", []) == [], "list-empty-contents")
    require(value.get("CommonPrefixes", []) == [] and "NextContinuationToken" not in value, "list-no-pagination")


def head(env, maximum, *, request=None):
    value = s3(env, "head-object", "--key", STATE_KEY, request=request)
    require(type(value.get("ContentLength")) is int and 0 < value["ContentLength"] <= maximum, "head-size")
    require(type(value.get("VersionId")) is str and value["VersionId"] not in {"", "null"}, "head-version")
    require(type(value.get("ETag")) is str and bool(value["ETag"]), "head-etag")
    require(value.get("StorageClass", "STANDARD") == "STANDARD" and not value.get("DeleteMarker"), "head-storage-marker")
    require(not value.get("Restore") and not value.get("SSECustomerAlgorithm") and not value.get("ContentEncoding"), "head-restoration-encoding")
    require(value.get("ServerSideEncryption") in {"AES256", "aws:kms", "aws:kms:dsse"}, "head-encryption")
    return value


def state_shape(raw, *, imported=False):
    state = decode(raw)
    require(type(state) is dict and {"version", "terraform_version", "serial", "lineage", "outputs", "resources"} <= set(state)
            <= {"version", "terraform_version", "serial", "lineage", "outputs", "resources", "check_results"}, "state-envelope")
    require(type(state["version"]) is int and state["version"] == 4, "state-version")
    require(type(state["serial"]) is int and state["serial"] >= 0, "state-serial")
    require(type(state["terraform_version"]) is str and re.fullmatch(r"\d+\.\d+\.\d+", state["terraform_version"]), "state-tf-version")
    try:
        valid_lineage = type(state["lineage"]) is str and str(uuid.UUID(state["lineage"])) == state["lineage"]
    except (ValueError, TypeError, AttributeError):
        valid_lineage = False
    require(valid_lineage, "state-lineage")
    checks_pass(state.get("check_results"), imported=imported)
    require(type(state["outputs"]) is dict and set(state["outputs"]) == {"environment", "project_id"}, "state-output-names")
    for key, expected in (("environment", "development"), ("project_id", "test-youtube-study-space")):
        output = state["outputs"][key]
        # Terraform 1.16.4 serializes false with json:"sensitive,omitempty".
        require(type(output) is dict and {"type", "value"} <= set(output) <= {"type", "value", "sensitive"}
                and output.get("sensitive", False) is False and output["type"] == "string"
                and output["value"] == expected, "state-output-value")
    require(type(state["resources"]) is list, "state-resource-list")
    addresses, containers = [], set()
    for resource in state["resources"]:
        require(type(resource) is dict and {"mode", "type", "name", "provider", "instances"} <= set(resource)
                <= {"module", "mode", "type", "name", "provider", "instances"}, "state-resource-envelope")
        require(resource["mode"] == "managed" and resource["provider"] == 'provider["registry.terraform.io/hashicorp/google"]', "state-resource-owner")
        container = (resource.get("module", ""), resource["type"], resource["name"])
        require(container not in containers, "state-resource-duplicate")
        containers.add(container)
        require(type(resource["instances"]) is list and bool(resource["instances"]), "state-instance-list")
        for instance in resource["instances"]:
            require(type(instance) is dict and {"schema_version", "attributes"} <= set(instance)
                    <= {"schema_version", "attributes", "sensitive_attributes", "private", "dependencies", "index_key",
                        "identity_schema_version", "identity", "create_before_destroy"}, "state-instance-envelope")
            require(type(instance["schema_version"]) is int and instance["schema_version"] >= 0 and type(instance["attributes"]) is dict, "state-instance-version")
            require(type(instance.get("identity_schema_version", 0)) is int and instance.get("identity_schema_version", 0) >= 0, "state-instance-identity-version")
            require(instance.get("identity") is None or type(instance["identity"]) is dict, "state-instance-identity")
            require(instance.get("create_before_destroy", False) is False, "state-instance-lifecycle")
            require(type(instance.get("sensitive_attributes", [])) is list, "state-instance-sensitive-paths")
            require(type(instance.get("private", "")) is str, "state-instance-private")
            require(type(instance.get("dependencies", [])) is list and all(type(d) is str for d in instance.get("dependencies", [])), "state-instance-dependencies")
            require(instance["attributes"].get("project") == "test-youtube-study-space", "state-instance-project")
            index = instance.get("index_key")
            require("index_key" not in instance or type(index) in (int, str), "state-instance-index")
            suffix = "" if "index_key" not in instance else "[" + json.dumps(index, separators=(",", ":")) + "]"
            address = (resource["module"] + "." if "module" in resource else "") + resource["type"] + "." + resource["name"] + suffix
            addresses.append(address)
    expected = BASELINE | ({TABLE} if imported else set())
    require(len(addresses) == len(expected) and len(set(addresses)) == len(expected)
            and set(addresses) == expected, "state-address-set")
    return state


def checks_pass(checks, *, imported=False):
    # Terraform1.16.4 state legitimately persists variable checks and resource
    # identities. Validate the known envelope rather than requiring them absent.
    if checks is None:
        return
    require(type(checks) is list, "checks-list")
    source = Path(__file__).resolve().parents[2] / "infra/gcp"
    modules = {"backup_bucket": "retained-backup-bucket", "firestore_export_dataset": "retained-bigquery-dataset",
               "order_history": "retained-order-history", "notification_channels": "monitoring-notification-channels",
               "youtube_quota_alerts": "youtube-quota-alerts", "export_topic": "firestore-export-topic",
               "export_scheduler": "firestore-export-scheduler", "export_function": "firestore-export-function",
               "runtime_wif": "runtime-aws-wif", "owned_apis": "owned-project-apis",
               "user_activity_history": "retained-user-activity-history"}
    known = {"resource": {re.sub(r'\[(?:\d+|"[^"]*")\]', "", a) for a in BASELINE | ({TABLE} if imported else set())},
             "var": set(), "output": set(), "check": set()}
    for prefix, directory in [("", source / "environments/dev"), *[("module." + key + ".", source / "modules" / value) for key, value in modules.items()]]:
        files = tuple(directory.glob("*.tf"))
        require(directory.is_dir() and bool(files), "checks-catalog-source")
        for path in files:
            try:
                text = path.read_text()
            except (OSError, UnicodeError):
                raise ReceiptDependencyError("checks-catalog-read") from None
            for keyword, name in re.findall(r'(?m)^\s*(variable|output|check)\s+"([a-zA-Z0-9_]+)"\s*\{', text):
                kind = "var" if keyword == "variable" else keyword
                address = prefix + kind + "." + name
                known[kind].add(address)
    require(all(address in known[kind] for kind, address in ZERO_INSTANCE_CHECKS), "checks-catalog-source")
    seen = set()
    for check in checks:
        require(type(check) is dict and set(check) == {"object_kind", "config_addr", "status", "objects"}, "checks-envelope")
        kind, address = check["object_kind"], check["config_addr"]
        require(type(kind) is str and kind in known and type(address) is str and address in known[kind], "checks-address")
        require((kind, address) not in seen and check["status"] == "pass", "checks-result")
        seen.add((kind, address))
        if check["objects"] is None or check["objects"] == []:
            # A known disabled count=0 module has no instance checks. Terraform
            # 1.16.4 writes its aggregate PASS with objects:null (or empty).
            require((kind, address) in ZERO_INSTANCE_CHECKS, "checks-object-list")
            continue
        require(type(check["objects"]) is list and bool(check["objects"]), "checks-object-list")
        objects = set()
        for item in check["objects"]:
            require(type(item) is dict and {"object_addr", "status"} <= set(item) <= {"object_addr", "status", "failure_messages"}, "checks-object-envelope")
            obj = item["object_addr"]
            require(type(obj) is str and obj not in objects and re.sub(r'\[(?:\d+|"[^"]*")\]', "", obj) == address, "checks-object-address")
            require(item["status"] == "pass" and item.get("failure_messages", []) == [], "checks-object-result")
            if kind == "resource":
                require(obj in BASELINE | ({TABLE} if imported else set()), "checks-resource-address")
            objects.add(obj)


def snapshot(env, phase, *, request=None):
    staged = phase == "before"
    directory, budget = (at_stage("history-policy", lambda: load_policy(env)) if staged
                         else load_policy(env, fresh=False))
    maximum = budget["profile"]["max_state_bytes"]
    before = (at_stage("history-head", lambda: head(env, maximum, request=request)) if staged
              else head(env, maximum, request=request))
    target = directory / f"history-plan-state-{phase}.json"
    def read_state():
        try:
            fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
            os.close(fd)
        except OSError:
            raise ReceiptDependencyError("get-file-open") from None
        result = s3(env, "get-object", "--key", STATE_KEY, "--range", f"bytes=0-{maximum}", str(target), request=request)
        raw = private_bytes(target)
        require(result.get("VersionId") == before["VersionId"] and result.get("ETag") == before["ETag"], "get-version-etag")
        require(result.get("ContentLength") == len(raw) == before["ContentLength"], "get-size")
        return raw, state_shape(raw)
    raw, state = (at_stage("history-get", read_state) if staged else read_state())
    if staged:
        at_stage("history-re-head", lambda: require(head(env, maximum, request=request) == before, "rehead-equal"))
    else:
        require(head(env, maximum, request=request) == before, "rehead-equal")
    return {"head": before, "sha256": hashlib.sha256(raw).hexdigest(),
            "serial": state["serial"], "lineage": state["lineage"]}


def snapshot_before(env, *, request=None):
    directory, _ = at_stage("history-policy", lambda: load_policy(env))
    captured = snapshot(env, "before", request=request)
    at_stage("history-lock-list", lambda: absent(env, STATE_KEY + ".tflock", request=request))
    at_stage("history-workspace-list", lambda: absent(env, STATE_KEY.rsplit("/", 1)[0] + "/workspaces/", request=request))
    write_private(directory / "history-plan-state-before-receipt.json", captured)


def before(env):
    directory, budget = load_policy(env)
    state = private_json(str(directory / "history-plan-state-before-receipt.json"))
    require(state["sha256"] == hashlib.sha256(private_bytes(directory / "history-plan-state-before.json")).hexdigest(), "before-state-hash")
    metadata = private_json(str(directory / "user-history-before.json"))
    prepare(metadata)
    json.dumps(metadata, allow_nan=False)
    write_private(directory / "history-plan-before.json", {"git_sha": env["GITHUB_SHA"], "state": state, "cost": budget,
                  "metadata_sha256": hashlib.sha256(json.dumps(metadata, sort_keys=True, separators=(",", ":")).encode()).hexdigest()})


def after(env, *, request=None, metadata_request=None):
    directory, budget = load_policy(env, fresh=False)
    initial = private_json(str(directory / "history-plan-before.json"))
    require(initial["git_sha"] == env["GITHUB_SHA"] and initial["cost"] == budget, "after-initial-evidence")
    # These reads execute even if Terraform init/plan/validation failed. Only
    # complete evidence plus a strict import1 summary can produce final PASS.
    checks = {"Persistent state invariant": False, "Stable table metadata invariant": False, "Exact native lock absence": False}
    volatile_changed = None
    try:
        final = snapshot(env, "after", request=request)
        checks["Persistent state invariant"] = (final == initial["state"] and
            private_bytes(directory / "history-plan-state-before.json") == private_bytes(directory / "history-plan-state-after.json"))
    except Exception:
        pass
    try:
        metadata_request = metadata_request or google
        status, metadata = metadata_request(TABLE_PATH, env["GCP_SMOKE_ACCESS_TOKEN"], host="bigquery.googleapis.com")
        require(status == 200, "after-metadata-status")
        prepare(metadata)
        json.dumps(metadata, allow_nan=False)
        write_private(directory / "history-plan-table-after.json", metadata)
        original = private_json(str(directory / "user-history-before.json"))
        checks["Stable table metadata invariant"] = (stable_table_metadata(metadata) == stable_table_metadata(original) and
            initial["metadata_sha256"] == hashlib.sha256(json.dumps(original, sort_keys=True, separators=(",", ":")).encode()).hexdigest())
        volatile_changed = volatile_table_metadata(metadata) != volatile_table_metadata(original)
    except Exception:
        pass
    try:
        absent(env, STATE_KEY + ".tflock", request=request)
        checks["Exact native lock absence"] = True
    except Exception:
        pass
    # Independent safety reads are each attempted once, even if another fails.
    # Only fixed labels and booleans may be reported. A partial PASS is not a
    # successful run receipt; all checks, cost and selected plan must pass.
    with open(env["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as handle:
        handle.write("History plan safety checks (final receipt still required):\n")
        for label, passed in checks.items():
            handle.write(f"- {label}: {'PASS' if passed else 'STOP'}\n")
        handle.write("- Volatile table observations: " + ("changed" if volatile_changed else "unchanged" if volatile_changed is False else "unknown") + "\n")
    require(all(checks.values()), "after-safety")
    require(cost_policy(json.dumps(budget["profile"]), env["GITHUB_SHA"]) == budget, "after-cost-fresh")
    require(all(env.get(key) == "success" for key in ("BACKEND_INIT_OUTCOME", "TERRAFORM_PLAN_OUTCOME", "PLAN_VALIDATION_OUTCOME")), "after-outcomes")
    require(env.get("PLAN_EXIT_CODE") in {"0", "2"}, "after-plan-exit")
    summary = private_json(str(directory / "sanitized-plan.json"))
    require(type(summary) is dict and set(summary) == {"schema_version", "git_sha", "environment", "terraform_version", "policy",
                                                     "policy_passed", "violations", "drift", "counts", "resources"}, "after-plan-envelope")
    require(type(summary["schema_version"]) is int and summary["schema_version"] == 1, "after-plan-version")
    require(summary.get("git_sha") == env["GITHUB_SHA"] and summary.get("environment") == "dev"
            and summary.get("terraform_version") == "1.16.4" and summary.get("policy") == "import-only"
            and summary.get("policy_passed") is True and summary.get("violations") == [] and summary.get("drift") == [], "after-plan-target")
    require(type(summary.get("counts")) is dict and all(type(v) is int for v in summary["counts"].values()), "after-plan-count-types")
    require(summary.get("counts") == {"import": 1, "create": 0, "update": 0, "delete": 0, "replace": 0,
                                     "read": 0, "no-op": 12, "drift": 0, "other": 0}, "after-plan-counts")
    resources = summary.get("resources")
    require(type(resources) is list and len(resources) == 12, "after-plan-resources-list")
    require(all(type(r) is dict and set(r) == {"address", "mode", "action", "import"} for r in resources), "after-plan-resources-envelope")
    require({r.get("address") for r in resources} == BASELINE | {TABLE}, "after-plan-addresses")
    require(all(r.get("mode") == "managed" and r.get("action") == "no-op" and r.get("import") is (r["address"] == TABLE) for r in resources), "after-plan-actions")
    write_private(directory / "history-plan-after.json", {"git_sha": env["GITHUB_SHA"], "state": final,
                  "stable_table_metadata_unchanged": True, "volatile_table_observations_changed": volatile_changed,
                  "native_lock_absent": True, "import": 1, "existing_no_op": 11,
                  "cost_upper_bound_usd": budget["upper_bound_usd"], "monthly_ledger_entry": budget["monthly_ledger_entry"]})


def main(argv=None):
    os.umask(0o077)
    try:
        parser = PrivateArgumentParser(description=__doc__)
        parser.add_argument("--phase", required=True, choices=("policy", "before", "after"))
        args = parser.parse_args(argv)
        env = dict(os.environ)
        {"policy": policy, "before": before, "after": after}[args.phase](env)
        if args.phase == "after":
            message = "Protected history plan receipt: PASS; import1; existing11 no-op; state and stable table metadata unchanged; volatile observations recorded privately; native lock absent; added current UTC month cost bound <= USD0.25. Retained lock storage must carry into later monthly cumulative costs; no deletion deadline assumed.\n"
            # The value-free projection digest is the only plan artifact handle
            # a future one-shot approval may cite. The binary plan is deleted.
            digest = hashlib.sha256(private_bytes(Path(env["RUNNER_TEMP"]) / "sanitized-plan.json")).hexdigest()
            with open(env["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as handle:
                handle.write(message + f"History sanitized summary SHA-256: `{digest}`\n")
        else:
            message = "Protected history plan private precheck passed. No Terraform execution or lock operation performed.\n"
    except (ReceiptInvariantError, ReceiptDependencyError) as error:
        print(f"STOP: protected history plan evidence rejected; reason={error.reason}; do not retry or force unlock.", file=sys.stderr)
        return 3
    except Exception:
        print("STOP: protected history plan evidence rejected; private diagnostic suppressed; do not retry or force unlock.", file=sys.stderr)
        return 3
    print(message.strip())
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
