#!/usr/bin/env python3
"""Fail-closed receipt for one protected history import; never performs an apply.

Only exact S3 state reads, native-lock LIST and BigQuery tables.get are used.
Private evidence stays in RUNNER_TEMP. Public diagnostics contain no state,
table metadata, identities, paths or plan values.
"""
from __future__ import annotations

import argparse
from datetime import datetime, timedelta, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys

import terraform_history_plan_receipt as receipt
from terraform_plan_summary import COUNT_KEYS
from terraform_identity_smoke import google

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "infra/gcp/scripts"))
from prepare_user_activity_history_adoption import normalize_field, prepare
from validate_user_activity_history_plan import TABLE, TABLE_ID

# plan_run_id and cost fields are reviewer references with shape checks only.
# Only SHA, expiry, approved summary bytes and the local binary digest are
# enforced against execution evidence in this source path.
APPROVAL_KEYS = {"target", "git_sha", "plan_run_id", "summary_sha256", "cost_evidence_sha256",
                 "max_added_current_month_usd", "issued_utc", "expires_utc"}
SUMMARY_COUNTS = {key: (1 if key == "import" else 12 if key == "no-op" else 0) for key in COUNT_KEYS}
RECEIPT = "history-import-before.json"
# BigQuery tables.get output-only observations that may change when an
# independent writer appends rows. Everything else, including unknown future
# fields, remains in the exact stable comparison. See REST Table resource.
VOLATILE_TABLE_FIELDS = frozenset({
    "etag", "lastModifiedTime", "streamingBuffer", "numRows", "numBytes",
    "numLongTermBytes", "numTimeTravelPhysicalBytes", "numTotalLogicalBytes",
    "numActiveLogicalBytes", "numLongTermLogicalBytes", "numTotalPhysicalBytes",
    "numActivePhysicalBytes", "numLongTermPhysicalBytes", "numPartitions",
})
VOLATILE_METRICS = VOLATILE_TABLE_FIELDS - {"etag", "streamingBuffer"}


def need(value):
    if not value:
        raise ValueError("history import STOP")


def approval(env, *, now=None):
    need(env.get("MODE") == "apply" and env.get("HISTORY_TARGET") == "dev"
         and env.get("TF_VAR_project_id") == "test-youtube-study-space"
         and env.get("TF_VAR_manage_user_activity_history") == "true"
         and env.get("TF_WORKSPACE") == "default"
         and env.get("STATE_KEY") == receipt.STATE_KEY
         and env.get("STATE_AWS_REGION") == "ap-northeast-1"
         and re.fullmatch(r"\d{12}", env.get("STATE_ACCOUNT_ID", ""))
         and re.fullmatch(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]", env.get("STATE_BUCKET", ""))
         and env.get("AWS_MAX_ATTEMPTS") == "1" and env.get("AWS_RETRY_MODE") == "standard")
    sha = env.get("GITHUB_SHA", "")
    need(re.fullmatch(r"[0-9a-f]{40}", sha))
    raw = env.get("HISTORY_IMPORT_APPROVAL", "")
    need(type(raw) is str and 0 < len(raw) <= 2048)
    value = receipt.decode(raw)
    need(type(value) is dict and set(value) == APPROVAL_KEYS)
    need(value["target"] == "dev" and value["git_sha"] == sha
         and type(value["plan_run_id"]) is int and value["plan_run_id"] > 0
         and type(value["summary_sha256"]) is str
         and re.fullmatch(r"[0-9a-f]{64}", value["summary_sha256"])
         and type(value["cost_evidence_sha256"]) is str
         and re.fullmatch(r"[0-9a-f]{64}", value["cost_evidence_sha256"])
         and type(value["max_added_current_month_usd"]) is str
         and re.fullmatch(r"0\.\d{1,9}", value["max_added_current_month_usd"])
         and float(value["max_added_current_month_usd"]) > 0)
    issued, expires = receipt.timestamp(value["issued_utc"]), receipt.timestamp(value["expires_utc"])
    now = now or datetime.now(timezone.utc)
    need(issued <= now < expires <= issued + timedelta(hours=24))
    return value


def directory(env):
    path = Path(env["RUNNER_TEMP"])
    need(path.is_absolute() and path.is_dir() and not path.is_symlink())
    return path


