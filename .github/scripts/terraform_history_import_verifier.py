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
from terraform_history_table_metadata import stable_table_metadata, volatile_table_metadata
import terraform_ownership_receipt as ownership

# plan_run_id is a reviewer reference with shape checks only.
# Only SHA, expiry, approved summary bytes and the local binary digest are
# enforced against execution evidence in this source path.
APPROVAL_KEYS = {"schema_version", "target", "git_sha", "plan_run_id", "summary_sha256",
                 "issued_utc", "expires_utc"}
SUMMARY_COUNTS = {key: (1 if key == "import" else 12 if key == "no-op" else 0) for key in COUNT_KEYS}
RECEIPT = "history-import-before.json"


def need(value):
    if not value:
        raise ValueError("history import STOP")


def approval(env, *, now=None):
    ownership.context(env)
    need(env.get("DEV_HISTORY_RECEIPT_EMITTER_ENABLED") == "true"
         and env.get("GITHUB_JOB") in {"plan", "apply"})
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
    need(type(value) is dict and set(value) == APPROVAL_KEYS
         and type(value["schema_version"]) is int and value["schema_version"] == 2)
    need(value["target"] == "dev" and value["git_sha"] == sha
         and type(value["plan_run_id"]) is int and value["plan_run_id"] > 0
         and type(value["summary_sha256"]) is str
         and re.fullmatch(r"[0-9a-f]{64}", value["summary_sha256"]))
    issued, expires = receipt.timestamp(value["issued_utc"]), receipt.timestamp(value["expires_utc"])
    now = now or datetime.now(timezone.utc)
    need(issued <= now < expires <= issued + timedelta(hours=24))
    need(not env.get("PLAN_COST_EVIDENCE", ""))
    receipt.execution_policy(env.get("PLAN_SAFETY_EVIDENCE", ""), sha, now=now)
    return value


def directory(env):
    path = Path(env["RUNNER_TEMP"])
    need(path.is_absolute() and path.is_dir() and not path.is_symlink())
    return path


def size_bound(env):
    """No cloud call or digest output; reject an oversized saved plan before approval."""
    need(env.get("MODE") in {"plan", "apply"} and env.get("HISTORY_TARGET") == "dev"
         and env.get("TF_VAR_manage_user_activity_history") == "true"
         and re.fullmatch(r"[0-9a-f]{40}", env.get("GITHUB_SHA", "")))
    receipt.private_bytes(directory(env) / "tfplan")


def capture(env, path, *, request=None):
    maximum = receipt.execution_policy(env.get("PLAN_SAFETY_EVIDENCE", ""), env["GITHUB_SHA"])["profile"]["max_state_bytes"]
    before = receipt.head(env, maximum, request=request)
    raw_target = directory(env) / path
    # The S3 helper creates this file and verifies version/ETag/length through
    # the response; it never accepts a short, stale or replaced body.
    fd = os.open(raw_target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    os.close(fd)
    result = receipt.s3(env, "get-object", "--key", receipt.STATE_KEY,
                        "--range", f"bytes=0-{maximum}", str(raw_target), request=request)
    raw = receipt.private_bytes(raw_target)
    need(result.get("VersionId") == before["VersionId"] and result.get("ETag") == before["ETag"]
         and result.get("ContentLength") == before["ContentLength"] == len(raw))
    need(receipt.head(env, maximum, request=request) == before)
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


def exact_summary(summary, sha, *, phase="before"):
    need(phase in {"before", "post"})
    counts = dict(SUMMARY_COUNTS, **({"import": 0} if phase == "post" else {}))
    need(type(summary) is dict and summary.get("environment") == "dev" and summary.get("git_sha") == sha
         and summary.get("terraform_version") == "1.16.4" and summary.get("policy") == "import-only"
         and summary.get("policy_passed") is True and summary.get("violations") == []
         and summary.get("drift") == [] and summary.get("counts") == counts)
    resources = summary.get("resources")
    need(type(resources) is list and len(resources) == 12
         and {r.get("address") for r in resources} == receipt.BASELINE | {TABLE}
         and all(type(r) is dict and r.get("mode") == "managed" and r.get("action") == "no-op"
                 and r.get("import") is (phase == "before" and r.get("address") == TABLE) for r in resources))


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
    reads = {"state": 0, "metadata": 0}
    def counted_state(*args):
        reads["state"] += 1
        return (request or receipt.aws)(*args)
    def counted_metadata(*args, **kwargs):
        reads["metadata"] += 1
        return (metadata_request or google)(*args, **kwargs)
    try:
        raw = capture(env, "history-import-state-after.json", request=counted_state)
        if raw == original:
            checks["state"] = "unchanged"
        else:
            state_delta(original, raw, saved["metadata"])
            checks["state"] = "imported"
        checks["lock"] = True
    except Exception:
        try:
            receipt.absent(env, receipt.STATE_KEY + ".tflock", request=counted_state)
            checks["lock"] = True
        except Exception:
            pass
    try:
        status, metadata = counted_metadata(receipt.TABLE_PATH, env["GCP_SMOKE_ACCESS_TOKEN"], host="bigquery.googleapis.com")
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
    ownership.context(env)
    need(env.get("GITHUB_JOB") == "apply")
    exact_summary(receipt.private_json(str(root / "post-apply-summary.json")), env["GITHUB_SHA"], phase="post")
    value = ownership.history_binding(env, metadata=metadata)
    machine = dict(schema_version=1, repository_id=ownership.REPOSITORY_ID, source_sha=env["GITHUB_SHA"],
                   run_id=int(env["GITHUB_RUN_ID"]), run_attempt=1, job_role="apply", environment="terraform-dev-apply",
                   target="dev", root_contract="gcp-dev-default", operation="apply", wave="history12", phase="post",
                   prior_receipt_sha256=None, prior_scope_commitment=None, prior_state_commitment=None,
                   state_after_commitment=ownership.state_commitment(value, receipt.decode(raw)),
                   scope_commitment=ownership.commitment(value, ownership.history_scope(value)),
                   resource_count=12, import_before_count=1, import_post_count=0,
                   checks={key: True for key in ownership.CHECKS},
                   accounting={"terraform_outcomes": {"apply": env["APPLY_OUTCOME"], "post_plan": env["POST_PLAN_OUTCOME"]},
                               "post_state_reads": reads["state"], "post_metadata_reads": reads["metadata"],
                               "provider_requests": "unknown", "actual_cost": "unknown"})
    receipt.write_private(root / ownership.RECORD, ownership.validate_receipt(machine))


def main():
    try:
        parser = argparse.ArgumentParser()
        parser.add_argument("--phase", required=True, choices=("authorization", "size", "before", "preapply", "post"))
        args = parser.parse_args()
        env = dict(os.environ)
        if args.phase == "authorization":
            approval(env)
        elif args.phase == "size":
            size_bound(env)
        else:
            {"before": before, "preapply": preapply, "post": post}[args.phase](env)
    except Exception:
        print("STOP: protected history import receipt rejected; inspect private evidence; never blindly retry or force unlock.", file=sys.stderr)
        return 3
    print("Protected history import " + args.phase + " passed; no private values emitted.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