def stable_table_metadata(metadata):
    """Reject any configuration/identity change while preserving write metrics privately."""
    prepare(metadata)
    need(type(metadata) is dict)
    for key in VOLATILE_METRICS & metadata.keys():
        need(type(metadata[key]) is str and re.fullmatch(r"[0-9]+", metadata[key]))
    if "etag" in metadata:
        need(type(metadata["etag"]) is str and bool(metadata["etag"]))
    if "streamingBuffer" in metadata:
        buffer = metadata["streamingBuffer"]
        need(type(buffer) is dict and set(buffer) <= {"estimatedRows", "estimatedBytes", "oldestEntryTime"}
             and all(type(value) is str and re.fullmatch(r"[0-9]+", value) for value in buffer.values()))
    return {key: value for key, value in metadata.items() if key not in VOLATILE_TABLE_FIELDS}


def volatile_table_metadata(metadata):
    return {key: value for key, value in metadata.items() if key in VOLATILE_TABLE_FIELDS}


def capture(env, path, *, request=None):
    before = receipt.head(env, receipt.MAX_BYTES, request=request)
    raw_target = directory(env) / path
    # The S3 helper creates this file and verifies version/ETag/length through
    # the response; it never accepts a short, stale or replaced body.
    fd = os.open(raw_target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    os.close(fd)
    result = receipt.s3(env, "get-object", "--key", receipt.STATE_KEY,
                        "--range", f"bytes=0-{receipt.MAX_BYTES}", str(raw_target), request=request)
    raw = receipt.private_bytes(raw_target)
    need(result.get("VersionId") == before["VersionId"] and result.get("ETag") == before["ETag"]
         and result.get("ContentLength") == before["ContentLength"] == len(raw))
    need(receipt.head(env, receipt.MAX_BYTES, request=request) == before)
    receipt.absent(env, receipt.STATE_KEY + ".tflock", request=request)
    return raw


def state_delta(before, after, metadata):
    old = receipt.state_shape(before)
    new = receipt.state_shape(after, imported=True)
    need(new["lineage"] == old["lineage"] and new["serial"] > old["serial"]
         and new["version"] == old["version"] and new["terraform_version"] == old["terraform_version"]
         and new["outputs"] == old["outputs"])
    def address(resource):
        return (resource.get("module", "") + "." if "module" in resource else "") + resource["type"] + "." + resource["name"]
    old_resources = {address(r): r for r in old["resources"]}
    new_resources = {address(r): r for r in new["resources"]}
    need(all(new_resources[key] == value for key, value in old_resources.items()))
    container = new_resources.get("module.user_activity_history[0].google_bigquery_table.retained")
    need(type(container) is dict and len(container["instances"]) == 1)
    instance = container["instances"][0]
    need(instance.get("index_key", None) is None and not instance.get("create_before_destroy", False))
    attrs = instance["attributes"]
    for key, expected in {"id": TABLE_ID, "project": "test-youtube-study-space", "dataset_id": "firestore_export",
                          "table_id": "user-activity-history", "location": "asia-southeast2", "deletion_protection": True}.items():
        need(attrs.get(key) == expected)
    fields = receipt.decode(attrs.get("schema"))
    need(type(fields) is list and [normalize_field(f) for f in fields] ==
         [normalize_field(f) for f in metadata["schema"]["fields"]])
    # Existing check results must remain identical. Only newly activated
    # history module checks can differ, and state_shape requires each PASS.
    def old_checks(state):
        return [c for c in state.get("check_results", [])
                if not c["config_addr"].startswith("module.user_activity_history.")]
    need(old_checks(new) == old_checks(old))


def exact_summary(summary, sha):
    need(type(summary) is dict and summary.get("environment") == "dev" and summary.get("git_sha") == sha
         and summary.get("terraform_version") == "1.16.4" and summary.get("policy") == "import-only"
         and summary.get("policy_passed") is True and summary.get("violations") == []
         and summary.get("drift") == [] and summary.get("counts") == SUMMARY_COUNTS)
    resources = summary.get("resources")
    need(type(resources) is list and len(resources) == 12
         and {r.get("address") for r in resources} == receipt.BASELINE | {TABLE}
         and all(type(r) is dict and r.get("mode") == "managed" and r.get("action") == "no-op"
                 and r.get("import") is (r.get("address") == TABLE) for r in resources))


def before(env, *, request=None):
    approved = approval(env)
    need(env.get("APPROVED_SUMMARY_DIGEST") == approved["summary_sha256"])
    root = directory(env)
    metadata = receipt.private_json(str(root / "user-history-before.json"))
    stable_table_metadata(metadata)
    raw = capture(env, "history-import-state-before.json", request=request)
    receipt.state_shape(raw)
    receipt.absent(env, receipt.STATE_KEY.rsplit("/", 1)[0] + "/workspaces/", request=request)
    receipt.write_private(root / RECEIPT, {"approval": approved, "state_sha256": hashlib.sha256(raw).hexdigest(),
                                                   "metadata": metadata})


def preapply(env, *, request=None):
    approved = approval(env)
    root = directory(env)
    saved = receipt.private_json(str(root / RECEIPT))
    need(saved["approval"] == approved)
    original = receipt.private_bytes(root / "history-import-state-before.json")
    need(hashlib.sha256(original).hexdigest() == saved["state_sha256"])
    current = capture(env, "history-import-state-preapply.json", request=request)
    need(current == original)
    need(receipt.private_json(str(root / "user-history-before.json")) == saved["metadata"])
    summary_raw = receipt.private_bytes(root / "sanitized-replan.json")
    need(hashlib.sha256(summary_raw).hexdigest() == approved["summary_sha256"])
    exact_summary(receipt.decode(summary_raw), env["GITHUB_SHA"])
    binary = receipt.private_bytes(root / "tfplan")
    digest = hashlib.sha256(binary).hexdigest()
    receipt.write_private(root / "history-import-preapply.json", {"approval": approved, "tfplan_sha256": digest})
    fd = os.open(env["GITHUB_OUTPUT"], os.O_WRONLY | os.O_APPEND | os.O_NOFOLLOW)
    with os.fdopen(fd, "w", encoding="utf-8") as output:
        info = os.fstat(output.fileno())
        need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid())
        output.write(f"history_plan_sha256={digest}\n")


def post(env, *, request=None, metadata_request=None):
    root = directory(env)
    saved = receipt.private_json(str(root / RECEIPT))
    approved = approval(env)
    need(saved["approval"] == approved)
    original = receipt.private_bytes(root / "history-import-state-before.json")
    need(hashlib.sha256(original).hexdigest() == saved["state_sha256"])
    checks = {"state": "unknown", "metadata": False, "lock": False}
    volatile_changed = None
    try:
        raw = capture(env, "history-import-state-after.json", request=request)
        if raw == original:
            checks["state"] = "unchanged"
        else:
            state_delta(original, raw, saved["metadata"])
            checks["state"] = "imported"
        checks["lock"] = True
    except Exception:
        try:
            receipt.absent(env, receipt.STATE_KEY + ".tflock", request=request)
            checks["lock"] = True
        except Exception:
            pass
    try:
        status, metadata = (metadata_request or google)(receipt.TABLE_PATH, env["GCP_SMOKE_ACCESS_TOKEN"], host="bigquery.googleapis.com")
        need(status == 200)
        checks["metadata"] = stable_table_metadata(metadata) == stable_table_metadata(saved["metadata"])
        volatile_changed = volatile_table_metadata(metadata) != volatile_table_metadata(saved["metadata"])
        receipt.write_private(root / "history-import-table-after.json", metadata)
    except Exception:
        pass
    with open(env["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as output:
        output.write("History import receipt: state=" + checks["state"] +
                     "; metadata=" + ("PASS" if checks["metadata"] else "STOP") +
                     "; volatile=" + ("unknown" if volatile_changed is None else
                                      "changed" if volatile_changed else "unchanged") +
                     "; lock=" + ("absent" if checks["lock"] else "unknown") + "\n")
    need(checks == {"state": "imported", "metadata": True, "lock": True}
         and env.get("APPLY_OUTCOME") == "success" and env.get("POST_PLAN_OUTCOME") == "success")


def main():
    try:
        parser = argparse.ArgumentParser()
        parser.add_argument("--phase", required=True, choices=("authorization", "before", "preapply", "post"))
        args = parser.parse_args()
        env = dict(os.environ)
        if args.phase == "authorization":
            approval(env)
        else:
            {"before": before, "preapply": preapply, "post": post}[args.phase](env)
    except Exception:
        print("STOP: protected history import receipt rejected; inspect private evidence; never blindly retry or force unlock.", file=sys.stderr)
        return 3
    print("Protected history import " + args.phase + " passed; no private values emitted.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
